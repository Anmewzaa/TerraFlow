package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
)

// A raw resource saved on the canvas must reach the generator with its kind and
// type, and come out as a resource block (D-040).
func TestCanvasInput_RawResourceReachesGenerator(t *testing.T) {
	raw := json.RawMessage(`{"nodes":[{"id":"r1","kind":"resource","resource_type":"azurerm_resource_group",
		"instance_name":"rg","target_folder":"net","module_name":"","module_version":"",
		"values":{"name":"rg-demo","location":"southeastasia"}}],"edges":[]}`)
	files, _, err := applyCanvas("", raw, nil, nil, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || !strings.Contains(files[0].Content, `resource "azurerm_resource_group" "rg" {`) {
		t.Fatalf("want a resource block, got %+v", files)
	}
	if strings.Contains(files[0].Content, "module \"") || strings.Contains(files[0].Content, "\n  source") {
		t.Fatalf("a raw resource must not be written as a module call:\n%s", files[0].Content)
	}
	in, _, _ := canvasInput("", raw)
	if in.Nodes[0].Kind != hcl.KindResourceNode {
		t.Fatal("kind was dropped")
	}
}
