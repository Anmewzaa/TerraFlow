package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/stacktpl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/validate"
)

// StackStore is what stack templates need from the database. *store.Queries implements it.
type StackStore interface {
	CreateStackTemplate(ctx context.Context, projectID, name, createdBy string, doc json.RawMessage) (store.StackTemplate, error)
	ListStackTemplates(ctx context.Context, projectID string) ([]store.StackTemplate, error)
	GetStackTemplate(ctx context.Context, projectID, id string) (store.StackTemplate, error)
	DeleteStackTemplate(ctx context.Context, projectID, id string) error
}

// StackAPI is a project's stack templates (8.15): made from a selection on the canvas, applied to
// any environment through the normal draft, so validate, review and approve still decide.
type StackAPI struct {
	Team     *TeamAPI
	Store    StackStore
	Catalog  moduleSource
	Project  func(ctx context.Context, id string) (store.Project, error)
	Repos    Repos
	Settings SettingsLoader
}

func (a *StackAPI) secretFn(ctx context.Context, projectID string) stacktpl.SecretFn {
	cat, err := catalogFor(ctx, a.Catalog, projectID)
	if err != nil || cat == nil {
		return nil
	}
	return func(module, field string) bool {
		// Any version of the module that marks the input as a secret makes it one.
		for _, v := range []string{"v1", "v2", "v3"} {
			m, ok := cat.Module(module, v)
			if !ok {
				continue
			}
			var s struct {
				Properties map[string]struct {
					UI struct {
						Secret bool `json:"secret"`
					} `json:"x-tf-ui"`
				} `json:"properties"`
			}
			if json.Unmarshal(m.Schema, &s) == nil && s.Properties[field].UI.Secret {
				return true
			}
		}
		return false
	}
}

type createStackBody struct {
	Name     string           `json:"name"`
	Canvas   json.RawMessage  `json:"canvas"`
	Selected []string         `json:"selected"`
	Params   []stacktpl.Param `json:"params"`
}

// Create: POST /projects/{id}/stack-templates makes a template from the selected nodes of a canvas
// (owner and co-owner).
func (a *StackAPI) Create() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.Team.need(w, r, store.RoleCoOwner); !ok {
			return
		}
		id := chi.URLParam(r, "id")
		r.Body = http.MaxBytesReader(w, r.Body, MaxCanvasBytes)
		var req createStackBody
		if !decodeBody(w, r, &req) {
			return
		}
		c, err := validate.Parse(req.Canvas)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if fs := validate.Structural(c); validate.HasStructural(fs) {
			writeFindings(w, http.StatusUnprocessableEntity, "the canvas has input that cannot be saved", fs)
			return
		}
		tpl, problems := stacktpl.Build(req.Name, c, req.Selected, req.Params, a.secretFn(r.Context(), id))
		if len(problems) > 0 {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "the template cannot be made", "problems": problems})
			return
		}
		doc, _ := json.Marshal(tpl)
		t, err := a.Store.CreateStackTemplate(r.Context(), id, tpl.Name, actor(r), doc)
		switch {
		case errors.Is(err, store.ErrStackTemplateExists):
			writeJSONError(w, http.StatusConflict, "this project already has a stack template with that name")
			return
		case err != nil:
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		a.Team.audit(r, "project.stack_template.create", id, map[string]any{"template": t.ID, "name": t.Name, "nodes": len(tpl.Nodes)})
		writeJSON(w, http.StatusCreated, map[string]any{"id": t.ID, "name": t.Name, "template": tpl})
	}
}

