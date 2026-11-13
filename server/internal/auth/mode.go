package auth

import (
	"fmt"
	"strings"
)

// entraVars are the settings that mean "this server is wired to a real Entra tenant".
var entraVars = []string{
	"TF_ENTRA_TENANT_ID", "TF_ENTRA_CLIENT_ID", "TF_ENTRA_CLIENT_SECRET",
	"TF_ENTRA_DEVOPS_GROUP_ID", "TF_ENTRA_DEV_GROUP_ID",
}

// signInVars are the settings that only mean something with a real sign-in: who may
// sign in, who is an administrator, who is approved.
var signInVars = []string{"TF_LOGIN_PROVIDERS", "TF_SIGNUP_MODE", "TF_ADMIN_USERS", "TF_AUTO_APPROVE"}

// ValidateMode keeps the development switches away from a real deployment.
//
// It refuses to start with sign-in switched off (TF_AUTH_DISABLED) while Entra or the
// sign-in policy is also configured: that mix means someone copied a production
// configuration into a dev switch, and everyone would be admitted without a real
// sign-in. It also refuses TF_DEV_LOGIN (pick a made-up user) without
// TF_AUTH_DISABLED, where it would do nothing and only suggest it is safe to leave on.
// getenv is os.Getenv in the server; tests pass a map. Values are never printed.
func ValidateMode(authDisabled bool, getenv func(string) string) error {
	if !authDisabled {
		if strings.TrimSpace(getenv("TF_DEV_LOGIN")) == "true" {
			return fmt.Errorf("TF_DEV_LOGIN=true is for development and needs TF_AUTH_DISABLED=true; remove it from a server with a real sign-in")
		}
		return nil
	}
	var set []string
	for _, name := range append(append([]string{}, entraVars...), signInVars...) {
		if strings.TrimSpace(getenv(name)) != "" {
			set = append(set, name)
		}
	}
	if len(set) > 0 {
		return fmt.Errorf("TF_AUTH_DISABLED=true cannot be combined with real sign-in settings (%s); unset one of them", strings.Join(set, ", "))
	}
	return nil
}
