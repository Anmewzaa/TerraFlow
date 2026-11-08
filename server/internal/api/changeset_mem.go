package api

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"crypto/rand"
	"fmt"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// MemChangesetStore is an in-memory store for NoDB/local dev mode.
type MemChangesetStore struct {
	mu   sync.RWMutex
	data map[string]store.Changeset // keyed by changeset id
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "published"})
	}
}

func MemChangesetPreviewHandler(s *MemChangesetStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csID := chi.URLParam(r, "csId")
		s.mu.RLock()
		cs, ok := s.data[csID]
		s.mu.RUnlock()
		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}

		var canvas store.CanvasState
		if err := json.Unmarshal(cs.CanvasJSON, &canvas); err != nil {
			http.Error(w, `{"error":"invalid canvas"}`, http.StatusInternalServerError)
			return
		}

		genNodes := make([]hcl.GenerateNode, len(canvas.Nodes))
		for i, cn := range canvas.Nodes {
			genNodes[i] = hcl.GenerateNode{
				ID:            cn.ID,
				ModuleName:    cn.ModuleName,
				ModuleVersion: cn.ModuleVersion,
				InstanceName:  cn.InstanceName,
				TargetFolder:  cn.TargetFolder,
				Source:        buildModuleSource(cn.ModuleName, cn.ModuleVersion),
				Values:        cn.Values,
			}
		}

		out, err := hcl.Generate(hcl.GenerateInput{Nodes: genNodes})
		if err != nil {
			http.Error(w, `{"error":"generation failed"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
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
