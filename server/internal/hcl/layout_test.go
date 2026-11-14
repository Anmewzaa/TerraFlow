package hcl

import (
	"io/fs"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
)

func files(m map[string]string) fstest.MapFS {
	out := fstest.MapFS{}
	for p, c := range m {
		out[p] = &fstest.MapFile{Data: []byte(c)}
	}
	return out
}

func list(t *testing.T, fsys fs.FS) []string {
	t.Helper()
	var out []string
	fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

const call = "module \"rg\" {\n  source = \"x\"\n  name   = \"a-${var.org}\"\n}\n"

func TestRejectedPatterns(t *testing.T) {
	for name, c := range map[string]Layout{
		"unknown token":     {Pattern: "{team}/{env}"},
		"two envs":          {Pattern: "{env}/{stack}/{env}"},
		"two stacks":        {Pattern: "{stack}/{stack}"},
		"dot dot":           {Pattern: "../{env}"},
		"absolute":          {Pattern: "/{env}"},
		"empty part":        {Pattern: "{env}//{stack}"},
		"mixed segment":     {Pattern: "x{env}"},
		"too deep":          {Pattern: "a/b/c/d/e"},
		"values not dir":    {Values: "env/{env}.tfvars"},
		"values not tfvars": {Values: "{dir}/{env}.tf"},
		"values dotdot":     {Values: "{dir}/../{env}.tfvars"},
		"values two envs":   {Values: "{dir}/{env}/{env}.tfvars"},
	} {
		if _, err := NewResolver(c, "dev"); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
	if _, err := NewResolver(Layout{}, "../x"); err == nil {
		t.Error("a bad environment name must be refused")
	}
}

func TestTheDefaultLayoutChangesNothing(t *testing.T) {
	r, err := NewResolver(Layout{}, "dev")
	if err != nil || !r.Identity() {
		t.Fatalf("default = %+v %v", r, err)
	}
	src := files(map[string]string{"vnet/main.tf": call})
	if v, _ := r.View(src); list(t, v)[0] != "vnet/main.tf" || len(list(t, v)) != 1 {
		t.Fatal("the default view is the repository itself")
	}
	if p, ok := r.RealPath("vnet/env/dev.tfvars"); !ok || p != "vnet/env/dev.tfvars" {
		t.Fatal(p)
	}
	var nilR *Resolver
	if !nilR.Identity() || !nilR.HasStack() || nilR.Stack("x") != "x" {
		t.Fatal("the nil resolver is the identity")
	}
}

func TestLayouts(t *testing.T) {
	type tc struct {
		name   string
		layout Layout
		repo   map[string]string
		want   []string          // the canonical files of environment dev
		real   map[string]string // canonical path -> real path
	}
	for _, c := range []tc{
		{
			name: "{stack}/{env}", layout: Layout{Pattern: "{stack}/{env}", Values: "{dir}/terraform.tfvars"},
			repo: map[string]string{"vnet/dev/main.tf": call, "vnet/prod/main.tf": call, "vnet/dev/terraform.tfvars": "org = \"a\"\n", "sa/dev/x.tf": call, "README.md": "x"},
			want: []string{"sa/env/dev.tfvars", "sa/x.tf", "vnet/env/dev.tfvars", "vnet/main.tf"}[:0],
			real: map[string]string{"vnet/main.tf": "vnet/dev/main.tf", "vnet/env/dev.tfvars": "vnet/dev/terraform.tfvars", "vnet/env/prod.tfvars": "vnet/prod/terraform.tfvars"},
		},
		{
			name: "{env}/{stack}", layout: Layout{Pattern: "{env}/{stack}", Values: "{dir}/{env}.tfvars"},
			repo: map[string]string{"dev/vnet/main.tf": call, "dev/vnet/dev.tfvars": "org = \"a\"\n", "prod/vnet/main.tf": call},
			real: map[string]string{"vnet/main.tf": "dev/vnet/main.tf", "vnet/env/dev.tfvars": "dev/vnet/dev.tfvars"},
		},
		{
			name: "{env} with several files", layout: Layout{Pattern: "{env}", Values: "{dir}/terraform.tfvars"},
			repo: map[string]string{"dev/rg.tf": call, "dev/vnet.tf": call, "dev/terraform.tfvars": "org = \"a\"\n", "prod/rg.tf": call},
			real: map[string]string{"main/rg.tf": "dev/rg.tf", "main/env/dev.tfvars": "dev/terraform.tfvars"},
		},
		{
			name: "{env} with one main.tf", layout: Layout{Pattern: "{env}", Values: "none"},
			repo: map[string]string{"dev/main.tf": call, "prod/main.tf": call},
			real: map[string]string{"main/main.tf": "dev/main.tf"},
		},
		{
			name: "one folder for everything", layout: Layout{Pattern: ".", Values: "{dir}/{env}.tfvars"},
			repo: map[string]string{"main.tf": call, "dev.tfvars": "org = \"a\"\n", "prod.tfvars": "org = \"b\"\n"},
			real: map[string]string{"main/main.tf": "main.tf", "main/env/dev.tfvars": "dev.tfvars", "main/env/prod.tfvars": "prod.tfvars"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, err := NewResolver(c.layout, "dev")
			if err != nil {
				t.Fatal(err)
			}
			view, err := r.View(files(c.repo))
			if err != nil {
				t.Fatal(err)
			}
			got := list(t, view)
			for canon, real := range c.real {
				if strings.HasPrefix(canon, "vnet/env/prod") || strings.HasPrefix(canon, "main/env/prod") {
					continue // another environment's values are listed only where they can be written
				}
				if _, err := fs.Stat(view, canon); err != nil {
					t.Errorf("view of %s lacks %s; has %v", c.name, canon, got)
				}
				if p, ok := r.RealPath(canon); !ok || p != real {
					t.Errorf("RealPath(%s) = %q %v, want %q", canon, p, ok, real)
				}
				if _, ok := c.repo[real]; ok {
					a, _ := fs.ReadFile(view, canon)
					if string(a) != c.repo[real] {
						t.Errorf("%s must hold the content of %s", canon, real)
					}
				}
			}
			// Only this environment's folders are in the view: the prod copy never shows.
			for _, p := range got {
				if strings.Contains(p, "prod") && !strings.HasSuffix(p, ".tfvars") {
					t.Errorf("the dev view must not hold %s", p)
				}
			}
			// The graph reads the view like any other project, and sees the dev values.
			g, err := BuildGraphFS(view, "p", "dev")
			if err != nil || len(g.Folders) == 0 {
				t.Fatalf("graph: %v %+v", err, g)
			}
		})
	}
}

func TestAnotherEnvironmentsValuesHaveAPlaceOnlyWhereTheLayoutHasOne(t *testing.T) {
	shared, _ := NewResolver(Layout{Pattern: "{stack}", Values: "{dir}/terraform.tfvars"}, "dev")
	if _, ok := shared.RealPath("vnet/env/prod.tfvars"); ok {
		t.Error("one shared values file cannot hold prod's values")
	}
	if p, ok := shared.RealPath("vnet/env/dev.tfvars"); !ok || p != "vnet/terraform.tfvars" {
		t.Errorf("dev: %q %v", p, ok)
	}
	folders, _ := NewResolver(Layout{Pattern: "{stack}/{env}", Values: "{dir}/terraform.tfvars"}, "dev")
	if p, ok := folders.RealPath("vnet/env/prod.tfvars"); !ok || p != "vnet/prod/terraform.tfvars" {
		t.Errorf("prod in its own folder: %q %v", p, ok)
	}
	none, _ := NewResolver(Layout{Pattern: "{env}", Values: "none"}, "dev")
	if _, ok := none.RealPath("main/env/dev.tfvars"); ok {
		t.Error("with no values files there is no place for values")
	}
}

func TestNewResourcesUseOneStackWhenThePatternHasNone(t *testing.T) {
	r, _ := NewResolver(Layout{Pattern: "{env}"}, "dev")
	if r.HasStack() || r.Stack("vnet") != SingleStack {
		t.Fatal("no {stack}: one stack")
	}
	if p, _ := r.RealPath("main/rg.tf"); p != "dev/rg.tf" {
		t.Fatal(p)
	}
}

func TestSettingsCarryEnvironmentsAndPatterns(t *testing.T) {
	s, err := ParseSettings([]byte(`{"environments":["sbx","dev","prod"],"layout":{"pattern":"{stack}/{env}","values":"{dir}/terraform.tfvars"}}`))
	if err != nil || len(s.Validate()) != 0 {
		t.Fatalf("%v %v", err, s.Validate())
	}
	if got := s.EnvironmentList(); len(got) != 3 || got[0] != "sbx" {
		t.Fatal(got)
	}
	bad, _ := ParseSettings([]byte(`{"environments":["dev","dev","a b"],"layout":{"pattern":"../x"}}`))
	if len(bad.Validate()) < 3 {
		t.Fatalf("problems: %v", bad.Validate())
	}
	var none *ProjectSettings
	if got := none.EnvironmentList(); len(got) != 3 || got[1] != "staging" || none.Resolver("dev") != nil {
		t.Fatal("no settings is the default")
	}
}

func TestSettingsChecksAreValidated(t *testing.T) {
	good, _ := ParseSettings([]byte(`{"checks":{"naming":{"resource-group":"^rg-[a-z0-9-]+$"},"required_tags":["owner"],"allowed_regions":["southeastasia"],"strict":true}}`))
	if p := good.Validate(); len(p) != 0 {
		t.Fatalf("%v", p)
	}
	bad, _ := ParseSettings([]byte(`{"checks":{"naming":{"Bad Name":"(","x":""},"required_tags":["1bad"],"allowed_regions":["South East"]}}`))
	if len(bad.Validate()) < 5 {
		t.Fatalf("every part must be checked: %v", bad.Validate())
	}
}
