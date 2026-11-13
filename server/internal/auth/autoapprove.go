package auth

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github"
)

// AutoApprove holds the rules of TF_AUTO_APPROVE (D-062): who is active at the first
// sign-in without waiting for an administrator. A rule only ever uses what the identity
// provider itself confirms; when a rule cannot be checked the person simply waits for
// approval, as without the rule.
type AutoApprove struct {
	// EntraTenant: everyone who signs in through the configured Entra tenant.
	EntraTenant bool
	// GitHubOrgs: members of these GitHub organisations (lower case).
	GitHubOrgs []string
	// EmailDomains: GitHub accounts with a verified e-mail address in one of these
	// domains (lower case, exact domain, no sub-domains).
	EmailDomains []string
}

var (
	orgName    = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,37}[a-z0-9])?$`)
	domainName = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$`)
)

// ParseAutoApprove reads TF_AUTO_APPROVE: comma separated rules, each one of
// entra-tenant, github-org:<organisation> or email-domain:<domain>. Anything else
// stops the start instead of guessing.
func ParseAutoApprove(s string) (AutoApprove, error) {
	var a AutoApprove
	for _, rule := range strings.Split(s, ",") {
		rule = strings.ToLower(strings.TrimSpace(rule))
		if rule == "" {
			continue
		}
		kind, value, _ := strings.Cut(rule, ":")
		value = strings.TrimSpace(value)
		switch {
		case rule == "entra-tenant":
			a.EntraTenant = true
		case kind == "github-org" && orgName.MatchString(value):
			a.GitHubOrgs = append(a.GitHubOrgs, value)
		case kind == "email-domain" && domainName.MatchString(value):
			a.EmailDomains = append(a.EmailDomains, value)
		default:
			return AutoApprove{}, fmt.Errorf("TF_AUTO_APPROVE: %q is not a rule (use entra-tenant, github-org:<organisation> or email-domain:<domain>)", rule)
		}
	}
	return a, nil
}

// NeedsGitHub reports whether a rule can only be checked at a sign-in with GitHub.
func (a AutoApprove) NeedsGitHub() bool { return len(a.GitHubOrgs) > 0 || len(a.EmailDomains) > 0 }

// domainMatches reports whether one of the addresses is in one of the domains.
func (a AutoApprove) domainMatches(emails []string) bool {
	for _, e := range emails {
		at := strings.LastIndex(e, "@")
		if at <= 0 {
			continue
		}
		for _, d := range a.EmailDomains {
			if strings.EqualFold(e[at+1:], d) {
				return true
			}
		}
	}
	return false
}

// githubClient is what the GitHub rules ask (the App in the server, a fake in tests).
type githubClient interface {
	IsOrgMember(ctx context.Context, t github.Token, org, login string) (bool, error)
	VerifiedEmails(ctx context.Context, t github.Token) ([]string, error)
}

// GitHubVouches reports whether a rule approves this GitHub account, and which one
// (for the log). The profile's public e-mail is never used: only addresses GitHub
// lists as verified count. A failed question is logged by the caller through warn and
// counts as "no".
func (a AutoApprove) GitHubVouches(ctx context.Context, gh githubClient, u github.User, tok github.Token, warn func(msg string, args ...any)) (bool, string) {
	for _, org := range a.GitHubOrgs {
		ok, err := gh.IsOrgMember(ctx, tok, org, u.Login)
		if err != nil {
			warn("auto-approve: organisation membership could not be checked", "org", org, "err", err.Error())
		}
		if ok {
			return true, "github-org:" + org
		}
	}
	if len(a.EmailDomains) > 0 {
		emails, err := gh.VerifiedEmails(ctx, tok)
		if err != nil {
			warn("auto-approve: verified e-mail addresses could not be read (the App needs the account permission Email addresses: read)", "err", err.Error())
		}
		if a.domainMatches(emails) {
			return true, "email-domain"
		}
	}
	return false, ""
}
