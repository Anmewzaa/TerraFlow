package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// AuditAllStore is what the admin audit page needs from the database.
type AuditAllStore interface {
	ListAuditAll(ctx context.Context, limit int) ([]store.AuditEntry, error)
}

// AuditAllHandler: GET /admin/audit?limit=N (admin). Everything that was done,
// across projects and module management, newest first.
func AuditAllHandler(s AuditAllStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 200
		if v := r.URL.Query().Get("limit"); v != "" {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}
		list, err := s.ListAuditAll(r.Context(), limit)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not read the audit log")
			return
		}
		if list == nil {
			list = []store.AuditEntry{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}
