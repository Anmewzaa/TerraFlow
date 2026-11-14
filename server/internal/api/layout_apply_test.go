package api

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
)

func rawNode(folder string) json.RawMessage {
	return json.RawMessage(`{"nodes":[{"id":"r1","kind":"resource","resource_type":"azurerm_resource_group","instance_name":"rg",
		"target_folder":"` + folder + `","values":{"name":"rg-demo","location":"southeastasia"}}],"edges":[]}`)
}

// A new resource lands in the folder the project's layout gives for its stack and the
// environment on the canvas, and a repository kept in another shape is read as one (8.9).
func TestNewResourcesGoWhereTheLayoutSays(t *testing.T) {
	repo := fstest.MapFS{
		"net/prod/main.tf": &fstest.MapFile{Data: []byte("# prod stays\n")},
		"net/dev/main.tf":  &fstest.MapFile{Data: []byte("# dev\n")},
	}
	for name, c := range map[string]struct {
		layout hcl.Layout
		stack  string
		want   string
	}{
		"{stack}/{env}": {hcl.Layout{Pattern: "{stack}/{env}"}, "net", "net/dev/main.tf"},
		"{env}":         {hcl.Layout{Pattern: "{env}"}, "net", "dev/main.tf"},
		"the default":   {hcl.Layout{}, "net", "net/main.tf"},
	} {
		st := &hcl.ProjectSettings{Layout: c.layout}
		snap, err := viewOf(gitprovider.Snapshot{FS: repo}, st, "dev")
		if err != nil {
			t.Fatal(err)
		}
		files, _, err := applyCanvas("", rawNode(c.stack), readRepoFilesFS(snap), st, "dev")
		if err != nil || len(files) == 0 {
			t.Fatalf("%s: %v %v", name, files, err)
		}
		var paths []string
		for _, f := range files {
			paths = append(paths, joinPath(f.Folder, f.Filename))
		}
		if !strings.Contains(strings.Join(paths, " "), c.want) {
			t.Errorf("%s: wrote %v, want %s", name, paths, c.want)
		}
		for _, p := range paths {
			if strings.Contains(p, "prod") {
				t.Errorf("%s: a dev change must not touch %s", name, p)
			}
		}
	}
}

func TestAnExistingFileIsEditedWhereItIs(t *testing.T) {
	repo := fstest.MapFS{"net/dev/main.tf": &fstest.MapFile{Data: []byte("# keep me\n")}}
	st := &hcl.ProjectSettings{Layout: hcl.Layout{Pattern: "{stack}/{env}"}}
	snap, _ := viewOf(gitprovider.Snapshot{FS: repo}, st, "dev")
	raw := json.RawMessage(`{"nodes":[{"id":"r1","kind":"resource","resource_type":"azurerm_resource_group","instance_name":"rg",
		"target_folder":"net","filename":"main.tf","values":{"name":"x","location":"y"}}],"edges":[]}`)
	files, _, err := applyCanvas("", raw, readRepoFilesFS(snap), st, "dev")
	if err != nil || len(files) != 1 {
		t.Fatalf("%v %v", files, err)
	}
	if files[0].Folder != "net/dev" || files[0].Filename != "main.tf" || files[0].Status != hcl.FileModified ||
		!strings.HasPrefix(files[0].Content, "# keep me\n") {
		t.Fatalf("the file must be appended to in place, got %+v", files[0])
	}
}
