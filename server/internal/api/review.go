package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/hcl"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// ReviewStore is what the review flow needs from persistence. *store.Queries
// implements it for Postgres; MemReviewStore wraps the NoDB in-memory store.
type ReviewStore interface {
	GetChangeset(ctx context.Context, id string) (store.Changeset, error)
	ReviewChangeset(ctx context.Context, id string, status store.ChangesetStatus,
		reviewerOID, reason, branch, commit string) error
	LogAudit(ctx context.Context, userOID, action, resource string, detail any) error
	ListAudit(ctx context.Context, projectID string, limit int) ([]store.AuditEntry, error)
}

type Reviewer struct {
	Store   ReviewStore
	Project func(ctx context.Context, id string) (store.Project, error)
	// Settings loads a project's variables and starter files (nil: none).
	Settings SettingsLoader
	// Repos reads a project's repository and creates its review branch.
	Repos  Repos
	Logger *slog.Logger
	// SeparationOfDuties forbids the requester from approving their own changeset.
	SeparationOfDuties bool
	// Drift, when set, records the files the base branch had at each approval (8.14).
	Drift interface {
		SetDriftBaseline(ctx context.Context, projectID string, files map[string]string) error
	}
	// Roles, when set, lets a project's owner and co-owners review and applies the
	// project's "second reviewer" switch. Nil in the no-database mode.
	Roles interface {
		MemberRole(ctx context.Context, projectID, oid string) (string, error)
		GetProjectRepo(ctx context.Context, projectID string) (store.ProjectRepo, error)
	}
}

func isDevOps(u *auth.User) bool {
	if u == nil {
		return false
	}
	for _, r := range u.Roles {
		if r == "devops" {
			return true
		}
	}
	return false
}

func actor(r *http.Request) string {
	if u := auth.UserFromContext(r.Context()); u != nil {
		return u.OID
	}
	return "unknown"
}

// load fetches the changeset and project and checks they belong together.
func (rv *Reviewer) load(w http.ResponseWriter, r *http.Request) (store.Changeset, store.Project, bool) {
	cs, err := rv.Store.GetChangeset(r.Context(), chi.URLParam(r, "csId"))
	if err != nil || cs.ProjectID != chi.URLParam(r, "id") {
		writeJSONError(w, http.StatusNotFound, "changeset not found")
		return cs, store.Project{}, false
	}
	p, err := rv.Project(r.Context(), cs.ProjectID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "project not found")
		return cs, p, false
	}
	return cs, p, true
}

// Planned generation shared by the diff view and approve.
type plan struct {
	// Files are the files the canvas creates or changes, each with its complete
	// final content merged into what the repository already has.
	Files  []hcl.ChangedFile
	Branch string
	// Repo is what the project folder contained when the plan was made.
	Repo map[string]string
	// Base is the commit the plan was made on ("" for a folder without a repository).
	Base string
}

func branchName(p store.Project, cs store.Changeset) string {
	short := slugify(cs.ID)
	if len(short) > 8 {
		short = short[:8]
	}
	return fmt.Sprintf("terraflow/%s/%s-%s", slugify(p.Slug), slugify(cs.Env), short)
}

// planError marks a plan that failed because the repository could not be read, not
// because the canvas is wrong, so the caller can answer with the provider's status.
type planError struct{ err error }

func (e *planError) Error() string { return "could not read the repository: " + e.err.Error() }
func (e *planError) Unwrap() error { return e.err }

// planStatus is the HTTP status for a plan error.
func planStatus(err error) int {
	var pe *planError
	if errors.As(err, &pe) {
		return providerStatus(pe.err)
	}
	return http.StatusUnprocessableEntity
}

