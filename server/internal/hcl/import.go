package hcl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// ImportModules reads every module call under a project directory (one
// sub-folder per root module) and returns one GenerateNode per call, each
// pointing back at its block via Existing. Literal arguments become plain
// values; anything that is not a literal (var.x, local.y, module.a.id,
// conditionals, for expressions...) is kept verbatim as {"$expr": "<text>"},
// so editing an imported node never turns a reference into a frozen value.
//
// Folders and files are visited in sorted order and blocks in file order, so
// the result is deterministic.
func ImportModules(projectDir string) ([]GenerateNode, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, fmt.Errorf("read project dir: %w", err)
	}
	var out []GenerateNode
	for _, e := range entries { // os.ReadDir is sorted by name
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(projectDir, e.Name())
		files, err := filepath.Glob(filepath.Join(dir, "*.tf"))
		if err != nil {
			return nil, err
		}
		sort.Strings(files)
		for _, path := range files {
			nodes, err := importFile(e.Name(), path)
			if err != nil {
				return nil, err
			}
			out = append(out, nodes...)
		}
	}
	return out, nil
}

func importFile(folder, path string) ([]GenerateNode, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, diags := hclparse.NewParser().ParseHCL(src, path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse %s: %s", path, diags.Error())
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return nil, nil
	}
	var nodes []GenerateNode
	for _, block := range body.Blocks {
		if block.Type == "resource" && len(block.Labels) == 2 {
			// Resources whose body cannot be written back exactly (dynamic or
			// labelled nested blocks) are left alone, not imported.
			if values, ok := importBody(block.Body, src); ok {
				label := block.Labels[0] + "." + block.Labels[1]
				nodes = append(nodes, GenerateNode{
					ID:           folder + ".resource." + label,
					Kind:         KindResourceNode,
					ResourceType: block.Labels[0],
					InstanceName: block.Labels[1],
					TargetFolder: folder,
					Filename:     filepath.Base(path),
					Values:       values,
					Existing:     &ExistingRef{Folder: folder, Filename: filepath.Base(path), Label: label},
				})
			}
			continue
		}
		if block.Type != "module" || len(block.Labels) < 1 {
			continue
		}
		label := block.Labels[0]
		n := GenerateNode{
			ID:           folder + ".module." + label,
			InstanceName: label,
			TargetFolder: folder,
			Filename:     filepath.Base(path),
			Values:       map[string]interface{}{},
			Existing:     &ExistingRef{Folder: folder, Filename: filepath.Base(path), Label: label},
		}
		names := make([]string, 0, len(block.Body.Attributes))
		for name := range block.Body.Attributes {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			attr := block.Body.Attributes[name]
			if name == "source" {
				if v, d := attr.Expr.Value(nil); !d.HasErrors() && v.Type().FriendlyName() == "string" {
					n.Source = v.AsString()
					n.ModuleName = moduleKeyFromSource(n.Source)
				}
				continue
			}
			n.Values[name] = importValue(attr.Expr, src)
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

// importValue returns the literal value of expr, or its source text as a
// {"$expr": ...} marker when it cannot be evaluated without context.
func importValue(expr hclsyntax.Expression, src []byte) interface{} {
	if v, d := expr.Value(nil); !d.HasErrors() && v.IsWhollyKnown() {
		if raw, err := ctyjson.Marshal(v, v.Type()); err == nil {
			var out interface{}
			if json.Unmarshal(raw, &out) == nil {
				return out
			}
		}
	}
	r := expr.Range()
	return map[string]interface{}{"$expr": string(r.SliceBytes(src))}
}

// importBody turns a block body into values: attributes as in importValue and
// nested blocks as {"$blocks": [...]} per block type. It reports false for a
// body that cannot be written back faithfully (dynamic or labelled blocks).
func importBody(body *hclsyntax.Body, src []byte) (map[string]interface{}, bool) {
	out := map[string]interface{}{}
	for name, attr := range body.Attributes {
		out[name] = importValue(attr.Expr, src)
	}
	groups := map[string][]interface{}{}
	var order []string
	for _, b := range body.Blocks {
		if b.Type == "dynamic" || len(b.Labels) > 0 {
			return nil, false
		}
		child, ok := importBody(b.Body, src)
		if !ok {
			return nil, false
		}
		if _, seen := groups[b.Type]; !seen {
			order = append(order, b.Type)
		}
		groups[b.Type] = append(groups[b.Type], child)
	}
	for _, t := range order {
		if _, clash := out[t]; clash {
			return nil, false
		}
		out[t] = map[string]interface{}{"$blocks": groups[t]}
	}
	return out, true
}
