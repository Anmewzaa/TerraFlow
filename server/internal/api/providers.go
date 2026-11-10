package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// ProviderStore is what the provider endpoints need from the database.
type ProviderStore interface {
	ListProviders(ctx context.Context) ([]store.Provider, error)
	CreateProvider(ctx context.Context, p store.Provider) error
	UpdateProvider(ctx context.Context, p store.Provider) error
	DeleteProvider(ctx context.Context, id string) error
	LogAudit(ctx context.Context, userOID, action, resource string, detail any) error
}

// ProviderAPI serves the cloud provider registry (D-058): every signed-in user reads it, admins
// change it. TerraFlow never contacts a cloud; a provider is only data.
type ProviderAPI struct{ Store ProviderStore }

// Category is a group of resources in the palette (compute, network...). They are data the server
// serves, so the screens do not carry their own list; the same set applies to every provider.
type Category struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Categories is the set of palette groups, in display order.
var Categories = []Category{
	{"compute", "Compute"}, {"network", "Network"}, {"storage", "Storage"}, {"database", "Database"},
	{"security", "Security & identity"}, {"monitoring", "Monitoring"}, {"containers", "Containers"},
	{"integration", "Integration & messaging"}, {"other", "Other"},
}

// ValidCategory reports whether id is a known category ("" means none).
func ValidCategory(id string) bool {
	if id == "" {
		return true
	}
	for _, c := range Categories {
		if c.ID == id {
			return true
		}
	}
	return false
}

const (
	maxLogoBytes    = 40 << 10
	maxProviderBody = 96 << 10
)

type providerBody struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Source   string   `json:"source"`
	Logo     string   `json:"logo"`
	Versions []string `json:"versions"`
	Block    string   `json:"block"`
}

// decodeBodyLimit reads a JSON body of at most max bytes, refusing unknown fields.
func decodeBodyLimit(w http.ResponseWriter, r *http.Request, v any, max int64) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, max))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSONError(w, http.StatusBadRequest, "the body is not valid JSON, has unknown fields, or is too large")
		return false
	}
	return true
}

// validLogo accepts an image as a data URI (SVG or PNG): the screens show it in an <img>, which
// never runs script, and nothing is fetched from another site.
func validLogo(logo string) bool {
	if logo == "" {
		return true
	}
	if len(logo) > maxLogoBytes {
		return false
	}
	for _, prefix := range []string{"data:image/svg+xml;base64,", "data:image/png;base64,"} {
		if rest, ok := strings.CutPrefix(logo, prefix); ok {
			_, err := base64.StdEncoding.DecodeString(rest)
			return err == nil
		}
	}
	return false
}

func (b providerBody) check() string {
	switch {
	case !hcl.ValidProviderID(b.ID):
		return "id must be 2-31 lowercase letters or digits, starting with a letter (azurerm, aws, google)"
	case strings.TrimSpace(b.Name) == "" || len(b.Name) > 60:
		return "name is required (at most 60 characters)"
	case !validSource(b.Source):
		return "source must look like hashicorp/azurerm"
	case !validLogo(b.Logo):
		return "the logo must be a PNG or SVG data URI of at most 40 KB"
	case len(b.Versions) > 20:
		return "at most 20 versions"
	case len(b.Block) > 2000:
		return "the provider block is at most 2000 characters"
	}
	for _, v := range b.Versions {
		if !hcl.ValidVersionConstraint(v) {
			return "version " + v + " must be one constraint such as ~> 4.0"
		}
	}
	return ""
}

// validSource accepts a Terraform source address: namespace/type, or host/namespace/type.
func validSource(s string) bool {
	parts := strings.Split(s, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return false
	}
	for _, p := range parts {
		if p == "" || strings.ContainsAny(p, " \t\r\n\"\\") || strings.Contains(p, "..") {
			return false
		}
	}
	return true
}

func (b providerBody) toStore() store.Provider {
	if b.Versions == nil {
		b.Versions = []string{}
	}
	return store.Provider{ID: b.ID, Name: strings.TrimSpace(b.Name), Source: b.Source, Logo: b.Logo, Versions: b.Versions, Block: strings.TrimSpace(b.Block)}
}

// List: GET /catalog/providers.
func (a *ProviderAPI) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := a.Store.ListProviders(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not load providers")
			return
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// Meta: GET /catalog/meta serves the palette categories.
func Meta() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"categories": Categories})
	}
}

// Create: POST /admin/providers.
func (a *ProviderAPI) Create() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b providerBody
		if !decodeBodyLimit(w, r, &b, maxProviderBody) {
			return
		}
		if msg := b.check(); msg != "" {
			writeJSONError(w, http.StatusBadRequest, msg)
			return
		}
		p := b.toStore()
		if err := a.Store.CreateProvider(r.Context(), p); errors.Is(err, store.ErrProviderExists) {
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		} else if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not save the provider")
			return
		}
		_ = a.Store.LogAudit(r.Context(), actor(r), "provider.create", "provider:"+p.ID, map[string]any{"source": p.Source})
		writeJSON(w, http.StatusCreated, p)
	}
}

// Update: PUT /admin/providers/{id}. The id never changes.
func (a *ProviderAPI) Update() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var b providerBody
		if !decodeBodyLimit(w, r, &b, maxProviderBody) {
			return
		}
		b.ID = chi.URLParam(r, "id")
		if msg := b.check(); msg != "" {
			writeJSONError(w, http.StatusBadRequest, msg)
			return
		}
		p := b.toStore()
		if err := a.Store.UpdateProvider(r.Context(), p); errors.Is(err, pgx.ErrNoRows) {
			writeJSONError(w, http.StatusNotFound, "provider not found")
			return
		} else if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not save the provider")
			return
		}
		_ = a.Store.LogAudit(r.Context(), actor(r), "provider.update", "provider:"+p.ID, map[string]any{"source": p.Source})
		writeJSON(w, http.StatusOK, p)
	}
}

// Delete: DELETE /admin/providers/{id}; refused while raw resources use it.
func (a *ProviderAPI) Delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		err := a.Store.DeleteProvider(r.Context(), id)
		switch {
		case errors.Is(err, store.ErrProviderInUse):
			writeJSONError(w, http.StatusConflict, err.Error()+": delete or move them first")
			return
		case errors.Is(err, pgx.ErrNoRows):
			writeJSONError(w, http.StatusNotFound, "provider not found")
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "could not delete the provider")
			return
		}
		_ = a.Store.LogAudit(r.Context(), actor(r), "provider.delete", "provider:"+id, map[string]any{})
		w.WriteHeader(http.StatusNoContent)
	}
}
