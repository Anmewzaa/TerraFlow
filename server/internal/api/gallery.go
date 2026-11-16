package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/stacktpl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// GalleryStore is what the shared template gallery needs from the database. *store.Queries implements it.
type GalleryStore interface {
	CreateGalleryTemplate(ctx context.Context, g store.GalleryTemplate) (store.GalleryTemplate, error)
	ListGalleryTemplates(ctx context.Context, f store.GalleryFilter) ([]store.GalleryTemplate, error)
	GetGalleryTemplate(ctx context.Context, id string) (store.GalleryTemplate, error)
	UpdateGalleryTemplate(ctx context.Context, id string, g store.GalleryTemplate) error
	SetGalleryStatus(ctx context.Context, id, status, note, by string) error
	DeleteGalleryTemplate(ctx context.Context, id string) error
	RecordGalleryImport(ctx context.Context, id, projectID, by string) error
	LogAudit(ctx context.Context, userOID, action, resource string, detail any) error
}

// ExportFormat marks a template file written by TerraFlow.
const ExportFormat = "terraflow-stack-template"

var categoryRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 &/_-]{0,39}$`)

// GalleryAPI is the shared template gallery (8.16): private and public templates, export and import,
// an admin review of public ones. A template is a stack template document (8.15); everything that
// comes in is untrusted and passes stacktpl.Check and stacktpl.Scan first.
type GalleryAPI struct {
	Store   GalleryStore
	Team    *TeamAPI
	Stacks  StackStore
	Catalog moduleSource
}

type galleryWho struct {
	oid, name string
	admin     bool
}

func whoIs(r *http.Request) galleryWho {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		return galleryWho{}
	}
	n := u.Name
	if n == "" {
		n = u.Email
	}
	return galleryWho{oid: u.OID, name: n, admin: isDevOps(u)}
}

// canSee: approved public templates are for every signed-in person; the rest is for the author and admins.
func (w galleryWho) canSee(g store.GalleryTemplate) bool {
	return w.admin || g.Author == w.oid || (g.Scope == store.GalleryPublic && g.Status == store.GalleryApproved)
}

func (a *GalleryAPI) audit(r *http.Request, action, id string, detail map[string]any) {
	_ = a.Store.LogAudit(r.Context(), actor(r), action, "gallery:"+id, detail)
}

func (a *GalleryAPI) load(w http.ResponseWriter, r *http.Request) (store.GalleryTemplate, galleryWho, bool) {
	who := whoIs(r)
	g, err := a.Store.GetGalleryTemplate(r.Context(), chi.URLParam(r, "gid"))
	if errors.Is(err, pgx.ErrNoRows) || isBadReference(err) || (err == nil && !who.canSee(g)) {
		// A template the person may not see looks the same as one that does not exist.
		writeJSONError(w, http.StatusNotFound, "template not found")
		return g, who, false
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "db error")
		return g, who, false
	}
	return g, who, true
}

// prepared is a template that passed the gate, with what the shelf stores next to it.
type prepared struct {
	tpl     stacktpl.Template
	doc     json.RawMessage
	modules []string
}

// prepare runs the gate on an untrusted document. It writes the 4xx answer itself and returns false
// when the template cannot be stored. For a public template every leak blocks, including ids.
func prepare(w http.ResponseWriter, raw json.RawMessage, name, scope string) (prepared, bool) {
	if len(raw) == 0 || len(raw) > MaxCanvasBytes {
		writeJSONError(w, http.StatusBadRequest, "the template is missing or too large")
		return prepared{}, false
	}
	var t stacktpl.Template
	if err := json.Unmarshal(raw, &t); err != nil {
		writeJSONError(w, http.StatusBadRequest, "the template is not valid: "+err.Error())
		return prepared{}, false
	}
	if name != "" {
		t.Name = name
	}
	if t.Edges == nil {
		t.Edges = []store.CanvasEdge{}
	}
	if t.Slots == nil {
		t.Slots = []stacktpl.Slot{}
	}
	if t.Params == nil {
		t.Params = []stacktpl.Param{}
	}
	if problems := stacktpl.Check(t); len(problems) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "the template cannot be accepted", "problems": problems})
		return prepared{}, false
	}
	leaks := stacktpl.Scan(t)
	block := stacktpl.Blocking(leaks)
	if scope == store.GalleryPublic && len(leaks) > 0 {
		block = true
	}
	if block {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "the template holds something that must not be shared", "leaks": leaks})
		return prepared{}, false
	}
	doc, _ := json.Marshal(t)
	return prepared{tpl: t, doc: doc, modules: stacktpl.Modules(t)}, true
}

type galleryBody struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Scope       string          `json:"scope"`
	Template    json.RawMessage `json:"template"`
	// ProjectID and StackTemplateID share a template of a project (owner and co-owner of it).
	ProjectID       string `json:"project_id"`
	StackTemplateID string `json:"stack_template_id"`
}

func (b *galleryBody) clean() string {
	b.Name, b.Description, b.Category = strings.TrimSpace(b.Name), strings.TrimSpace(b.Description), strings.TrimSpace(b.Category)
	if b.Scope == "" {
		b.Scope = store.GalleryPrivate
	}
	switch {
	case b.Scope != store.GalleryPrivate && b.Scope != store.GalleryPublic:
		return "the scope is private or public"
	case len(b.Description) > 500:
		return "the description has at most 500 characters"
	case b.Category != "" && !categoryRe.MatchString(b.Category):
		return "the category is up to 40 letters, digits, spaces and & / _ -"
	}
	return ""
}

// statusFor: a private template is live at once; a public one waits for an admin (an admin's own
// public template is live: they could approve it anyway).
func statusFor(scope string, who galleryWho) string {
	if scope == store.GalleryPublic && !who.admin {
		return store.GalleryPending
	}
	return store.GalleryApproved
}

// List: GET /templates?q=&category=&mine=1 lists the shelf the caller may see (no documents).
func (a *GalleryAPI) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		who := whoIs(r)
		q := r.URL.Query()
		list, err := a.Store.ListGalleryTemplates(r.Context(), store.GalleryFilter{
			Viewer: who.oid, Admin: false, Mine: q.Get("mine") == "1",
			Query: strings.TrimSpace(q.Get("q")), Category: q.Get("category"),
		})
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// Create: POST /templates puts a template on the shelf: a document, or a template of a project.
func (a *GalleryAPI) Create() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		who := whoIs(r)
		r.Body = http.MaxBytesReader(w, r.Body, MaxCanvasBytes+64<<10)
		var req galleryBody
		if !decodeBody(w, r, &req) {
			return
		}
		if msg := req.clean(); msg != "" {
			writeJSONError(w, http.StatusBadRequest, msg)
			return
		}
		raw := req.Template
		if req.ProjectID != "" {
			role, err := a.Team.callerRole(r, req.ProjectID)
			if isBadReference(err) {
				writeJSONError(w, http.StatusNotFound, "project not found")
				return
			}
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "db error")
				return
			}
			if role == "" || store.RoleRank(role) < store.RoleRank(store.RoleCoOwner) {
				writeJSONError(w, http.StatusForbidden, "sharing a template needs the co-owner role of the project")
				return
			}
			st, err := a.Stacks.GetStackTemplate(r.Context(), req.ProjectID, req.StackTemplateID)
			if errors.Is(err, pgx.ErrNoRows) || isBadReference(err) {
				writeJSONError(w, http.StatusNotFound, "stack template not found")
				return
			}
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "db error")
				return
			}
			raw = st.Template
			if req.Name == "" {
				req.Name = st.Name
			}
		}
		p, ok := prepare(w, raw, req.Name, req.Scope)
		if !ok {
			return
		}
		g, err := a.Store.CreateGalleryTemplate(r.Context(), store.GalleryTemplate{
			Name: p.tpl.Name, Description: req.Description, Category: req.Category, Author: who.oid, AuthorName: who.name,
			Scope: req.Scope, Status: statusFor(req.Scope, who), Template: p.doc, Modules: p.modules,
		})
		if errors.Is(err, store.ErrGalleryExists) {
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		a.audit(r, "gallery.template.create", g.ID, map[string]any{"name": g.Name, "scope": g.Scope, "status": g.Status})
		writeJSON(w, http.StatusCreated, g)
	}
}

// Get: GET /templates/{gid} returns a template with its document.
func (a *GalleryAPI) Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if g, _, ok := a.load(w, r); ok {
			writeJSON(w, http.StatusOK, g)
		}
	}
}

// Update: PUT /templates/{gid} (the author) edits the details and, when a template is sent, the
// document; the version goes up and a public template goes back to review.
func (a *GalleryAPI) Update() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g, who, ok := a.load(w, r)
		if !ok {
			return
		}
		if g.Author != who.oid {
			writeJSONError(w, http.StatusForbidden, "only the author edits a template")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, MaxCanvasBytes+64<<10)
		var req galleryBody
		if !decodeBody(w, r, &req) {
			return
		}
		if req.Scope == "" {
			req.Scope = g.Scope
		}
		if msg := req.clean(); msg != "" {
			writeJSONError(w, http.StatusBadRequest, msg)
			return
		}
		raw := req.Template
		if len(raw) == 0 {
			raw = g.Template
		}
		name := req.Name
		if name == "" {
			name = g.Name
		}
		p, ok := prepare(w, raw, name, req.Scope)
		if !ok {
			return
		}
		next := store.GalleryTemplate{Name: p.tpl.Name, Description: req.Description, Category: req.Category,
			Scope: req.Scope, Status: statusFor(req.Scope, who), Template: p.doc, Modules: p.modules}
		err := a.Store.UpdateGalleryTemplate(r.Context(), g.ID, next)
		if errors.Is(err, store.ErrGalleryExists) {
			writeJSONError(w, http.StatusConflict, err.Error())
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		a.audit(r, "gallery.template.update", g.ID, map[string]any{"name": next.Name, "scope": next.Scope, "status": next.Status})
		out, _ := a.Store.GetGalleryTemplate(r.Context(), g.ID)
		writeJSON(w, http.StatusOK, out)
	}
}

// Withdraw: POST /templates/{gid}/withdraw (the author) takes a template off the shelf; editing it
// puts it back (a public one through review again).
func (a *GalleryAPI) Withdraw() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g, who, ok := a.load(w, r)
		if !ok {
			return
		}
		if g.Author != who.oid {
			writeJSONError(w, http.StatusForbidden, "only the author withdraws a template")
			return
		}
		if err := a.Store.SetGalleryStatus(r.Context(), g.ID, store.GalleryWithdrawn, "", who.oid); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		a.audit(r, "gallery.template.withdraw", g.ID, map[string]any{"name": g.Name})
		w.WriteHeader(http.StatusNoContent)
	}
}

// Delete: DELETE /templates/{gid} (the author, or an admin) removes a template for good.
func (a *GalleryAPI) Delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g, who, ok := a.load(w, r)
		if !ok {
			return
		}
		if g.Author != who.oid && !who.admin {
			writeJSONError(w, http.StatusForbidden, "only the author or an admin removes a template")
			return
		}
		if err := a.Store.DeleteGalleryTemplate(r.Context(), g.ID); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		a.audit(r, "gallery.template.delete", g.ID, map[string]any{"name": g.Name})
		w.WriteHeader(http.StatusNoContent)
	}
}

// Fork: POST /templates/{gid}/fork copies a template the caller can see into their own private shelf.
func (a *GalleryAPI) Fork() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g, who, ok := a.load(w, r)
		if !ok {
			return
		}
		var last error
		for i := 0; i < 6; i++ {
			name := g.Name + " (copy)"
			if i > 0 {
				name = fmt.Sprintf("%s (copy %d)", g.Name, i+1)
			}
			if len(name) > stacktpl.MaxName {
				name = name[len(name)-stacktpl.MaxName:]
			}
			var t stacktpl.Template
			_ = json.Unmarshal(g.Template, &t)
			t.Name = name
			doc, _ := json.Marshal(t)
			c, err := a.Store.CreateGalleryTemplate(r.Context(), store.GalleryTemplate{
				Name: name, Description: g.Description, Category: g.Category, Author: who.oid, AuthorName: who.name,
				Scope: store.GalleryPrivate, Status: store.GalleryApproved, Template: doc, Modules: g.Modules,
			})
			if errors.Is(err, store.ErrGalleryExists) {
				last = err
				continue
			}
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "db error")
				return
			}
			a.audit(r, "gallery.template.fork", c.ID, map[string]any{"from": g.ID})
			writeJSON(w, http.StatusCreated, c)
			return
		}
		writeJSONError(w, http.StatusConflict, last.Error())
	}
}

// exportFile is the file a template is exported as.
type exportFile struct {
	Format      string          `json:"format"`
	Version     int             `json:"version"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Category    string          `json:"category,omitempty"`
	Template    json.RawMessage `json:"template"`
}

