package hcl_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
)

func TestBuildGraph_GoldenProject1Dev(t *testing.T) {
	samplesDir := findSamplesDir(t)
	projectDir := filepath.Join(samplesDir, "iac", "project-1")

	graph, err := hcl.BuildGraph(projectDir, "dev")
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	got, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got = append(got, '\n')

	goldenPath := filepath.Join("testdata", "project-1-dev.golden.json")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("graph does not match golden file.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBuildGraph_CrossFolderEdges(t *testing.T) {
	samplesDir := findSamplesDir(t)
	projectDir := filepath.Join(samplesDir, "iac", "project-1")

	graph, err := hcl.BuildGraph(projectDir, "dev")
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	expectedEdges := map[string]string{
		"vnet.data.azurerm_resource_group.network":  "common.module.rg_network",
		"keyvault.data.azurerm_resource_group.app":  "common.module.rg_app",
		"storage.data.azurerm_resource_group.data":  "common.module.rg_data",
		"keyvault.data.azurerm_subnet.pe":           "vnet.subnet.snet-pe",
		"storage.data.azurerm_subnet.pe":            "vnet.subnet.snet-pe",
	}

	foundEdges := make(map[string]string)
	for _, folder := range graph.Folders {
		for _, edge := range folder.Edges {
			if edge.Kind == "cross-folder-data" {
				foundEdges[edge.From] = edge.To
			}
		}
	}

	for from, to := range expectedEdges {
		got, ok := foundEdges[from]
		if !ok {
			t.Errorf("missing cross-folder edge from %s", from)
		} else if got != to {
			t.Errorf("edge %s -> got %s, want %s", from, got, to)
		}
	}
}

func TestBuildGraph_HubExternals(t *testing.T) {
	samplesDir := findSamplesDir(t)
	projectDir := filepath.Join(samplesDir, "iac", "project-1")

	graph, err := hcl.BuildGraph(projectDir, "dev")
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	expectedExternals := map[string]string{
		"hub.privatelink.vaultcore.azure.net":   "azurerm_private_dns_zone",
		"hub.privatelink.blob.core.windows.net": "azurerm_private_dns_zone",
	}

	for _, ext := range graph.Externals {
		wantKind, ok := expectedExternals[ext.ID]
		if !ok {
			t.Errorf("unexpected external: %s", ext.ID)
			continue
		}
		if ext.Kind != wantKind {
			t.Errorf("external %s kind = %s, want %s", ext.ID, ext.Kind, wantKind)
		}
		if ext.Source != "hub" {
			t.Errorf("external %s source = %s, want hub", ext.ID, ext.Source)
		}
		delete(expectedExternals, ext.ID)
	}
	for id := range expectedExternals {
		t.Errorf("missing external: %s", id)
	}
}

func findSamplesDir(t *testing.T) string {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	for d := dir; d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "samples")); err == nil {
			return filepath.Join(d, "samples")
		}
	}
	t.Fatal("cannot find samples/ directory")
	return ""
}
