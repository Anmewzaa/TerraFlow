package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/Anmewzaa/TerraFlow/server/internal/api"
	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/catalog"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	ctx := context.Background()

	addr := envOr("TF_ADDR", ":8080")
	authDisabled := envOr("TF_AUTH_DISABLED", "false") == "true"
	insecureCookies := envOr("TF_INSECURE_COOKIES", "false") == "true"
	noDB := envOr("TF_NO_DB", "false") == "true"

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

		if err := store.Migrate(dbCfg.DSN()); err != nil {
			logger.Error("migrate", "err", err)
			os.Exit(1)
		}
		logger.Info("migrations applied")

		q = store.New(pool)

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
		oidcCfg := auth.OIDCConfig{
			TenantID:     os.Getenv("TF_ENTRA_TENANT_ID"),
			ClientID:     os.Getenv("TF_ENTRA_CLIENT_ID"),
			ClientSecret: os.Getenv("TF_ENTRA_CLIENT_SECRET"),
			RedirectURI:  envOr("TF_ENTRA_REDIRECT_URI", "http://localhost:8080/auth/callback"),
		}
		oidcProvider, err := auth.NewOIDCProvider(ctx, oidcCfg)
		if err != nil {
			logger.Error("oidc init", "err", err)
			os.Exit(1)
		}
		roles := auth.RoleConfig{
			DevOpsGroupID:    os.Getenv("TF_ENTRA_DEVOPS_GROUP_ID"),
			DeveloperGroupID: os.Getenv("TF_ENTRA_DEV_GROUP_ID"),
		}
		authMiddleware = auth.RequireAuth(q)
		authHandlers = auth.NewHandlers(oidcProvider, q, roles, logger, insecureCookies)
	}

	sampleDir := envOr("TF_SAMPLE_DIR", "./samples/iac/AIC")
	catalogDir := envOr("TF_CATALOG_DIR", "./catalog/modules")

	var catalogSvc *catalog.Service
	if noDB {
		catalogSvc = catalog.NewService(catalog.NewFSStore(catalogDir))
	} else {
		pgCatalog := catalog.NewPGStore(q)
		catalogSvc = catalog.NewService(pgCatalog)
		if err := catalog.SeedFromDir(ctx, pgCatalog, logger, catalogDir); err != nil {
			logger.Error("seed catalog", "err", err)
			os.Exit(1)
		}
		logger.Info("catalog seeded", "dir", catalogDir)
	}

	router := api.NewRouter(api.RouterConfig{
		Logger:         logger,
		Queries:        q,
		AuthHandlers:   authHandlers,
		AuthMiddleware: authMiddleware,
		CatalogService: catalogSvc,
		NoDB:           noDB,
		SampleDir:      sampleDir,
		DevLogin:       devLogin,
		WorkspaceDir:   envOr("TF_WORKSPACE_DIR", "./workspace"),
		SeparationOfDuties: envOr("TF_SEPARATION_OF_DUTIES", "false") == "true",
	})

	logger.Info("terraflow server", "addr", addr, "auth_disabled", authDisabled, "no_db", noDB)
	fmt.Fprintf(os.Stderr, "listening on %s\n", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
