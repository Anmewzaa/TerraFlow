// Package githubtest is a small fake of the parts of the GitHub API that TerraFlow
// uses: App authentication, installation tokens, and the Git Data API. It checks what
// real GitHub checks (a signed App JWT that is not too old, a token that is known,
// not expired, and limited to its repository), so tests exercise the real protocol.
package githubtest

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Fake is a GitHub for tests. Create it with New and close it with Close.
type Fake struct {
	*httptest.Server

	mu     sync.Mutex
	pub    *rsa.PublicKey
	appID  string
	now    func() time.Time
	seq    int
	tokens map[string]*issued
	insts  map[int64]*installation
	repos  map[string]*repo // "owner/name"

	// Requests logs "METHOD path" of every API call, in order.
	Requests []string
	// TokensMinted counts installation tokens made.
	TokensMinted int
	// RateLimit makes the next call fail with GitHub's rate-limit answer.
	RateLimit bool
	// TruncateTrees makes tree listings report "truncated", as GitHub does for a very
	// large repository.
	TruncateTrees bool
	// Reject401Once makes the next repository call answer 401, as if the token had
	// just expired.
	Reject401Once bool
}

type issued struct {
	repos   map[string]bool
	perms   map[string]string
	expires time.Time
}

type installation struct {
	id      int64
	account string
	repos   map[string]bool
}

type commit struct {
	tree    string
	parents []string
	message string
}

type repo struct {
	defaultBranch string
	blobs         map[string][]byte
	trees         map[string]map[string]string // tree sha -> path -> blob sha (flat)
	commits       map[string]commit
	refs          map[string]string // "heads/main" -> commit sha
}

// New starts a fake that accepts JWTs signed by the key matching pub for appID.
func New(appID string, pub *rsa.PublicKey, now func() time.Time) *Fake {
	f := &Fake{pub: pub, appID: appID, now: now, tokens: map[string]*issued{}, insts: map[int64]*installation{}, repos: map[string]*repo{}}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	return f
}

func sha(parts ...string) string {
	h := sha1.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// AddInstallation registers an installation that may use the named repositories
// ("owner/name").
func (f *Fake) AddInstallation(id int64, account string, repos ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	in := &installation{id: id, account: account, repos: map[string]bool{}}
	for _, r := range repos {
		in.repos[r] = true
	}
	f.insts[id] = in
}

// AddRepo creates a repository whose first commit on "main" holds files (paths
// relative to prefix, which is the project folder inside the repository).
func (f *Fake) AddRepo(full, prefix string, files map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := &repo{defaultBranch: "main", blobs: map[string][]byte{}, trees: map[string]map[string]string{}, commits: map[string]commit{}, refs: map[string]string{}}
	flat := map[string]string{}
	for p, c := range files {
		flat[prefix+p] = r.addBlob([]byte(c))
	}
	tree := r.addTree(flat)
	c := r.addCommit(commit{tree: tree, message: "initial"})
	r.refs["heads/main"] = c
	f.repos[full] = r
}

func (r *repo) addBlob(b []byte) string {
	s := sha("blob", string(b))
	r.blobs[s] = b
	return s
}

func (r *repo) addTree(flat map[string]string) string {
	keys := make([]string, 0, len(flat))
	for k := range flat {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{"tree"}
	for _, k := range keys {
		parts = append(parts, k, flat[k])
	}
	s := sha(parts...)
	cp := map[string]string{}
	for k, v := range flat {
		cp[k] = v
	}
	r.trees[s] = cp
	return s
}

var commitSeq int

func (r *repo) addCommit(c commit) string {
	commitSeq++
	s := sha("commit", c.tree, strings.Join(c.parents, ","), c.message, strconv.Itoa(commitSeq))
	r.commits[s] = c
	return s
}

// Push simulates someone pushing to the default branch: it sets (or, with an empty
// value, removes) files under prefix and returns the new commit.
func (f *Fake) Push(full, prefix string, changes map[string]string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.repos[full]
	head := r.refs["heads/"+r.defaultBranch]
	flat := map[string]string{}
	for k, v := range r.trees[r.commits[head].tree] {
		flat[k] = v
	}
	for p, c := range changes {
		if c == "" {
			delete(flat, prefix+p)
		} else {
			flat[prefix+p] = r.addBlob([]byte(c))
		}
	}
	c := r.addCommit(commit{tree: r.addTree(flat), parents: []string{head}, message: "push"})
	r.refs["heads/"+r.defaultBranch] = c
	return c
}

// Branch returns the files of a branch under prefix (nil if the branch does not exist).
func (f *Fake) Branch(full, prefix, name string) map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.repos[full]
	c, ok := r.refs["heads/"+name]
	if !ok {
		return nil
	}
	out := map[string]string{}
	for p, b := range r.trees[r.commits[c].tree] {
		if strings.HasPrefix(p, prefix) {
			out[strings.TrimPrefix(p, prefix)] = string(r.blobs[b])
		}
	}
	return out
}

// Parents returns the parent commits of a commit.
func (f *Fake) Parents(full, commitSHA string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.repos[full].commits[commitSHA].parents
}

// Head returns the commit of a branch.
func (f *Fake) Head(full, name string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.repos[full].refs["heads/"+name]
}

// ExpireTokens makes every token made so far expired (they answer 401).
func (f *Fake) ExpireTokens() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.tokens {
		t.expires = f.now().Add(-time.Minute)
	}
}

