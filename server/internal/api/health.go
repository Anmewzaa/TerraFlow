package api

import (
	"net/http"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func HealthHandler(q *store.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}
}
