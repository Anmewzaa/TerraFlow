package auth

import (
	"fmt"
	"strings"
)

// Sign-up modes (D-050).
const (
	SignupApproval   = "approval"    // a new person waits for an admin (the default)
	SignupOpen       = "open"        // anyone who signs in is active at once
	SignupInviteOnly = "invite_only" // only people who already have an account
)

// Policy decides what happens to a person who signs in: whether they are an
// administrator named in the settings, and what a new person becomes.
type Policy struct {
	Mode string
	// Admins holds lower-cased ids (github:<id>, an Entra object id) and e-mail
	// addresses from TF_ADMIN_USERS. A login name never identifies anyone.
	Admins map[string]bool
	// MaxPending caps how many sign-ups may wait for approval (0 = no cap).
	MaxPending int
}

// NewPolicy parses TF_SIGNUP_MODE and TF_ADMIN_USERS (comma separated). An empty mode
// is "approval"; anything unknown stops the start instead of guessing.
func NewPolicy(mode, admins string, maxPending int) (Policy, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = SignupApproval
	}
	switch mode {
	case SignupApproval, SignupOpen, SignupInviteOnly:
	default:
		return Policy{}, fmt.Errorf("TF_SIGNUP_MODE must be approval, open or invite_only, not %q", mode)
	}
	p := Policy{Mode: mode, Admins: map[string]bool{}, MaxPending: maxPending}
	for _, a := range strings.Split(admins, ",") {
		if a = strings.ToLower(strings.TrimSpace(a)); a != "" {
			p.Admins[a] = true
		}
	}
	return p, nil
}

// IsAdmin reports whether this person is named in TF_ADMIN_USERS, by id or by e-mail.
func (p Policy) IsAdmin(id, email string) bool {
	if p.Admins[strings.ToLower(id)] {
		return true
	}
	return email != "" && p.Admins[strings.ToLower(email)]
}

// Initial is the status a new person gets. autoApproved is true when the identity
// provider already vouches for them (a member of the configured Entra group). An empty
// result means "no account, no sign-up" (invite only).
func (p Policy) Initial(autoApproved bool) string {
	if autoApproved {
		return "active"
	}
	switch p.Mode {
	case SignupOpen:
		return "active"
	case SignupInviteOnly:
		return ""
	default:
		return "pending"
	}
}