// canvasInput turns saved canvas JSON into generator input. projectID decides which catalog a
// module name is looked up in (the project's own, then the global one; D-060).
func canvasInput(projectID string, raw json.RawMessage) (hcl.GenerateInput, store.CanvasState, error) {
	var canvas store.CanvasState
	if err := json.Unmarshal(raw, &canvas); err != nil {
		return hcl.GenerateInput{}, canvas, fmt.Errorf("invalid canvas data: %w", err)
	}
	nodes := make([]hcl.GenerateNode, len(canvas.Nodes))
	for i, cn := range canvas.Nodes {
		var pickRefs []hcl.PickRef
		if pr, ok := cn.Values["_pick_refs"]; ok {
			if b, err := json.Marshal(pr); err == nil {
				json.Unmarshal(b, &pickRefs)
			}
			delete(cn.Values, "_pick_refs")
		}
		nodes[i] = hcl.GenerateNode{
			ID: cn.ID, ModuleName: cn.ModuleName, ModuleVersion: cn.ModuleVersion,
			InstanceName: cn.InstanceName, TargetFolder: cn.TargetFolder,
			Source: buildModuleSource(projectID, cn.ModuleName, cn.ModuleVersion),
			Values: cn.Values, PickRefs: pickRefs,
			ParentID: cn.ParentID, NestField: cn.NestField, Filename: cn.Filename,
			Kind: cn.Kind, ResourceType: cn.ResourceType,
		}
		if cn.Kind == hcl.KindResourceNode {
			nodes[i].Source = "" // a raw resource has no module source
		}
		if cn.Existing != nil {
			// An edit keeps the source line the repository already has.
			nodes[i].Source = ""
			nodes[i].Existing = &hcl.ExistingRef{
				Folder: cn.Existing.Folder, Filename: cn.Existing.Filename, Label: cn.Existing.Label,
			}
		}
	}
	return hcl.GenerateInput{Nodes: nodes}, canvas, nil
}

// applyCanvas renders the canvas against the repository's files. It never
// replaces a file: new resources are appended, edits replace one block.
//
// repo is in the canonical shape (viewOf); the files that come back are where the layout puts them
// in the repository (8.9).
func applyCanvas(projectID string, raw json.RawMessage, repo []hcl.GeneratedFile, settings *hcl.ProjectSettings, env string) ([]hcl.ChangedFile, store.CanvasState, error) {
	in, canvas, err := canvasInput(projectID, raw)
	if err != nil {
		return nil, canvas, err
	}
	in.Settings = settings
	lay := settings.Resolver(env)
	if !lay.HasStack() {
		for i := range in.Nodes {
			if in.Nodes[i].Existing == nil {
				in.Nodes[i].TargetFolder = lay.Stack(in.Nodes[i].TargetFolder)
			}
		}
	}
	files, err := hcl.Apply(in, repo)
	if err != nil || lay.Identity() {
		return files, canvas, err
	}
	out := make([]hcl.ChangedFile, 0, len(files))
	for _, f := range files {
		if folder, name, ok := lay.Real(f.Folder, f.Filename); ok {
			f.Folder, f.Filename = folder, name
			out = append(out, f)
		}
	}
	return out, canvas, nil
}

func (rv *Reviewer) plan(ctx context.Context, p store.Project, cs store.Changeset) (plan, store.CanvasState, error) {
	// A repository that cannot be read must stop the plan: planning against an empty
	// repository would generate everything again.
	snap, err := snapshotOf(ctx, rv.Repos, p)
	if err != nil {
		return plan{}, store.CanvasState{}, &planError{err}
	}
	st := rv.Settings.of(ctx, p.ID)
	snap, err = viewOf(snap, st, cs.Env)
	if err != nil {
		return plan{}, store.CanvasState{}, &planError{err}
	}
	repo := readRepoFilesFS(snap)
	files, canvas, err := applyCanvas(p.ID, cs.CanvasJSON, repo, st, cs.Env)
	if err != nil {
		return plan{}, canvas, err
	}
	pl := plan{Files: files, Branch: branchName(p, cs), Repo: map[string]string{}, Base: snap.Commit}
	for _, f := range realFiles(st.Resolver(cs.Env), repo) {
		pl.Repo[joinPath(f.Folder, f.Filename)] = f.Content
	}
	return pl, canvas, nil
}

