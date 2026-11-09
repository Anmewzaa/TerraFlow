package auth

import (
	"encoding/json"
	"net/http"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func RequireAuth(q *store.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(sessionCookieName)
			if err != nil {
				http.Error(w, `{"error":"not authenticated"}`, http.StatusUnauthorized)
				return
			}
			sess, err := q.GetSession(r.Context(), c.Value)
			if err != nil {
				http.Error(w, `{"error":"session expired"}`, http.StatusUnauthorized)
				return
			}
			// Only an active user gets past here. A pending user may ask who they are
			// (/api/me) so the page can say they are waiting; nothing else.
			st, err := q.GetUserStatus(r.Context(), sess.UserOID)
			if err != nil {
				http.Error(w, `{"error":"session expired"}`, http.StatusUnauthorized)
				return
			}
			if st.Status != store.StatusActive && r.URL.Path != "/api/me" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "your account is " + st.Status, "status": st.Status, "reason": st.Reason})
				return
			}
			u := &User{
				Status:   st.Status,
				OID:      sess.UserOID,
				Email:    sess.Email,
				Name:     sess.Name,
				Roles:    sess.Roles,
				GroupIDs: sess.GroupIDs,
			}
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
		})
	}
}
