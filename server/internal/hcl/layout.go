package hcl

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing/fstest"
)

// The layout resolver (8.9). A repository may keep an environment in a folder (`vnet/dev`,
// `dev/vnet`, `dev`) or in a values file (`vnet/env/dev.tfvars`). The parser, importer,
// generator and merge layer all think in one shape: a project folder holds one folder per
// stack, with .tf files and `env/<env>.tfvars` inside. The resolver is the only code that
// knows any other shape: View shows a repository in that one shape for an environment, and
// Real says where a path of that shape lives in the repository. With the default settings
// both are the identity, so nothing changes for a project that does not use them.
const (
	// DefaultFolderPattern is today's layout: one folder per stack, directly under the project.
	DefaultFolderPattern = "{stack}"
	// DefaultValuesPattern is today's values file.
	DefaultValuesPattern = "{dir}/env/{env}.tfvars"
	// ValuesNone means the project keeps no per-environment values files.
	ValuesNone = "none"
	// SingleStack is the stack name used when the folder pattern has no {stack}: every
	// resource of an environment lives in one folder.
	SingleStack = "main"
)

const maxPatternSegments = 4

var literalSegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$`)

type segKind int

const (
	segLit segKind = iota
	segStack
	segEnv
)

type seg struct {
	kind segKind
	lit  string
}

// Resolver maps between the repository's shape and the canonical one, for one environment.
// The nil Resolver is the identity.
type Resolver struct {
	segs     []seg
	values   string // template with {dir} and {env}; "" when none
	env      string
	identity bool
	hasStack bool
	hasEnv   bool
}

// ValidateLayoutPatterns lists what is wrong with the folder and values patterns.
func ValidateLayoutPatterns(folder, values string) []string {
	_, err := NewResolver(Layout{Pattern: folder, Values: values}, "dev")
	if err == nil {
		return nil
	}
	return []string{err.Error()}
}

// NewResolver validates the patterns and builds the resolver for env.
func NewResolver(l Layout, env string) (*Resolver, error) {
	if !ValidEnv(env) {
		return nil, fmt.Errorf("invalid environment name %q", env)
	}
	folder, values := l.Pattern, l.Values
	if folder == "" {
		folder = DefaultFolderPattern
	}
	if values == "" {
		values = DefaultValuesPattern
	}
	r := &Resolver{env: env}
	if folder != "." {
		parts := strings.Split(folder, "/")
		if len(parts) > maxPatternSegments {
			return nil, fmt.Errorf("layout.pattern has more than %d folders", maxPatternSegments)
		}
		for _, p := range parts {
			switch {
			case p == "{stack}":
				if r.hasStack {
					return nil, errors.New("layout.pattern uses {stack} twice")
				}
				r.hasStack = true
				r.segs = append(r.segs, seg{kind: segStack})
			case p == "{env}":
				if r.hasEnv {
					return nil, errors.New("layout.pattern uses {env} twice")
				}
				r.hasEnv = true
				r.segs = append(r.segs, seg{kind: segEnv})
			case strings.ContainsAny(p, "{}"):
				return nil, fmt.Errorf("layout.pattern: %q is not a folder; only {stack} and {env} can stand for a whole folder", p)
			case p == "" || p == "." || p == "..":
				return nil, errors.New("layout.pattern must be folder names separated by /, with no empty, . or .. part")
			case !literalSegment.MatchString(p):
				return nil, fmt.Errorf("layout.pattern: %q is not a plain folder name", p)
			default:
				r.segs = append(r.segs, seg{kind: segLit, lit: p})
			}
		}
	}
	if values != ValuesNone {
		if !strings.HasPrefix(values, "{dir}/") || !strings.HasSuffix(values, ".tfvars") {
			return nil, errors.New(`layout.values must be "none" or start with {dir}/ and end with .tfvars`)
		}
		rest := strings.ReplaceAll(strings.ReplaceAll(values, "{dir}", "x"), "{env}", "x")
		if strings.ContainsAny(rest, "{}\\") || strings.Contains(rest, "..") || strings.Contains(rest, "//") || strings.Count(values, "{dir}") != 1 || strings.Count(values, "{env}") > 1 {
			return nil, errors.New("layout.values may use {dir} once (at the start) and {env} at most once, and no .. or other braces")
		}
		r.values = values
	}
	r.identity = folder == DefaultFolderPattern && values == DefaultValuesPattern
	return r, nil
}

// Identity reports whether the resolver changes nothing.
func (r *Resolver) Identity() bool { return r == nil || r.identity }

// HasStack says whether the folder pattern names the stack. Without it every resource of an
// environment goes to one folder.
func (r *Resolver) HasStack() bool { return r == nil || r.hasStack }

// Stack is the stack a new resource targeting target belongs to under this layout.
func (r *Resolver) Stack(target string) string {
	if r.HasStack() {
		return target
	}
	return SingleStack
}

func (r *Resolver) folderFor(stack, env string) string {
	parts := make([]string, 0, len(r.segs))
	for _, s := range r.segs {
		switch s.kind {
		case segStack:
			parts = append(parts, stack)
		case segEnv:
			parts = append(parts, env)
		default:
			parts = append(parts, s.lit)
		}
	}
	return strings.Join(parts, "/")
}

func (r *Resolver) valuesPath(dir, env string) string {
	p := strings.ReplaceAll(r.values, "{env}", env)
	if dir == "" {
		return strings.TrimPrefix(p, "{dir}/")
	}
	return strings.ReplaceAll(p, "{dir}", dir)
}

// match reads a real folder as (stack, env). The folder "" is the project folder itself.
func (r *Resolver) match(dir string) (stack, env string, ok bool) {
	var parts []string
	if dir != "" {
		parts = strings.Split(dir, "/")
	}
	if len(parts) != len(r.segs) {
		return "", "", false
	}
	stack = SingleStack
	for i, s := range r.segs {
		switch s.kind {
		case segLit:
			if parts[i] != s.lit {
				return "", "", false
			}
		case segStack:
			if validFolder(parts[i]) != nil {
				return "", "", false
			}
			stack = parts[i]
		case segEnv:
			if !ValidEnv(parts[i]) {
				return "", "", false
			}
			env = parts[i]
		}
	}
	return stack, env, true
}

const maxViewFiles = 2000

// View shows fsys in the canonical shape for the resolver's environment: folders are stacks,
// and the values files of every environment found are at <stack>/env/<env>.tfvars.
func (r *Resolver) View(fsys fs.FS) (fs.FS, error) {
	if r.Identity() || fsys == nil {
		return fsys, nil
	}
	out := fstest.MapFS{}
	var dirs []string
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if depth == len(r.segs) {
			dirs = append(dirs, dir)
			return
		}
		ents, err := fs.ReadDir(fsys, dirOrDot(dir))
		if err != nil {
			return
		}
		for _, e := range ents {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				walk(path.Join(dir, e.Name()), depth+1)
				if len(dirs) > maxViewFiles {
					return
				}
			}
		}
	}
	walk("", 0)
	if len(dirs) > maxViewFiles {
		return nil, errors.New("the repository has too many folders for this layout")
	}
	for _, d := range dirs {
		stack, env, ok := r.match(d)
		if !ok {
			continue
		}
		if env == "" || env == r.env {
			ents, err := fs.ReadDir(fsys, dirOrDot(d))
			if err != nil {
				continue
			}
			for _, e := range ents {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".tf") {
					continue
				}
				if b, err := fs.ReadFile(fsys, path.Join(d, e.Name())); err == nil {
					out[path.Join(stack, e.Name())] = &fstest.MapFile{Data: b}
				}
			}
		}
		if r.values == "" {
			continue
		}
		forEnv := env
		if forEnv == "" {
			forEnv = r.env
		}
		if b, err := fs.ReadFile(fsys, r.valuesPath(d, forEnv)); err == nil {
			out[path.Join(stack, "env", forEnv+".tfvars")] = &fstest.MapFile{Data: b}
		}
		if r.hasEnv {
			continue
		}
		// One folder, one values file per environment: the others are listed so that adding a
		// value to one of them does not overwrite it.
		if strings.Contains(r.values, "{env}") {
			vd, vf := path.Split(strings.ReplaceAll(r.valuesPath(d, "\x00"), "\x00", "{env}"))
			ents, err := fs.ReadDir(fsys, dirOrDot(strings.TrimSuffix(vd, "/")))
			if err != nil {
				continue
			}
			pre, suf, _ := strings.Cut(vf, "{env}")
			for _, e := range ents {
				n := e.Name()
				if e.IsDir() || !strings.HasPrefix(n, pre) || !strings.HasSuffix(n, suf) || len(n) <= len(pre)+len(suf) {
					continue
				}
				other := strings.TrimSuffix(strings.TrimPrefix(n, pre), suf)
				if !ValidEnv(other) {
					continue
				}
				if b, err := fs.ReadFile(fsys, path.Join(strings.TrimSuffix(vd, "/"), n)); err == nil {
					out[path.Join(stack, "env", other+".tfvars")] = &fstest.MapFile{Data: b}
				}
			}
		}
	}
	return out, nil
}

func dirOrDot(d string) string {
	if d == "" {
		return "."
	}
	return d
}

// Real says where a path of the canonical shape (<stack>/<file> or <stack>/env/<env>.tfvars)
// lives in the repository. ok is false for a file this layout has no place for.
func (r *Resolver) Real(folder, filename string) (realFolder, realFilename string, ok bool) {
	if r.Identity() {
		return folder, filename, true
	}
	stack, rest := folder, filename
	if i := strings.Index(folder, "/"); i >= 0 {
		stack, rest = folder[:i], folder[i+1:]+"/"+filename
	}
	var real string
	if strings.HasPrefix(rest, "env/") && strings.HasSuffix(rest, ".tfvars") {
		x := strings.TrimSuffix(strings.TrimPrefix(rest, "env/"), ".tfvars")
		if r.values == "" || !ValidEnv(x) {
			return "", "", false
		}
		if x != r.env && !r.hasEnv && !strings.Contains(r.values, "{env}") {
			return "", "", false // one shared values file cannot hold another environment's values
		}
		real = r.valuesPath(r.folderFor(stack, x), x)
	} else {
		real = path.Join(r.folderFor(stack, r.env), rest)
	}
	d := path.Dir(real)
	if d == "." {
		d = ""
	}
	return d, path.Base(real), true
}

// RealPath is Real for one joined path.
func (r *Resolver) RealPath(p string) (string, bool) {
	dir, base := path.Split(p)
	f, n, ok := r.Real(strings.TrimSuffix(dir, "/"), base)
	if !ok {
		return "", false
	}
	if f == "." || f == "" {
		return n, true
	}
	return f + "/" + n, true
}
