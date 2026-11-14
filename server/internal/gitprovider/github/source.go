package github

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"testing/fstest"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
)

const (
	// maxFiles and maxFileSize bound what one snapshot downloads.
	maxFiles    = 500
	maxFileSize = 256 << 10
)

// ErrTooLarge: the repository tree is too large for GitHub to list in one answer.
var ErrTooLarge = errors.New("the repository is too large to read")

// Source is a gitprovider.Source for one project: a repository, its branch and the
// folder inside it. It holds no token; each operation makes its own.
type Source struct {
	App            *App
	InstallationID int64
	Owner, Repo    string
	Branch         string // the branch to read; "" means the repository's default
	Path           string // the project folder inside the repository ("" = the root)
}

var _ gitprovider.Source = (*Source)(nil)

// session is one operation's token, renewed once if GitHub says it has expired.
type session struct {
	s     *Source
	token Token
	fresh bool
}

func (s *Source) open(ctx context.Context) (*session, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	t, err := s.App.InstallationToken(ctx, s.InstallationID, s.Repo)
	if err != nil {
		return nil, err
	}
	return &session{s: s, token: t, fresh: true}, nil
}

func (s *Source) validate() error {
	for _, v := range []string{s.Owner, s.Repo} {
		if v == "" || strings.ContainsAny(v, "/\\ \x00") || v == "." || v == ".." {
			return fmt.Errorf("%w: invalid repository name", gitprovider.ErrNotFound)
		}
	}
	if s.Path != "" {
		if _, err := gitprovider.ValidatePath(s.Path); err != nil {
			return err
		}
	}
	return nil
}

// call sends a request on the repository; one 401 makes a new token and retries once.
func (se *session) call(ctx context.Context, method, rel string, body, out any) error {
	p := "/repos/" + url.PathEscape(se.s.Owner) + "/" + url.PathEscape(se.s.Repo) + rel
	err := se.s.App.call(ctx, se.token.value, method, p, body, out)
	if err != nil && errors.Is(err, gitprovider.ErrUnauthorized) && se.fresh {
		se.fresh = false // retry once
		t, terr := se.s.App.InstallationToken(ctx, se.s.InstallationID, se.s.Repo)
		if terr != nil {
			return terr
		}
		se.token = t
		return se.s.App.call(ctx, se.token.value, method, p, body, out)
	}
	return err
}

// MaxBranches caps how many branch names Branches returns.
const MaxBranches = 500

// Branches lists the repository's branch names (paged, capped at MaxBranches). The second
// answer is true when the list was cut short.
func (s *Source) Branches(ctx context.Context) ([]string, bool, error) {
	se, err := s.open(ctx)
	if err != nil {
		return nil, false, err
	}
	var names []string
	for page := 1; len(names) < MaxBranches; page++ {
		var got []struct {
			Name string `json:"name"`
		}
		if err := se.call(ctx, http.MethodGet, fmt.Sprintf("/branches?per_page=100&page=%d", page), nil, &got); err != nil {
			return nil, false, err
		}
		for _, b := range got {
			names = append(names, b.Name)
		}
		if len(got) < 100 {
			return names, false, nil
		}
	}
	return names[:MaxBranches], true, nil
}

// BranchExists says whether the branch is there.
func (s *Source) BranchExists(ctx context.Context, name string) (bool, error) {
	se, err := s.open(ctx)
	if err != nil {
		return false, err
	}
	var ref struct {
		Ref string `json:"ref"`
	}
	switch err := se.call(ctx, http.MethodGet, "/git/ref/heads/"+escapeRef(name), nil, &ref); {
	case err == nil:
		return true, nil
	case errors.Is(err, gitprovider.ErrNotFound):
		return false, nil
	default:
		return false, err
	}
}

type commitInfo struct {
	SHA    string `json:"sha"`
	Commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	} `json:"commit"`
}

type treeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
}

