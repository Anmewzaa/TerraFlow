package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/catalog"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

type RouterConfig struct {
	Logger         *slog.Logger
	Queries        *store.Queries
	AuthHandlers   *auth.Handlers
	AuthMiddleware func(http.Handler) http.Handler
	CatalogService *catalog.Service
	NoDB           bool
	SampleDir      string
}

func NewRouter(cfg RouterConfig) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	// Auth routes (no auth required)
	if cfg.AuthHandlers != nil {
		r.Get("/auth/login", cfg.AuthHandlers.Login)
		r.Get("/auth/callback", cfg.AuthHandlers.Callback)
		r.Get("/auth/logout", cfg.AuthHandlers.Logout)
	}

	// Health check (no auth)
	r.Get("/api/healthz", HealthHandler(cfg.Queries))

	// API routes (auth required)
	r.Route("/api", func(api chi.Router) {
		if cfg.AuthMiddleware != nil {
			api.Use(cfg.AuthMiddleware)
		}
		if cfg.AuthHandlers != nil {
			api.Get("/me", cfg.AuthHandlers.Me)
		}

		if cfg.NoDB {
			api.Get("/projects", NoDBProjectsHandler(cfg.SampleDir))
			api.Get("/projects/{id}/graph", NoDBGraphHandler(cfg.Logger, cfg.SampleDir))
			api.Get("/graph", GraphHandler(cfg.Logger))
		} else {
			api.Get("/projects", ProjectsListHandler(cfg.Queries))
			api.Get("/projects/{id}/graph", ProjectGraphHandler(cfg.Logger, cfg.Queries))
		}

		if cfg.CatalogService != nil {
			api.Get("/catalog", CatalogListHandler(cfg.CatalogService))
			api.Get("/catalog/{name}/{version}", CatalogGetHandler(cfg.CatalogService))
			api.Route("/projects/{id}/drafts", func(dr chi.Router) {
				dr.Get("/", DraftsListHandler(cfg.CatalogService))
				dr.Get("/{moduleId}", DraftGetHandler(cfg.CatalogService))
				dr.Put("/{moduleId}", DraftSaveHandler(cfg.CatalogService))
			})
		}
	})

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
