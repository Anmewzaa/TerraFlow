package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// ReviewBranchPrefix is what every branch TerraFlow makes starts with. A base branch must not
// be one of them: the app only ever creates these, it never reads from them as a base (8.8).
const ReviewBranchPrefix = "terraflow/"

// validBranchName is the part of git's ref rules the app relies on.
func validBranchName(b string) bool {
	if b == "" || len(b) > 200 || strings.ContainsAny(b, " ~^:?*[\\\x00") || strings.Contains(b, "..") ||
		strings.HasPrefix(b, "/") || strings.HasSuffix(b, "/") || strings.HasSuffix(b, ".lock") || strings.Contains(b, "//") {
		return false
	}
	for _, r := range b {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// BaseBranchAPI is the branch a project works on (8.8). Owner and co-owner only.
type BaseBranchAPI struct {
	Team *TeamAPI
	App  *github.App
}

func (b *BaseBranchAPI) source(r *http.Request, id string) (*github.Source, store.ProjectRepo, int, string) {
	if b.App == nil {
		return nil, store.ProjectRepo{}, http.StatusServiceUnavailable, "GitHub is not set up on this server"
	}
	pr, err := b.Team.Store.GetProjectRepo(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, pr, http.StatusConflict, "this project has no GitHub repository"
	}
	if err != nil {
		return nil, pr, http.StatusInternalServerError, "db error"
	}
	conn, err := b.Team.Store.GetConnection(r.Context(), pr.ConnectionID)
	if err != nil || conn.Status != store.ConnActive {
		return nil, pr, http.StatusConflict, "the GitHub connection of this project needs attention"
	}
	inst, err := strconv.ParseInt(conn.ExternalID, 10, 64)
	owner, repo, ok := strings.Cut(pr.RepoFullName, "/")
	if err != nil || !ok {
		return nil, pr, http.StatusConflict, "the repository of this project is not valid"
	}
	return &github.Source{App: b.App, InstallationID: inst, Owner: owner, Repo: repo, Path: pr.RepoPath}, pr, 0, ""
}

// Exists says whether a branch is in the project's repository.
func (b *BaseBranchAPI) Exists(r *http.Request, projectID, branch string) (bool, error) {
	src, _, _, msg := b.source(r, projectID)
	if src == nil {
		return false, errors.New(msg)
	}
	return src.BranchExists(r.Context(), branch)
}

// List: GET /projects/{id}/branches lists the repository's real branches, without the
// branches TerraFlow made itself.
func (b *BaseBranchAPI) List() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := b.Team.need(w, r, store.RoleCoOwner); !ok {
			return
		}
		src, pr, code, msg := b.source(r, chi.URLParam(r, "id"))
		if src == nil {
			writeJSONError(w, code, msg)
			return
		}
		names, truncated, err := src.Branches(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, "could not list the branches on GitHub")
			return
		}
		out := []string{}
		for _, n := range names {
			if !strings.HasPrefix(n, ReviewBranchPrefix) {
				out = append(out, n)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"branches": out, "truncated": truncated, "default_branch": pr.DefaultBranch, "base_branch": pr.BaseBranch,
		})
	}
}

// Set: PUT /projects/{id}/base-branch {"branch": "..."} ("" goes back to the default). The
// branch must exist and must not be a review branch. It is only read: Approve still makes a
// new branch and never commits to this one.
func (b *BaseBranchAPI) Set() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := b.Team.need(w, r, store.RoleCoOwner); !ok {
			return
		}
		var req struct {
			Branch string `json:"branch"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		id := chi.URLParam(r, "id")
		branch := strings.TrimSpace(req.Branch)
		if branch != "" {
			if !validBranchName(branch) {
				writeJSONError(w, http.StatusUnprocessableEntity, "that is not a valid branch name")
				return
			}
			if strings.HasPrefix(branch, ReviewBranchPrefix) {
				writeJSONError(w, http.StatusUnprocessableEntity, "a TerraFlow review branch cannot be the base branch")
				return
			}
		}
		src, pr, code, msg := b.source(r, id)
		if src == nil {
			writeJSONError(w, code, msg)
			return
		}
		if branch == pr.DefaultBranch {
			branch = "" // the same as the default: keep it following the default
		}
		if branch != "" {
			ok, err := src.BranchExists(r.Context(), branch)
			switch {
			case errors.Is(err, gitprovider.ErrUnauthorized):
				writeJSONError(w, http.StatusConflict, "the GitHub connection of this project needs attention")
				return
			case err != nil:
				writeJSONError(w, http.StatusBadGateway, "could not check the branch on GitHub")
				return
			case !ok:
				writeJSONError(w, http.StatusUnprocessableEntity, "that branch does not exist in the repository")
				return
			}
		}
		if err := b.Team.Store.SetBaseBranch(r.Context(), id, branch); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		b.Team.audit(r, "project.base_branch", id, map[string]any{"base_branch": branch})
		b.Team.Get()(w, r)
	}
}
