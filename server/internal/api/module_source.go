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
// The catalog is re-read at most once a minute so an imported module is picked up
// without a restart.
type sourceResolver struct {
	mu     sync.Mutex
	base   string
	list   func(ctx context.Context) ([]catalog.Module, error)
	loaded time.Time
	byName map[string]string // name@version -> full source without ?ref=
	now    func() time.Time
}

var moduleSources = &sourceResolver{base: DefaultModuleSourceBase, now: time.Now}

// ConfigureModuleSources sets the base for bare module names (empty keeps the default)
// and where the catalog is read from (nil means only the base is used).
func ConfigureModuleSources(base string, list func(ctx context.Context) ([]catalog.Module, error)) {
	moduleSources.mu.Lock()
	defer moduleSources.mu.Unlock()
	if base != "" {
		moduleSources.base = strings.TrimRight(base, "/")
	}
	moduleSources.list = list
	moduleSources.loaded = time.Time{}
}

func isFullSource(s string) bool { return strings.Contains(s, "::") || strings.Contains(s, "://") }

func (r *sourceResolver) refresh() {
	if r.list == nil || (!r.loaded.IsZero() && r.now().Sub(r.loaded) < moduleSourceTTL) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mods, err := r.list(ctx)
	if err != nil {
		return // keep what we had; try again at the next call
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
	r.byName, r.loaded = m, r.now()
}

func (r *sourceResolver) build(name, version string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refresh()
	src, ok := r.byName[name+"@"+version]
	if !ok {
		src = r.base + "/" + name
	}
	if strings.Contains(src, "?ref=") {
		return src
	}
	return src + "?ref=" + version
}

func buildModuleSource(moduleName, version string) string {
	return moduleSources.build(moduleName, version)
}
