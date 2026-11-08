package hcl_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	hashihcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
)

// readProject loads every folder/*.tf of a sample project, like the API does.
func readProject(t *testing.T, project string) []hcl.GeneratedFile {
	t.Helper()
	root := filepath.Join(findSamplesDir(t), "iac", project)
	var files []hcl.GeneratedFile
	folders, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range folders {
		if !f.IsDir() {
			continue
		}
		entries, _ := os.ReadDir(filepath.Join(root, f.Name()))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".tf") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(root, f.Name(), e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			files = append(files, hcl.GeneratedFile{Folder: f.Name(), Filename: e.Name(), Content: strings.ReplaceAll(string(b), "\r\n", "\n")})
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Folder+"/"+files[i].Filename < files[j].Folder+"/"+files[j].Filename
	})
	return files
}

func parses(t *testing.T, name, src string) {
	t.Helper()
	if _, diags := hclsyntax.ParseConfig([]byte(src), name, hashihcl.InitialPos); diags.HasErrors() {
		t.Fatalf("%s does not parse: %s", name, diags.Error())
	}
}

// Adding a resource to ANY file of the sample projects must keep that file's
// existing bytes. This is the rg.tf incident, checked on every real sample file.
func TestApply_AppendKeepsEverySampleFile(t *testing.T) {
	for _, project := range []string{"AIC", "AIH"} {
		repo := readProject(t, project)
		for _, f := range repo {
			t.Run(project+"/"+f.Folder+"/"+f.Filename, func(t *testing.T) {
				node := hcl.GenerateNode{
					ID: "new", ModuleName: "resource-group", InstanceName: "added_by_test",
					TargetFolder: f.Folder, Filename: f.Filename, Source: "git::x//resource-group?ref=v1",
					Values: map[string]interface{}{"name": "rg-test", "location": "southeastasia"},
				}
				out, err := hcl.Apply(hcl.GenerateInput{Nodes: []hcl.GenerateNode{node}}, repo)
				if err != nil {
					t.Fatalf("Apply: %v", err)
				}
				if len(out) != 1 || out[0].Status != hcl.FileModified {
					t.Fatalf("want exactly this file modified, got %+v", out)
				}
				if !strings.HasPrefix(out[0].Content, f.Content) {
					t.Fatal("existing bytes changed")
				}
				parses(t, f.Filename, out[0].Content)
			})
		}
	}
}

var moduleLabel = regexp.MustCompile(`(?m)^module "([^"]+)" \{`)

// Editing any existing module call must change only that block.
func TestApply_EditTouchesOnlyTheBlockForEverySampleModule(t *testing.T) {
	for _, project := range []string{"AIC", "AIH"} {
		repo := readProject(t, project)
		for _, f := range repo {
			for _, m := range moduleLabel.FindAllStringSubmatch(f.Content, -1) {
				label := m[1]
				t.Run(project+"/"+f.Folder+"/"+label, func(t *testing.T) {
					node := hcl.GenerateNode{
						ID: "edit", ModuleName: "x", InstanceName: label, Source: "",
						Existing: &hcl.ExistingRef{Folder: f.Folder, Filename: f.Filename, Label: label},
						Values:   map[string]interface{}{"name": "edited-by-test"},
					}
					out, err := hcl.Apply(hcl.GenerateInput{Nodes: []hcl.GenerateNode{node}}, repo)
					if err != nil {
						t.Fatalf("Apply: %v", err)
					}
					if len(out) != 1 {
						t.Fatalf("want one changed file, got %d", len(out))
					}

					// Find the original block's byte range with the parser.
					file, _ := hclsyntax.ParseConfig([]byte(f.Content), f.Filename, hashihcl.InitialPos)
					var start, end int
					for _, b := range file.Body.(*hclsyntax.Body).Blocks {
						if b.Type == "module" && b.Labels[0] == label {
							start, end = b.TypeRange.Start.Byte, b.CloseBraceRange.End.Byte
						}
					}
					before, after := f.Content[:start], f.Content[end:]
					got := out[0].Content
					if !strings.HasPrefix(got, before) || !strings.HasSuffix(got, after) {
						t.Fatalf("bytes outside module %q changed", label)
					}
					if !strings.Contains(got[len(before):len(got)-len(after)], `"edited-by-test"`) {
						t.Fatal("edit not applied")
					}
					// The repository's own source line survives the edit.
					orig := f.Content[start:end]
					src := regexp.MustCompile(`source\s*=\s*"[^"]+"`).FindString(orig)
					if !strings.Contains(got, src) {
						t.Fatalf("source line %q lost", src)
					}
					parses(t, f.Filename, got)
				})
			}
		}
	}
}

// A second resource with an existing name must be refused on real code too.
func TestApply_DuplicateOfSampleModuleIsRefused(t *testing.T) {
	repo := readProject(t, "AIC")
	node := hcl.GenerateNode{
		ID: "dup", ModuleName: "resource-group", InstanceName: "rg_app", TargetFolder: "common",
		Filename: "rg.tf", Source: "x", Values: map[string]interface{}{"name": "n"},
	}
	if _, err := hcl.Apply(hcl.GenerateInput{Nodes: []hcl.GenerateNode{node}}, repo); err == nil {
		t.Fatal("rg_app already exists in AIC/common and must not be generated again")
	}
}