// Token facts, for assertions.
func (f *Fake) TokenFacts() (reposPerToken []string, permsPerToken []map[string]string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.tokens {
		var rs []string
		for r := range t.repos {
			rs = append(rs, r)
		}
		sort.Strings(rs)
		reposPerToken = append(reposPerToken, strings.Join(rs, ","))
		permsPerToken = append(permsPerToken, t.perms)
	}
	return
}

func (f *Fake) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (f *Fake) fail(w http.ResponseWriter, status int, msg string) {
	f.writeJSON(w, status, map[string]string{"message": msg})
}

// jwtOK verifies an App JWT the way GitHub does: signature, issuer and lifetime.
func (f *Fake) jwtOK(tok string) (bool, string) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return false, "A JSON web token could not be decoded"
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false, "A JSON web token could not be decoded"
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(f.pub, crypto.SHA256, sum[:], sig) != nil {
		return false, "Bad credentials"
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false, "A JSON web token could not be decoded"
	}
	var c struct {
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
		Iss string `json:"iss"`
	}
	if json.Unmarshal(raw, &c) != nil || c.Iss != f.appID {
		return false, "Bad credentials"
	}
	now := f.now().Unix()
	if c.Exp <= now {
		return false, "'Expiration time' claim ('exp') must be a numeric value representing the future time at which the assertion expires"
	}
	if c.Exp-c.Iat > 11*60 {
		return false, "'Expiration time' claim ('exp') is too far in the future"
	}
	return true, ""
}

