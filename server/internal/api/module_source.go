package api

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/Anmewzaa/TerraFlow/server/internal/catalog"
)

// DefaultModuleSourceBase is where a module that names only itself ("resource-group")
// is looked up: the team's module repository, one folder per module.
const DefaultModuleSourceBase = "git::https://github.com/Anmewzaa/infrastructure-terraform-modules.git//modules"

const moduleSourceTTL = time.Minute

// sourceResolver turns a module name and version into the `source` of the generated
// module call. A catalog schema may carry the full source in x-tf-module.source
// ("git::https://...//modules/resource-group"); a bare name is placed under the base.
// A project's catalog is looked up first, then the global one (list returns what the
// project sees, D-060). Each project's view is re-read at most once a minute so an
// imported module is picked up without a restart.
type sourceResolver struct {
	mu     sync.Mutex
	base   string
	list   func(ctx context.Context, projectID string) ([]catalog.Module, error)
	scopes map[string]scopeSources // project id ("" is the global catalog alone)
	now    func() time.Time
}

type scopeSources struct {
	loaded time.Time
	byName map[string]string // name@version -> full source without ?ref=
}

var moduleSources = &sourceResolver{base: DefaultModuleSourceBase, now: time.Now}

// ConfigureModuleSources sets the base for bare module names (empty keeps the default)
// and where the modules a project sees are read from (nil means only the base is used).
func ConfigureModuleSources(base string, list func(ctx context.Context, projectID string) ([]catalog.Module, error)) {
	moduleSources.mu.Lock()
	defer moduleSources.mu.Unlock()
	if base != "" {
		moduleSources.base = strings.TrimRight(base, "/")
	}
	moduleSources.list = list
	moduleSources.scopes = nil
}

func isFullSource(s string) bool { return strings.Contains(s, "::") || strings.Contains(s, "://") }

func (r *sourceResolver) refresh(projectID string) {
	if r.list == nil {
		return
	}
	if sc, ok := r.scopes[projectID]; ok && r.now().Sub(sc.loaded) < moduleSourceTTL {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mods, err := r.list(ctx, projectID)
	if err != nil {
		return // keep what we had; try again at the next call
	}
	if r.scopes == nil {
		r.scopes = map[string]scopeSources{}
	}
	for id, sc := range r.scopes { // forget projects nobody generated for lately
		if r.now().Sub(sc.loaded) >= 10*moduleSourceTTL {
			delete(r.scopes, id)
		}
	}
	m := make(map[string]string, len(mods))
	for _, mod := range mods {
		var s struct {
			Meta struct {
				Source string `json:"source"`
			} `json:"x-tf-module"`
		}
		if json.Unmarshal(mod.SchemaJSON, &s) == nil && isFullSource(s.Meta.Source) {
			m[mod.Name+"@"+mod.Version] = strings.TrimRight(s.Meta.Source, "/")
		}
	}
	r.scopes[projectID] = scopeSources{loaded: r.now(), byName: m}
}

func (r *sourceResolver) build(projectID, name, version string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refresh(projectID)
	src, ok := r.scopes[projectID].byName[name+"@"+version]
	if !ok {
		src = r.base + "/" + name
	}
	if strings.Contains(src, "?ref=") {
		return src
	}
	return src + "?ref=" + version
}

func buildModuleSource(projectID, moduleName, version string) string {
	return moduleSources.build(projectID, moduleName, version)
}
