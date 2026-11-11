package catalog

import (
	"context"
	"errors"
)

// Module lifecycle states.
const (
	StatusPublished  = "published"
	StatusDeprecated = "deprecated"
)

var (
	// ErrModuleExists: a published version is immutable, so importing the same
	// name+version again is refused.
	ErrModuleExists = errors.New("module version already exists")
	// ErrModuleNotFound is returned for an unknown name+version.
	ErrModuleNotFound = errors.New("module not found")
)

// Store keeps module schemas in scopes: projectID "" is the global catalog, anything else one
// project's own catalog. No method falls back from one scope to another; Service does the merging.
type Store interface {
	ListModules(ctx context.Context, projectID string) ([]Module, error)
	GetModule(ctx context.Context, projectID, name, version string) (Module, error)
	GetModuleByID(ctx context.Context, id string) (Module, error)
	// UpsertModule is for seeding the global catalog from files: it refreshes a version but keeps its status.
	UpsertModule(ctx context.Context, m Module) error
	// InsertModule adds a new version to the scope in m.ProjectID (ErrModuleExists if the name+version exists there).
	InsertModule(ctx context.Context, m Module) error
	// SetModuleStatus publishes or deprecates a version (ErrModuleNotFound if unknown).
	SetModuleStatus(ctx context.Context, projectID, name, version, status string) error

	ListDrafts(ctx context.Context, projectID, userOID string) ([]Draft, error)
	GetDraft(ctx context.Context, projectID, moduleID, userOID string) (Draft, error)
	SaveDraft(ctx context.Context, d Draft) error
	DeleteDraft(ctx context.Context, id string) error
}