func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Requests = append(f.Requests, r.Method+" "+r.URL.Path)
	if f.RateLimit {
		f.RateLimit = false
		w.Header().Set("X-RateLimit-Remaining", "0")
		f.fail(w, http.StatusForbidden, "API rate limit exceeded")
		return
	}
	bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if bearer == "" || r.Header.Get("Authorization") == bearer {
		f.fail(w, http.StatusUnauthorized, "Requires authentication")
		return
	}
	path := r.URL.Path

	// ---- App-level endpoints (JWT)
	if strings.HasPrefix(path, "/app/installations/") {
		if ok, msg := f.jwtOK(bearer); !ok {
			f.fail(w, http.StatusUnauthorized, msg)
			return
		}
		rest := strings.TrimPrefix(path, "/app/installations/")
		idStr, tail, _ := strings.Cut(rest, "/")
		id, _ := strconv.ParseInt(idStr, 10, 64)
		in, ok := f.insts[id]
		if !ok {
			f.fail(w, http.StatusNotFound, "Not Found")
			return
		}
		if tail == "" && r.Method == http.MethodGet {
			f.writeJSON(w, 200, map[string]any{"id": in.id, "account": map[string]any{"login": in.account, "type": "User", "id": 1}, "repository_selection": "selected"})
			return
		}
		if tail == "access_tokens" && r.Method == http.MethodPost {
			var body struct {
				Repositories []string          `json:"repositories"`
				Permissions  map[string]string `json:"permissions"`
			}
			raw, _ := io.ReadAll(r.Body)
			json.Unmarshal(raw, &body)
			t := &issued{repos: map[string]bool{}, perms: body.Permissions, expires: f.now().Add(time.Hour)}
			for _, name := range body.Repositories {
				full := in.account + "/" + name
				if !in.repos[full] {
					f.fail(w, http.StatusUnprocessableEntity, "repository not accessible to the installation")
					return
				}
				t.repos[full] = true
			}
			f.seq++
			f.TokensMinted++
			tok := fmt.Sprintf("ghs_fake%04d", f.seq)
			f.tokens[tok] = t
			f.writeJSON(w, 201, map[string]any{"token": tok, "expires_at": t.expires.UTC().Format(time.RFC3339)})
			return
		}
		f.fail(w, 404, "Not Found")
		return
	}

	// ---- Repository endpoints (installation token)
	t, ok := f.tokens[bearer]
	if !ok || !t.expires.After(f.now()) || f.Reject401Once {
		f.Reject401Once = false
		f.fail(w, http.StatusUnauthorized, "Bad credentials")
		return
	}
	if !strings.HasPrefix(path, "/repos/") {
		f.fail(w, 404, "Not Found")
		return
	}
	parts := strings.SplitN(strings.TrimPrefix(path, "/repos/"), "/", 3)
	if len(parts) < 2 {
		f.fail(w, 404, "Not Found")
		return
	}
	full := parts[0] + "/" + parts[1]
	rp, exists := f.repos[full]
	if !exists || !t.repos[full] {
		f.fail(w, http.StatusNotFound, "Not Found") // like GitHub: no hint that it exists
		return
	}
	tail := ""
	if len(parts) == 3 {
		tail = parts[2]
	}
	write := r.Method != http.MethodGet
	if write && t.perms["contents"] != "write" {
		f.fail(w, http.StatusForbidden, "Resource not accessible by integration")
		return
	}
	f.repoCall(w, r, rp, tail)
}

