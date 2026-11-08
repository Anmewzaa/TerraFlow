package auth

import (
	"encoding/json"
	"net/http"
)

// DevUser is a made-up person for the local sign-in page (auth disabled only).
type DevUser struct {
	OID   string `json:"oid"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
	Note  string `json:"note"`
}

// DevUsers are the people one can pick on the dev sign-in page, so admin and
// non-admin views can be tried without Entra. They are never used when auth is on.
var DevUsers = []DevUser{
	{OID: "dev-admin-0001", Email: "ada.admin@terraflow.local", Name: "Ada Admin", Role: "devops", Note: "Sees every project, manages modules, approves changesets"},
	{OID: "dev-alice-0002", Email: "alice@terraflow.local", Name: "Alice Developer", Role: "developer", Note: "Sees only the projects an admin assigned to her"},
	{OID: "dev-bob-0003", Email: "bob@terraflow.local", Name: "Bob Developer", Role: "developer", Note: "Sees only the projects an admin assigned to him"},
}

// DevCookie holds the chosen dev user's ID.
const DevCookie = "tf_dev_user"

func findDevUser(oid string) *DevUser {
	for i := range DevUsers {
		if DevUsers[i].OID == oid {
			return &DevUsers[i]
		}
	}
	return nil
}

// DevLogin is the sign-in-by-picking-a-user flow for TF_AUTH_DISABLED runs.
type DevLogin struct{}

// Middleware signs the request in as the user in the cookie. No valid cookie
// means 401, so the UI shows the sign-in page.
func (DevLogin) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(DevCookie)
		var du *DevUser
		if err == nil {
			du = findDevUser(c.Value)
		}
		if du == nil {
			http.Error(w, `{"error":"not authenticated"}`, http.StatusUnauthorized)
			return
		}
		u := &User{OID: du.OID, Email: du.Email, Name: du.Name, Roles: []string{du.Role}}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
	})
}

// Users: GET /api/dev/users. Who can be picked, and who is signed in (if anyone).
func (DevLogin) Users(w http.ResponseWriter, r *http.Request) {
	current := ""
	if c, err := r.Cookie(DevCookie); err == nil && findDevUser(c.Value) != nil {
		current = c.Value
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"enabled": true, "current": current, "users": DevUsers})
}

// Login: POST /api/dev/login {"oid": "..."}.
func (DevLogin) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OID string `json:"oid"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&body); err != nil || findDevUser(body.OID) == nil {
		http.Error(w, `{"error":"unknown user"}`, http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: DevCookie, Value: body.OID, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

// Logout: POST /api/dev/logout.
func (DevLogin) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: DevCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}
