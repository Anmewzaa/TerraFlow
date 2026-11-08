package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/catalog"
	"github.com/Anmewzaa/TerraFlow/server/internal/localrepo"
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
	WorkspaceDir   string
	// SeparationOfDuties forbids requesters from approving their own changesets.
	SeparationOfDuties bool
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
			api.Post("/projects", ProjectCreateHandler(cfg.Queries, cfg.WorkspaceDir, cfg.SampleDir))
			api.Delete("/projects/{id}", ProjectDeleteHandler(cfg.Queries))
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

		if cfg.Queries != nil {
			rv := &Reviewer{
				Store: cfg.Queries, Project: cfg.Queries.GetProject, Branch: localrepo.CreateBranch,
				Logger: cfg.Logger, WorkspaceDir: cfg.WorkspaceDir, SeparationOfDuties: cfg.SeparationOfDuties,
			}
			api.Get("/projects/{id}/files", ProjectFilesHandler(cfg.Queries.GetProject))
			api.Route("/projects/{id}/changesets", func(cs chi.Router) {
				cs.Get("/", ChangesetListHandler(cfg.Queries))
				cs.Post("/", ChangesetCreateHandler(cfg.Queries))
				cs.Get("/{csId}", ChangesetGetHandler(cfg.Queries))
				cs.Put("/{csId}", ChangesetSaveHandler(cfg.Queries))
				cs.Post("/{csId}/publish", ChangesetPublishHandler(cfg.Queries))
				cs.Get("/{csId}/preview", ChangesetPreviewHandler(cfg.Queries, cfg.Queries.GetProject))
				cs.Post("/{csId}/revise", ChangesetReviseHandler(cfg.Queries))
				cs.Delete("/{csId}", ChangesetDeleteHandler(cfg.Queries))
				cs.Get("/{csId}/review", rv.Detail())
				cs.Post("/{csId}/approve", rv.Approve())
				cs.Post("/{csId}/reject", rv.Reject())
			})
			api.Get("/projects/{id}/audit", rv.Audit())
		} else if cfg.NoDB {
			memCS := NewMemChangesetStore()
			memReview := NewMemReviewStore(memCS)
			memCS.Audit = memReview
			rv := &Reviewer{
				Store: memReview, Branch: localrepo.CreateBranch, Logger: cfg.Logger,
				WorkspaceDir: cfg.WorkspaceDir, SeparationOfDuties: cfg.SeparationOfDuties,
				Project: func(_ context.Context, id string) (store.Project, error) {
					return store.Project{ID: id, Name: "Project 1 (Local)", Slug: "project-1-local",
						IACPath: cfg.SampleDir, SourceType: "local"}, nil
				},
			}
			api.Get("/projects/{id}/files", ProjectFilesHandler(rv.Project))
			api.Route("/projects/{id}/changesets", func(cs chi.Router) {
				cs.Get("/", MemChangesetListHandler(memCS))
				cs.Post("/", MemChangesetCreateHandler(memCS))
				cs.Get("/{csId}", MemChangesetGetHandler(memCS))
				cs.Put("/{csId}", MemChangesetSaveHandler(memCS))
				cs.Post("/{csId}/publish", MemChangesetPublishHandler(memCS))
				cs.Get("/{csId}/preview", ChangesetPreviewHandler(memCS, rv.Project))
				cs.Post("/{csId}/revise", ChangesetReviseHandler(memCS))
				cs.Delete("/{csId}", MemChangesetDeleteHandler(memCS))
				cs.Get("/{csId}/review", rv.Detail())
				cs.Post("/{csId}/approve", rv.Approve())
				cs.Post("/{csId}/reject", rv.Reject())
			})
			api.Get("/projects/{id}/audit", rv.Audit())
		}
	})

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
