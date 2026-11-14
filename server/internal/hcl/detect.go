package hcl

import (
	"io/fs"
	"path"
	"sort"
	"strings"
)

// CommonEnvNames are looked for among folder and file names when the project has no list.
var CommonEnvNames = []string{"sbx", "int", "dev", "test", "stage", "staging", "uat", "qa", "prod"}

// DetectedFolder is one folder of Terraform the scan found and how the proposal reads it.
type DetectedFolder struct {
	Path  string `json:"path"`
	Stack string `json:"stack,omitempty"`
	Env   string `json:"env,omitempty"`
	// Fits is false for a folder the proposed pattern does not describe.
	Fits bool `json:"fits"`
}

// LayoutProposal is what the scan understood. It is a suggestion: the owner accepts or edits it.
type LayoutProposal struct {
	Pattern      string           `json:"pattern"`
	Values       string           `json:"values"`
	Environments []string         `json:"environments"`
	Folders      []DetectedFolder `json:"folders"`
	// Confidence is "high" (every folder fits), "low" (some do not) or "none" (nothing to read).
	Confidence string `json:"confidence"`
}

const (
	detectMaxDepth = 4
	detectMaxDirs  = 500
)

// DetectLayout reads the folder tree and proposes the folder and values patterns. envs are the
// environment names to look for (CommonEnvNames when empty). It reads names only, never contents.
func DetectLayout(fsys fs.FS, envs []string) LayoutProposal {
	if len(envs) == 0 {
		envs = CommonEnvNames
	}
	isEnv := map[string]bool{}
	for _, e := range envs {
		isEnv[e] = true
	}
	tfDirs, tfvars := scanTerraform(fsys)
	out := LayoutProposal{Pattern: DefaultFolderPattern, Values: DefaultValuesPattern, Environments: []string{}, Folders: []DetectedFolder{}, Confidence: "none"}
	if len(tfDirs) == 0 {
		return out
	}

	split := func(d string) []string {
		if d == "" {
			return nil
		}
		return strings.Split(d, "/")
	}
	// The shape most folders have: how deep, and which part (if any) is an environment name.
	type shape struct{ depth, env int }
	shapeOf := func(d string) shape {
		parts := split(d)
		for i := len(parts) - 1; i >= 0; i-- {
			if isEnv[parts[i]] {
				return shape{len(parts), i}
			}
		}
		return shape{len(parts), -1}
	}
	votes := map[shape]int{}
	for _, d := range tfDirs {
		votes[shapeOf(d)]++
	}
	best, bestN := shape{}, -1
	for s, n := range votes {
		// Prefer the larger group; on a tie the one with an environment folder, then the shallower.
		if n > bestN || (n == bestN && (s.env >= 0 && best.env < 0 || s.env >= 0 == (best.env >= 0) && s.depth < best.depth)) {
			best, bestN = s, n
		}
	}
	// Which parts of the best shape vary between folders: that one is the stack.
	distinct := make([]map[string]bool, best.depth)
	for i := range distinct {
		distinct[i] = map[string]bool{}
	}
	for _, d := range tfDirs {
		if shapeOf(d) != best {
			continue
		}
		for i, p := range split(d) {
			distinct[i][p] = true
		}
	}
	segs := make([]string, best.depth)
	stackAt := -1
	for i := range segs {
		switch {
		case i == best.env:
			segs[i] = "{env}"
		case stackAt < 0 && (len(distinct[i]) > 1 || best.depth-btoi(best.env >= 0) == 1):
			segs[i], stackAt = "{stack}", i
		default:
			for v := range distinct[i] {
				segs[i] = v
			}
		}
	}
	if best.depth == 0 {
		out.Pattern = "."
	} else {
		out.Pattern = strings.Join(segs, "/")
	}
	if _, err := NewResolver(Layout{Pattern: out.Pattern}, "dev"); err != nil {
		out.Pattern = DefaultFolderPattern
	}
	r, _ := NewResolver(Layout{Pattern: out.Pattern}, "dev")

	seenEnv := map[string]bool{}
	fits := 0
	for _, d := range tfDirs {
		stack, env, ok := r.match(d)
		if ok && (!r.hasEnv || isEnv[env]) {
			fits++
			if env != "" {
				seenEnv[env] = true
			}
		} else {
			ok = false
		}
		out.Folders = append(out.Folders, DetectedFolder{Path: dirOrDot(d), Stack: stackIf(ok, stack), Env: env, Fits: ok})
	}

	// Values files: which template names the files that exist.
	score := map[string]int{}
	for _, d := range tfDirs {
		stack, env, ok := r.match(d)
		if !ok {
			continue
		}
		_ = stack
		for _, tmpl := range []string{DefaultValuesPattern, "{dir}/{env}.tfvars", "{dir}/terraform.tfvars"} {
			probe := []string{env}
			if env == "" {
				probe = envs
			}
			for _, e := range probe {
				if p, ok := valuesAt(tmpl, d, e); ok && tfvars[p] {
					score[tmpl]++
					if env == "" {
						seenEnv[e] = true
					}
				}
			}
		}
	}
	bestV, bestS := "", 0
	for _, tmpl := range []string{DefaultValuesPattern, "{dir}/{env}.tfvars", "{dir}/terraform.tfvars"} {
		if score[tmpl] > bestS {
			bestV, bestS = tmpl, score[tmpl]
		}
	}
	switch {
	case bestV != "":
		out.Values = bestV
	case len(tfvars) == 0:
		out.Values = ValuesNone
	}
	for _, e := range envs {
		if seenEnv[e] {
			out.Environments = append(out.Environments, e)
		}
	}
	switch {
	case fits == len(tfDirs):
		out.Confidence = "high"
	case fits > 0:
		out.Confidence = "low"
	}
	return out
}

