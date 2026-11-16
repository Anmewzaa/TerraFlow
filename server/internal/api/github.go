package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Anmewzaa/TerraFlow/server/internal/auth"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// GitHubStore is what the GitHub endpoints need from persistence. *store.Queries
// implements it; tests use an in-memory fake.
type GitHubStore interface {
	UpsertConnection(ctx context.Context, c store.Connection) (store.Connection, error)
	GetConnection(ctx context.Context, id string) (store.Connection, error)
	ListConnections(ctx context.Context) ([]store.Connection, error)
	SetConnectionStatus(ctx context.Context, id, status string) error
	ProjectsOfConnection(ctx context.Context, connectionID string) (int, error)

	ReplaceGrants(ctx context.Context, userOID, connectionID string, grants []store.RepoGrant) error
	ListGrants(ctx context.Context, userOID string) ([]store.RepoGrant, error)
	GetGrant(ctx context.Context, userOID, connectionID, repoExternalID string) (store.RepoGrant, error)
	DeleteGrant(ctx context.Context, userOID, connectionID, repoExternalID string) error

	CreateProject(ctx context.Context, p store.Project) (store.Project, error)
	DeleteProject(ctx context.Context, id string) error
	CreateOwner(ctx context.Context, projectID, oid string) error
	SetProjectRepo(ctx context.Context, r store.ProjectRepo) error
	OwnedRepos(ctx context.Context, oid string) ([]store.OwnedRepo, error)
	SetRepoAccess(ctx context.Context, projectID, status string) error

	LogAudit(ctx context.Context, userOID, action, resource string, detail any) error
}

const (
	stateCookie = "tf_gh_state"
	// grantTTL is how long a repository stays connectable after the user authorized.
	defaultGrantTTL = 10 * time.Minute
	maxFolders      = 200
	maxFolderDepth  = 4
)

// GitHubAPI serves the connection flow (D-049): installing the App, the user's own
// authorization (layer 2), the repository and folder pickers, and creating a project
// from a repository. It never stores a token.
type GitHubAPI struct {
	// App is nil when GitHub is not configured; every endpoint then says so.
	App   *github.App
	Store GitHubStore
	// AppSlug builds the install link (https://github.com/apps/<slug>/installations/new).
	AppSlug string
	// AllowedOwners limits which GitHub accounts may be connected (TF_GITHUB_ALLOWED_OWNERS);
	// empty means any. Compared case-insensitively.
	AllowedOwners []string
	// RedirectURI is the callback URL registered on the App (optional when only one is).
	RedirectURI string
	// ReturnPath is the page the browser is sent back to (default /#/projects/new) and
	// WebURL the origin of the screens, for when they are served elsewhere.
	ReturnPath   string
	WebURL       string
	SecureCookie bool
	// ProjectSource is how projects are created on this server: "github" or "local".
	// The other way is refused by the API and not offered by the screens.
	ProjectSource string
	GrantTTL      time.Duration
	Now           func() time.Time
}

func (g *GitHubAPI) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}

func (g *GitHubAPI) ttl() time.Duration {
	if g.GrantTTL > 0 {
		return g.GrantTTL
	}
	return defaultGrantTTL
}

func (g *GitHubAPI) back(w http.ResponseWriter, r *http.Request, q url.Values) {
	p := g.ReturnPath
	if p == "" {
		p = "/#/projects/new" // the screen routes by URL hash
	}
	// WebURL is the address of the screens when they are not served by this server
	// (the Vite dev server); empty means the same origin.
	http.Redirect(w, r, strings.TrimRight(g.WebURL, "/")+p+"?"+q.Encode(), http.StatusFound)
}

func (g *GitHubAPI) allowed(login string) bool {
	if len(g.AllowedOwners) == 0 {
		return true
	}
	for _, o := range g.AllowedOwners {
		if strings.EqualFold(strings.TrimSpace(o), login) {
			return true
		}
	}
	return false
}

func (g *GitHubAPI) configured(w http.ResponseWriter) bool {
	if g.App == nil || g.Store == nil {
		// 409, not 503: this is a setting the operator has not made, not a fault, and
		// the route matrix requires that no request is answered with a 5xx.
		writeJSONError(w, http.StatusConflict, "GitHub is not configured on this server")
		return false
	}
	return true
}

