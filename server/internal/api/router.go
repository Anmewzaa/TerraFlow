package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

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
	// DevLogin, when set, serves the pick-a-user sign-in of an auth-disabled run.
	DevLogin       *auth.DevLogin
	CatalogService *catalog.Service
	// GitHub serves the connection flow and creating projects from a repository.
	// Nil means GitHub is not set up; its Store is filled in from Queries.
	GitHub       *GitHubAPI
	NoDB         bool
	SampleDir    string
	WorkspaceDir string
	// SeparationOfDuties forbids requesters from approving their own changesets.
	SeparationOfDuties bool
	// AllowedOrigins lists the origins allowed to call the API from a browser.
	// Empty (the default) means same-origin only.
	AllowedOrigins []string
	// HSTS turns on Strict-Transport-Security (set it when served over HTTPS).
	HSTS bool
	// AuthRateBurst, when above zero, limits sign-in and callback requests per
	// client address: AuthRateBurst at once, one more every AuthRateEvery.
	AuthRateBurst int
	AuthRateEvery time.Duration
}

func NewRouter(cfg RouterConfig) chi.Router {
	// Local projects are folders of this machine; GitHub projects go through the App.
	repos := LocalRepos(cfg.WorkspaceDir)
	if cfg.Queries != nil && !cfg.NoDB && cfg.GitHub != nil && cfg.GitHub.App != nil {
		repos = ProjectRepos(repos, cfg.GitHub.App, cfg.Queries)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(SecurityHeaders(cfg.HSTS))
	r.Use(CORS(cfg.AllowedOrigins))
	r.Use(SameOriginWrites(cfg.AllowedOrigins))

	// Auth routes (no auth required)
	if cfg.AuthHandlers != nil {
		authRoutes := r.With()
		if cfg.AuthRateBurst > 0 {
			authRoutes = r.With(RateLimit(cfg.AuthRateBurst, cfg.AuthRateEvery, time.Now))
		}
		authRoutes.Get("/auth/login", cfg.AuthHandlers.Login)
		authRoutes.Get("/auth/callback", cfg.AuthHandlers.Callback)
		r.Get("/auth/logout", cfg.AuthHandlers.Logout)
	}

	// Dev sign-in: pick a made-up user (asking who is available needs no sign-in).
	if cfg.DevLogin != nil {
		r.Get("/api/dev/users", cfg.DevLogin.Users)
		r.Post("/api/dev/login", cfg.DevLogin.Login)
		r.Post("/api/dev/logout", cfg.DevLogin.Logout)
	}

	// Health check (no auth)
	r.Get("/api/healthz", HealthHandler(cfg.Queries))

	// API routes (auth required)
	r.Route("/api", func(api chi.Router) {
		if cfg.AuthMiddleware != nil {
			api.Use(cfg.AuthMiddleware)
		}
		if cfg.Queries != nil && !cfg.NoDB {
			api.Use(UserSync(cfg.Queries, cfg.Logger))
		}
		// guard limits /projects/{id}/... to admins and assigned members.
		var guard chi.Router = api
		if cfg.Queries != nil && !cfg.NoDB {
			guard = api.With(ProjectGuard(cfg.Queries))
		}
		admin := api.With(RequireAdmin)
		if cfg.AuthHandlers != nil {
			api.Get("/me", cfg.AuthHandlers.Me)
		}

		if cfg.NoDB {
			api.Get("/projects", NoDBProjectsHandler(cfg.SampleDir))
			api.Get("/projects/{id}/graph", NoDBGraphHandler(cfg.Logger, cfg.SampleDir))
			api.Get("/graph", GraphHandler(cfg.Logger))
		} else {
			adm := &AdminAPI{Store: cfg.Queries, WorkspaceDir: cfg.WorkspaceDir, SampleDir: cfg.SampleDir}
			api.Get("/projects", adm.List())
			admin.Post("/projects", adm.Create())
			admin.Patch("/projects/{id}", adm.Update())
			team := &TeamAPI{Store: cfg.Queries}
			// An admin deletes any project; an owner deletes their own GitHub project.
			guard.Delete("/projects/{id}", team.OwnerDelete(adm.Delete()))
			guard.Get("/projects/{id}/team", team.Get())
			guard.Post("/projects/{id}/team", team.Invite())
			guard.Patch("/projects/{id}/team/{oid}", team.SetRole())
			guard.Delete("/projects/{id}/team/{oid}", team.Remove())
			guard.Put("/projects/{id}/settings", team.Settings())
			guard.Post("/projects/{id}/transfer", team.Transfer())
			admin.Get("/projects/{id}/members", adm.Members())
			admin.Put("/projects/{id}/members", adm.SetMembers())
			admin.Get("/admin/users", adm.Users())
			admin.Get("/admin/audit", AuditAllHandler(cfg.Queries))
			gh := cfg.GitHub
			if gh == nil {
				gh = &GitHubAPI{}
			}
			gh.Store = cfg.Queries
			api.Get("/github/status", gh.Status())
			api.Get("/github/authorize", gh.Authorize())
			api.Get("/github/callback", gh.Callback())
			api.Get("/github/repos", gh.Repos())
			api.Get("/github/folders", gh.Folders())
			api.Post("/projects/from-repo", gh.CreateFromRepo())
			admin.Get("/admin/connections", gh.Connections())
			admin.Post("/admin/connections/{id}/disconnect", gh.Disconnect())
			admin.Post("/admin/connections/{id}/recheck", gh.Recheck())
			rr := &RawResourceAPI{Store: cfg.Queries}
			api.Get("/catalog/resources", rr.List())
			admin.Post("/admin/resources", rr.Create())
			admin.Put("/admin/resources/{name}", rr.Update())
			admin.Delete("/admin/resources/{name}", rr.Delete())
			guard.Get("/projects/{id}/graph", ProjectGraphHandler(cfg.Logger, cfg.Queries, repos))
		}

		if cfg.CatalogService != nil {
			api.Get("/catalog", CatalogListHandler(cfg.CatalogService))
			api.Get("/catalog/{name}/{version}", CatalogGetHandler(cfg.CatalogService))
			if cfg.Queries != nil && !cfg.NoDB {
				ca := &CatalogAdmin{Svc: cfg.CatalogService, Audit: cfg.Queries}
				admin.Get("/admin/catalog/modules", ca.List())
				admin.Post("/admin/catalog/validate", ca.Validate())
				admin.Post("/admin/catalog/modules", ca.Import())
				admin.Patch("/admin/catalog/modules/{name}/{version}", ca.SetStatus())
			}
			guard.Route("/projects/{id}/drafts", func(dr chi.Router) {
				dr.Get("/", DraftsListHandler(cfg.CatalogService))
				dr.Get("/{moduleId}", DraftGetHandler(cfg.CatalogService))
				dr.Put("/{moduleId}", DraftSaveHandler(cfg.CatalogService))
			})
		}

		if cfg.Queries != nil {
			rv := &Reviewer{
				Store: cfg.Queries, Project: cfg.Queries.GetProject, Repos: repos, Roles: cfg.Queries,
				Logger: cfg.Logger, SeparationOfDuties: cfg.SeparationOfDuties,
			}
			guard.Get("/projects/{id}/component-config", ComponentConfigList(cfg.Queries))
			guard.Put("/projects/{id}/component-config", ComponentConfigSet(cfg.Queries))
			guard.Get("/projects/{id}/files", ProjectFilesHandler(cfg.Queries.GetProject, repos))
			guard.Get("/projects/{id}/import", ProjectImportHandler(cfg.Queries.GetProject, repos))
			guard.Route("/projects/{id}/changesets", func(list chi.Router) {
				list.Get("/", ChangesetListHandler(cfg.Queries))
				list.Post("/", ChangesetCreateHandler(cfg.Queries))
				// Every route with a {csId} first proves the changeset is this project's.
				list.With(ChangesetInProject(cfg.Queries.GetChangeset)).Group(func(cs chi.Router) {
					cs.Get("/{csId}", ChangesetGetHandler(cfg.Queries))
					cs.Put("/{csId}", ChangesetSaveHandler(cfg.Queries))
					cs.Post("/{csId}/versions", ChangesetSaveVersionHandler(cfg.Queries))
					cs.Post("/{csId}/publish", ChangesetPublishHandler(cfg.Queries))
					cs.Get("/{csId}/preview", ChangesetPreviewHandler(cfg.Queries, cfg.Queries.GetProject, repos))
					cs.Post("/{csId}/revise", ChangesetReviseHandler(cfg.Queries))
					cs.Delete("/{csId}", ChangesetDeleteHandler(cfg.Queries))
					cs.Get("/{csId}/review", rv.Detail())
					cs.Post("/{csId}/approve", rv.Approve())
					cs.Post("/{csId}/reject", rv.Reject())
				})
			})
			guard.Get("/projects/{id}/audit", rv.Audit())
		} else if cfg.NoDB {
			memCS := NewMemChangesetStore()
			memReview := NewMemReviewStore(memCS)
			memCS.Audit = memReview
			rv := &Reviewer{
				Store: memReview, Repos: repos, Logger: cfg.Logger,
				SeparationOfDuties: cfg.SeparationOfDuties,
				Project: func(_ context.Context, id string) (store.Project, error) {
					return store.Project{ID: id, Name: "Project 1 (Local)", Slug: "project-1-local",
						IACPath: cfg.SampleDir, SourceType: "local"}, nil
				},
			}
			api.Get("/projects/{id}/files", ProjectFilesHandler(rv.Project, repos))
			api.Get("/projects/{id}/import", ProjectImportHandler(rv.Project, repos))
			api.Route("/projects/{id}/changesets", func(cs chi.Router) {
				cs.Get("/", MemChangesetListHandler(memCS))
				cs.Post("/", MemChangesetCreateHandler(memCS))
				cs.Get("/{csId}", MemChangesetGetHandler(memCS))
				cs.Put("/{csId}", MemChangesetSaveHandler(memCS))
				cs.Post("/{csId}/versions", MemChangesetSaveVersionHandler(memCS))
				cs.Post("/{csId}/publish", MemChangesetPublishHandler(memCS))
				cs.Get("/{csId}/preview", ChangesetPreviewHandler(memCS, rv.Project, repos))
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
