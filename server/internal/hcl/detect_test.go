package hcl

import (
	"reflect"
	"testing"
)

func TestDetectLayout(t *testing.T) {
	const tf = "# x\n"
	for name, c := range map[string]struct {
		repo           map[string]string
		pattern, value string
		envs           []string
		confidence     string
	}{
		"today's layout": {map[string]string{"vnet/main.tf": tf, "sa/main.tf": tf, "vnet/env/dev.tfvars": "", "vnet/env/prod.tfvars": ""},
			"{stack}", "{dir}/env/{env}.tfvars", []string{"dev", "prod"}, "high"},
		"stack then env": {map[string]string{"vnet/dev/main.tf": tf, "vnet/prod/main.tf": tf, "sa/dev/main.tf": tf, "vnet/dev/terraform.tfvars": ""},
			"{stack}/{env}", "{dir}/terraform.tfvars", []string{"dev", "prod"}, "high"},
		"env then stack": {map[string]string{"dev/vnet/main.tf": tf, "dev/sa/main.tf": tf, "prod/vnet/main.tf": tf, "dev/vnet/dev.tfvars": ""},
			"{env}/{stack}", "{dir}/{env}.tfvars", []string{"dev", "prod"}, "high"},
		"a folder per environment": {map[string]string{"dev/rg.tf": tf, "dev/vnet.tf": tf, "prod/rg.tf": tf},
			"{env}", "none", []string{"dev", "prod"}, "high"},
		"one folder": {map[string]string{"main.tf": tf, "dev.tfvars": "", "prod.tfvars": ""},
			".", "{dir}/{env}.tfvars", []string{"dev", "prod"}, "high"},
		"a prefix folder": {map[string]string{"AIC/vnet/dev/main.tf": tf, "AIC/sa/dev/main.tf": tf, "AIC/vnet/prod/main.tf": tf},
			"AIC/{stack}/{env}", "none", []string{"dev", "prod"}, "high"},
	} {
		got := DetectLayout(files(c.repo), nil)
		if got.Pattern != c.pattern || got.Values != c.value || got.Confidence != c.confidence || !reflect.DeepEqual(got.Environments, c.envs) {
			t.Errorf("%s: %+v", name, got)
		}
		if _, err := NewResolver(Layout{Pattern: got.Pattern, Values: got.Values}, "dev"); err != nil {
			t.Errorf("%s: the proposal must be valid: %v", name, err)
		}
	}
}

func TestDetectLayoutIsHonestWhenFoldersDoNotFit(t *testing.T) {
	got := DetectLayout(files(map[string]string{"vnet/dev/a.tf": "", "vnet/prod/a.tf": "", "sa/dev/a.tf": "", "odd/a.tf": ""}), nil)
	if got.Confidence != "low" {
		t.Fatalf("%+v", got)
	}
	var misfit int
	for _, f := range got.Folders {
		if !f.Fits {
			misfit++
		}
	}
	if misfit != 1 {
		t.Fatalf("exactly the odd folder must not fit: %+v", got.Folders)
	}
	if empty := DetectLayout(files(map[string]string{"README.md": "x"}), nil); empty.Confidence != "none" || empty.Pattern != DefaultFolderPattern {
		t.Fatalf("nothing to read: %+v", empty)
	}
}