// newLines counts lines of next that are not matched by a line of prev.
func newLines(prev, next string) int {
	have := map[string]int{}
	for _, l := range strings.Split(prev, "\n") {
		have[l]++
	}
	n := 0
	for _, l := range strings.Split(strings.TrimRight(next, "\n"), "\n") {
		if have[l] > 0 {
			have[l]--
		} else {
			n++
		}
	}
	return n
}

type reviewDiffFile struct {
	Path     string `json:"path"`
	Status   string `json:"status"` // "added" (new file) | "modified" (existing file changed)
	Content  string `json:"content"`
	LinesAdd int    `json:"lines_added"`
}

type reviewDetail struct {
	Changeset  store.Changeset  `json:"changeset"`
	Project    store.Project    `json:"project"`
	Branch     string           `json:"branch"`
	CanvasDiff canvasDiff       `json:"canvas_diff"`
	HCLDiff    []reviewDiffFile `json:"hcl_diff"`
	Conflicts  []string         `json:"conflicts"`
}

type canvasDiff struct {
	AddedNodes []store.CanvasNode `json:"added_nodes"`
	Edges      []store.CanvasEdge `json:"edges"`
}

// Detail: GET /projects/{id}/changesets/{csId}/review
func (rv *Reviewer) Detail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cs, p, ok := rv.load(w, r)
		if !ok {
			return
		}
		if cs.Status == store.ChangesetDraft || cs.Status == store.ChangesetSaved {
			writeJSONError(w, http.StatusConflict, "changeset is still a draft")
			return
		}
		pl, canvas, err := rv.plan(r.Context(), p, cs)
		if err != nil {
			writeJSONError(w, planStatus(err), "generation failed: "+err.Error())
			return
		}
		diff := make([]reviewDiffFile, 0, len(pl.Files))
		for _, f := range pl.Files {
			path := joinPath(f.Folder, f.Filename)
			diff = append(diff, reviewDiffFile{
				Path: path, Status: f.Status, Content: f.Content,
				LinesAdd: newLines(pl.Repo[path], f.Content),
			})
		}
		if canvas.Nodes == nil {
			canvas.Nodes = []store.CanvasNode{}
		}
		if canvas.Edges == nil {
			canvas.Edges = []store.CanvasEdge{}
		}
		writeJSON(w, http.StatusOK, reviewDetail{
			Changeset: cs, Project: p, Branch: pl.Branch,
			CanvasDiff: canvasDiff{AddedNodes: canvas.Nodes, Edges: canvas.Edges},
			HCLDiff:    diff, Conflicts: []string{},
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// authorize checks who may review: the global devops role, or (D-050) the project's
// owner and co-owners. Separation of duties applies when the server requires it or the
// project turned on "second reviewer".
func (rv *Reviewer) authorize(w http.ResponseWriter, r *http.Request, cs store.Changeset) bool {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		writeJSONError(w, http.StatusUnauthorized, "not authenticated")
		return false
	}
	allowed := isDevOps(u)
	if !allowed && rv.Roles != nil {
		role, err := rv.Roles.MemberRole(r.Context(), cs.ProjectID, u.OID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not check your role")
			return false
		}
		allowed = store.CanApprove(role)
	}
	if !allowed {
		writeJSONError(w, http.StatusForbidden, "only the devops role, or the project's owner and co-owners, can review changesets")
		return false
	}
	sod := rv.SeparationOfDuties
	if !sod && rv.Roles != nil {
		// No link (a local project) or a failed lookup means the switch is off.
		if pr, err := rv.Roles.GetProjectRepo(r.Context(), cs.ProjectID); err == nil && pr.SecondReviewer {
			sod = true
		}
	}
	if sod && u.OID == cs.UserOID {
		writeJSONError(w, http.StatusForbidden, "separation of duties: you cannot review your own changeset")
		return false
	}
	return true
}

func (rv *Reviewer) audit(r *http.Request, action string, cs store.Changeset, extra map[string]any) {
	detail := map[string]any{"project_id": cs.ProjectID, "changeset_id": cs.ID, "env": cs.Env, "title": cs.Title}
	for k, v := range extra {
		detail[k] = v
	}
	if err := rv.Store.LogAudit(r.Context(), actor(r), action, "changeset:"+cs.ID, detail); err != nil && rv.Logger != nil {
		rv.Logger.Error("audit log", "err", err, "action", action)
	}
}

// Approve: POST /projects/{id}/changesets/{csId}/approve
func (rv *Reviewer) Approve() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cs, p, ok := rv.load(w, r)
		if !ok || !rv.authorize(w, r, cs) {
			return
		}
		if cs.Status != store.ChangesetPublished {
			writeJSONError(w, http.StatusConflict, "only published changesets can be approved (status: "+string(cs.Status)+")")
			return
		}
		pl, _, err := rv.plan(r.Context(), p, cs)
		if err != nil {
			writeJSONError(w, planStatus(err), "generation failed: "+err.Error())
			return
		}
		if len(pl.Files) == 0 {
			writeJSONError(w, http.StatusUnprocessableEntity, "nothing to generate: the canvas has no modules")
			return
		}

		files := make([]gitprovider.FileChange, len(pl.Files))
		for i, f := range pl.Files {
			path := joinPath(f.Folder, f.Filename)
			files[i] = gitprovider.FileChange{Path: path, Content: f.Content}
			if f.Status == hcl.FileModified {
				// Only write over a file that still has the content the plan was built on.
				files[i].Update = true
				files[i].Previous = pl.Repo[path]
			}
		}
		reviewer := actor(r)
		src, err := rv.Repos(r.Context(), p)
		if err != nil {
			writeJSONError(w, providerStatus(err), "could not open the repository: "+err.Error())
			return
		}
		res, err := src.CreateBranch(r.Context(), gitprovider.BranchRequest{
			Base: pl.Base, Branch: pl.Branch,
			Message: fmt.Sprintf("%s\n\nTerraFlow changeset %s (%s)\nRequested-by: %s\nApproved-by: %s",
				titleOr(cs), cs.ID, cs.Env, cs.UserOID, reviewer),
			Files: files,
		})
		if err != nil {
			status := providerStatus(err)
			if status == http.StatusInternalServerError {
				if rv.Logger != nil {
					rv.Logger.Error("create branch", "err", err, "changeset", cs.ID)
				}
				writeJSONError(w, status, "could not create the branch: "+err.Error())
				return
			}
			writeJSONError(w, status, err.Error())
			return
		}

		if err := rv.Store.ReviewChangeset(r.Context(), cs.ID, store.ChangesetApproved, reviewer, "", res.Branch, res.Commit); err != nil {
			code := http.StatusInternalServerError
			if errors.Is(err, pgx.ErrNoRows) {
				code = http.StatusConflict
			}
			writeJSONError(w, code, "branch "+res.Branch+" was created but the status update failed")
			return
		}
		if rv.Drift != nil {
			// The base branch as this change was planned on it: later differences are drift. Best effort.
			if err := rv.Drift.SetDriftBaseline(r.Context(), p.ID, manifestOfContents(pl.Repo)); err != nil && rv.Logger != nil {
				rv.Logger.Warn("drift baseline", "err", err, "project", p.ID)
			}
		}
		rv.audit(r, "changeset.approve", cs, map[string]any{"branch": res.Branch, "commit": res.Commit, "repo": res.Repo, "files": len(files)})
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "approved", "branch": res.Branch, "commit": res.Commit, "repo": res.Repo,
		})
	}
}

