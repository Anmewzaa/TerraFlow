package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func ProjectsListHandler(q *store.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFromContext(r.Context())
		var projects []store.Project
		var err error
		if u != nil && len(u.GroupIDs) > 0 {
			projects, err = q.ListProjectsByGroups(r.Context(), u.GroupIDs)
		} else {
			projects, err = q.ListProjects(r.Context())
		}
		if err != nil {
			http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(projects)
	}
}

func NoDBProjectsHandler(sampleDir string) http.HandlerFunc {
	fakeProject := []map[string]any{{
		"id":          "local-sample",
		"name":        "Project 1 (Local)",
		"slug":        "project-1-local",
		"iac_path":    sampleDir,
		"source_type": "local",
	}}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fakeProject)
	}
}

func NoDBGraphHandler(logger *slog.Logger, sampleDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		env := r.URL.Query().Get("env")
		if env == "" {
			env = "dev"
		}
		graph, err := hcl.BuildGraph(sampleDir, env)
		if err != nil {
			logger.Error("build graph", "err", err, "dir", sampleDir, "env", env)
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(graph)
	}
}

func ProjectGraphHandler(logger *slog.Logger, q *store.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		project, err := q.GetProject(r.Context(), id)
		if err != nil {
			http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
			return
		}

		env := r.URL.Query().Get("env")
		if env == "" {
			env = "dev"
		}

		graph, err := hcl.BuildGraph(project.IACPath, env)
		if err != nil {
			logger.Error("build graph", "err", err, "project", project.Slug, "env", env)
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(graph)
	}
}
