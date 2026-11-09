package auth

import (
	"strings"
	"testing"
)

func TestValidateMode(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	tests := []struct {
		name     string
		disabled bool
		env      map[string]string
		wantErr  string
	}{
		{"auth on, Entra set", false, map[string]string{"TF_ENTRA_CLIENT_ID": "c", "TF_ENTRA_TENANT_ID": "t"}, ""},
		{"auth on, nothing set", false, nil, ""},
		{"dev sign-in, nothing set", true, nil, ""},
		{"dev sign-in, unrelated settings", true, map[string]string{"TF_ADDR": ":9000", "TF_DEV_LOGIN": "true"}, ""},
		{"dev sign-in with a tenant", true, map[string]string{"TF_ENTRA_TENANT_ID": "t"}, "TF_ENTRA_TENANT_ID"},
		{"dev sign-in with a client secret", true, map[string]string{"TF_ENTRA_CLIENT_SECRET": "s"}, "TF_ENTRA_CLIENT_SECRET"},
		{"dev sign-in with a group id", true, map[string]string{"TF_ENTRA_DEVOPS_GROUP_ID": "g"}, "TF_ENTRA_DEVOPS_GROUP_ID"},
		{"blank value does not count", true, map[string]string{"TF_ENTRA_CLIENT_ID": "  "}, ""},
		{"all of them are named", true, map[string]string{"TF_ENTRA_CLIENT_ID": "c", "TF_ENTRA_TENANT_ID": "t"}, "TF_ENTRA_TENANT_ID, TF_ENTRA_CLIENT_ID"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateMode(tc.disabled, env(tc.env))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("got %v, want an error naming %q", err, tc.wantErr)
			}
			if strings.Contains(err.Error(), "=s") {
				t.Error("the error must not print values")
			}
		})
	}
}
