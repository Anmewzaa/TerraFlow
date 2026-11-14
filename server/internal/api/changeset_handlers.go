package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func ChangesetListHandler(q *store.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		u := auth.UserFromContext(r.Context())
		userOID := ""
		if u != nil {
			userOID = u.OID
		}
		list, err := q.ListChangesets(r.Context(), projectID, userOID)
		if err != nil {
			http.Error(w, `{"error":"failed to list changesets"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	}
}

func ChangesetGetHandler(q *store.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csID := chi.URLParam(r, "csId")
		cs, err := q.GetChangeset(r.Context(), csID)
		if err != nil {
			http.Error(w, `{"error":"changeset not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cs)
	}
}

func ChangesetCreateHandler(q *store.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		u := auth.UserFromContext(r.Context())
		userOID := ""
		if u != nil {
			userOID = u.OID
		}

		var body struct {
			Env   string `json:"env"`
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}
		if body.Env == "" {
			body.Env = "dev"
		}
		if !hcl.ValidEnv(body.Env) {
			writeJSONError(w, http.StatusBadRequest, "invalid environment name")
			return
		}

		cs := store.Changeset{
			ProjectID:  projectID,
			Env:        body.Env,
			UserOID:    userOID,
			Title:      body.Title,
			CanvasJSON: json.RawMessage(`{"nodes":[],"edges":[]}`),
		}
		id, err := q.CreateChangeset(r.Context(), cs)
		if err != nil {
			http.Error(w, `{"error":"failed to create changeset"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	}
}

func ChangesetSaveHandler(q *store.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csID := chi.URLParam(r, "csId")

		body, c, ok := readCanvasBody(w, r)
		if !ok {
			return
		}
		if refuseUnsafeCanvas(w, c) {
			return
		}

		if err := q.UpdateChangesetCanvas(r.Context(), csID, body); err != nil {
			http.Error(w, `{"error":"changeset not found or not draft"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
	}
}

// ChangesetSaveVersionHandler snapshots the working draft as a saved version.
func ChangesetSaveVersionHandler(q *store.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csID := chi.URLParam(r, "csId")
		var body struct {
			Title string `json:"title"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body) // the title is optional
		id, title, err := q.SaveChangesetVersion(r.Context(), csID, body.Title)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "changeset not found or not a draft")
			return
		}
		if u := auth.UserFromContext(r.Context()); u != nil {
			q.LogAudit(r.Context(), u.OID, "changeset.save", "changeset:"+id,
				map[string]any{"project_id": chi.URLParam(r, "id"), "changeset_id": id, "draft_id": csID, "title": title})
		}
		writeJSON(w, http.StatusCreated, map[string]string{"id": id, "title": title})
	}
}

func ChangesetPublishHandler(q *store.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csID := chi.URLParam(r, "csId")

		var body struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}

		if err := q.PublishChangeset(r.Context(), csID, body.Title); err != nil {
			writePublishError(w, err)
			return
		}
		if u := auth.UserFromContext(r.Context()); u != nil {
			q.LogAudit(r.Context(), u.OID, "changeset.publish", "changeset:"+csID,
				map[string]any{"project_id": chi.URLParam(r, "id"), "changeset_id": csID, "title": body.Title})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "published"})
	}
}

func ChangesetDeleteHandler(q *store.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csID := chi.URLParam(r, "csId")
		if err := q.DeleteChangeset(r.Context(), csID); err != nil {
			http.Error(w, `{"error":"failed to delete"}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// writePublishError: 409 with the changeset that is in the way, else 404.
func writePublishError(w http.ResponseWriter, err error) {
	var pend *store.PendingReviewError
	if errors.As(err, &pend) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":   pend.Error(),
			"pending": map[string]string{"id": pend.ID, "title": pend.Title},
		})
		return
	}
	writeJSONError(w, http.StatusNotFound, "changeset not found or already sent for review")
}
