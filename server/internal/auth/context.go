package auth

import "context"

type ctxKey struct{}

type User struct {
	OID      string   `json:"oid"`
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Roles    []string `json:"roles"`
	GroupIDs []string `json:"group_ids"`
	// Status is active, pending, rejected or suspended; only an active user may use
	// TerraFlow. Empty means a source that has no approval step (the dev sign-in).
	Status string `json:"status,omitempty"`
}

func WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(ctxKey{}).(*User)
	return u
}
