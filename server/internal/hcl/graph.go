package hcl

import (
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	gohcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// BuildGraph parses a project directory and returns the graph for the given
// environment. Each subdirectory with .tf files is treated as a root module.
type producerEntry struct {
	nodeID string
	folder string
	name   string
}

func BuildGraph(projectDir, env string) (*Graph, error) {
	return BuildGraphFS(os.DirFS(projectDir), filepath.Base(projectDir), env)
}

// BuildGraphFS is BuildGraph over any file system: a directory (os.DirFS) or an
// in-memory snapshot of a repository. name is the project name shown in the graph.
func BuildGraphFS(fsys iofs.FS, name, env string) (*Graph, error) {
	if !ValidEnv(env) {
		return nil, fmt.Errorf("invalid environment name %q", env)
	}
	entries, err := iofs.ReadDir(fsys, ".")
	if errors.Is(err, iofs.ErrNotExist) {
		// A project whose folder is not there yet (new, or the workspace was
		// wiped) has nothing on the canvas; it is not an error.
		return &Graph{Project: name, Env: env, Folders: []Folder{}, Externals: []External{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read project dir: %w", err)
	}

	g := &Graph{
		Project: name,
		Env:     env,
	}

	// Parse all folders first so we can do cross-folder matching.
	type folderData struct {
		state *FolderState
		eval  *EvalContext
	}
	folders := make(map[string]*folderData)
	var folderOrder []string

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !hasTFFiles(fsys, e.Name()) {
			continue
		}
		fs, err := ParseFolderFS(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("parse folder %s: %w", e.Name(), err)
		}
		ec, err := NewEvalContext(fs, env)
		if err != nil {
			return nil, fmt.Errorf("eval folder %s: %w", e.Name(), err)
		}
		fd := &folderData{state: fs, eval: ec}
		folders[e.Name()] = fd
		folderOrder = append(folderOrder, e.Name())
	}

	// Collect all hub provider aliases across folders.
	hubAliases := make(map[string]bool)
	for _, name := range folderOrder {
		fd := folders[name]
		for _, p := range fd.state.Providers {
			if p.Alias != "" {
				hubAliases[p.Name+"."+p.Alias] = true
			}
		}
	}

	var producers []producerEntry

	// Phase 1: build nodes per folder.
	for _, folderName := range folderOrder {
		fd := folders[folderName]
		folder := Folder{Name: folderName}

		// Module call nodes.
		for _, mc := range fd.state.Modules {
			nodeID := folderName + ".module." + mc.Name
			catalogKey := moduleKeyFromSource(mc.Source)

			var resolvedName *string
			nameState := NameUnknown
			if attr, ok := mc.Body.Attributes["name"]; ok {
				resolvedName = fd.eval.EvalString(attr.Expr)
				if resolvedName != nil {
					nameState = NameResolved
				}
			}

			inputs := fd.eval.ResolveInputs(mc.Body)
			inputsJSON, _ := json.Marshal(inputs)

			node := Node{
				ID:        nodeID,
				Folder:    folderName,
				Kind:      KindModule,
				Type:      catalogKey,
				Domain:    DomainForModule(catalogKey),
				Name:      resolvedName,
				NameState: nameState,
				Source:    mc.Source,
				Container: nil,
				Inputs:    inputsJSON,
				Label:     mc.Name,
				Filename:  mc.File,
			}
			folder.Nodes = append(folder.Nodes, node)

			if resolvedName != nil {
				producers = append(producers, producerEntry{
					nodeID: nodeID,
					folder: folderName,
					name:   *resolvedName,
				})
			}

			// Extract subnet nodes from VNet module's subnets input.
			if catalogKey == "virtual-network" {
				if attr, ok := mc.Body.Attributes["subnets"]; ok {
					val, known := fd.eval.Eval(attr.Expr)
					if known && (val.Type().IsMapType() || val.Type().IsObjectType()) {
						for it := val.ElementIterator(); it.Next(); {
							key, _ := it.Element()
							subnetName := key.AsString()
							subnetID := folderName + ".subnet." + subnetName
							sn := subnetName
							folder.Nodes = append(folder.Nodes, Node{
								ID:        subnetID,
								Folder:    folderName,
								Kind:      KindResource,
								Type:      "subnet",
								Domain:    DomainNetwork,
								Name:      &sn,
								NameState: NameResolved,
								Container: &nodeID,
								Inputs:    json.RawMessage(`{}`),
							})
							// Register subnet as a producer (data lookups match by name).
							producers = append(producers, producerEntry{
								nodeID: subnetID,
								folder: folderName,
								name:   subnetName,
							})
						}
					}
				}
			}
		}

		// Data source nodes.
		for _, ds := range fd.state.DataSources {
			// Hub/alias data sources become externals.
			if ds.ProviderRef != "" && hubAliases[ds.ProviderRef] {
				var resolvedName string
				if attr, ok := ds.Body.Attributes["name"]; ok {
					s := fd.eval.EvalString(attr.Expr)
					if s != nil {
						resolvedName = *s
					}
				}
				extID := "hub." + resolvedName
				if resolvedName == "" {
					extID = "hub." + ds.Type + "." + ds.Name
				}
				g.Externals = append(g.Externals, External{
					ID:     extID,
					Kind:   ds.Type,
					Name:   resolvedName,
					Source: "hub",
				})
				continue
			}

			// Skip data sources that aren't cross-folder lookups (e.g. azurerm_client_config).
			if !isCrossFolderDataType(ds.Type) {
				continue
			}

			nodeID := folderName + ".data." + ds.Type + "." + ds.Name
			var resolvedName *string
			nameState := NameUnknown
			if attr, ok := ds.Body.Attributes["name"]; ok {
				resolvedName = fd.eval.EvalString(attr.Expr)
				if resolvedName != nil {
					nameState = NameResolved
				}
			}

			inputs := fd.eval.ResolveInputs(ds.Body)
			inputsJSON, _ := json.Marshal(inputs)

			folder.Nodes = append(folder.Nodes, Node{
				ID:        nodeID,
				Folder:    folderName,
				Kind:      KindData,
				Type:      ds.Type,
				Domain:    DomainNeutral,
				Name:      resolvedName,
				NameState: nameState,
				Container: nil,
				Inputs:    inputsJSON,
			})
		}

		// Same-folder module-output edges: detect references like module.vnet.subnet_ids
		for _, mc := range fd.state.Modules {
			nodeID := folderName + ".module." + mc.Name
			// Attributes come from a map: walk them in source order so the graph
			// is identical on every run, and report each target module once.
			attrs := make([]*hclsyntax.Attribute, 0, len(mc.Body.Attributes))
			for _, attr := range mc.Body.Attributes {
				attrs = append(attrs, attr)
			}
			sort.Slice(attrs, func(i, j int) bool {
				return attrs[i].SrcRange.Start.Byte < attrs[j].SrcRange.Start.Byte
			})
			seenRef := map[string]bool{}
			for _, attr := range attrs {
				refs := findModuleRefs(attr.Expr)
				for _, ref := range refs {
					targetID := folderName + ".module." + ref
					if targetID != nodeID && !seenRef[targetID] {
						seenRef[targetID] = true
						folder.Edges = append(folder.Edges, Edge{
							From: nodeID,
							To:   targetID,
							Kind: EdgeModuleOutput,
						})
					}
				}
			}
		}

		g.Folders = append(g.Folders, folder)
	}

	// Phase 2: cross-folder edges — match data sources to producers by resolved name.
	for fi, folder := range g.Folders {
		for _, node := range folder.Nodes {
			if node.Kind != KindData || node.Name == nil {
				continue
			}
			for _, p := range producers {
				if p.folder == folder.Name {
					continue // same folder, not cross-folder
				}
				if matchDataToProducer(node, p) {
					g.Folders[fi].Edges = append(g.Folders[fi].Edges, Edge{
						From: node.ID,
						To:   p.nodeID,
						Kind: EdgeCrossFolderData,
					})
				}
			}
		}
	}

	// Phase 3: container assignment via cross-folder edges.
	// For each folder, find its data.azurerm_resource_group.* edge to an RG producer.
	// All non-RG modules in that folder become children of that RG.
	folderRG := map[string]string{} // folder name → RG node ID
	for _, folder := range g.Folders {
		for _, edge := range folder.Edges {
			if edge.Kind == EdgeCrossFolderData && strings.Contains(edge.To, ".module.rg_") {
				for _, node := range folder.Nodes {
					if node.ID == edge.From && node.Kind == KindData && node.Type == "azurerm_resource_group" {
						folderRG[folder.Name] = edge.To
						break
					}
				}
			}
		}
	}
	for fi, folder := range g.Folders {
		rgID, ok := folderRG[folder.Name]
		if !ok {
			continue
		}
		for ni, node := range folder.Nodes {
			if node.Container == nil && node.Kind == KindModule {
				g.Folders[fi].Nodes[ni].Container = &rgID
			}
		}
	}

	return g, nil
}

var moduleSourceRe = regexp.MustCompile(`//([^?]+)`)

func moduleKeyFromSource(source string) string {
	m := moduleSourceRe.FindStringSubmatch(source)
	if len(m) < 2 {
		return source
	}
	// Return just the last path segment: "resource-group" from "org/repo//resource-group"
	parts := strings.Split(m[1], "/")
	return parts[len(parts)-1]
}

func hasTFFiles(fsys iofs.FS, dir string) bool {
	entries, err := iofs.ReadDir(fsys, dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tf") {
			return true
		}
	}
	return false
}

func isCrossFolderDataType(t string) bool {
	switch t {
	case "azurerm_resource_group", "azurerm_subnet", "azurerm_virtual_network":
		return true
	default:
		return false
	}
}

// matchDataToProducer determines if a data source node matches a producer.
func matchDataToProducer(data Node, p producerEntry) bool {
	if data.Name == nil {
		return false
	}
	switch data.Type {
	case "azurerm_resource_group":
		return *data.Name == p.name && strings.Contains(p.nodeID, ".module.rg_")
	case "azurerm_subnet":
		return *data.Name == p.name && strings.Contains(p.nodeID, ".subnet.")
	case "azurerm_virtual_network":
		return *data.Name == p.name && strings.Contains(p.nodeID, ".module.vnet")
	default:
		return *data.Name == p.name
	}
}

// findModuleRefs extracts module names referenced in an expression (e.g. module.vnet).
func findModuleRefs(expr gohcl.Expression) []string {
	var refs []string
	for _, trav := range expr.Variables() {
		if len(trav) >= 2 {
			if root, ok := trav[0].(gohcl.TraverseRoot); ok && root.Name == "module" {
				if attr, ok := trav[1].(gohcl.TraverseAttr); ok {
					refs = append(refs, attr.Name)
				}
			}
		}
	}
	return refs
}