// Status: GET /github/status tells the screen what to offer.
func (g *GitHubAPI) Status() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out := map[string]any{"configured": g.App != nil && g.Store != nil, "project_source": g.ProjectSource}
		if g.App != nil && g.AppSlug != "" {
			out["install_url"] = "https://github.com/apps/" + url.PathEscape(g.AppSlug) + "/installations/new"
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// Authorize: GET /github/authorize sends the browser to GitHub to authorize the App.
// The anti-forgery state lives in a short cookie, not in server memory.
func (g *GitHubAPI) Authorize() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.configured(w) {
			return
		}
		b := make([]byte, 24)
		if _, err := rand.Read(b); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "could not start the GitHub authorization")
			return
		}
		state := hex.EncodeToString(b)
		http.SetCookie(w, &http.Cookie{
			Name: stateCookie, Value: state, Path: "/api/github", MaxAge: 600,
			HttpOnly: true, Secure: g.SecureCookie, SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, g.App.AuthorizeURL(state, g.RedirectURI), http.StatusFound)
	}
}

// Callback: GET /github/callback. It checks the state, trades the code for a user
// token, asks GitHub which installations and repositories this user can reach, saves
// that as short-lived grants and forgets the token.
func (g *GitHubAPI) Callback() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.configured(w) {
			return
		}
		u := auth.UserFromContext(r.Context())
		if u == nil {
			writeJSONError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		// The reason goes to the server log (never the token or the code), so a failed
		// connect can be diagnosed; the browser only gets the short code.
		fail := func(code string, cause ...error) {
			if len(cause) > 0 && cause[0] != nil {
				slog.Warn("github callback failed", "code", code, "err", cause[0].Error())
			} else {
				slog.Warn("github callback failed", "code", code)
			}
			g.back(w, r, url.Values{"error": {code}})
		}

		c, err := r.Cookie(stateCookie)
		http.SetCookie(w, &http.Cookie{Name: stateCookie, Value: "", Path: "/api/github", MaxAge: -1, HttpOnly: true, Secure: g.SecureCookie, SameSite: http.SameSiteLaxMode})
		state := r.URL.Query().Get("state")
		if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
			fail("state")
			return
		}
		if r.URL.Query().Get("error") != "" {
			fail("denied")
			return
		}
		ctx := r.Context()
		tok, err := g.App.ExchangeCode(ctx, r.URL.Query().Get("code"), g.RedirectURI)
		if err != nil {
			fail("code", err)
			return
		}
		gh, err := g.App.GetUser(ctx, tok)
		if err != nil {
			fail("github", err)
			return
		}
		installs, err := g.App.UserInstallations(ctx, tok)
		if err != nil {
			fail("github", err)
			return
		}

		var connected, blocked int
		for _, in := range installs {
			if !g.allowed(in.Account.Login) {
				blocked++
				continue
			}
			repos, err := g.App.UserInstallationRepos(ctx, tok, in.ID)
			if err != nil {
				fail("github", err)
				return
			}
			conn, err := g.Store.UpsertConnection(ctx, store.Connection{
				Provider: "github", ExternalID: strconv.FormatInt(in.ID, 10),
				AccountLogin: in.Account.Login, AccountType: in.Account.Type, CreatedBy: u.OID,
			})
			if err != nil {
				fail("server", err)
				return
			}
			exp := g.now().Add(g.ttl())
			grants := make([]store.RepoGrant, 0, len(repos))
			for _, rp := range repos {
				grants = append(grants, store.RepoGrant{
					RepoExternalID: strconv.FormatInt(rp.ID, 10), RepoFullName: rp.FullName,
					DefaultBranch: rp.DefaultBranch, Private: rp.Private,
					GitHubUserID: strconv.FormatInt(gh.ID, 10), GitHubLogin: gh.Login, ExpiresAt: exp,
				})
			}
			if err := g.Store.ReplaceGrants(ctx, u.OID, conn.ID, grants); err != nil {
				fail("server", err)
				return
			}
			connected++
			_ = g.Store.LogAudit(ctx, u.OID, "github.authorize", "connection:"+conn.ID,
				map[string]any{"account": in.Account.Login, "github_user": gh.Login, "repositories": len(grants)})
		}
		// The person has just proven who they are to GitHub: check that they still reach
		// the repositories of the projects they own (D-061). A failure changes nothing.
		if err := g.RecheckOwnerAccess(ctx, u.OID, tok); err != nil {
			slog.Warn("github callback: owner access not re-checked", "err", err.Error())
		}
		switch {
		case connected > 0:
			g.back(w, r, url.Values{"connected": {strconv.Itoa(connected)}})
		case blocked > 0:
			fail("not_allowed")
		default:
			fail("no_installation")
		}
	}
}