func titleOr(cs store.Changeset) string {
	if strings.TrimSpace(cs.Title) != "" {
		return cs.Title
	}
	return "TerraFlow changeset"
}

// Reject: POST /projects/{id}/changesets/{csId}/reject {"reason": "..."}
func (rv *Reviewer) Reject() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cs, _, ok := rv.load(w, r)
		if !ok || !rv.authorize(w, r, cs) {
			return
		}
		var body struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		reason := strings.TrimSpace(body.Reason)
		if reason == "" {
			writeJSONError(w, http.StatusBadRequest, "a reason is required to reject a changeset")
			return
		}
		if cs.Status != store.ChangesetPublished {
			writeJSONError(w, http.StatusConflict, "only published changesets can be rejected (status: "+string(cs.Status)+")")
			return
		}
		if err := rv.Store.ReviewChangeset(r.Context(), cs.ID, store.ChangesetRejected, actor(r), reason, "", ""); err != nil {
			code := http.StatusInternalServerError
			if errors.Is(err, pgx.ErrNoRows) {
				code = http.StatusConflict
			}
			writeJSONError(w, code, "could not reject the changeset")
			return
		}
		rv.audit(r, "changeset.reject", cs, map[string]any{"reason": reason})
		writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
	}
}

// Audit: GET /projects/{id}/audit?limit=
func (rv *Reviewer) Audit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			var n int
			if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}
		list, err := rv.Store.ListAudit(r.Context(), chi.URLParam(r, "id"), limit)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not read the audit log")
			return
		}
		if list == nil {
			list = []store.AuditEntry{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// ---- NoDB in-memory implementation ----

// MemReviewStore adapts MemChangesetStore to ReviewStore and keeps an audit list.
type MemReviewStore struct {
	cs    *MemChangesetStore
	mu    sync.Mutex
	audit []store.AuditEntry
	next  int64
}

func NewMemReviewStore(cs *MemChangesetStore) *MemReviewStore {
	return &MemReviewStore{cs: cs}
}

func (m *MemReviewStore) GetChangeset(_ context.Context, id string) (store.Changeset, error) {
	m.cs.mu.RLock()
	defer m.cs.mu.RUnlock()
	c, ok := m.cs.data[id]
	if !ok {
		return store.Changeset{}, pgx.ErrNoRows
	}
	return c, nil
}

func (m *MemReviewStore) ReviewChangeset(_ context.Context, id string, status store.ChangesetStatus,
	reviewerOID, reason, branch, commit string) error {
	m.cs.mu.Lock()
	defer m.cs.mu.Unlock()
	c, ok := m.cs.data[id]
	if !ok || c.Status != store.ChangesetPublished {
		return pgx.ErrNoRows
	}
	now := time.Now()
	c.Status, c.ReviewerOID, c.ReviewedAt, c.UpdatedAt = status, &reviewerOID, &now, now
	if reason != "" {
		c.RejectReason = &reason
	}
	if branch != "" {
		c.BranchName, c.CommitSHA = &branch, &commit
	}
	m.cs.data[id] = c
	return nil
}

func (m *MemReviewStore) LogAudit(_ context.Context, userOID, action, resource string, detail any) error {
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.next++
	m.audit = append(m.audit, store.AuditEntry{
		ID: m.next, UserOID: userOID, Action: action, Resource: resource, Detail: raw, CreatedAt: time.Now(),
	})
	return nil
}

func (m *MemReviewStore) ListAudit(_ context.Context, projectID string, limit int) ([]store.AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []store.AuditEntry
	for i := len(m.audit) - 1; i >= 0 && len(out) < limit; i-- {
		var d struct {
			ProjectID string `json:"project_id"`
		}
		json.Unmarshal(m.audit[i].Detail, &d)
		if d.ProjectID == projectID {
			out = append(out, m.audit[i])
		}
	}
	return out, nil
}