type treeAnswer struct {
	SHA       string      `json:"sha"`
	Tree      []treeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

// branchHead returns the head commit of the branch to read.
func (se *session) branchHead(ctx context.Context) (commitInfo, error) {
	branch := se.s.Branch
	if branch == "" {
		var repo struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := se.call(ctx, http.MethodGet, "", nil, &repo); err != nil {
			return commitInfo{}, err
		}
		if repo.DefaultBranch == "" {
			return commitInfo{}, fmt.Errorf("%w: the repository has no default branch", gitprovider.ErrNotFound)
		}
		branch = repo.DefaultBranch
	}
	var out commitInfo
	err := se.call(ctx, http.MethodGet, "/commits/"+url.PathEscape(branch), nil, &out)
	var he *HTTPError
	if errors.As(err, &he) && he.Status == http.StatusConflict && strings.Contains(strings.ToLower(he.Message), "empty") {
		return commitInfo{}, gitprovider.ErrEmptyRepo
	}
	return out, err
}

func (se *session) commit(ctx context.Context, sha string) (commitInfo, error) {
	// GET /git/commits/{sha} puts the tree at the top level ({sha, tree: {sha}}), unlike
	// GET /commits/{ref} ({sha, commit: {tree: {sha}}}); both end up in a commitInfo.
	var raw struct {
		SHA  string `json:"sha"`
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	err := se.call(ctx, http.MethodGet, "/git/commits/"+url.PathEscape(sha), nil, &raw)
	out := commitInfo{SHA: raw.SHA}
	if out.SHA == "" {
		out.SHA = sha
	}
	out.Commit.Tree.SHA = raw.Tree.SHA
	return out, err
}

// folderEntries lists the files below the project folder of a tree, with paths
// relative to that folder.
func (se *session) folderEntries(ctx context.Context, treeSHA string) ([]treeEntry, error) {
	var tr treeAnswer
	if err := se.call(ctx, http.MethodGet, "/git/trees/"+url.PathEscape(treeSHA)+"?recursive=1", nil, &tr); err != nil {
		return nil, err
	}
	if tr.Truncated {
		return nil, ErrTooLarge
	}
	prefix := ""
	if se.s.Path != "" {
		prefix = strings.Trim(path.Clean(se.s.Path), "/") + "/"
	}
	var out []treeEntry
	for _, e := range tr.Tree {
		if e.Type != "blob" || !strings.HasPrefix(e.Path, prefix) {
			continue
		}
		e.Path = strings.TrimPrefix(e.Path, prefix)
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func (se *session) blob(ctx context.Context, sha string) (string, error) {
	key := se.s.Owner + "/" + se.s.Repo + "@" + sha
	if c, ok := cache.blob(key); ok {
		return c, nil
	}
	var out struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := se.call(ctx, http.MethodGet, "/git/blobs/"+url.PathEscape(sha), nil, &out); err != nil {
		return "", err
	}
	if out.Encoding != "base64" {
		return "", fmt.Errorf("unexpected blob encoding %q", out.Encoding)
	}
	b, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(out.Content, "\n", ""))
	if err != nil {
		return "", errors.New("a blob was not valid base64")
	}
	cache.putBlob(key, string(b))
	return string(b), nil
}

func wanted(rel string) bool {
	// Only Terraform text matters to TerraFlow; everything else in the repository
	// (binaries, docs, state) is neither downloaded nor shown.
	return strings.HasSuffix(rel, ".tf") || strings.HasSuffix(rel, ".tfvars")
}

// Snapshot reads the project's Terraform files at the head of the branch.
func (s *Source) Snapshot(ctx context.Context) (gitprovider.Snapshot, error) {
	se, err := s.open(ctx)
	if err != nil {
		return gitprovider.Snapshot{}, err
	}
	head, err := se.branchHead(ctx)
	if err != nil {
		return gitprovider.Snapshot{}, err
	}
	// The tree of a commit never changes: a snapshot of it is kept by its SHA.
	snapKey := s.Owner + "/" + s.Repo + "/" + s.Path + "@" + head.Commit.Tree.SHA
	if head.Commit.Tree.SHA != "" {
		if m, ok := cache.snapshot(snapKey); ok {
			return gitprovider.Snapshot{Commit: head.SHA, FS: fs.FS(m)}, nil
		}
	}
	entries, err := se.folderEntries(ctx, head.Commit.Tree.SHA)
	if err != nil {
		return gitprovider.Snapshot{}, err
	}
	m := fstest.MapFS{}
	n := 0
	for _, e := range entries {
		if !wanted(e.Path) || e.Size > maxFileSize {
			continue
		}
		if n++; n > maxFiles {
			return gitprovider.Snapshot{}, fmt.Errorf("%w: more than %d Terraform files", ErrTooLarge, maxFiles)
		}
		content, err := se.blob(ctx, e.SHA)
		if err != nil {
			return gitprovider.Snapshot{}, err
		}
		m[e.Path] = &fstest.MapFile{Data: []byte(content)}
	}
	if head.Commit.Tree.SHA != "" {
		cache.putSnapshot(snapKey, m)
	}
	return gitprovider.Snapshot{Commit: head.SHA, FS: fs.FS(m)}, nil
}

// Outline lists the Terraform files below the project folder from the git tree alone:
// two calls, whatever the size of the repository, and no file is downloaded.
func (s *Source) Outline(ctx context.Context) (fs.FS, error) {
	se, err := s.open(ctx)
	if err != nil {
		return nil, err
	}
	head, err := se.branchHead(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := se.folderEntries(ctx, head.Commit.Tree.SHA)
	if err != nil {
		return nil, err
	}
	m := fstest.MapFS{}
	for _, e := range entries {
		if wanted(e.Path) {
			m[e.Path] = &fstest.MapFile{}
		}
	}
	return fs.FS(m), nil
}

var _ gitprovider.Outliner = (*Source)(nil)

// Initialize creates README.md as the first commit of an empty repository, which also
// creates its default branch. It refuses (ErrNotEmpty) a repository that has commits,
// so it can never overwrite anything, and it only ever creates this one file.
func (s *Source) Initialize(ctx context.Context) (string, error) {
	se, err := s.open(ctx)
	if err != nil {
		return "", err
	}
	if _, err := se.branchHead(ctx); err == nil {
		return "", gitprovider.ErrNotEmpty
	} else if !errors.Is(err, gitprovider.ErrEmptyRepo) {
		return "", err
	}
	readme := "# " + s.Repo + "\n\nInfrastructure code managed with TerraFlow.\n"
	var out struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	body := map[string]string{
		"message": "Initialize repository (TerraFlow)",
		"content": base64.StdEncoding.EncodeToString([]byte(readme)),
	}
	if err := se.call(ctx, http.MethodPut, "/contents/README.md", body, &out); err != nil {
		return "", err
	}
	return out.Commit.SHA, nil
}

var _ gitprovider.Initializer = (*Source)(nil)

// CreateBranch writes one commit on a new branch made from req.Base (the head of the
// branch if empty). It uses the Git Data API, so the commit is atomic, and it never
// moves an existing reference.
func (s *Source) CreateBranch(ctx context.Context, req BranchRequest) (gitprovider.Result, error) {
	if req.Branch == "" || strings.ContainsAny(req.Branch, " ~^:?*[\\\x00") || strings.Contains(req.Branch, "..") ||
		strings.HasPrefix(req.Branch, "/") || strings.HasSuffix(req.Branch, "/") || strings.HasSuffix(req.Branch, ".lock") {
		return gitprovider.Result{}, fmt.Errorf("invalid branch name")
	}
	// Check every path before any request, so a bad one costs nothing.
	rels := make([]string, len(req.Files))
	for i, f := range req.Files {
		p, err := gitprovider.ValidatePath(f.Path)
		if err != nil {
			return gitprovider.Result{}, err
		}
		rels[i] = p
	}
	if len(req.Files) == 0 {
		return gitprovider.Result{}, errors.New("no files to commit")
	}

	se, err := s.open(ctx)
	if err != nil {
		return gitprovider.Result{}, err
	}

	// 1. The branch name must be free.
	var ref struct {
		Ref string `json:"ref"`
	}
	switch err := se.call(ctx, http.MethodGet, "/git/ref/heads/"+escapeRef(req.Branch), nil, &ref); {
	case err == nil:
		return gitprovider.Result{}, fmt.Errorf("%w: %s", gitprovider.ErrBranchExists, req.Branch)
	case errors.Is(err, gitprovider.ErrNotFound):
		// free, as expected
	default:
		return gitprovider.Result{}, err
	}

	// 2. The commit to branch from.
	var base commitInfo
	if req.Base != "" {
		base, err = se.commit(ctx, req.Base)
	} else {
		base, err = se.branchHead(ctx)
	}
	if err != nil {
		return gitprovider.Result{}, err
	}

	// 3. What the project folder holds at that commit, for the file checks.
	entries, err := se.folderEntries(ctx, base.Commit.Tree.SHA)
	if err != nil {
		return gitprovider.Result{}, err
	}
	have := map[string]treeEntry{}
	for _, e := range entries {
		have[e.Path] = e
	}
	for i, f := range req.Files {
		cur, exists := have[rels[i]]
		switch {
		case !f.Update && exists:
			return gitprovider.Result{}, fmt.Errorf("%w: %s", gitprovider.ErrFileExists, rels[i])
		case f.Update && !exists:
			return gitprovider.Result{}, fmt.Errorf("%w: %s", gitprovider.ErrFileMissing, rels[i])
		case f.Update && f.Previous != "":
			text, err := se.blob(ctx, cur.SHA)
			if err != nil {
				return gitprovider.Result{}, err
			}
			if normEOL(text) != normEOL(f.Previous) {
				return gitprovider.Result{}, fmt.Errorf("%w: %s", gitprovider.ErrFileChanged, rels[i])
			}
		}
	}

	// 4. Blobs, tree, commit, then the reference (the only step that is visible).
	prefix := ""
	if s.Path != "" {
		prefix = strings.Trim(path.Clean(s.Path), "/") + "/"
	}
	tree := make([]map[string]string, 0, len(req.Files))
	for i, f := range req.Files {
		var blob struct {
			SHA string `json:"sha"`
		}
		err := se.call(ctx, http.MethodPost, "/git/blobs", map[string]string{
			"content": base64.StdEncoding.EncodeToString([]byte(f.Content)), "encoding": "base64",
		}, &blob)
		if err != nil {
			return gitprovider.Result{}, err
		}
		tree = append(tree, map[string]string{"path": prefix + rels[i], "mode": "100644", "type": "blob", "sha": blob.SHA})
	}
	var newTree struct {
		SHA string `json:"sha"`
	}
	if err := se.call(ctx, http.MethodPost, "/git/trees", map[string]any{"base_tree": base.Commit.Tree.SHA, "tree": tree}, &newTree); err != nil {
		return gitprovider.Result{}, err
	}
	var newCommit struct {
		SHA string `json:"sha"`
	}
	if err := se.call(ctx, http.MethodPost, "/git/commits", map[string]any{
		"message": req.Message, "tree": newTree.SHA, "parents": []string{base.SHA},
	}, &newCommit); err != nil {
		return gitprovider.Result{}, err
	}
	err = se.call(ctx, http.MethodPost, "/git/refs", map[string]string{"ref": "refs/heads/" + req.Branch, "sha": newCommit.SHA}, nil)
	var he *HTTPError
	if errors.As(err, &he) && he.Status == http.StatusUnprocessableEntity && strings.Contains(strings.ToLower(he.Message), "already exists") {
		// Someone took the name between the check and the write.
		return gitprovider.Result{}, fmt.Errorf("%w: %s", gitprovider.ErrBranchExists, req.Branch)
	}
	if err != nil {
		return gitprovider.Result{}, err
	}
	return gitprovider.Result{
		Branch: req.Branch, Commit: newCommit.SHA, Repo: s.Owner + "/" + s.Repo,
		URL: s.App.web() + "/" + s.Owner + "/" + s.Repo + "/tree/" + escapeRef(req.Branch),
	}, nil
}

// BranchRequest is gitprovider.BranchRequest, repeated so callers of this package
// need one import less.
type BranchRequest = gitprovider.BranchRequest

func normEOL(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

// escapeRef escapes each segment of a branch name but keeps the slashes.
func escapeRef(ref string) string {
	parts := strings.Split(ref, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}
