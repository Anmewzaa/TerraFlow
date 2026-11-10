package hcl

import (
	"strings"
	"testing"
)

func TestProviderNamesAndConstraints(t *testing.T) {
	for _, ok := range []string{"azurerm", "aws", "google", "oci2"} {
		if !ValidProviderID(ok) {
			t.Errorf("%q should be a provider id", ok)
		}
	}
	for _, bad := range []string{"", "a", "Azure", "azure-rm", "1aws", "x_y"} {
		if ValidProviderID(bad) {
			t.Errorf("%q must not be a provider id", bad)
		}
	}
	for _, ok := range []string{"~> 4.0", ">= 3.100", "= 4.1.0", "4.0", "~>4.0"} {
		if !ValidVersionConstraint(ok) {
			t.Errorf("%q should be a constraint", ok)
		}
	}
	for _, bad := range []string{"", "latest", "~> 4.0, < 5", "4.x", ">= "} {
		if ValidVersionConstraint(bad) {
			t.Errorf("%q must not be a constraint", bad)
		}
	}
	if ProviderOfType("azurerm_resource_group") != "azurerm" || ProviderOfType("aws_s3_bucket") != "aws" || ProviderOfType("nounderscore") != "" {
		t.Error("ProviderOfType")
	}
	if !ValidBound("") || !ValidBound("3.0") || ValidBound("3") || ValidBound("v3.0") {
		t.Error("ValidBound")
	}
}

func TestACompatibleRawResourceIsOneWhoseRangeContainsTheProjectVersion(t *testing.T) {
	for _, c := range []struct {
		project, min, max string
		want              bool
	}{
		{"~> 4.0", "", "", true},
		{"~> 4.0", "3.0", "", true},
		{"~> 4.0", "4.0", "4.0", true},
		{"~> 4.0", "4.1", "", false},    // added in a newer provider
		{"~> 3.100", "", "3.50", false}, // removed before the project's version
		{"~> 3.100", "3.0", "4.0", true},
		{"= 4.1.0", "4.1", "", true},
		{"", "9.9", "", true},       // a project with no version accepts everything
		{"latest", "9.9", "", true}, // an unreadable version is not a reason to hide anything
	} {
		if got := Compatible(c.project, c.min, c.max); got != c.want {
			t.Errorf("Compatible(%q, %q, %q) = %v, want %v", c.project, c.min, c.max, got, c.want)
		}
	}
}

func TestProvidersFile(t *testing.T) {
	got := providersFile([]ProjectProvider{
		{ID: "azurerm", Version: "~> 4.0", Source: "hashicorp/azurerm", Block: "provider \"azurerm\" {\n  features {}\n}"},
		{ID: "random", Source: "hashicorp/random"},
		{ID: "nosource", Version: "1.0"}, // not in the registry: left out, never guessed
	})
	want := "terraform {\n  required_providers {\n    azurerm = {\n      source  = \"hashicorp/azurerm\"\n      version = \"~> 4.0\"\n    }\n    random = {\n      source  = \"hashicorp/random\"\n    }\n  }\n}\n\nprovider \"azurerm\" {\n  features {}\n}\n"
	if got != want {
		t.Errorf("providers.tf:\n%s\nwant:\n%s", got, want)
	}
	if providersFile(nil) != "" || strings.Contains(providersFile([]ProjectProvider{{ID: "x"}}), "x") {
		t.Error("nothing to write without a source")
	}
}