// Export: GET /templates/{gid}/export downloads a template as a JSON file.
func (a *GalleryAPI) Export() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g, _, ok := a.load(w, r)
		if !ok {
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="stack-template.json"`)
		writeJSON(w, http.StatusOK, exportFile{Format: ExportFormat, Version: 1, Name: g.Name, Description: g.Description, Category: g.Category, Template: g.Template})
	}
}

// ExportProject: GET /projects/{id}/stack-templates/{tid}/export downloads a project's template.
func (a *GalleryAPI) ExportProject() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, err := a.Stacks.GetStackTemplate(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "tid"))
		if errors.Is(err, pgx.ErrNoRows) || isBadReference(err) {
			writeJSONError(w, http.StatusNotFound, "stack template not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="stack-template.json"`)
		writeJSON(w, http.StatusOK, exportFile{Format: ExportFormat, Version: 1, Name: t.Name, Template: t.Template})
	}
}

type galleryImportBody struct {
	// GalleryID imports a template of the gallery; Document imports an exported file.
	GalleryID string          `json:"gallery_id"`
	Document  json.RawMessage `json:"document"`
	// Name replaces the template's name in the project.
	Name string `json:"name"`
}

// Import: POST /projects/{id}/stack-templates/import makes a template of this project from a gallery
// template or an exported file (owner and co-owner). The input is untrusted: it passes the gate, and
// every module it calls must exist in this project's catalog, otherwise the answer lists what is
// missing and nothing is stored. The result is a project template; applying it makes a draft.
func (a *GalleryAPI) Import() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.Team.need(w, r, store.RoleCoOwner); !ok {
			return
		}
		id := chi.URLParam(r, "id")
		who := whoIs(r)
		r.Body = http.MaxBytesReader(w, r.Body, MaxCanvasBytes+64<<10)
		var req galleryImportBody
		if !decodeBody(w, r, &req) {
			return
		}
		var raw json.RawMessage
		var galleryID string
		switch {
		case req.GalleryID != "" && len(req.Document) == 0:
			g, err := a.Store.GetGalleryTemplate(r.Context(), req.GalleryID)
			if errors.Is(err, pgx.ErrNoRows) || isBadReference(err) || (err == nil && !who.canSee(g)) {
				writeJSONError(w, http.StatusNotFound, "template not found")
				return
			}
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "db error")
				return
			}
			raw, galleryID = g.Template, g.ID
		case len(req.Document) > 0 && req.GalleryID == "":
			var f exportFile
			if json.Unmarshal(req.Document, &f) != nil || f.Format != ExportFormat || f.Version != 1 || len(f.Template) == 0 {
				writeJSONError(w, http.StatusBadRequest, "this is not a TerraFlow stack template file")
				return
			}
			raw = f.Template
		default:
			writeJSONError(w, http.StatusBadRequest, "send either gallery_id or document")
			return
		}
		// A file is held to the public standard: whoever wrote it, it may have come from anywhere.
		p, ok := prepare(w, raw, strings.TrimSpace(req.Name), store.GalleryPublic)
		if !ok {
			return
		}
		if cat, err := catalogFor(r.Context(), a.Catalog, id); err == nil {
			if missing := stacktpl.Missing(p.tpl, cat); len(missing) > 0 {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
					"error":   "this project's catalog does not have every module the template calls",
					"missing": missing,
				})
				return
			}
		}
		t, err := a.Stacks.CreateStackTemplate(r.Context(), id, p.tpl.Name, who.oid, p.doc)
		if errors.Is(err, store.ErrStackTemplateExists) {
			writeJSONError(w, http.StatusConflict, "this project already has a stack template with that name")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		if galleryID != "" {
			_ = a.Store.RecordGalleryImport(r.Context(), galleryID, id, who.oid)
		}
		a.Team.audit(r, "project.stack_template.import", id, map[string]any{"template": t.ID, "name": t.Name, "gallery": galleryID})
		writeJSON(w, http.StatusCreated, map[string]any{"id": t.ID, "name": t.Name, "template": p.tpl})
	}
}