type repoView struct {
	ConnectionID  string `json:"connection_id"`
	Account       string `json:"account"`
	RepoID        string `json:"repo_id"`
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	// ExpiresAt is when GitHub's confirmation of this user's access runs out (RFC 3339).
	ExpiresAt string `json:"expires_at"`
}

// Repos: GET /github/repos lists the repositories this user may connect right now
// (live grants whose connection is still active).
func (g *GitHubAPI) Repos() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.configured(w) {
			return
		}
		u := auth.UserFromContext(r.Context())
		if u == nil {
			writeJSONError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		grants, err := g.Store.ListGrants(r.Context(), u.OID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		out := []repoView{}
		accounts := map[string]store.Connection{}
		for _, gr := range grants {
			c, ok := accounts[gr.ConnectionID]
			if !ok {
				if c, err = g.Store.GetConnection(r.Context(), gr.ConnectionID); err != nil {
					continue
				}
				accounts[gr.ConnectionID] = c
			}
			if c.Status != store.ConnActive {
				continue
			}
			out = append(out, repoView{ConnectionID: gr.ConnectionID, Account: c.AccountLogin, RepoID: gr.RepoExternalID,
				FullName: gr.RepoFullName, DefaultBranch: gr.DefaultBranch, Private: gr.Private,
				ExpiresAt: gr.ExpiresAt.UTC().Format(time.RFC3339)})
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type myConnectionView struct {
	ID           string `json:"id"`
	Account      string `json:"account"`
	AccountType  string `json:"account_type"`
	Status       string `json:"status"`
	Projects     int    `json:"projects"`
	Repositories int    `json:"repositories"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

// MyConnections: GET /github/connections lists the GitHub accounts this user connected,
// how many projects use each, and how many repositories GitHub confirmed for them just
// now (live grants), so the screen can say when to authorize again.
func (g *GitHubAPI) MyConnections() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.configured(w) {
			return
		}
		u := auth.UserFromContext(r.Context())
		if u == nil {
			writeJSONError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		all, err := g.Store.ListConnections(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		grants, err := g.Store.ListGrants(r.Context(), u.OID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		out := []myConnectionView{}
		for _, c := range all {
			if c.CreatedBy != u.OID || c.Status == store.ConnDisconnected {
				continue
			}
			v := myConnectionView{ID: c.ID, Account: c.AccountLogin, AccountType: c.AccountType, Status: c.Status}
			v.Projects, _ = g.Store.ProjectsOfConnection(r.Context(), c.ID)
			var latest time.Time
			for _, gr := range grants {
				if gr.ConnectionID == c.ID {
					v.Repositories++
					if gr.ExpiresAt.After(latest) {
						latest = gr.ExpiresAt
					}
				}
			}
			if !latest.IsZero() {
				v.ExpiresAt = latest.UTC().Format(time.RFC3339)
			}
			out = append(out, v)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// DisconnectMine: POST /github/connections/{id}/disconnect lets the person who connected
// an account disconnect it, but only while no project uses it (an admin can force it).
func (g *GitHubAPI) DisconnectMine() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.configured(w) {
			return
		}
		u := auth.UserFromContext(r.Context())
		if u == nil {
			writeJSONError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		id := chi.URLParam(r, "id")
		c, err := g.Store.GetConnection(r.Context(), id)
		if err != nil || c.CreatedBy != u.OID {
			// Not found and not yours look the same: a connection id is not a secret to guess.
			writeJSONError(w, http.StatusNotFound, "connection not found")
			return
		}
		if n, _ := g.Store.ProjectsOfConnection(r.Context(), id); n > 0 {
			writeJSONError(w, http.StatusConflict, "projects still use this connection; delete or transfer them first")
			return
		}
		if err := g.Store.SetConnectionStatus(r.Context(), id, store.ConnDisconnected); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		_ = g.Store.LogAudit(r.Context(), u.OID, "github.disconnect", "connection:"+id, map[string]any{"by": "owner"})
		writeJSON(w, http.StatusOK, map[string]string{"status": store.ConnDisconnected})
	}
}

// grantFor finds the live grant and active connection the request names, or writes
// the refusal. The user must authorize again when the grant has expired.
func (g *GitHubAPI) grantFor(w http.ResponseWriter, r *http.Request, connID, repoID string) (store.RepoGrant, store.Connection, bool) {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		writeJSONError(w, http.StatusUnauthorized, "not authenticated")
		return store.RepoGrant{}, store.Connection{}, false
	}
	gr, err := g.Store.GetGrant(r.Context(), u.OID, connID, repoID)
	if errors.Is(err, pgx.ErrNoRows) || isBadReference(err) {
		writeJSONError(w, http.StatusForbidden, "GitHub did not confirm your access to this repository recently; authorize with GitHub again")
		return store.RepoGrant{}, store.Connection{}, false
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "db error")
		return store.RepoGrant{}, store.Connection{}, false
	}
	conn, err := g.Store.GetConnection(r.Context(), connID)
	if err != nil || conn.Status != store.ConnActive {
		writeJSONError(w, http.StatusConflict, "this GitHub connection is not active")
		return store.RepoGrant{}, store.Connection{}, false
	}
	return gr, conn, true
}

// sourceOf makes the Source for a repository folder of a connection.
func (g *GitHubAPI) sourceOf(conn store.Connection, fullName, branch, folder string) (*github.Source, error) {
	id, err := strconv.ParseInt(conn.ExternalID, 10, 64)
	if err != nil {
		return nil, gitprovider.ErrNotFound
	}
	owner, repo, ok := strings.Cut(fullName, "/")
	if !ok {
		return nil, gitprovider.ErrNotFound
	}
	return &github.Source{App: g.App, InstallationID: id, Owner: owner, Repo: repo, Branch: branch, Path: folder}, nil
}

// Folders: GET /github/folders?connection_id=&repo_id= lists the folders of the
// repository that hold Terraform, for the folder picker. Needs a live grant.
func (g *GitHubAPI) Folders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.configured(w) {
			return
		}
		gr, conn, ok := g.grantFor(w, r, r.URL.Query().Get("connection_id"), r.URL.Query().Get("repo_id"))
		if !ok {
			return
		}
		branch, ok := g.chosenBranch(w, r, conn, gr, r.URL.Query().Get("branch"))
		if !ok {
			return
		}
		src, err := g.sourceOf(conn, gr.RepoFullName, branch, "")
		if err != nil {
			writeJSONError(w, providerStatus(err), "repository not found")
			return
		}
		tree, err := outlineOf(r.Context(), src)
		if errors.Is(err, gitprovider.ErrEmptyRepo) {
			// A code, so the screen can offer to initialize the repository.
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "could not read the repository: " + err.Error(), "code": "empty_repo"})
			return
		}
		if err != nil {
			writeJSONError(w, providerStatus(err), "could not read the repository: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"folders": terraformFolders(tree), "default_branch": gr.DefaultBranch, "branch": branch})
	}
}

// chosenBranch is the branch a screen asked to read: empty means the repository's default. A branch must
// be a valid name, not one TerraFlow made itself, and must exist; it answers the request when not.
func (g *GitHubAPI) chosenBranch(w http.ResponseWriter, r *http.Request, conn store.Connection, gr store.RepoGrant, asked string) (string, bool) {
	asked = strings.TrimSpace(asked)
	if asked == "" || asked == gr.DefaultBranch {
		return gr.DefaultBranch, true
	}
	if !validBranchName(asked) || strings.HasPrefix(asked, ReviewBranchPrefix) {
		writeJSONError(w, http.StatusUnprocessableEntity, "that is not a branch a project can use")
		return "", false
	}
	src, err := g.sourceOf(conn, gr.RepoFullName, gr.DefaultBranch, "")
	if err != nil {
		writeJSONError(w, providerStatus(err), "repository not found")
		return "", false
	}
	ok, err := src.BranchExists(r.Context(), asked)
	if err != nil {
		writeJSONError(w, providerStatus(err), "could not check the branch on GitHub")
		return "", false
	}
	if !ok {
		writeJSONError(w, http.StatusUnprocessableEntity, "that branch does not exist in the repository")
		return "", false
	}
	return asked, true
}

// Branches: GET /github/branches?connection_id=&repo_id= lists the repository's branches for the
// branch picker (the default first, without TerraFlow review branches). Needs a live grant.
func (g *GitHubAPI) Branches() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.configured(w) {
			return
		}
		gr, conn, ok := g.grantFor(w, r, r.URL.Query().Get("connection_id"), r.URL.Query().Get("repo_id"))
		if !ok {
			return
		}
		src, err := g.sourceOf(conn, gr.RepoFullName, gr.DefaultBranch, "")
		if err != nil {
			writeJSONError(w, providerStatus(err), "repository not found")
			return
		}
		names, truncated, err := src.Branches(r.Context())
		if errors.Is(err, gitprovider.ErrEmptyRepo) {
			names, err = nil, nil
		}
		if err != nil {
			writeJSONError(w, providerStatus(err), "could not list the branches on GitHub")
			return
		}
		out := []string{gr.DefaultBranch}
		for _, n := range names {
			if n != gr.DefaultBranch && !strings.HasPrefix(n, ReviewBranchPrefix) {
				out = append(out, n)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"branches": out, "truncated": truncated, "default_branch": gr.DefaultBranch})
	}
}

// outlineOf is the cheap read: the file names of a source, without their contents
// when the source can (GitHub), else a full snapshot.
func outlineOf(ctx context.Context, src gitprovider.Source) (fs.FS, error) {
	if o, ok := src.(gitprovider.Outliner); ok {
		return o.Outline(ctx)
	}
	snap, err := src.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return snap.FS, nil
}

// onlyFor refuses a route when this server creates projects the other way.
func onlyFor(source, current string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if current == "none" {
			writeJSONError(w, http.StatusConflict, "creating projects is switched off on this server: the GitHub App is not set up")
			return
		}
		if current != source {
			writeJSONError(w, http.StatusConflict, "this server creates projects from "+current+" only")
			return
		}
		next(w, r)
	}
}

// InitRepo: POST /github/init {connection_id, repo_id} creates the first commit (a
// README.md, which also creates the default branch) of a repository that has no commit
// at all, when the user confirmed it on screen (D-056). It needs a live grant like any
// other use of the repository, refuses a repository that has commits, and is audited.
func (g *GitHubAPI) InitRepo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.configured(w) {
			return
		}
		var req struct {
			ConnectionID string `json:"connection_id"`
			RepoID       string `json:"repo_id"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		gr, conn, ok := g.grantFor(w, r, req.ConnectionID, req.RepoID)
		if !ok {
			return
		}
		src, err := g.sourceOf(conn, gr.RepoFullName, gr.DefaultBranch, "")
		if err != nil {
			writeJSONError(w, providerStatus(err), "repository not found")
			return
		}
		sha, err := src.Initialize(r.Context())
		if errors.Is(err, gitprovider.ErrNotEmpty) {
			writeJSONError(w, http.StatusConflict, "this repository already has commits")
			return
		}
		if err != nil {
			writeJSONError(w, providerStatus(err), "could not initialize the repository: "+err.Error())
			return
		}
		_ = g.Store.LogAudit(r.Context(), actor(r), "repo.initialize", "repo:"+gr.RepoFullName,
			map[string]any{"commit": sha, "file": "README.md"})
		writeJSON(w, http.StatusCreated, map[string]string{"commit": sha})
	}
}

// terraformFolders returns every directory (relative path, "" for the root) that has
// a .tf file below it, shallowest first, capped.
func terraformFolders(fsys fs.FS) []string {
	set := map[string]bool{}
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".tf") {
			return nil
		}
		dir := path.Dir(p)
		for {
			if dir == "." {
				set[""] = true
				break
			}
			if strings.Count(dir, "/") < maxFolderDepth {
				set[dir] = true
			}
			dir = path.Dir(dir)
		}
		return nil
	})
	out := make([]string, 0, len(set))
	for d := range set {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if di, dj := strings.Count(out[i], "/"), strings.Count(out[j], "/"); di != dj {
			return di < dj
		}
		return out[i] < out[j]
	})
	if len(out) > maxFolders {
		out = out[:maxFolders]
	}
	return out
}

