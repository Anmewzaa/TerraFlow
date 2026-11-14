package catalog

import (
	"encoding/json"
	"time"
)

// Scopes of a module: the global catalog (an admin's) or one project's own (D-060).
const (
	ScopeGlobal  = "global"
	ScopeProject = "project"
)

type Module struct {
	ID string `json:"id"            db:"id"`
	// ProjectID is the project the module belongs to; "" is the global catalog.
	ProjectID    string          `json:"project_id,omitempty" db:"project_id"`
	Name         string          `json:"name"          db:"name"`
	Version      string          `json:"version"       db:"version"`
	Title        string          `json:"title"         db:"title"`
	Description  string          `json:"description"   db:"description"`
	Icon         string          `json:"icon"          db:"icon"`
	TargetFolder string          `json:"target_folder" db:"target_folder"`
	SchemaJSON   json.RawMessage `json:"schema"        db:"schema_json"`
	Checksum     string          `json:"-"             db:"checksum"`
	// Status is "published" or "deprecated" ("" is read as published).
	Status     string    `json:"status"      db:"status"`
	ImportedBy string    `json:"imported_by" db:"imported_by"`
	CreatedAt  time.Time `json:"created_at"    db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"    db:"updated_at"`
}

type ModuleSummary struct {
	Name          string   `json:"name"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Icon          string   `json:"icon"`
	Versions      []string `json:"versions"`
	LatestVersion string   `json:"latest_version"`
	// Canvas is the schema's x-tf-canvas block (container/nesting hints), if any.
	Canvas json.RawMessage `json:"canvas,omitempty"`
	// Required lists the schema's required input names (for "incomplete" markers).
	Required []string `json:"required,omitempty"`
	// Provider is the cloud provider the module is written for (x-tf-module.provider), "" for none.
	Provider string `json:"provider,omitempty"`
	// Category groups the module in the palette: x-tf-canvas.category, else the one that matches its
	// canvas domain (see CategoryOfDomain).
	Category string `json:"category,omitempty"`
	// Offers is what the module offers to other resources (x-tf-link.offers, 8.11): a kind of link
	// (private_link, private_dns_zone...), the output that carries it and, for a private link, the
	// sub-resource name the endpoint needs.
	Offers []LinkOffer `json:"offers,omitempty"`
	// LinkFields maps an input that takes a link to the kind it asks for (x-tf-ui.pick.link), so the
	// canvas can draw the reference as a link.
	LinkFields map[string]string `json:"link_fields,omitempty"`
	// Scope is "project" for a module of the project's own catalog, else "global".
	Scope string `json:"scope"`
	// ShadowsGlobal is true for a project module whose name the global catalog also has: inside
	// the project its own module is the one used, and the screens say so.
	ShadowsGlobal bool `json:"shadows_global,omitempty"`
}

// ScopeOf names the scope of a project id ("" is global).
func ScopeOf(projectID string) string {
	if projectID == "" {
		return ScopeGlobal
	}
	return ScopeProject
}

type Draft struct {
	ID         string          `json:"id"          db:"id"`
	ProjectID  string          `json:"project_id"  db:"project_id"`
	ModuleID   string          `json:"module_id"   db:"module_id"`
	UserOID    string          `json:"user_oid"    db:"user_oid"`
	ValuesJSON json.RawMessage `json:"values"      db:"values_json"`
	CreatedAt  time.Time       `json:"created_at"  db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"  db:"updated_at"`
}

type TFModuleMeta struct {
	Source       string `json:"source"`
	Version      string `json:"version"`
	TargetFolder string `json:"target_folder"`
	Icon         string `json:"icon"`
}

// LinkOffer is one thing a module offers to the fields that ask for a link of its kind.
type LinkOffer struct {
	Kind        string `json:"kind"`
	Output      string `json:"output"`
	Subresource string `json:"subresource,omitempty"`
}
