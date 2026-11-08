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
	CreatedAt    time.Time       `json:"created_at"    db:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"    db:"updated_at"`
}

type ModuleSummary struct {
	Name          string   `json:"name"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Icon          string   `json:"icon"`
	Versions      []string `json:"versions"`
	LatestVersion string   `json:"latest_version"`
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
