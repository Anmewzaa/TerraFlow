package githubtest

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// ClientID and ClientSecret are what the fake accepts for user authorization.
const (
	ClientID     = "Iv1.fakeclient"
	ClientSecret = "fake-client-secret"
)

type fuser struct {
	id    int64
	login string
	// installs the user can access; repos the user can access ("owner/name").
	installs map[int64]bool
	repos    map[string]bool
}

// AddUser registers a GitHub account that can reach the given installations and
// repositories. It returns a one-time authorization code for it (what the browser
// brings back after the user authorizes the App).
func (f *Fake) AddUser(id int64, login string, installs []int64, repos ...string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.users == nil {
		f.users, f.codes, f.utokens = map[int64]*fuser{}, map[string]*fuser{}, map[string]*fuser{}
	}
	u := &fuser{id: id, login: login, installs: map[int64]bool{}, repos: map[string]bool{}}
	for _, i := range installs {
		u.installs[i] = true
	}
	for _, r := range repos {
		u.repos[r] = true
	}
	f.users[id] = u
	return f.newCode(u)
}

// NewCode makes another one-time code for a registered user.
func (f *Fake) NewCode(id int64) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.newCode(f.users[id])
}

func (f *Fake) newCode(u *fuser) string {
	f.seq++
	code := fmt.Sprintf("code%04d", f.seq)
	f.codes[code] = u
	return code
}

// RevokeUserTokens makes every user token fail with 401.
func (f *Fake) RevokeUserTokens() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.utokens = map[string]*fuser{}
}

func (f *Fake) exchange(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	u, ok := f.codes[r.Form.Get("code")]
	if r.Form.Get("client_id") != ClientID || r.Form.Get("client_secret") != ClientSecret || !ok {
		// GitHub answers 200 with an error body for a bad or reused code.
		f.writeJSON(w, 200, map[string]any{"error": "bad_verification_code"})
		return
	}
	delete(f.codes, r.Form.Get("code")) // one use only
	f.seq++
	tok := fmt.Sprintf("ghu_fake%04d", f.seq)
	f.utokens[tok] = u
	f.writeJSON(w, 200, map[string]any{"access_token": tok, "token_type": "bearer"})
}

func (f *Fake) userCall(w http.ResponseWriter, r *http.Request, bearer string) {
	u, ok := f.utokens[bearer]
	if !ok {
		f.fail(w, http.StatusUnauthorized, "Bad credentials")
		return
	}
	path := r.URL.Path
	switch {
	case path == "/user":
		f.writeJSON(w, 200, map[string]any{"id": u.id, "login": u.login})
	case path == "/user/installations":
		list := []map[string]any{}
		ids := []int64{}
		for id := range u.installs {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			if in, ok := f.insts[id]; ok {
				list = append(list, map[string]any{"id": in.id, "account": map[string]any{"login": in.account, "type": "User", "id": 1}, "repository_selection": "selected"})
			}
		}
		f.writeJSON(w, 200, map[string]any{"total_count": len(list), "installations": list})
	case strings.HasPrefix(path, "/user/installations/") && strings.HasSuffix(path, "/repositories"):
		idStr := strings.TrimSuffix(strings.TrimPrefix(path, "/user/installations/"), "/repositories")
		id, _ := strconv.ParseInt(idStr, 10, 64)
		in, ok := f.insts[id]
		if !ok || !u.installs[id] {
			f.fail(w, http.StatusNotFound, "Not Found")
			return
		}
		names := []string{}
		for full := range in.repos {
			if u.repos[full] { // installation AND user can reach it
				names = append(names, full)
			}
		}
		sort.Strings(names)
		repos := []map[string]any{}
		for i, full := range names {
			def := "main"
			if rp, ok := f.repos[full]; ok {
				def = rp.defaultBranch
			}
			repos = append(repos, map[string]any{"id": int64(1000 + i), "full_name": full, "default_branch": def, "private": true})
		}
		f.writeJSON(w, 200, map[string]any{"total_count": len(repos), "repositories": repos})
	default:
		f.fail(w, 404, "Not Found")
	}
}
