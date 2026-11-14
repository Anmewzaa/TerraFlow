package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// DriftStore is what repository drift keeps per project (8.14). *store.Queries implements it.
type DriftStore interface {
	SetDriftBaseline(ctx context.Context, projectID string, files map[string]string) error
	GetDriftBaseline(ctx context.Context, projectID string) (*store.DriftBaseline, error)
	DismissDrift(ctx context.Context, projectID, key, note, by string) error
	RestoreDrift(ctx context.Context, projectID, key string) error
	ListDriftDismissals(ctx context.Context, projectID string) ([]store.Dismissal, error)
}

func hashContent(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// manifestOf is the files (path in the repository -> short hash) a project is made of.
func manifestOf(files []hcl.GeneratedFile) map[string]string {
	m := make(map[string]string, len(files))
	for _, f := range files {
		m[joinPath(f.Folder, f.Filename)] = hashContent(f.Content)
	}
	return m
}

// manifestOfContents is manifestOf for a path -> content map (what a plan was built on).
func manifestOfContents(repo map[string]string) map[string]string {
	m := make(map[string]string, len(repo))
	for p, c := range repo {
		m[p] = hashContent(c)
	}
	return m
}

// DriftAPI reports where the repository has moved away from what TerraFlow knows (8.14). It is not
// infrastructure drift: nothing here asks Azure (hard rule 2).
type DriftAPI struct {
	Team     *TeamAPI
	Store    DriftStore
	Project  func(ctx context.Context, id string) (store.Project, error)
	Repos    Repos
	Settings SettingsLoader
}

type driftSeen struct {
	hcl.DriftItem
	Note string `json:"note"`
	By   string `json:"by"`
}

// List: GET /projects/{id}/drift?env= lists what is worth a look: blocks the canvas cannot show,
// values that differ from the project variables, and files changed since the baseline. Items a
// person dismissed are listed apart, with their note.
func (d *DriftAPI) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		p, err := d.Project(r.Context(), id)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "project not found")
			return
		}
		env := envOf(r)
		snap, err := snapshotOf(r.Context(), d.Repos, p)
		if err != nil {
			writeJSONError(w, providerStatus(err), err.Error())
			return
		}
		st := d.Settings.of(r.Context(), id)
		view, err := viewOf(snap, st, env)
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		items := []hcl.DriftItem{}
		if un, err := hcl.FindUnrepresented(view.FS); err == nil {
			items = append(items, un...)
		}
		if g, err := hcl.BuildGraphFS(view.FS, graphName(p), env); err == nil {
			items = append(items, hcl.VariableDrift(g, st, env)...)
		}
		current := manifestOf(realFiles(st.Resolver(env), repoFilesOf(r.Context(), d.Repos, p, st, env)))
		base, err := d.Store.GetDriftBaseline(r.Context(), id)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		out := map[string]any{"env": env, "has_baseline": base != nil}
		if base != nil {
			items = append(items, hcl.DiffManifests(base.Files, current)...)
			out["baseline_at"] = base.At
		}
		dismissals, err := d.Store.ListDriftDismissals(r.Context(), id)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		hidden := map[string]store.Dismissal{}
		for _, x := range dismissals {
			hidden[x.Key] = x
		}
		shown, dismissed := []hcl.DriftItem{}, []driftSeen{}
		for _, it := range items {
			if x, ok := hidden[it.Key]; ok {
				dismissed = append(dismissed, driftSeen{DriftItem: it, Note: x.Note, By: x.By})
			} else {
				shown = append(shown, it)
			}
		}
		out["items"], out["dismissed"] = shown, dismissed
		writeJSON(w, http.StatusOK, out)
	}
}

// Dismiss: POST /projects/{id}/drift/dismiss {key, note} hides an item and keeps the note.
// DELETE /projects/{id}/drift/dismiss?key= shows it again. Owner and co-owner only.
func (d *DriftAPI) Dismiss() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := d.Team.need(w, r, store.RoleCoOwner); !ok {
			return
		}
		id := chi.URLParam(r, "id")
		if r.Method == http.MethodDelete {
			key := r.URL.Query().Get("key")
			if key == "" || d.Store.RestoreDrift(r.Context(), id, key) != nil {
				writeJSONError(w, http.StatusBadRequest, "nothing to restore")
				return
			}
			d.Team.audit(r, "project.drift.restore", id, map[string]any{"key": key})
			w.WriteHeader(http.StatusNoContent)
			return
		}
		var req struct {
			Key  string `json:"key"`
			Note string `json:"note"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		req.Key, req.Note = strings.TrimSpace(req.Key), strings.TrimSpace(req.Note)
		if req.Key == "" || len(req.Key) > 400 || len(req.Note) > 500 {
			writeJSONError(w, http.StatusBadRequest, "a key and a note of at most 500 characters")
			return
		}
		if err := d.Store.DismissDrift(r.Context(), id, req.Key, req.Note, actor(r)); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		d.Team.audit(r, "project.drift.dismiss", id, map[string]any{"key": req.Key})
		w.WriteHeader(http.StatusNoContent)
	}
}

// Baseline: POST /projects/{id}/drift/baseline?env= accepts the base branch as it is now: later
// drift is measured from here. Owner and co-owner only.
func (d *DriftAPI) Baseline() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := d.Team.need(w, r, store.RoleCoOwner); !ok {
			return
		}
		id := chi.URLParam(r, "id")
		p, err := d.Project(r.Context(), id)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "project not found")
			return
		}
		env := envOf(r)
		st := d.Settings.of(r.Context(), id)
		current := manifestOf(realFiles(st.Resolver(env), repoFilesOf(r.Context(), d.Repos, p, st, env)))
		if err := d.Store.SetDriftBaseline(r.Context(), id, current); err != nil {
			writeJSONError(w, http.StatusConflict, "this project has no repository to take a baseline of")
			return
		}
		d.Team.audit(r, "project.drift.baseline", id, map[string]any{"files": len(current)})
		w.WriteHeader(http.StatusNoContent)
	}
}
