package catalog

import "context"

type Store interface {
	ListModules(ctx context.Context) ([]Module, error)
	GetModule(ctx context.Context, name, version string) (Module, error)
	GetModuleByID(ctx context.Context, id string) (Module, error)
	UpsertModule(ctx context.Context, m Module) error

	ListDrafts(ctx context.Context, projectID, userOID string) ([]Draft, error)
	GetDraft(ctx context.Context, projectID, moduleID, userOID string) (Draft, error)
	SaveDraft(ctx context.Context, d Draft) error
	DeleteDraft(ctx context.Context, id string) error
}
