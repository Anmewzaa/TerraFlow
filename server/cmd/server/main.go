package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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

	ghAPI := githubFromEnv(logger, insecureCookies)

	// Auth
	var providers []auth.ProviderInfo
	var githubLogin *auth.GitHubLogin
	noEntraLogin := true
	var authHandlers *auth.Handlers
	var authMiddleware func(http.Handler) http.Handler
	var devLogin *auth.DevLogin

	if authDisabled {
		logger.Warn("auth disabled — using dev bypass")
		role := envOr("TF_DEV_USER_ROLE", "developer")
		authMiddleware = auth.DevBypass(role)
		// TF_DEV_LOGIN=true: sign in by picking a made-up user. Off by default; without
		// it there is one fixed dev user (scripts and tests that send no cookie).
		if envOr("TF_DEV_LOGIN", "false") == "true" {
			devLogin = &auth.DevLogin{}
			authMiddleware = devLogin.Middleware
		}
		authHandlers = auth.NewHandlers(nil, q, auth.RoleConfig{}, logger, insecureCookies)
	} else {
		policy, err := auth.NewPolicy(os.Getenv("TF_SIGNUP_MODE"), os.Getenv("TF_ADMIN_USERS"), envInt("TF_MAX_PENDING", 200))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		login := &auth.Login{Queries: q, Policy: policy, Logger: logger, Insecure: insecureCookies, WebURL: os.Getenv("TF_WEB_URL")}

		entraSet := os.Getenv("TF_ENTRA_TENANT_ID") != ""
		githubSet := ghAPI != nil && ghAPI.App != nil && ghAPI.App.ClientID != "" && ghAPI.App.ClientSecret != ""
		enabled, err := loginProviders(os.Getenv("TF_LOGIN_PROVIDERS"), entraSet, githubSet)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		var oidcProvider *auth.OIDCProvider
		roles := auth.RoleConfig{}
		for _, p := range enabled {
			switch p {
			case "entra":
				oidcCfg := auth.OIDCConfig{
					TenantID:     os.Getenv("TF_ENTRA_TENANT_ID"),
					ClientID:     os.Getenv("TF_ENTRA_CLIENT_ID"),
					ClientSecret: os.Getenv("TF_ENTRA_CLIENT_SECRET"),
					RedirectURI:  envOr("TF_ENTRA_REDIRECT_URI", "http://localhost:8080/auth/callback"),
				}
				oidcProvider, err = auth.NewOIDCProvider(ctx, oidcCfg)
				if err != nil {
					logger.Error("oidc init", "err", err)
					os.Exit(1)
				}
				roles = auth.RoleConfig{
					DevOpsGroupID:    os.Getenv("TF_ENTRA_DEVOPS_GROUP_ID"),
					DeveloperGroupID: os.Getenv("TF_ENTRA_DEV_GROUP_ID"),
				}
				noEntraLogin = false
				providers = append(providers, auth.ProviderInfo{ID: "entra", Label: "Microsoft", Href: "/auth/login"})
			case "github":
				githubLogin = &auth.GitHubLogin{
					App: ghAPI.App, Login: login,
					RedirectURI: envOr("TF_GITHUB_LOGIN_REDIRECT_URI", githubLoginRedirect(os.Getenv("TF_GITHUB_REDIRECT_URI"))),
				}
				providers = append(providers, auth.ProviderInfo{ID: "github", Label: "GitHub", Href: "/auth/github/login"})
				logger.Info("sign in with GitHub is on; the App needs this callback URL", "url", githubLogin.RedirectURI)
			}
		}
		// TF_AUTO_APPROVE (D-062). A rule for a provider that is not switched on could
		// never match, so it stops the start instead of being silently useless.
		rules, err := auth.ParseAutoApprove(os.Getenv("TF_AUTO_APPROVE"))
		if err == nil {
			err = autoApproveFits(rules, enabled)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		login.Policy.AutoApprove = rules
		if githubLogin != nil {
			// A person who signs in with GitHub has just proven who they are: check that
			// they still reach the repositories of the projects they own (D-061).
			githubLogin.Verified = func(ctx context.Context, oid string, tok github.Token) {
				ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
				defer cancel()
				if err := ghAPI.RecheckOwnerAccess(ctx, oid, tok); err != nil {
					logger.Warn("github sign-in: owner access not re-checked", "user", oid, "err", err.Error())
				}
			}
		}
		logger.Info("sign-in", "providers", enabled, "signup_mode", policy.Mode, "admins_named", len(policy.Admins),
			"auto_approve_entra_tenant", rules.EntraTenant, "auto_approve_github_orgs", len(rules.GitHubOrgs), "auto_approve_email_domains", len(rules.EmailDomains))
		authMiddleware = auth.RequireAuth(q)
		authHandlers = auth.NewHandlers(oidcProvider, q, roles, logger, insecureCookies)
		authHandlers.SetLogin(login)
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

	// How projects are created: from a GitHub repository or from a local folder. The
	// default follows what is set up; a wrong value stops the start instead of guessing.
	projectSource := strings.ToLower(strings.TrimSpace(os.Getenv("TF_PROJECT_SOURCE")))
	switch projectSource {
	case "":
		projectSource = "local"
		if ghAPI != nil && ghAPI.App != nil {
			projectSource = "github"
		}
	case "github", "local":
	default:
		fmt.Fprintf(os.Stderr, "error: TF_PROJECT_SOURCE must be github or local, not %q\n", projectSource)
		os.Exit(1)
	}
	// A project from a folder of this machine is for development: with a real sign-in
	// it would let people name paths on the server. Asking for it stops the start; when
	// it is only the default (no GitHub App), creating projects is switched off.
	if !authDisabled && projectSource == "local" {
		if os.Getenv("TF_PROJECT_SOURCE") != "" {
			fmt.Fprintln(os.Stderr, "error: TF_PROJECT_SOURCE=local is for development (TF_AUTH_DISABLED=true); with a real sign-in, projects come from GitHub")
			os.Exit(1)
		}
		projectSource = "none"
		logger.Warn("creating projects is switched off: a real sign-in needs the GitHub App (TF_GITHUB_APP_*) to create projects")
	}
	if projectSource == "github" && (ghAPI == nil || ghAPI.App == nil) {
		fmt.Fprintln(os.Stderr, "error: TF_PROJECT_SOURCE=github needs the GitHub App settings (TF_GITHUB_APP_*)")
		os.Exit(1)
	}
	logger.Info("projects are created from", "source", projectSource)

	if catalogSvc != nil {
		api.ConfigureModuleSources(os.Getenv("TF_MODULE_SOURCE_BASE"), func(ctx context.Context, projectID string) ([]catalog.Module, error) {
			mods, _, err := catalogSvc.VisibleModules(ctx, projectID)
			return mods, err
		})
	}
	router := api.NewRouter(api.RouterConfig{
		GitHub:             ghAPI,
		ProjectSource:      projectSource,
		GitHubLogin:        githubLogin,
		Providers:          providers,
		NoEntraLogin:       noEntraLogin,
		Logger:             logger,
		Queries:            q,
		AuthHandlers:       authHandlers,
		AuthMiddleware:     authMiddleware,
		CatalogService:     catalogSvc,
		NoDB:               noDB,
		SampleDir:          sampleDir,
		WebDir:             envOr("TF_WEB_DIR", ""),
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

// loginProviders decides which sign-in buttons exist. TF_LOGIN_PROVIDERS lists them
// (entra, github); empty means every provider that is configured. Asking for one that
// is not configured, or having none at all, stops the start.
func loginProviders(list string, entraSet, githubSet bool) ([]string, error) {
	configured := map[string]bool{"entra": entraSet, "github": githubSet}
	var out []string
	if strings.TrimSpace(list) == "" {
		for _, p := range []string{"entra", "github"} {
			if configured[p] {
				out = append(out, p)
			}
		}
	} else {
		for _, p := range splitList(strings.ToLower(list)) {
			set, known := configured[p]
			switch {
			case !known:
				return nil, fmt.Errorf("TF_LOGIN_PROVIDERS: unknown provider %q (use entra or github)", p)
			case !set:
				return nil, fmt.Errorf("TF_LOGIN_PROVIDERS lists %s but it is not configured", p)
			}
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no sign-in provider is configured: set the Entra settings (TF_ENTRA_*) or the GitHub App (TF_GITHUB_APP_* with a client id and secret), or TF_AUTH_DISABLED=true for development")
	}
	return out, nil
}

// autoApproveFits refuses a TF_AUTO_APPROVE rule whose sign-in provider is not enabled.
func autoApproveFits(rules auth.AutoApprove, enabled []string) error {
	on := map[string]bool{}
	for _, p := range enabled {
		on[p] = true
	}
	if rules.EntraTenant && !on["entra"] {
		return errors.New("TF_AUTO_APPROVE has entra-tenant but sign-in with Entra is not enabled")
	}
	if rules.NeedsGitHub() && !on["github"] {
		return errors.New("TF_AUTO_APPROVE has github-org or email-domain rules but sign-in with GitHub is not enabled")
	}
	return nil
}

// githubLoginRedirect is the sign-in callback on the same host as the connect callback:
// the App lists both URLs. Without a connect URL it assumes a local server.
func githubLoginRedirect(connectURI string) string {
	u, err := url.Parse(connectURI)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "http://localhost:8080/auth/github/callback"
	}
	return u.Scheme + "://" + u.Host + "/auth/github/callback"
}

func envInt(key string, fallback int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key))); err == nil && n >= 0 {
		return n
	}
	return fallback
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
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
