package auth

import (
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
			u := &User{
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
