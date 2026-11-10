package hcl

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func settingsOrg() *ProjectSettings {
	return &ProjectSettings{Variables: []ProjectVariable{
		{Name: "org", Description: "Organisation short name.", Values: map[string]string{"dev": "aic", "prod": "aic"}},
		{Name: "location", Values: map[string]string{"dev": "southeastasia"}},
	}}
}

func rgNode(name string, values map[string]interface{}) GenerateNode {
	return GenerateNode{ID: name, ModuleName: "resource-group", InstanceName: name, TargetFolder: "common", Source: "../modules/resource-group", Values: values}
}

func filesByPath(t *testing.T, changed []ChangedFile) map[string]ChangedFile {
	t.Helper()
	m := map[string]ChangedFile{}
	for _, f := range changed {
		m[f.Folder+"/"+f.Filename] = f
		if _, d := hclsyntax.ParseConfig([]byte(f.Content), f.Filename, hcl.InitialPos); d.HasErrors() {
			t.Fatalf("%s/%s does not parse: %s\n%s", f.Folder, f.Filename, d.Error(), f.Content)
		}
	}
	return m
}

func TestTokensBecomeVariableReferences(t *testing.T) {
	vars := settingsOrg().varSet()
	hit := map[string]bool{}
	cases := []struct {
		in   string
		want interface{}
	}{
		{"{org}", Expr("var.org")},
		{"rg-{org}-dev", Expr(`"rg-${var.org}-dev"`)},
		{"{org}-{location}", Expr(`"${var.org}-${var.location}"`)},
		{"keep {unknown} as typed", "keep {unknown} as typed"},
		{"no tokens", "no tokens"},
		{"cost$" + "{org}", Expr(`"cost\u0024${var.org}"`)}, // a $ before a token must not become $${
		{`say "hi" {org}`, Expr(`"say \"hi\" ${var.org}"`)},
	}
	for _, c := range cases {
		if got := substituteString(c.in, vars, hit); got != c.want {
			t.Errorf("%q -> %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestApplyWritesVariableDeclarationsAndValuesAndUsesTheVariable(t *testing.T) {
	in := GenerateInput{Settings: settingsOrg(), Nodes: []GenerateNode{
		rgNode("rg_main", map[string]interface{}{"name": "rg-{org}-app", "location": "{location}", "tags": map[string]interface{}{"org": "{org}"}}),
	}}
	changed, err := Apply(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := filesByPath(t, changed)
	call := f["common/main.tf"].Content
	for _, want := range []string{`name`, `"rg-${var.org}-app"`, `var.location`, `var.org`} {
		if !strings.Contains(call, want) {
			t.Errorf("main.tf lacks %s:\n%s", want, call)
		}
	}
	if strings.Contains(call, "{org}") || strings.Contains(call, "{location}") {
		t.Errorf("a token was left in the module call:\n%s", call)
	}
	vars := f["common/variables.tf"].Content
	if !strings.Contains(vars, "variable \"org\" {\n  type        = string\n  description = \"Organisation short name.\"\n}\n") || !strings.Contains(vars, "variable \"location\" {\n  type = string\n}\n") {
		t.Errorf("variables.tf:\n%s", vars)
	}
	dev, prod := f["common/env/dev.tfvars"].Content, f["common/env/prod.tfvars"].Content
	if dev != "location = \"southeastasia\"\norg = \"aic\"\n" {
		t.Errorf("dev.tfvars:\n%q", dev)
	}
	// location has no prod value: prod gets only org, and the variable is not invented.
	if prod != "org = \"aic\"\n" {
		t.Errorf("prod.tfvars:\n%q", prod)
	}
	for _, p := range []string{"common/variables.tf", "common/env/dev.tfvars"} {
		if f[p].Status != FileAdded {
			t.Errorf("%s status %q, want added", p, f[p].Status)
		}
	}
}

func TestExistingVariablesAndValuesAreNeverRewritten(t *testing.T) {
	repo := []GeneratedFile{
		{Folder: "common", Filename: "main.tf", Content: "# my own file\nmodule \"old\" {\n  source = \"x\"\n}\n"},
		// org is declared in another file than variables.tf: it must not be declared again.
		{Folder: "common", Filename: "vars.tf", Content: "variable \"org\" {\n  type = string\n}\n"},
		{Folder: "common/env", Filename: "dev.tfvars", Content: "# team values\norg = \"someone-else\"\n"},
	}
	in := GenerateInput{Settings: settingsOrg(), Nodes: []GenerateNode{
		rgNode("rg_main", map[string]interface{}{"name": "rg-{org}", "location": "{location}"}),
	}}
	changed, err := Apply(in, repo)
	if err != nil {
		t.Fatal(err)
	}
	f := filesByPath(t, changed)
	if got := f["common/variables.tf"].Content; strings.Contains(got, `variable "org"`) || !strings.Contains(got, `variable "location"`) {
		t.Errorf("variables.tf must declare only the missing variable:\n%s", got)
	}
	dev := f["common/env/dev.tfvars"]
	if dev.Status != FileModified {
		t.Errorf("dev.tfvars status %q, want modified", dev.Status)
	}
	if !strings.HasPrefix(dev.Content, "# team values\norg = \"someone-else\"\n") || strings.Count(dev.Content, "org =") != 1 {
		t.Errorf("an existing value must stay untouched:\n%s", dev.Content)
	}
	if !strings.Contains(dev.Content, `location = "southeastasia"`) {
		t.Errorf("the missing value must be added:\n%s", dev.Content)
	}
	if got := f["common/main.tf"].Content; !strings.HasPrefix(got, "# my own file\nmodule \"old\" {\n  source = \"x\"\n}\n") {
		t.Errorf("the existing file must keep its bytes:\n%s", got)
	}
	if _, ok := f["common/vars.tf"]; ok {
		t.Error("vars.tf was not touched and must not be returned")
	}
}

func TestStarterFilesOnlyForNewFoldersThatLackThem(t *testing.T) {
	s := &ProjectSettings{Starters: []StarterFile{
		{Filename: "providers.tf", Content: "provider \"azurerm\" {\n  features {}\n}\n"},
		{Filename: "backend.tf", Content: "terraform {\n  backend \"azurerm\" {\n    key = \"{folder}.tfstate\"\n  }\n}\n"},
	}}
	repo := []GeneratedFile{{Folder: "common", Filename: "providers.tf", Content: "# mine\n"}}
	in := GenerateInput{Settings: s, Nodes: []GenerateNode{rgNode("rg_main", map[string]interface{}{"name": "rg-a"})}}
	changed, err := Apply(in, repo)
	if err != nil {
		t.Fatal(err)
	}
	f := filesByPath(t, changed)
	if _, ok := f["common/providers.tf"]; ok {
		t.Error("an existing providers.tf must be left alone")
	}
	if got := f["common/backend.tf"].Content; !strings.Contains(got, `key = "common.tfstate"`) {
		t.Errorf("backend.tf:\n%s", got)
	}
}

func TestWithoutSettingsNothingChanges(t *testing.T) {
	nodes := []GenerateNode{rgNode("rg_main", map[string]interface{}{"name": "rg-{org}"})}
	a, err := Apply(GenerateInput{Nodes: nodes}, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Apply(GenerateInput{Nodes: nodes, Settings: &ProjectSettings{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 1 || len(b) != 1 || a[0].Content != b[0].Content || !strings.Contains(a[0].Content, "{org}") {
		t.Fatalf("empty settings must behave as no settings, and a token stays literal:\n%v\n%v", a, b)
	}
}

func TestSettingsValidation(t *testing.T) {
	ok := `{"layout":{"mode":"single","folder":"terraform","file":"main.tf","files":{"resource-group":"rg.tf"}},
	        "variables":[{"name":"org","values":{"dev":"aic"}}],
	        "starters":[{"filename":"providers.tf","content":"provider \"azurerm\" {\n  features {}\n}\n"}]}`
	s, err := ParseSettings([]byte(ok))
	if err != nil || len(s.Validate()) != 0 {
		t.Fatalf("a good document: %v %v", err, s.Validate())
	}
	bad := map[string]string{
		"unknown key":      `{"layout":{"colour":"red"}}`,
		"mode":             `{"layout":{"mode":"mystery"}}`,
		"nested folder":    `{"layout":{"mode":"single","folder":"a/b"}}`,
		"traversal":        `{"layout":{"folder":".."}}`,
		"file":             `{"layout":{"file":"main.txt"}}`,
		"variable name":    `{"variables":[{"name":"Org"}]}`,
		"reserved":         `{"variables":[{"name":"source"}]}`,
		"duplicate":        `{"variables":[{"name":"a"},{"name":"a"}]}`,
		"env":              `{"variables":[{"name":"a","values":{"../x":"1"}}]}`,
		"multiline value":  `{"variables":[{"name":"a","values":{"dev":"x\ny"}}]}`,
		"starter not hcl":  `{"starters":[{"filename":"providers.tf","content":"provider {{{"}]}`,
		"starter filename": `{"starters":[{"filename":"../x.tf","content":""}]}`,
	}
	for name, doc := range bad {
		s, err := ParseSettings([]byte(doc))
		if err == nil && len(s.Validate()) == 0 {
			t.Errorf("%s: accepted %s", name, doc)
		}
	}
	if s, err := ParseSettings(nil); err != nil || len(s.Validate()) != 0 {
		t.Errorf("empty settings: %v %v", err, s)
	}
}

func TestProjectProvidersWriteProvidersTfToANewFolderUnlessOneIsChosenExplicitly(t *testing.T) {
	az := ProjectProvider{ID: "azurerm", Version: "~> 4.0", Source: "hashicorp/azurerm", Block: "provider \"azurerm\" {\n  features {}\n}"}
	in := GenerateInput{Settings: &ProjectSettings{Providers: []ProjectProvider{az}}, Nodes: []GenerateNode{rgNode("rg_main", map[string]interface{}{"name": "rg-a"})}}
	changed, err := Apply(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	f := filesByPath(t, changed)
	got := f["common/providers.tf"].Content
	if !strings.Contains(got, `source  = "hashicorp/azurerm"`) || !strings.Contains(got, `version = "~> 4.0"`) || !strings.Contains(got, "features {}") {
		t.Errorf("providers.tf:\n%s", got)
	}

	// An existing providers.tf is never touched, and an explicit starter of that name wins.
	changed, _ = Apply(in, []GeneratedFile{{Folder: "common", Filename: "providers.tf", Content: "# mine\n"}})
	if _, ok := filesByPath(t, changed)["common/providers.tf"]; ok {
		t.Error("an existing providers.tf must be left alone")
	}
	in.Settings.Starters = []StarterFile{{Filename: "providers.tf", Content: "# chosen by the owner\n"}}
	changed, _ = Apply(in, nil)
	if got := filesByPath(t, changed)["common/providers.tf"].Content; got != "# chosen by the owner\n" {
		t.Errorf("an explicit starter must win:\n%s", got)
	}
}

func TestProviderSettingsValidation(t *testing.T) {
	for name, doc := range map[string]string{
		"id":        `{"providers":[{"id":"Azure"}]}`,
		"duplicate": `{"providers":[{"id":"aws"},{"id":"aws"}]}`,
		"version":   `{"providers":[{"id":"aws","version":"latest"}]}`,
	} {
		s, err := ParseSettings([]byte(doc))
		if err == nil && len(s.Validate()) == 0 {
			t.Errorf("%s accepted: %s", name, doc)
		}
	}
	// Source and Block cannot be sent by a client: unknown to the JSON document.
	if _, err := ParseSettings([]byte(`{"providers":[{"id":"aws","source":"evil/aws"}]}`)); err == nil {
		t.Error("a client must not be able to set a provider's source")
	}
	if s, err := ParseSettings([]byte(`{"providers":[{"id":"azurerm","version":"~> 4.0"}]}`)); err != nil || len(s.Validate()) != 0 {
		t.Errorf("a good provider: %v %v", err, s.Validate())
	}
}
