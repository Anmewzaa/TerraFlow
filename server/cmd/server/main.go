package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Anmewzaa/TerraFlow/server/internal/api"
	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/catalog"
	"github.com/Anmewzaa/TerraFlow/server/internal/envfile"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	ctx := context.Background()

	// Development convenience: settings from .env.local, then .env. Real environment
	// variables always win, so a deployment is unaffected. Only names are logged.
	for _, file := range []string{".env.local", ".env"} {
		names, err := envfile.Load(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if len(names) > 0 {
			logger.Info("loaded settings from file", "file", file, "names", names)
		}
	}

	addr := envOr("TF_ADDR", ":8080")
	authDisabled := envOr("TF_AUTH_DISABLED", "false") == "true"
	if err := auth.ValidateMode(authDisabled, os.Getenv); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	insecureCookies := envOr("TF_INSECURE_COOKIES", "false") == "true"
	noDB := envOr("TF_NO_DB", "false") == "true"

	// Migrations and the catalog seed run at every start unless TF_MIGRATE_ON_START=false
	// (then `terraflow migrate` does them as a separate step).
	migrateOnStart := envOr("TF_MIGRATE_ON_START", "true") != "false"
	migrateOnly := len(os.Args) > 1 && os.Args[1] == "migrate"
	if migrateOnly && noDB {
		fmt.Fprintln(os.Stderr, "error: terraflow migrate needs the database (TF_NO_DB is set)")
		os.Exit(1)
	}

	var q *store.Queries

	if !noDB {
		// Database
		dbCfg := store.Config{
			Host:     envOr("TF_PG_HOST", "localhost"),
			Port:     envOr("TF_PG_PORT", "5432"),
			Database: envOr("TF_PG_DATABASE", "terraflow"),
			User:     envOr("TF_PG_USER", "terraflow"),
			Password: envOr("TF_PG_PASSWORD", "localdev"),
		}

		pool, err := store.NewPool(ctx, dbCfg)
		if err != nil {
			logger.Error("db connect", "err", err)
			os.Exit(1)
		}
		defer pool.Close()
		logger.Info("database connected")

		if migrateOnStart || migrateOnly {
			if err := store.Migrate(dbCfg.DSN()); err != nil {
				logger.Error("migrate", "err", err)
				os.Exit(1)
			}
			logger.Info("migrations applied")
		}

		q = store.New(pool)

		if migrateOnly {
			// `terraflow migrate`: migrations and the catalog seed, then exit, so a
			// deployment can run them once instead of at every start.
			catalogDir := envOr("TF_CATALOG_DIR", "./catalog/modules")
			if err := catalog.SeedFromDir(ctx, catalog.NewPGStore(q), logger, catalogDir); err != nil {
				logger.Error("seed catalog", "err", err)
				os.Exit(1)
			}
			logger.Info("catalog seeded", "dir", catalogDir)
			return
		}

		// Seed
		if envOr("TF_SEED", "false") == "true" {
			sampleDir := envOr("TF_SAMPLE_DIR", "./samples/iac/AIC")
			if err := store.Seed(ctx, q, logger, sampleDir); err != nil {
				logger.Error("seed", "err", err)
				os.Exit(1)
			}
			if authDisabled {
				// With auth off there is no sign-in, so assign the dev user to the
				// seeded project; otherwise a non-admin dev role would see nothing.
				if err := q.UpsertUser(ctx, store.User{OID: auth.DevUserOID, Email: "dev@terraflow.local", Name: "Dev User"}); err != nil {
					logger.Error("seed dev user", "err", err)
					os.Exit(1)
				}
				if err := q.AddMember(ctx, store.SeedProjectID, auth.DevUserOID, "seed"); err != nil {
					logger.Error("seed dev membership", "err", err)
					os.Exit(1)
				}
			}
		}
	} else {
		logger.Warn("running without database (TF_NO_DB=true)")
	}

	// Auth
	var authHandlers *auth.Handlers
	var authMiddleware func(http.Handler) http.Handler
	var devLogin *auth.DevLogin
	var githubLogin *auth.GitHubLogin
	var loginProviders []api.ProviderInfo
	ghAPI := githubFromEnv(logger, insecureCookies)

	if authDisabled {
		logger.Warn("auth disabled — using dev bypass")
		role := envOr("TF_DEV_USER_ROLE", "developer")
		authMiddleware = auth.DevBypass(role)
		// Default: sign in by picking a made-up user. TF_DEV_LOGIN=false keeps the
		// single fixed dev user (scripts and tests that send no cookie).
		if envOr("TF_DEV_LOGIN", "true") != "false" {
			devLogin = &auth.DevLogin{}
			authMiddleware = devLogin.Middleware
		}
		authHandlers = auth.NewHandlers(nil, q, auth.RoleConfig{}, logger, insecureCookies)
	} else {
		// Sign-in providers: TF_LOGIN_PROVIDERS is a list of "entra" and "github".
		providers := splitList(envOr("TF_LOGIN_PROVIDERS", "entra"))
		signup := auth.SignupPolicy{
			Mode:       envOr("TF_SIGNUP_MODE", auth.SignupApproval),
			Admins:     splitList(os.Getenv("TF_ADMIN_USERS")),
			MaxPending: envInt("TF_SIGNUP_MAX_PENDING", 200),
		}
		switch signup.Mode {
		case auth.SignupApproval, auth.SignupOpen, auth.SignupInviteOnly:
		default:
			logger.Error("TF_SIGNUP_MODE must be approval, open or invite_only", "value", signup.Mode)
			os.Exit(1)
		}
		var oidcProvider *auth.OIDCProvider
		roles := auth.RoleConfig{
			DevOpsGroupID:    os.Getenv("TF_ENTRA_DEVOPS_GROUP_ID"),
			DeveloperGroupID: os.Getenv("TF_ENTRA_DEV_GROUP_ID"),
		}
		for _, p := range providers {
			switch p {
			case "entra":
				oidcCfg := auth.OIDCConfig{
					TenantID:     os.Getenv("TF_ENTRA_TENANT_ID"),
					ClientID:     os.Getenv("TF_ENTRA_CLIENT_ID"),
					ClientSecret: os.Getenv("TF_ENTRA_CLIENT_SECRET"),
					RedirectURI:  envOr("TF_ENTRA_REDIRECT_URI", "http://localhost:8080/auth/callback"),
				}
				var err error
				oidcProvider, err = auth.NewOIDCProvider(ctx, oidcCfg)
				if err != nil {
					logger.Error("oidc init", "err", err)
					os.Exit(1)
				}
				loginProviders = append(loginProviders, api.ProviderInfo{ID: "entra", Label: "Microsoft Entra ID", URL: "/auth/login"})
			case "github":
				if ghAPI.App == nil || ghAPI.App.ClientID == "" || ghAPI.App.ClientSecret == "" {
					logger.Error("sign-in with GitHub needs the GitHub App (TF_GITHUB_APP_ID, a private key, TF_GITHUB_APP_CLIENT_ID and TF_GITHUB_APP_CLIENT_SECRET)")
					os.Exit(1)
				}
				loginProviders = append(loginProviders, api.ProviderInfo{ID: "github", Label: "GitHub", URL: "/auth/github/login"})
			default:
				logger.Error("unknown sign-in provider in TF_LOGIN_PROVIDERS", "value", p)
				os.Exit(1)
			}
		}
		if len(providers) == 0 {
			logger.Error("TF_LOGIN_PROVIDERS is empty")
			os.Exit(1)
		}
		authMiddleware = auth.RequireAuth(q)
		authHandlers = auth.NewHandlers(oidcProvider, q, roles, logger, insecureCookies).WithSignup(signup)
		for _, lp := range loginProviders {
			if lp.ID == "github" {
				githubLogin = &auth.GitHubLogin{
					App:         ghAPI.App,
					RedirectURI: envOr("TF_GITHUB_LOGIN_REDIRECT_URI", "http://localhost:8080/auth/github/callback"),
					Handlers:    authHandlers,
				}
			}
		}
		logger.Info("sign-in", "providers", providers, "signup_mode", signup.Mode, "admins", len(signup.Admins))
	}

	sampleDir := envOr("TF_SAMPLE_DIR", "./samples/iac/AIC")
	catalogDir := envOr("TF_CATALOG_DIR", "./catalog/modules")

	var catalogSvc *catalog.Service
	if noDB {
		catalogSvc = catalog.NewService(catalog.NewFSStore(catalogDir))
	} else {
		pgCatalog := catalog.NewPGStore(q)
		catalogSvc = catalog.NewService(pgCatalog)
		if migrateOnStart {
			if err := catalog.SeedFromDir(ctx, pgCatalog, logger, catalogDir); err != nil {
				logger.Error("seed catalog", "err", err)
				os.Exit(1)
			}
			logger.Info("catalog seeded", "dir", catalogDir)
		}
	}

	if catalogSvc != nil {
		api.ConfigureModuleSources(os.Getenv("TF_MODULE_SOURCE_BASE"), catalogSvc.ListAllModules)
	}
	router := api.NewRouter(api.RouterConfig{
		GitHub:             ghAPI,
		GitHubLogin:        githubLogin,
		LoginProviders:     loginProviders,
		Logger:             logger,
		Queries:            q,
		AuthHandlers:       authHandlers,
		AuthMiddleware:     authMiddleware,
		CatalogService:     catalogSvc,
		NoDB:               noDB,
		SampleDir:          sampleDir,
		DevLogin:           devLogin,
		WorkspaceDir:       envOr("TF_WORKSPACE_DIR", "./workspace"),
		SeparationOfDuties: envOr("TF_SEPARATION_OF_DUTIES", "false") == "true",
		AllowedOrigins:     splitList(os.Getenv("TF_ALLOWED_ORIGINS")),
		HSTS:               !insecureCookies,
		AuthRateBurst:      20,
		AuthRateEvery:      6 * time.Second,
	})

	logger.Info("terraflow server", "addr", addr, "auth_disabled", authDisabled, "no_db", noDB)
	fmt.Fprintf(os.Stderr, "listening on %s\n", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// githubFromEnv builds the GitHub App from TF_GITHUB_APP_*. Without an app id and a
// private key GitHub stays off and the endpoints say so; a key that cannot be read
// is logged (never its contents) and also leaves GitHub off.
func githubFromEnv(logger *slog.Logger, insecureCookies bool) *api.GitHubAPI {
	id := os.Getenv("TF_GITHUB_APP_ID")
	b64, file := os.Getenv("TF_GITHUB_APP_PRIVATE_KEY_B64"), os.Getenv("TF_GITHUB_APP_PRIVATE_KEY_FILE")
	g := &api.GitHubAPI{
		AppSlug:       os.Getenv("TF_GITHUB_APP_SLUG"),
		AllowedOwners: splitList(os.Getenv("TF_GITHUB_ALLOWED_OWNERS")),
		RedirectURI:   os.Getenv("TF_GITHUB_REDIRECT_URI"),
		WebURL:        os.Getenv("TF_WEB_URL"),
		SecureCookie:  !insecureCookies,
	}
	if id == "" || (b64 == "" && file == "") {
		logger.Info("GitHub is not configured (TF_GITHUB_APP_ID and a private key are needed)")
		return g
	}
	key, err := github.LoadPrivateKey(b64, file, os.ReadFile)
	if err != nil {
		logger.Error("GitHub App private key", "err", err)
		return g
	}
	g.App = &github.App{
		ID: id, Key: key,
		ClientID: os.Getenv("TF_GITHUB_APP_CLIENT_ID"), ClientSecret: os.Getenv("TF_GITHUB_APP_CLIENT_SECRET"),
	}
	logger.Info("GitHub App configured", "app_id", id, "allowed_owners", len(g.AllowedOwners))
	return g
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil && n >= 0 {
		return n
	}
	return fallback
}

// splitList turns "a, b ,c" into [a b c], dropping blanks.
func splitList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