// AdminList: GET /admin/templates?status=pending lists the shelf for an admin (everything, no documents).
func (a *GalleryAPI) AdminList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		who := whoIs(r)
		q := r.URL.Query()
		list, err := a.Store.ListGalleryTemplates(r.Context(), store.GalleryFilter{
			Viewer: who.oid, Admin: true, Status: q.Get("status"), Query: strings.TrimSpace(q.Get("q")), Category: q.Get("category"),
		})
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		writeJSON(w, http.StatusOK, list)
	}
}

type reviewBody struct {
	Action string `json:"action"` // approve | reject | hide
	Note   string `json:"note"`
}

// Review: POST /admin/templates/{gid}/review approves, rejects or hides a public template. A
// rejection or a hide needs a note, which the author sees.
func (a *GalleryAPI) Review() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g, who, ok := a.load(w, r)
		if !ok {
			return
		}
		var req reviewBody
		if !decodeBody(w, r, &req) {
			return
		}
		req.Note = strings.TrimSpace(req.Note)
		if len(req.Note) > 500 {
			writeJSONError(w, http.StatusBadRequest, "the note has at most 500 characters")
			return
		}
		if g.Scope != store.GalleryPublic {
			writeJSONError(w, http.StatusConflict, "only a public template is reviewed")
			return
		}
		var to string
		switch req.Action {
		case "approve":
			if g.Status != store.GalleryPending && g.Status != store.GalleryHidden {
				writeJSONError(w, http.StatusConflict, "only a pending or hidden template is approved")
				return
			}
			to = store.GalleryApproved
		case "reject":
			if g.Status != store.GalleryPending {
				writeJSONError(w, http.StatusConflict, "only a pending template is rejected")
				return
			}
			to = store.GalleryRejected
		case "hide":
			if g.Status != store.GalleryApproved {
				writeJSONError(w, http.StatusConflict, "only an approved template is hidden")
				return
			}
			to = store.GalleryHidden
		default:
			writeJSONError(w, http.StatusBadRequest, "the action is approve, reject or hide")
			return
		}
		if to != store.GalleryApproved && req.Note == "" {
			writeJSONError(w, http.StatusBadRequest, "say why in a note; the author will read it")
			return
		}
		if err := a.Store.SetGalleryStatus(r.Context(), g.ID, to, req.Note, who.oid); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		a.audit(r, "gallery.template."+req.Action, g.ID, map[string]any{"name": g.Name, "author": g.Author, "note": req.Note})
		out, _ := a.Store.GetGalleryTemplate(r.Context(), g.ID)
		out.Template = nil
		writeJSON(w, http.StatusOK, out)
	}
}
