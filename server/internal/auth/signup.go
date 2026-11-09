package auth

import (
	"strings"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// Sign-up modes (D-050).
const (
	SignupApproval   = "approval"    // a new user waits for an admin (the default)
	SignupOpen       = "open"        // a new user is active at once
	SignupInviteOnly = "invite_only" // an unknown user cannot sign up at all
)

// SignupPolicy decides what a first sign-in becomes.
type SignupPolicy struct {
	// Mode is one of the Signup* constants; "" means approval.
	Mode string
	// Admins are user ids or emails (TF_ADMIN_USERS); they are active at once and
	// get the admin role. People are matched by id, never by a login name.
	Admins []string
	// MaxPending caps the number of users waiting for approval (0 = no cap).
	MaxPending int
}

// IsAdmin reports whether the id or the email is listed in Admins.
func (p SignupPolicy) IsAdmin(oid, email string) bool {
	for _, a := range p.Admins {
		if a == "" {
			continue
		}
		if a == oid || (email != "" && strings.EqualFold(a, email)) {
			return true
		}
	}
	return false
}

// Initial is the status of a user who signs in for the first time, or "" when no
// new user is taken. adminRole says the identity provider already made them an admin.
func (p SignupPolicy) Initial(oid, email string, adminRole bool) string {
	if adminRole || p.IsAdmin(oid, email) {
		return store.StatusActive
	}
	switch p.Mode {
	case SignupOpen:
		return store.StatusActive
	case SignupInviteOnly:
		return ""
	default:
		return store.StatusPending
	}
}
