package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"crypto/rand"
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// MemChangesetStore is an in-memory store for NoDB/local dev mode.
type MemChangesetStore struct {
	mu   sync.RWMutex
	data map[string]store.Changeset // keyed by changeset id
	// Audit, when set, receives publish events.
	Audit *MemReviewStore
}

func NewMemChangesetStore() *MemChangesetStore {
	return &MemChangesetStore{data: make(map[string]store.Changeset)}
}

func MemChangesetListHandler(s *MemChangesetStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		s.mu.RLock()
		defer s.mu.RUnlock()
		var list []store.Changeset
		for _, cs := range s.data {
			if cs.ProjectID == projectID {
				list = append(list, cs)
			}
		}
		if list == nil {
			list = []store.Changeset{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	}
}

func MemChangesetCreateHandler(s *MemChangesetStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
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

		id := newID()
		now := time.Now()
		cs := store.Changeset{
			ID:         id,
			ProjectID:  projectID,
			Env:        body.Env,
			UserOID:    "dev-user",
			Title:      body.Title,
			Status:     store.ChangesetDraft,
			CanvasJSON: json.RawMessage(`{"nodes":[],"edges":[]}`),
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		s.mu.Lock()
		s.data[id] = cs
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	}
}

func MemChangesetGetHandler(s *MemChangesetStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csID := chi.URLParam(r, "csId")
		s.mu.RLock()
		cs, ok := s.data[csID]
		s.mu.RUnlock()
		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cs)
	}
}

func MemChangesetSaveHandler(s *MemChangesetStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csID := chi.URLParam(r, "csId")
		body, err := io.ReadAll(r.Body)
		if err != nil || !json.Valid(body) {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		cs, ok := s.data[csID]
		if ok {
			cs.CanvasJSON = body
			cs.UpdatedAt = time.Now()
			s.data[csID] = cs
		}
		s.mu.Unlock()

		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
	}
}

// MemChangesetSaveVersionHandler snapshots a draft as a saved version.
func MemChangesetSaveVersionHandler(s *MemChangesetStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csID := chi.URLParam(r, "csId")
		var body struct {
			Title string `json:"title"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		s.mu.Lock()
		d, ok := s.data[csID]
		if !ok || d.Status != store.ChangesetDraft {
			s.mu.Unlock()
			writeJSONError(w, http.StatusNotFound, "changeset not found or not a draft")
			return
		}
		title := body.Title
		if title == "" {
			n := 1
			for _, o := range s.data {
				if o.ProjectID == d.ProjectID && o.Env == d.Env && o.UserOID == d.UserOID && o.Status != store.ChangesetDraft {
					n++
				}
			}
			title = fmt.Sprintf("Version %d", n)
		}
		id := newID()
		now := time.Now()
		s.data[id] = store.Changeset{
			ID: id, ProjectID: d.ProjectID, Env: d.Env, UserOID: d.UserOID, Title: title,
			Status: store.ChangesetSaved, CanvasJSON: d.CanvasJSON, BaseCanvas: d.BaseCanvas,
			CreatedAt: now, UpdatedAt: now,
		}
		s.mu.Unlock()
		writeJSON(w, http.StatusCreated, map[string]string{"id": id, "title": title})
	}
}

func MemChangesetPublishHandler(s *MemChangesetStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csID := chi.URLParam(r, "csId")
		var body struct {
			Title string `json:"title"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		s.mu.Lock()
		cs, ok := s.data[csID]
		if ok {
			for _, o := range s.data {
				if o.ProjectID == cs.ProjectID && o.Status == store.ChangesetPublished && o.ID != cs.ID {
					s.mu.Unlock()
					writePublishError(w, &store.PendingReviewError{ID: o.ID, Title: o.Title})
					return
				}
			}
			cs.Status = store.ChangesetPublished
			cs.Title = body.Title
			cs.UpdatedAt = time.Now()
			s.data[csID] = cs
		}
		s.mu.Unlock()

		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		if s.Audit != nil {
			s.Audit.LogAudit(r.Context(), actor(r), "changeset.publish", "changeset:"+csID,
				map[string]any{"project_id": cs.ProjectID, "changeset_id": csID, "title": body.Title})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "published"})
	}
}

// GetChangeset and CreateRevision make MemChangesetStore a ChangesetSource.
func (s *MemChangesetStore) GetChangeset(_ context.Context, id string) (store.Changeset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.data[id]
	if !ok {
		return store.Changeset{}, pgx.ErrNoRows
	}
	return c, nil
}

func (s *MemChangesetStore) CreateRevision(_ context.Context, src store.Changeset, userOID string) (string, error) {
	id := newID()
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = store.Changeset{
		ID: id, ProjectID: src.ProjectID, Env: src.Env, UserOID: userOID, Title: src.Title,
		Status: store.ChangesetDraft, CanvasJSON: src.CanvasJSON, BaseCanvas: src.CanvasJSON,
		CreatedAt: now, UpdatedAt: now,
	}
	return id, nil
}

func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func MemChangesetDeleteHandler(s *MemChangesetStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csID := chi.URLParam(r, "csId")
		s.mu.Lock()
		delete(s.data, csID)
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}
}