func (f *Fake) repoCall(w http.ResponseWriter, r *http.Request, rp *repo, tail string) {
	body := map[string]json.RawMessage{}
	if r.Method == http.MethodPost {
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &body)
	}
	str := func(k string) string {
		var s string
		json.Unmarshal(body[k], &s)
		return s
	}
	switch {
	case tail == "" && r.Method == http.MethodGet:
		f.writeJSON(w, 200, map[string]any{"default_branch": rp.defaultBranch})

	case strings.HasPrefix(tail, "commits/") && r.Method == http.MethodGet:
		name := strings.TrimPrefix(tail, "commits/")
		c, ok := rp.refs["heads/"+name]
		if !ok {
			f.fail(w, 404, "Not Found")
			return
		}
		f.writeJSON(w, 200, commitJSON(c, rp.commits[c]))

	case strings.HasPrefix(tail, "git/commits/") && r.Method == http.MethodGet:
		c := strings.TrimPrefix(tail, "git/commits/")
		cm, ok := rp.commits[c]
		if !ok {
			f.fail(w, 404, "Not Found")
			return
		}
		f.writeJSON(w, 200, commitJSON(c, cm))

	case strings.HasPrefix(tail, "git/trees/") && r.Method == http.MethodGet:
		s := strings.TrimPrefix(tail, "git/trees/")
		flat, ok := rp.trees[s]
		if !ok {
			f.fail(w, 404, "Not Found")
			return
		}
		keys := make([]string, 0, len(flat))
		for k := range flat {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		entries := []map[string]any{}
		for _, k := range keys {
			entries = append(entries, map[string]any{"path": k, "mode": "100644", "type": "blob", "sha": flat[k], "size": len(rp.blobs[flat[k]])})
		}
		f.writeJSON(w, 200, map[string]any{"sha": s, "tree": entries, "truncated": f.TruncateTrees})

	case strings.HasPrefix(tail, "git/blobs/") && r.Method == http.MethodGet:
		b, ok := rp.blobs[strings.TrimPrefix(tail, "git/blobs/")]
		if !ok {
			f.fail(w, 404, "Not Found")
			return
		}
		f.writeJSON(w, 200, map[string]any{"content": base64.StdEncoding.EncodeToString(b), "encoding": "base64"})

	case strings.HasPrefix(tail, "git/ref/heads/") && r.Method == http.MethodGet:
		name := strings.TrimPrefix(tail, "git/ref/heads/")
		if c, ok := rp.refs["heads/"+name]; ok {
			f.writeJSON(w, 200, map[string]any{"ref": "refs/heads/" + name, "object": map[string]string{"sha": c}})
			return
		}
		f.fail(w, 404, "Not Found")

	case tail == "git/blobs" && r.Method == http.MethodPost:
		raw, err := base64.StdEncoding.DecodeString(str("content"))
		if err != nil || str("encoding") != "base64" {
			f.fail(w, 422, "bad blob")
			return
		}
		f.writeJSON(w, 201, map[string]string{"sha": rp.addBlob(raw)})

	case tail == "git/trees" && r.Method == http.MethodPost:
		flat := map[string]string{}
		for k, v := range rp.trees[str("base_tree")] {
			flat[k] = v
		}
		var entries []struct{ Path, Mode, Type, SHA string }
		json.Unmarshal(body["tree"], &entries)
		for _, e := range entries {
			if _, ok := rp.blobs[e.SHA]; !ok || e.Mode != "100644" || e.Type != "blob" {
				f.fail(w, 422, "bad tree entry")
				return
			}
			flat[e.Path] = e.SHA
		}
		f.writeJSON(w, 201, map[string]string{"sha": rp.addTree(flat)})

	case tail == "git/commits" && r.Method == http.MethodPost:
		var parents []string
		json.Unmarshal(body["parents"], &parents)
		if _, ok := rp.trees[str("tree")]; !ok {
			f.fail(w, 422, "tree missing")
			return
		}
		for _, p := range parents {
			if _, ok := rp.commits[p]; !ok {
				f.fail(w, 422, "parent missing")
				return
			}
		}
		f.writeJSON(w, 201, map[string]string{"sha": rp.addCommit(commit{tree: str("tree"), parents: parents, message: str("message")})})

	case tail == "git/refs" && r.Method == http.MethodPost:
		ref := str("ref")
		if !strings.HasPrefix(ref, "refs/heads/") {
			f.fail(w, 422, "bad ref")
			return
		}
		key := strings.TrimPrefix(ref, "refs/")
		if _, taken := rp.refs[key]; taken {
			f.fail(w, 422, "Reference already exists")
			return
		}
		if _, ok := rp.commits[str("sha")]; !ok {
			f.fail(w, 422, "Object does not exist")
			return
		}
		rp.refs[key] = str("sha")
		f.writeJSON(w, 201, map[string]any{"ref": ref, "object": map[string]string{"sha": str("sha")}})

	default:
		f.fail(w, 404, "Not Found")
	}
}

func commitJSON(sha string, c commit) map[string]any {
	return map[string]any{"sha": sha, "commit": map[string]any{"tree": map[string]string{"sha": c.tree}, "message": c.message}}
}
