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
			// Who is allowed in is checked on every request, not only at sign-in, so a
			// suspension or a rejection takes effect at once. No row (a user from before
			// migration 015) counts as active.
			status, err := q.UserStatus(r.Context(), sess.UserOID)
			if err != nil {
				http.Error(w, `{"error":"could not check the account"}`, http.StatusInternalServerError)
				return
			}
			switch status {
			case store.UserRejected, store.UserSuspended:
				_ = q.DeleteSession(r.Context(), c.Value)
				http.Error(w, `{"error":"this account is not active"}`, http.StatusForbidden)
				return
			case store.UserPending:
				if r.URL.Path != "/api/me" {
					http.Error(w, `{"error":"waiting for approval"}`, http.StatusForbidden)
					return
				}
			}
			u := &User{
				OID:      sess.UserOID,
				Email:    sess.Email,
				Name:     sess.Name,
				Roles:    sess.Roles,
				GroupIDs: sess.GroupIDs,
			}
			if status == store.UserPending {
				u.Status = status
			}
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
		})
	}
}
