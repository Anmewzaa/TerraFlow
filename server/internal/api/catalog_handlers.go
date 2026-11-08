package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/catalog"
)

func CatalogListHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summaries, err := svc.ListModuleSummaries(r.Context())
		if err != nil {
			http.Error(w, `{"error":"failed to list catalog"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(summaries)
	}
}

func CatalogGetHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		version := chi.URLParam(r, "version")
		m, err := svc.GetModule(r.Context(), name, version)
		if err != nil {
			http.Error(w, `{"error":"module not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m)
	}
}

func DraftsListHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		u := auth.UserFromContext(r.Context())
		userOID := ""
		if u != nil {
			userOID = u.OID
		}
		drafts, err := svc.ListDrafts(r.Context(), projectID, userOID)
		if err != nil {
			http.Error(w, `{"error":"failed to list drafts"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(drafts)
	}
}

func DraftGetHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		moduleID := chi.URLParam(r, "moduleId")
		u := auth.UserFromContext(r.Context())
		userOID := ""
		if u != nil {
			userOID = u.OID
		}
		draft, err := svc.GetDraft(r.Context(), projectID, moduleID, userOID)
		if err != nil {
			http.Error(w, `{"error":"draft not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(draft)
	}
}

func DraftSaveHandler(svc *catalog.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID := chi.URLParam(r, "id")
		moduleID := chi.URLParam(r, "moduleId")
		u := auth.UserFromContext(r.Context())
		userOID := ""
		if u != nil {
			userOID = u.OID
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}

		if !json.Valid(body) {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}

		d := catalog.Draft{
			ProjectID:  projectID,
			ModuleID:   moduleID,
			UserOID:    userOID,
			ValuesJSON: body,
		}
		if err := svc.SaveDraft(r.Context(), d); err != nil {
			http.Error(w, `{"error":"failed to save draft"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
	}
}