// List: GET /projects/{id}/stack-templates lists the project's templates (any member).
func (a *StackAPI) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := a.Store.ListStackTemplates(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		if list == nil {
			list = []store.StackTemplate{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// Get: GET /projects/{id}/stack-templates/{tid} returns a template with its document.
func (a *StackAPI) Get() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, err := a.Store.GetStackTemplate(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "tid"))
		if errors.Is(err, pgx.ErrNoRows) || isBadReference(err) {
			writeJSONError(w, http.StatusNotFound, "stack template not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		writeJSON(w, http.StatusOK, t)
	}
}

// Delete: DELETE /projects/{id}/stack-templates/{tid} (owner and co-owner). Resources already made from
// it are not touched.
func (a *StackAPI) Delete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.Team.need(w, r, store.RoleCoOwner); !ok {
			return
		}
		id := chi.URLParam(r, "id")
		err := a.Store.DeleteStackTemplate(r.Context(), id, chi.URLParam(r, "tid"))
		if errors.Is(err, pgx.ErrNoRows) || isBadReference(err) {
			writeJSONError(w, http.StatusNotFound, "stack template not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		a.Team.audit(r, "project.stack_template.delete", id, map[string]any{"template": chi.URLParam(r, "tid")})
		w.WriteHeader(http.StatusNoContent)
	}
}

type applyStackBody struct {
	Env    string            `json:"env"`
	Prefix string            `json:"prefix"`
	Suffix string            `json:"suffix"`
	Slots  map[string]string `json:"slots"`
	// Values is the value of each parameter per environment; a missing one takes the default.
	Values map[string]map[string]string `json:"values"`
	// DraftLabels are the "folder/instance" names already on the canvas of this environment.
	DraftLabels []string `json:"draft_labels"`
	OffsetX     float64  `json:"offset_x"`
	OffsetY     float64  `json:"offset_y"`
}

// Apply: POST /projects/{id}/stack-templates/{tid}/apply makes the nodes of a template for one
// environment. It writes nothing: the screen adds them to that environment's draft, and the project
// variables for the parameters come back as a patch to save with the project's settings. A name that
// clashes with the draft or the repository is refused (D-036).
func (a *StackAPI) Apply() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req applyStackBody
		if !decodeBody(w, r, &req) {
			return
		}
		if !hcl.ValidEnv(req.Env) {
			writeJSONError(w, http.StatusBadRequest, "choose an environment")
			return
		}
		t, err := a.Store.GetStackTemplate(r.Context(), id, chi.URLParam(r, "tid"))
		if errors.Is(err, pgx.ErrNoRows) || isBadReference(err) {
			writeJSONError(w, http.StatusNotFound, "stack template not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		var tpl stacktpl.Template
		if json.Unmarshal(t.Template, &tpl) != nil {
			writeJSONError(w, http.StatusInternalServerError, "the template cannot be read")
			return
		}
		taken := map[string]bool{}
		for _, l := range req.DraftLabels {
			taken[l] = true
		}
		if a.Project != nil && a.Repos != nil {
			if p, err := a.Project(r.Context(), id); err == nil {
				if snap, err := snapshotOf(r.Context(), a.Repos, p); err == nil {
					st := a.Settings.of(r.Context(), id)
					if view, err := viewOf(snap, st, req.Env); err == nil {
						if g, err := hcl.BuildGraphFS(view.FS, graphName(p), req.Env); err == nil {
							for _, f := range g.Folders {
								for _, n := range f.Nodes {
									if n.Kind == hcl.KindModule {
										taken[f.Name+"/"+n.Label] = true
									}
								}
							}
						}
					}
				}
			}
		}
		stamp := time.Now().UnixMilli()
		nodes, problems := stacktpl.Instantiate(tpl, stacktpl.Input{
			Env: req.Env, Prefix: req.Prefix, Suffix: req.Suffix, Slots: req.Slots, ExistingLabels: taken,
			OffsetX: req.OffsetX, OffsetY: req.OffsetY,
			NewID: func(i int) string { return fmt.Sprintf("draft-%d-%d", stamp, i) },
		})
		if len(problems) > 0 {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "the template cannot be applied", "problems": problems})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"nodes": nodes, "env": req.Env,
			"variables": stacktpl.Variables(tpl, []string{req.Env}, req.Values),
		})
	}
}
