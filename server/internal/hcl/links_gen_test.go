package hcl

import (
	"strings"
	"testing"
)

// A private endpoint linked to a key vault and a private DNS zone is written as references (8.11).
func TestLinkedEndpointIsWrittenAsReferences(t *testing.T) {
	out, err := Generate(GenerateInput{Nodes: []GenerateNode{{
		ID: "pe", ModuleName: "private-endpoint", ModuleVersion: "v1", InstanceName: "pe_kv", TargetFolder: "vnet", Filename: "pe.tf",
		Source: "git::https://example.invalid/modules//private-endpoint?ref=v1",
		Values: map[string]interface{}{
			"name":                 "pe-kv",
			"target_resource_id":   map[string]interface{}{"$expr": "module.kv.id"},
			"subresource_names":    []interface{}{"vault"},
			"private_dns_zone_ids": []interface{}{map[string]interface{}{"$expr": "module.dns.id"}},
		},
	}}})
	if err != nil || len(out.Files) != 1 {
		t.Fatalf("%v %+v", err, out)
	}
	c := out.Files[0].Content
	for _, want := range []string{"target_resource_id", "= module.kv.id", `subresource_names`, `"vault"`, "private_dns_zone_ids", "[module.dns.id]"} {
		if !strings.Contains(c, want) {
			t.Errorf("missing %q in:\n%s", want, c)
		}
	}
	if strings.Contains(c, `"module.kv.id"`) {
		t.Errorf("a reference must not be quoted:\n%s", c)
	}
}