type fromRepoBody struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	ConnectionID string `json:"connection_id"`
	RepoID       string `json:"repo_id"`
	Folder       string `json:"folder"`
	// NewFolder starts the project in a folder that holds no Terraform yet (it need not
	// exist); the first publish creates it on a branch. The repository must have commits.
	NewFolder bool `json:"new_folder"`
	// Branch is the branch the project reads and starts review branches from ("" = the repository's
	// default). It must exist; it is kept as the project's base branch (8.8).
	Branch string `json:"branch"`
}

// CreateFromRepo: POST /projects/from-repo. Any signed-in user may create a project
// from a repository they can access; they become its owner. The server checks the
// grant, the folder and the Terraform again; nothing from the client is trusted.
func (g *GitHubAPI) CreateFromRepo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.configured(w) {
			return
		}
		u := auth.UserFromContext(r.Context())
		if u == nil {
			writeJSONError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		var req fromRepoBody
		if !decodeBody(w, r, &req) {
			return
		}
		name, desc := strings.TrimSpace(req.Name), strings.TrimSpace(req.Description)
		if !validName(name) {
			writeJSONError(w, http.StatusBadRequest, "name is required (letters/digits, max 80 chars)")
			return
		}
		if len(desc) > maxDescription {
			writeJSONError(w, http.StatusBadRequest, "description is too long (max 500 chars)")
			return
		}
		folder := strings.Trim(strings.TrimSpace(req.Folder), "/")
		if folder != "" {
			if _, err := gitprovider.ValidatePath(folder); err != nil {
				writeJSONError(w, http.StatusUnprocessableEntity, "the folder is not a valid path")
				return
			}
		}
		gr, conn, ok := g.grantFor(w, r, req.ConnectionID, req.RepoID)
		if !ok {
			return
		}
		branch, ok := g.chosenBranch(w, r, conn, gr, req.Branch)
		if !ok {
			return
		}
		src, err := g.sourceOf(conn, gr.RepoFullName, branch, folder)
		if err != nil {
			writeJSONError(w, providerStatus(err), "repository not found")
			return
		}
		tree, err := outlineOf(r.Context(), src)
		if err != nil {
			writeJSONError(w, providerStatus(err), "could not read the folder: "+err.Error())
			return
		}
		has := len(terraformFolders(tree)) > 0
		switch {
		case req.NewFolder && folder == "":
			writeJSONError(w, http.StatusUnprocessableEntity, "name the new folder")
			return
		case req.NewFolder && has:
			writeJSONError(w, http.StatusUnprocessableEntity, "this folder already holds Terraform; choose it from the list instead")
			return
		case !req.NewFolder && !has:
			writeJSONError(w, http.StatusUnprocessableEntity, "this folder holds no .tf files")
			return
		}

		p, err := g.Store.CreateProject(r.Context(), store.Project{Name: name, Slug: slugify(name), Description: desc, SourceType: "github"})
		if errors.Is(err, store.ErrSlugTaken) {
			writeJSONError(w, http.StatusConflict, "a project with this name already exists")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		undo := func() { _ = g.Store.DeleteProject(context.WithoutCancel(r.Context()), p.ID) }
		if err := g.Store.CreateOwner(r.Context(), p.ID, u.OID); err != nil {
			undo()
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		if err := g.Store.SetProjectRepo(r.Context(), store.ProjectRepo{
			ProjectID: p.ID, ConnectionID: conn.ID, RepoExternalID: gr.RepoExternalID, RepoFullName: gr.RepoFullName,
			DefaultBranch: gr.DefaultBranch, RepoPath: folder, ConnectedBy: u.OID, BaseBranch: baseOf(branch, gr.DefaultBranch),
		}); err != nil {
			undo()
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		_ = g.Store.DeleteGrant(r.Context(), u.OID, conn.ID, gr.RepoExternalID)
		_ = g.Store.LogAudit(r.Context(), u.OID, "project.create", "project:"+p.ID,
			map[string]any{"project_id": p.ID, "repository": gr.RepoFullName, "folder": folder, "branch": branch, "github_user": gr.GitHubLogin})
		writeJSON(w, http.StatusCreated, p)
	}
}

// baseOf is what the project stores as its base branch: nothing when it is the default, so the project
// keeps following the repository's default branch.
func baseOf(branch, def string) string {
	if branch == def {
		return ""
	}
	return branch
}

type connectionView struct {
	store.Connection
	Projects int `json:"projects"`
}

// Connections: GET /admin/connections lists every connection with how many projects use it.
func (g *GitHubAPI) Connections() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if g.Store == nil {
			writeJSON(w, http.StatusOK, []connectionView{})
			return
		}
		list, err := g.Store.ListConnections(r.Context())
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		out := make([]connectionView, 0, len(list))
		for _, c := range list {
			n, _ := g.Store.ProjectsOfConnection(r.Context(), c.ID)
			out = append(out, connectionView{Connection: c, Projects: n})
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// Disconnect: POST /admin/connections/{id}/disconnect stops every project that uses it.
func (g *GitHubAPI) Disconnect() http.HandlerFunc {
	return g.setStatus(store.ConnDisconnected, "github.disconnect")
}

func (g *GitHubAPI) setStatus(status, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		err := g.Store.SetConnectionStatus(r.Context(), id, status)
		if errors.Is(err, pgx.ErrNoRows) || isBadReference(err) {
			writeJSONError(w, http.StatusNotFound, "connection not found")
			return
		}
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		_ = g.Store.LogAudit(r.Context(), actor(r), action, "connection:"+id, nil)
		writeJSON(w, http.StatusOK, map[string]string{"status": status})
	}
}

// Recheck: POST /admin/connections/{id}/recheck asks GitHub whether the installation
// still exists and marks the connection active or "needs attention".
func (g *GitHubAPI) Recheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !g.configured(w) {
			return
		}
		id := chi.URLParam(r, "id")
		c, err := g.Store.GetConnection(r.Context(), id)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "connection not found")
			return
		}
		inst, perr := strconv.ParseInt(c.ExternalID, 10, 64)
		status := store.ConnActive
		if perr == nil {
			if _, err := g.App.GetInstallation(r.Context(), inst); err != nil {
				if errors.Is(err, gitprovider.ErrNotFound) || errors.Is(err, gitprovider.ErrUnauthorized) {
					status = store.ConnNeedsAttention
				} else {
					writeJSONError(w, providerStatus(err), "GitHub could not be reached")
					return
				}
			}
		} else {
			status = store.ConnNeedsAttention
		}
		if c.Status == store.ConnDisconnected && status == store.ConnActive {
			status = store.ConnDisconnected // an admin's disconnect is not undone by a check
		}
		if err := g.Store.SetConnectionStatus(r.Context(), id, status); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "db error")
			return
		}
		_ = g.Store.LogAudit(r.Context(), actor(r), "github.recheck", "connection:"+id, map[string]any{"status": status})
		writeJSON(w, http.StatusOK, map[string]string{"status": status})
	}
}

