package hcl_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
)

func buildGraph(t *testing.T, project, env string) *hcl.Graph {
	t.Helper()
	graph, err := hcl.BuildGraph(filepath.Join(findSamplesDir(t), "iac", project), env)
	if err != nil {
		t.Fatalf("BuildGraph(%s, %s): %v", project, env, err)
	}
	return graph
}

func TestBuildGraph_Golden(t *testing.T) {
	for _, tc := range []struct{ project, env, golden string }{
		{"AIC", "dev", "aic-dev.golden.json"},
		{"AIC", "prod", "aic-prod.golden.json"}, // conditionals resolve differently in prod
		{"AIH", "dev", "aih-dev.golden.json"},
	} {
		t.Run(tc.golden, func(t *testing.T) {
			got, err := json.MarshalIndent(buildGraph(t, tc.project, tc.env), "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			got = append(got, '\n')
			want, err := os.ReadFile(filepath.Join("testdata", tc.golden))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			if os.Getenv("UPDATE_GOLDEN") != "" {
				if err := os.WriteFile(filepath.Join("testdata", tc.golden), got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			if string(got) != string(want) {
				t.Errorf("graph does not match %s.\n--- got ---\n%s\n--- want ---\n%s", tc.golden, got, want)
			}
		})
	}
}

func crossFolderEdges(g *hcl.Graph) map[string]string {
	found := map[string]string{}
	for _, folder := range g.Folders {
		for _, edge := range folder.Edges {
			if edge.Kind == "cross-folder-data" {
				found[edge.From] = edge.To
			}
		}
	}
	return found
}

func TestBuildGraph_CrossFolderEdges(t *testing.T) {
	cases := map[string]map[string]string{
		"AIC": {
			"vnet.data.azurerm_resource_group.network": "common.module.rg_network",
			"keyvault.data.azurerm_resource_group.app": "common.module.rg_app",
			"identity.data.azurerm_resource_group.app": "common.module.rg_app",
			"storage.data.azurerm_resource_group.data": "common.module.rg_data",
			"keyvault.data.azurerm_subnet.pe":          "vnet.subnet.snet-pe",
			"storage.data.azurerm_subnet.pe":           "vnet.subnet.snet-pe",
		},
		// AIH keeps its resource groups next to their resources, so the only
		// cross-folder link is the private endpoint subnet.
		"AIH": {
			"workload.data.azurerm_subnet.pe": "network.subnet.snet-pe",
		},
	}
	for project, want := range cases {
		t.Run(project, func(t *testing.T) {
			got := crossFolderEdges(buildGraph(t, project, "dev"))
			for from, to := range want {
				if g, ok := got[from]; !ok {
					t.Errorf("missing cross-folder edge from %s", from)
				} else if g != to {
					t.Errorf("edge %s -> got %s, want %s", from, g, to)
				}
			}
			if len(got) != len(want) {
				t.Errorf("got %d cross-folder edges, want %d: %v", len(got), len(want), got)
			}
		})
	}
}

func TestBuildGraph_HubExternals(t *testing.T) {
	for _, project := range []string{"AIC", "AIH"} {
		t.Run(project, func(t *testing.T) {
			want := map[string]string{
				"hub.privatelink.vaultcore.azure.net":   "azurerm_private_dns_zone",
				"hub.privatelink.blob.core.windows.net": "azurerm_private_dns_zone",
			}
			for _, ext := range buildGraph(t, project, "dev").Externals {
				kind, ok := want[ext.ID]
				if !ok {
					t.Errorf("unexpected external: %s", ext.ID)
					continue
				}
				if ext.Kind != kind || ext.Source != "hub" {
					t.Errorf("external %s = kind %s source %s", ext.ID, ext.Kind, ext.Source)
				}
				delete(want, ext.ID)
			}
			for id := range want {
				t.Errorf("missing external: %s", id)
			}
		})
	}
}

func TestBuildGraph_SameFolderResourceGroupAndNetwork(t *testing.T) {
	// AIH defines the resource group and the VNet in one main.tf; no data lookup
	// is needed and the nodes must still be found.
	g := buildGraph(t, "AIH", "dev")
	ids := map[string]bool{}
	for _, f := range g.Folders {
		for _, n := range f.Nodes {
			ids[n.ID] = true
		}
	}
	for _, id := range []string{"network.module.rg_net", "network.module.vnet", "network.module.nsg_workload", "workload.module.rg_workload"} {
		if !ids[id] {
			t.Errorf("missing node %s", id)
		}
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

func TestBuildGraph_MissingFolderIsAnEmptyGraph(t *testing.T) {
	g, err := hcl.BuildGraph(filepath.Join(t.TempDir(), "not-there"), "dev")
	if err != nil {
		t.Fatalf("a missing folder must not be an error: %v", err)
	}
	if g.Folders == nil || g.Externals == nil || len(g.Folders) != 0 {
		t.Fatalf("want empty non-nil lists, got %+v", g)
	}
}
