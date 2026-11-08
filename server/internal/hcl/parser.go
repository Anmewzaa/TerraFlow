package hcl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// ModuleCall is a parsed module block.
type ModuleCall struct {
	Name   string
	Source string
	File   string // .tf file the block lives in
	Body   *hclsyntax.Body
}

// DataSource is a parsed data block.
type DataSource struct {
	Type         string
	Name         string
	ProviderRef  string // e.g. "azurerm.hub" or "" for default
	Body         *hclsyntax.Body
}

// VariableDef is a parsed variable block.
type VariableDef struct {
	Name    string
	Type    hcl.Expression
	Default hcl.Expression
}

// LocalsDef holds a single locals block's attributes.
type LocalsDef struct {
	Attrs hclsyntax.Attributes
}

// ProviderConfig is a parsed provider block.
type ProviderConfig struct {
	Name  string // e.g. "azurerm"
	Alias string // e.g. "hub" or ""
}

// FolderState holds the parsed content of one root-module folder.
type FolderState struct {
	Name      string
	Dir       string
	Modules   []ModuleCall
	DataSources []DataSource
	Variables []VariableDef
	Locals    []LocalsDef
	Providers []ProviderConfig
}

// ParseFolder reads all .tf files in dir and returns a FolderState.
func ParseFolder(dir string) (*FolderState, error) {
	parser := hclparse.NewParser()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	fs := &FolderState{
		Name: filepath.Base(dir),
		Dir:  dir,
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tf") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, diags := parser.ParseHCLFile(path)
		if diags.HasErrors() {
			return nil, fmt.Errorf("parse %s: %s", path, diags.Error())
		}
		body, ok := f.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		if err := extractBlocks(fs, e.Name(), body); err != nil {
			return nil, fmt.Errorf("extract %s: %w", path, err)
		}
	}
	return fs, nil
}

func extractBlocks(fs *FolderState, file string, body *hclsyntax.Body) error {
	for _, block := range body.Blocks {
		switch block.Type {
		case "variable":
			if len(block.Labels) < 1 {
				continue
			}
			vd := VariableDef{Name: block.Labels[0]}
			if attr, ok := block.Body.Attributes["default"]; ok {
				vd.Default = attr.Expr
			}
			if attr, ok := block.Body.Attributes["type"]; ok {
				vd.Type = attr.Expr
			}
			fs.Variables = append(fs.Variables, vd)

		case "locals":
			fs.Locals = append(fs.Locals, LocalsDef{Attrs: block.Body.Attributes})

		case "module":
			if len(block.Labels) < 1 {
				continue
			}
			mc := ModuleCall{Name: block.Labels[0], File: file, Body: block.Body}
			if attr, ok := block.Body.Attributes["source"]; ok {
				val, diags := attr.Expr.Value(nil)
				if !diags.HasErrors() {
					mc.Source = val.AsString()
				}
			}
			fs.Modules = append(fs.Modules, mc)

		case "data":
			if len(block.Labels) < 2 {
				continue
			}
			ds := DataSource{
				Type: block.Labels[0],
				Name: block.Labels[1],
				Body: block.Body,
			}
			if attr, ok := block.Body.Attributes["provider"]; ok {
				trav, diags := hcl.AbsTraversalForExpr(attr.Expr)
				if !diags.HasErrors() {
					ds.ProviderRef = traversalToString(trav)
				}
			}
			fs.DataSources = append(fs.DataSources, ds)

		case "provider":
			if len(block.Labels) < 1 {
				continue
			}
			pc := ProviderConfig{Name: block.Labels[0]}
			if attr, ok := block.Body.Attributes["alias"]; ok {
				val, diags := attr.Expr.Value(nil)
				if !diags.HasErrors() {
					pc.Alias = val.AsString()
				}
			}
			fs.Providers = append(fs.Providers, pc)
		}
	}
	return nil
}

func traversalToString(t hcl.Traversal) string {
	var parts []string
	for _, step := range t {
		switch s := step.(type) {
		case hcl.TraverseRoot:
			parts = append(parts, s.Name)
		case hcl.TraverseAttr:
			parts = append(parts, s.Name)
		}
	}
	return strings.Join(parts, ".")
}

// ParseTFVars reads a .tfvars file and returns its attributes.
func ParseTFVars(path string) (hclsyntax.Attributes, error) {
	parser := hclparse.NewParser()
	f, diags := parser.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse tfvars %s: %s", path, diags.Error())
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return nil, fmt.Errorf("unexpected body type in %s", path)
	}
	return body.Attributes, nil
}