func valuesAt(tmpl, dir, env string) (string, bool) {
	p := strings.ReplaceAll(tmpl, "{env}", env)
	if dir == "" {
		return strings.TrimPrefix(p, "{dir}/"), true
	}
	return strings.ReplaceAll(p, "{dir}", dir), true
}

func stackIf(ok bool, s string) string {
	if ok {
		return s
	}
	return ""
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// scanTerraform lists the folders that hold .tf files (sorted) and the .tfvars files, by name only.
func scanTerraform(fsys fs.FS) ([]string, map[string]bool) {
	var tfDirs []string
	tfvars := map[string]bool{}
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		ents, err := fs.ReadDir(fsys, dirOrDot(dir))
		if err != nil || len(tfDirs) > detectMaxDirs {
			return
		}
		hasTF := false
		for _, e := range ents {
			switch {
			case e.IsDir():
				if depth < detectMaxDepth && !strings.HasPrefix(e.Name(), ".") {
					walk(path.Join(dir, e.Name()), depth+1)
				}
			case strings.HasSuffix(e.Name(), ".tf"):
				hasTF = true
			case strings.HasSuffix(e.Name(), ".tfvars"):
				tfvars[path.Join(dir, e.Name())] = true
			}
		}
		if hasTF {
			tfDirs = append(tfDirs, dir)
		}
	}
	walk("", 0)
	sort.Strings(tfDirs)
	return tfDirs, tfvars
}

// CheckLayout reads the folders of the repository against the patterns and says which fit.
// A folder whose environment part is not in envs does not fit either. An invalid pattern is an error.
func CheckLayout(fsys fs.FS, l Layout, envs []string) ([]DetectedFolder, error) {
	r, err := NewResolver(l, "dev")
	if err != nil {
		return nil, err
	}
	if len(envs) == 0 {
		envs = CommonEnvNames
	}
	isEnv := map[string]bool{}
	for _, e := range envs {
		isEnv[e] = true
	}
	dirs, _ := scanTerraform(fsys)
	out := []DetectedFolder{}
	for _, d := range dirs {
		stack, env, ok := r.match(d)
		ok = ok && (!r.hasEnv || isEnv[env])
		out = append(out, DetectedFolder{Path: dirOrDot(d), Stack: stackIf(ok, stack), Env: env, Fits: ok})
	}
	return out, nil
}
