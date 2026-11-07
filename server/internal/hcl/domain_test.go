package hcl

import "testing"

func TestDomainForModule(t *testing.T) {
	cases := map[string]Domain{
		"virtual-network": DomainNetwork,
		"storage-account": DomainData,
		"key-vault":       DomainSecurity,
		"resource-group":  DomainNeutral, // container frame
		"made-up-module":  DomainNeutral,
	}
	for module, want := range cases {
		if got := DomainForModule(module); got != want {
			t.Errorf("DomainForModule(%q) = %q, want %q", module, got, want)
		}
	}
}

func TestDomainsOrder(t *testing.T) {
	if len(Domains()) != 5 {
		t.Fatalf("expected 5 domains, got %d", len(Domains()))
	}
}
