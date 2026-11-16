package stacktpl

import (
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/store"
	"github.com/Anmewzaa/TerraFlow/server/internal/validate"
)

func goodTemplate(t *testing.T) Template {
	t.Helper()
	tpl, problems := Build("Private endpoint group", canvas(), []string{"s", "p"}, nil, nil)
	if len(problems) != 0 {
		t.Fatalf("%v", problems)
	}
	return tpl
}

func TestCheckAcceptsABuiltTemplate(t *testing.T) {
	if p := Check(goodTemplate(t)); len(p) != 0 {
		t.Fatalf("a template made here passes its own gate: %v", p)
	}
}

func TestCheckRefusesHostileInput(t *testing.T) {
	cases := map[string]func(*Template){
		"bad name":      func(x *Template) { x.Name = "../etc" },
		"no nodes":      func(x *Template) { x.Nodes = nil },
		"path folder":   func(x *Template) { x.Nodes[0].TargetFolder = "../../x" },
		"bad filename":  func(x *Template) { x.Nodes[0].Filename = "/etc/passwd" },
		"bad name node": func(x *Template) { x.Nodes[0].InstanceName = `a" { }` },
		"two exprs": func(x *Template) {
			x.Nodes[0].Values["name"] = map[string]interface{}{"$expr": "var.a }\nresource \"x\" \"y\" {"}
		},
		"existing block": func(x *Template) {
			x.Nodes[0].Existing = &store.ExistingRef{Folder: "a", Filename: "main.tf", Label: "x"}
		},
		"orphan arrow":   func(x *Template) { x.Edges = append(x.Edges, store.CanvasEdge{ID: "z", From: "s", To: "nope"}) },
		"unknown parent": func(x *Template) { x.Nodes[0].ParentID = "nope" },
		"bad slot kind":  func(x *Template) { x.Slots = append(x.Slots, Slot{Name: "k", Kind: "exec", Node: "sn"}) },
		"param on nothing": func(x *Template) {
			x.Params = append(x.Params, Param{Name: "p", Node: "ghost", Field: "name"})
		},
		"param name": func(x *Template) { x.Params = append(x.Params, Param{Name: "A B", Node: "sn", Field: "size"}) },
		"too many": func(x *Template) {
			for i := 0; i <= MaxNodes; i++ {
				x.Nodes = append(x.Nodes, store.CanvasNode{ID: "n" + strings.Repeat("x", i%5) + string(rune('a'+i%26)) + itoa(i), InstanceName: "a"})
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			tpl := goodTemplate(t)
			mutate(&tpl)
			if p := Check(tpl); len(p) == 0 {
				t.Fatal("the template was accepted")
			}
		})
	}
}

func itoa(n int) string {
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s + "z"
}

func TestScanFindsSecretsWithoutQuotingThem(t *testing.T) {
	const key = "-----BEGIN RSA PRIVATE KEY-----\nMIIabc"
	tpl := goodTemplate(t)
	tpl.Nodes[0].Values["admin_password"] = "hunter2hunter2"
	tpl.Nodes[0].Values["blob"] = key
	tpl.Nodes[0].Values["conn"] = "DefaultEndpointsProtocol=https;AccountName=a;AccountKey=abcd1234abcd1234=="
	tpl.Nodes[0].Values["pat"] = "ghp_abcdefghijklmnopqrstuvwxyz0123456789"
	tpl.Nodes[0].Values["nested"] = map[string]interface{}{"client_secret": "literal-secret"}
	leaks := Scan(tpl)
	rules := map[string]bool{}
	for _, l := range leaks {
		rules[l.Rule] = true
		if !l.Blocking {
			t.Fatalf("a secret blocks: %+v", l)
		}
		for _, bad := range []string{"hunter2", "MIIabc", "abcd1234", "ghp_", "literal-secret"} {
			if strings.Contains(l.Message, bad) {
				t.Fatalf("a finding never repeats the value: %q", l.Message)
			}
		}
	}
	for _, want := range []string{"secret-name", "key-material", "connection-string", "token"} {
		if !rules[want] {
			t.Fatalf("missing rule %s in %+v", want, leaks)
		}
	}
	if !Blocking(leaks) {
		t.Fatal("Blocking")
	}
}

func TestScanLetsReferencesAndVariablesThrough(t *testing.T) {
	tpl := goodTemplate(t)
	tpl.Nodes[0].Values["admin_password"] = map[string]interface{}{"$expr": "module.kv.secret"}
	tpl.Nodes[0].Values["token"] = "{token_name}"
	tpl.Nodes[0].Values["secret_name"] = "" // empty is nothing
	if l := Scan(tpl); len(l) != 0 {
		t.Fatalf("a reference is not a secret: %+v", l)
	}
}

func TestScanWarnsAboutAzureIds(t *testing.T) {
	tpl := goodTemplate(t)
	tpl.Nodes[0].Values["scope"] = "/subscriptions/00000000-1111-2222-3333-444444444444/resourceGroups/rg"
	l := Scan(tpl)
	if len(l) != 1 || l[0].Rule != "azure-id" || l[0].Blocking || Blocking(l) {
		t.Fatalf("an id is a warning: %+v", l)
	}
}

func TestScanChecksParameterDefaults(t *testing.T) {
	tpl := goodTemplate(t)
	tpl.Params = append(tpl.Params, Param{Name: "db_password", Node: "sn", Field: "size", Default: "abc123456"})
	if !Blocking(Scan(tpl)) {
		t.Fatal("a default in a parameter named like a secret blocks")
	}
}

func TestModulesAndMissing(t *testing.T) {
	tpl := goodTemplate(t)
	tpl.Nodes = append(tpl.Nodes, store.CanvasNode{ID: "r", Kind: "resource", ResourceType: "azurerm_resource_group", InstanceName: "rg"})
	if got := strings.Join(Modules(tpl), ","); got != "azurerm_resource_group,private-endpoint,subnet" {
		t.Fatalf("modules: %s", got)
	}
	cat := validate.NewMapCatalog([]validate.Module{{Name: "subnet", Version: "v1"}})
	if got := strings.Join(Missing(tpl, cat), ","); got != "private-endpoint@v1" {
		t.Fatalf("missing: %q (a raw resource is not checked)", got)
	}
	if Missing(tpl, nil) != nil {
		t.Fatal("no catalog, nothing to say")
	}
}
