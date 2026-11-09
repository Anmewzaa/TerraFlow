package hcl_test

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
)

// memCopy loads a directory into memory, the way a snapshot of a remote
// repository is held (no disk is needed to read it).
func memCopy(t *testing.T, dir string) fstest.MapFS {
	t.Helper()
	m := fstest.MapFS{}
	root := os.DirFS(dir)
	err := fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(root, p)
		if err != nil {
			return err
		}
		m[p] = &fstest.MapFile{Data: b}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestBuildGraphFS_MemoryEqualsDirectory(t *testing.T) {
	for _, project := range []string{"AIC", "AIH"} {
		dir := filepath.Join(findSamplesDir(t), "iac", project)
		for _, env := range []string{"dev", "prod"} {
			want, err := hcl.BuildGraph(dir, env)
			if err != nil {
				t.Fatal(err)
			}
			got, err := hcl.BuildGraphFS(memCopy(t, dir), project, env)
			if err != nil {
				t.Fatalf("%s/%s: %v", project, env, err)
			}
			a, _ := json.Marshal(want)
			b, _ := json.Marshal(got)
			if string(a) != string(b) {
				t.Errorf("%s/%s: the in-memory graph differs from the directory graph", project, env)
			}
		}
	}
}

func TestImportModulesFS_MemoryEqualsDirectory(t *testing.T) {
	for _, project := range []string{"AIC", "AIH"} {
		dir := filepath.Join(findSamplesDir(t), "iac", project)
		want, err := hcl.ImportModules(dir)
		if err != nil {
			t.Fatal(err)
		}
		got, err := hcl.ImportModulesFS(memCopy(t, dir))
		if err != nil {
			t.Fatal(err)
		}
		if len(want) == 0 || !reflect.DeepEqual(want, got) {
			t.Errorf("%s: imported nodes differ (%d vs %d)", project, len(want), len(got))
		}
	}
}

func TestBuildGraphFS_EnvFileComesFromTheSameSnapshot(t *testing.T) {
	m := fstest.MapFS{
		"net/main.tf":        {Data: []byte("variable \"project\" {\n  default = \"fromdefault\"\n}\nlocals {\n  rg = \"rg-${var.project}\"\n}\nmodule \"rg\" {\n  source = \"git::https://example.invalid/m//resource-group?ref=v1\"\n  name   = local.rg\n}\n")},
		"net/env/dev.tfvars": {Data: []byte("project = \"fromfile\"\n")},
	}
	g, err := hcl.BuildGraphFS(m, "p", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Folders) != 1 || len(g.Folders[0].Nodes) != 1 {
		t.Fatalf("graph: %+v", g)
	}
	n := g.Folders[0].Nodes[0]
	if n.Name == nil || *n.Name != "rg-fromfile" {
		t.Fatalf("the tfvars next to the folder must win over the default, got %v", n.Name)
	}
	// Another environment has no file, so the default applies.
	g2, err := hcl.BuildGraphFS(m, "p", "prod")
	if err != nil {
		t.Fatal(err)
	}
	if n := g2.Folders[0].Nodes[0]; n.Name == nil || *n.Name != "rg-fromdefault" {
		t.Fatalf("prod: %v", n.Name)
	}
}

func TestBuildGraphFS_EmptyAndBrokenSnapshots(t *testing.T) {
	g, err := hcl.BuildGraphFS(fstest.MapFS{}, "empty", "dev")
	if err != nil || len(g.Folders) != 0 {
		t.Fatalf("an empty snapshot is an empty graph: %v %+v", err, g)
	}
	broken := fstest.MapFS{"a/main.tf": {Data: []byte("module \"x\" {\n")}}
	if _, err := hcl.BuildGraphFS(broken, "p", "dev"); err == nil {
		t.Fatal("a file that does not parse must be an error")
	}
	// A repository can hold files that are not Terraform; they are ignored.
	mixed := fstest.MapFS{"README.md": {Data: []byte("# x")}, "a/notes.txt": {Data: []byte("x")}}
	g, err = hcl.BuildGraphFS(mixed, "p", "dev")
	if err != nil || len(g.Folders) != 0 {
		t.Fatalf("non-terraform files: %v %+v", err, g)
	}
}
