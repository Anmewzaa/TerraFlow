package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
)

// GraphHandler returns an http.HandlerFunc that serves the graph JSON for a
// project directory. Phase 1: the dir is passed as a query parameter; from
// Phase 2 it will come from the project's ADO repo.
func GraphHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dir := r.URL.Query().Get("dir")
		env := r.URL.Query().Get("env")
		if dir == "" {
			http.Error(w, `{"error":"dir is required"}`, http.StatusBadRequest)
			return
		}
		if env == "" {
			env = "dev"
		}
		if !hcl.ValidEnv(env) {
			writeJSONError(w, http.StatusBadRequest, "invalid environment name")
			return
		}

		graph, err := hcl.BuildGraph(dir, env)
		if err != nil {
			logger.Error("build graph", "err", err, "dir", dir, "env", env)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(graph)
	}
}
