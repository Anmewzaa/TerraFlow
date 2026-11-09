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

// ValidateMode refuses to start with sign-in switched off (TF_AUTH_DISABLED, the
// dev sign-in) while Entra is also configured. That mix means someone copied a
// production configuration into a dev switch, and everyone would be admitted
// without a real sign-in. getenv is os.Getenv in the server; tests pass a map.
func ValidateMode(authDisabled bool, getenv func(string) string) error {
	if !authDisabled {
		return nil
	}
	var set []string
	for _, name := range entraVars {
		if strings.TrimSpace(getenv(name)) != "" {
			set = append(set, name)
		}
	}
	if len(set) > 0 {
		return fmt.Errorf("TF_AUTH_DISABLED=true cannot be combined with Entra settings (%s); unset one of them", strings.Join(set, ", "))
	}
	return nil
}