// ProjectRepos serves local projects from a folder and GitHub projects through the
// App. A GitHub project whose connection is not active (or was never linked) is an
// error, never an empty repository (the plan must stop, not generate against nothing).
func ProjectRepos(local Repos, app *github.App, s interface {
	GetProjectRepo(ctx context.Context, projectID string) (store.ProjectRepo, error)
	GetConnection(ctx context.Context, id string) (store.Connection, error)
}) Repos {
	return func(ctx context.Context, p store.Project) (gitprovider.Source, error) {
		if p.SourceType != "github" {
			return local(ctx, p)
		}
		if app == nil || s == nil {
			return nil, gitprovider.ErrUnauthorized
		}
		pr, err := s.GetProjectRepo(ctx, p.ID)
		if err != nil {
			return nil, gitprovider.ErrNotFound
		}
		conn, err := s.GetConnection(ctx, pr.ConnectionID)
		if err != nil {
			return nil, gitprovider.ErrNotFound
		}
		if conn.Status != store.ConnActive {
			return nil, gitprovider.ErrUnauthorized
		}
		if pr.AccessStatus == store.RepoAccessNeedsAttention {
			return nil, errOwnerLostAccess
		}
		inst, err := strconv.ParseInt(conn.ExternalID, 10, 64)
		if err != nil {
			return nil, gitprovider.ErrNotFound
		}
		owner, repo, ok := strings.Cut(pr.RepoFullName, "/")
		if !ok {
			return nil, gitprovider.ErrNotFound
		}
		return &github.Source{App: app, InstallationID: inst, Owner: owner, Repo: repo, Branch: pr.Branch(), Path: pr.RepoPath}, nil
	}
}
