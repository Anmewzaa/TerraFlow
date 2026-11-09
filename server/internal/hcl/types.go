package hcl

import "encoding/json"

// Graph is the top-level structure emitted by the visualizer CLI and the
// GET /api/graph endpoint. It matches docs/09-graph-contract.md.
type Graph struct {
	Project   string     `json:"project"`
	Env       string     `json:"env"`
	Folders   []Folder   `json:"folders"`
	Externals []External `json:"externals"`
}

type Folder struct {
	Name  string `json:"name"`
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type NodeKind string

const (
	KindModule   NodeKind = "module"
	KindData     NodeKind = "data"
	KindResource NodeKind = "resource"
)

type NameState string

const (
	NameResolved NameState = "resolved"
	NameUnknown  NameState = "unknown"
)

type Node struct {
	ID        string          `json:"id"`
	Folder    string          `json:"folder"`
	Kind      NodeKind        `json:"kind"`
	Type      string          `json:"type"`
	Domain    Domain          `json:"domain"`
	Name      *string         `json:"name"`
	NameState NameState       `json:"nameState"`
	Source    string          `json:"source,omitempty"`
	Container *string         `json:"container"`
	Inputs    json.RawMessage `json:"inputs"`
	// Label/Filename locate a module call in the repository (module nodes only).
	Label    string `json:"label,omitempty"`
	Filename string `json:"filename,omitempty"`
}

type EdgeKind string

const (
	EdgeModuleOutput    EdgeKind = "module-output"
	EdgeCrossFolderData EdgeKind = "cross-folder-data"
	EdgeNesting         EdgeKind = "nesting"
)

type Edge struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Kind EdgeKind `json:"kind"`
}

type External struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Source string `json:"source"`
}
