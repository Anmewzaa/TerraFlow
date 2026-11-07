package auth

import "net/http"

func DevBypass(role string) func(http.Handler) http.Handler {
	u := &User{
		OID:   "dev-user-00000000",
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
