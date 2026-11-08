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

type Store interface {
	ListModules(ctx context.Context) ([]Module, error)
	GetModule(ctx context.Context, name, version string) (Module, error)
	GetModuleByID(ctx context.Context, id string) (Module, error)
	// UpsertModule is for seeding from files: it refreshes a version but keeps its status.
	UpsertModule(ctx context.Context, m Module) error
	// InsertModule adds a new version (ErrModuleExists if the name+version exists).
	InsertModule(ctx context.Context, m Module) error
	// SetModuleStatus publishes or deprecates a version (ErrModuleNotFound if unknown).
	SetModuleStatus(ctx context.Context, name, version, status string) error

	ListDrafts(ctx context.Context, projectID, userOID string) ([]Draft, error)
	GetDraft(ctx context.Context, projectID, moduleID, userOID string) (Draft, error)
	SaveDraft(ctx context.Context, d Draft) error
	DeleteDraft(ctx context.Context, id string) error
}
