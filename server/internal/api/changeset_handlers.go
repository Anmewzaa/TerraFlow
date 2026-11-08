package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
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

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}
		if !json.Valid(body) {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
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
			http.Error(w, `{"error":"changeset not found or not draft"}`, http.StatusNotFound)
			return
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
