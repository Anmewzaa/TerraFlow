package catalog

import (
	"encoding/json"
	"time"
)

type Module struct {
	ID           string          `json:"id"            db:"id"`
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
