package hcl

import (
	"encoding/json"
	"fmt"
	iofs "io/fs"
	"path"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// Kinds of drift item (8.14). Drift here is the repository against what TerraFlow knows: it
// never asks Azure (hard rule 2).
const (
	DriftUnrepresented = "unrepresented"    // a block the canvas cannot show or edit exactly
	DriftVariable      = "variable_differs" // a value that differs from the project variable of the same name
	DriftFileChanged   = "file_changed"     // a file changed on the base branch since the baseline
	DriftFileAdded     = "file_added"
	DriftFileRemoved   = "file_removed"
)

// DriftItem is one thing worth a look. Key is stable, so a dismissal survives a reload.
type DriftItem struct {
	Key     string `json:"key"`
	Kind    string `json:"kind"`
	Folder  string `json:"folder,omitempty"`
	File    string `json:"file,omitempty"`
	Label   string `json:"label,omitempty"`
	Field   string `json:"field,omitempty"`
	NodeID  string `json:"node_id,omitempty"`
	Message string `json:"message"`
	// Importable: the block is on the canvas as a repository resource and can be opened there (D-036).
	Importable bool `json:"importable,omitempty"`
}

// FindUnrepresented lists the module and resource blocks the canvas cannot show exactly: a
// for_each or count (many copies behind one block) or a dynamic block. It reads names only.
func FindUnrepresented(fsys iofs.FS) ([]DriftItem, error) {
	entries, err := iofs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var out []DriftItem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		files, err := iofs.ReadDir(fsys, e.Name())
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".tf") {
				continue
			}
			p := path.Join(e.Name(), f.Name())
			src, err := iofs.ReadFile(fsys, p)
			if err != nil {
				continue
			}
			file, diags := hclparse.NewParser().ParseHCL(src, p)
			if diags.HasErrors() {
				continue
			}
			body, ok := file.Body.(*hclsyntax.Body)
			if !ok {
				continue
			}
			for _, b := range body.Blocks {
				if b.Type != "module" && b.Type != "resource" {
					continue
				}
				reason := unrepresentedReason(b.Body)
				if reason == "" {
					continue
				}
				label := strings.Join(b.Labels, ".")
				out = append(out, DriftItem{
					Key: fmt.Sprintf("%s|%s|%s", DriftUnrepresented, p, label), Kind: DriftUnrepresented,
					Folder: e.Name(), File: f.Name(), Label: label,
					Message: fmt.Sprintf("%s %q %s; the canvas cannot show it exactly, so edit it in git", b.Type, label, reason),
				})
			}
		}
	}
	return out, nil
}

func unrepresentedReason(b *hclsyntax.Body) string {
	if _, ok := b.Attributes["for_each"]; ok {
		return "uses for_each (one block, many copies)"
	}
	if _, ok := b.Attributes["count"]; ok {
		return "uses count (one block, many copies)"
	}
	for _, nb := range b.Blocks {
		if nb.Type == "dynamic" {
			return "has a dynamic block"
		}
		if r := unrepresentedReason(nb.Body); r != "" {
			return r
		}
	}
	return ""
}

// VariableDrift lists module inputs whose literal value differs from the project variable of the
// same name in env. An input that is a reference, or has no literal value, is not judged.
func VariableDrift(g *Graph, st *ProjectSettings, env string) []DriftItem {
	if g == nil || st == nil {
		return nil
	}
	want := map[string]string{}
	for _, v := range st.Variables {
		if val := v.Values[env]; val != "" {
			want[v.Name] = val
		}
	}
	names := make([]string, 0, len(want))
	for name := range want {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []DriftItem
	for _, f := range g.Folders {
		for _, n := range f.Nodes {
			if n.Kind != KindModule {
				continue
			}
			var inputs map[string]json.RawMessage
			if json.Unmarshal(n.Inputs, &inputs) != nil {
				continue
			}
			for _, name := range names {
				raw, ok := inputs[name]
				if !ok {
					continue
				}
				var have string
				if json.Unmarshal(raw, &have) != nil || have == want[name] {
					continue
				}
				out = append(out, DriftItem{
					Key: fmt.Sprintf("%s|%s|%s", DriftVariable, n.ID, name), Kind: DriftVariable,
					Folder: f.Name, File: n.Filename, Label: n.Label, Field: name, NodeID: n.ID, Importable: true,
					Message: fmt.Sprintf("%s %s is %q but the project variable %s is %q in %s", n.Label, name, have, name, want[name], env),
				})
			}
		}
	}
	return out
}

// DiffManifests compares the files the baseline accepted with the files now (path -> hash).
func DiffManifests(baseline, now map[string]string) []DriftItem {
	var out []DriftItem
	paths := map[string]bool{}
	for p := range baseline {
		paths[p] = true
	}
	for p := range now {
		paths[p] = true
	}
	sorted := make([]string, 0, len(paths))
	for p := range paths {
		sorted = append(sorted, p)
	}
	sort.Strings(sorted)
	for _, p := range sorted {
		b, inB := baseline[p]
		n, inN := now[p]
		folder, file := path.Dir(p), path.Base(p)
		if folder == "." {
			folder = ""
		}
		switch {
		case inB && !inN:
			out = append(out, DriftItem{Key: DriftFileRemoved + "|" + p, Kind: DriftFileRemoved, Folder: folder, File: file, Message: p + " was removed from the base branch since the baseline"})
		case !inB && inN:
			out = append(out, DriftItem{Key: DriftFileAdded + "|" + p, Kind: DriftFileAdded, Folder: folder, File: file, Message: p + " was added to the base branch since the baseline"})
		case b != n:
			out = append(out, DriftItem{Key: DriftFileChanged + "|" + p, Kind: DriftFileChanged, Folder: folder, File: file, Message: p + " changed on the base branch since the baseline"})
		}
	}
	return out
}
