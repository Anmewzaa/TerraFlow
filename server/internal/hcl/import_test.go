package hcl_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
)

func TestImportModules_KeepsReferencesAsExpressions(t *testing.T) {
	nodes, err := hcl.ImportModules(filepath.Join(findSamplesDir(t), "iac", "AIC"))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) == 0 {
		t.Fatal("no module calls imported")
	}
	var exprs, literals int
	for _, n := range nodes {
		if n.Existing == nil || n.Existing.Label == "" || n.Existing.Filename == "" || n.Existing.Folder == "" {
			t.Fatalf("node %s has no location: %+v", n.ID, n.Existing)
		}
		if n.Source == "" || n.ModuleName == "" {
			t.Errorf("node %s lost its source", n.ID)
		}
		for _, v := range n.Values {
			if m, ok := v.(map[string]interface{}); ok && len(m) == 1 && m["$expr"] != nil {
				exprs++
			} else {
				literals++
			}
		}
	}
	if exprs == 0 || literals == 0 {
		t.Fatalf("expected both references and literals, got %d exprs / %d literals", exprs, literals)
	}
}

// Importing every module call and writing it straight back must not change
// what the project means: the graph built from the rewritten files equals the
// graph of the originals. This is what keeps "edit one node" from freezing
// var/local/module references into literals.
func TestImportModules_ApplyRoundTripKeepsGraph(t *testing.T) {
	for _, project := range []string{"AIC", "AIH"} {
		t.Run(project, func(t *testing.T) {
			src := filepath.Join(findSamplesDir(t), "iac", project)
			nodes, err := hcl.ImportModules(src)
			if err != nil {
				t.Fatal(err)
			}
			out, err := hcl.Apply(hcl.GenerateInput{Nodes: nodes}, readProject(t, project))
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			tmp := t.TempDir()
			if err := os.CopyFS(tmp, os.DirFS(src)); err != nil {
				t.Fatal(err)
			}
			for _, f := range out {
				if err := os.WriteFile(filepath.Join(tmp, f.Folder, f.Filename), []byte(f.Content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for _, env := range []string{"dev", "prod"} {
				want, err := hcl.BuildGraph(src, env)
				if err != nil {
					t.Fatal(err)
				}
				got, err := hcl.BuildGraph(tmp, env)
				if err != nil {
					t.Fatalf("rewritten project does not build: %v", err)
				}
				got.Project = want.Project
				a, _ := json.Marshal(want)
				b, _ := json.Marshal(got)
				if string(a) != string(b) {
					t.Errorf("%s/%s: graph changed after round trip", project, env)
				}
			}
		})
	}
}
