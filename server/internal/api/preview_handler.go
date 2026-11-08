package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

func ChangesetPreviewHandler(q *store.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		csID := chi.URLParam(r, "csId")

		cs, err := q.GetChangeset(r.Context(), csID)
		if err != nil {
			http.Error(w, `{"error":"changeset not found"}`, http.StatusNotFound)
			return
		}

		var canvas store.CanvasState
		if err := json.Unmarshal(cs.CanvasJSON, &canvas); err != nil {
			http.Error(w, `{"error":"invalid canvas data"}`, http.StatusInternalServerError)
			return
		}

		genNodes := make([]hcl.GenerateNode, len(canvas.Nodes))
		for i, cn := range canvas.Nodes {
			var pickRefs []hcl.PickRef
			if pr, ok := cn.Values["_pick_refs"]; ok {
				if prBytes, err := json.Marshal(pr); err == nil {
					json.Unmarshal(prBytes, &pickRefs)
				}
				delete(cn.Values, "_pick_refs")
			}

			genNodes[i] = hcl.GenerateNode{
				ID:            cn.ID,
				ModuleName:    cn.ModuleName,
				ModuleVersion: cn.ModuleVersion,
				InstanceName:  cn.InstanceName,
				TargetFolder:  cn.TargetFolder,
				Source:        buildModuleSource(cn.ModuleName, cn.ModuleVersion),
				Values:        cn.Values,
				PickRefs:      pickRefs,
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

func buildModuleSource(moduleName, version string) string {
	return "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//" + moduleName + "?ref=" + version
}
