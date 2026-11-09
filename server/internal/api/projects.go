package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

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
		if !hcl.ValidEnv(env) {
			writeJSONError(w, http.StatusBadRequest, "invalid environment name")
			return
		}
		graph, err := hcl.BuildGraph(sampleDir, env)
		if err != nil {
			logger.Error("build graph", "err", err, "dir", sampleDir, "env", env)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
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
		if !hcl.ValidEnv(env) {
			writeJSONError(w, http.StatusBadRequest, "invalid environment name")
			return
		}

		graph, err := hcl.BuildGraph(project.IACPath, env)
		if err != nil {
			logger.Error("build graph", "err", err, "project", project.Slug, "env", env)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(graph)
	}
}
