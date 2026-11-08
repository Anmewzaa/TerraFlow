package auth

import "net/http"

// DevUserOID is the ID of the signed-in user when auth is disabled.
const DevUserOID = "dev-user-00000000"

func DevBypass(role string) func(http.Handler) http.Handler {
	u := &User{
		OID:   DevUserOID,
		Email: "dev@terraflow.local",
		Name:  "Dev User",
		Roles: []string{role},
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
		})
	}
}
