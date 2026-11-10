package hcl

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// Project variables (D-057). A person types {org} in a field; the generator writes var.org (or
// "rg-${var.org}-dev" inside a longer value), declares the variable in the folder's variables.tf
// when it is not declared there yet, and puts the value into env/<env>.tfvars. Nothing here
// replaces or rewrites what is already in the repository: missing blocks and missing lines are
// appended, everything else stays byte for byte.

var tokenRe = regexp.MustCompile(`\{([a-z][a-z0-9_]*)\}`)

// applyVariables rewrites {name} tokens of declared variables in every value and returns, per
// folder, the variables that folder now uses. Unknown {words} are left as typed.
func applyVariables(nodes []GenerateNode, s *ProjectSettings) (map[string]map[string]bool, error) {
	used := map[string]map[string]bool{}
	vars := s.varSet()
	if len(vars) == 0 {
		return used, nil
	}
	for i := range nodes {
		n := &nodes[i]
		folder := n.TargetFolder
		if n.Existing != nil {
			folder = n.Existing.Folder
		}
		hit := map[string]bool{}
		for k, v := range n.Values {
			n.Values[k] = substitute(v, vars, hit)
		}
		if len(hit) == 0 {
			continue
		}
		if folder == "" {
			return nil, fmt.Errorf("%q uses a project variable but has no folder", n.InstanceName)
		}
		if used[folder] == nil {
			used[folder] = map[string]bool{}
		}
		for name := range hit {
			used[folder][name] = true
		}
	}
	return used, nil
}

func substitute(v interface{}, vars map[string]*ProjectVariable, hit map[string]bool) interface{} {
	switch t := v.(type) {
	case Expr:
		return t // an expression the person wrote is theirs
	case string:
		return substituteString(t, vars, hit)
	case map[string]interface{}:
		for k, val := range t {
			t[k] = substitute(val, vars, hit)
		}
		return t
	case []interface{}:
		for i, val := range t {
			t[i] = substitute(val, vars, hit)
		}
		return t
	}
	return v
}

func substituteString(s string, vars map[string]*ProjectVariable, hit map[string]bool) interface{} {
	locs := tokenRe.FindAllStringSubmatchIndex(s, -1)
	type tok struct {
		from, to int
		name     string
	}
	var toks []tok
	for _, l := range locs {
		if name := s[l[2]:l[3]]; vars[name] != nil {
			toks = append(toks, tok{l[0], l[1], name})
		}
	}
	if len(toks) == 0 {
		return s
	}
	for _, t := range toks {
		hit[t.name] = true
	}
	if len(toks) == 1 && toks[0].from == 0 && toks[0].to == len(s) {
		return Expr("var." + toks[0].name)
	}
	var b strings.Builder
	b.WriteByte('"')
	pos := 0
	for _, t := range toks {
		b.WriteString(templateLiteral(s[pos:t.from]))
		b.WriteString("${var." + t.name + "}")
		pos = t.to
	}
	b.WriteString(templateLiteral(s[pos:]))
	b.WriteByte('"')
	return Expr(b.String())
}

// templateLiteral quotes text for the inside of an HCL string. A trailing $ or % is written as an
// escape so that it cannot join the ${ that follows it.
func templateLiteral(text string) string {
	if text == "" {
		return ""
	}
	q := hclString(text)
	q = q[1 : len(q)-1]
	switch {
	case strings.HasSuffix(q, "$") && !strings.HasSuffix(q, "$$"):
		q = q[:len(q)-1] + "\\" + "u0024"
	case strings.HasSuffix(q, "%") && !strings.HasSuffix(q, "%%"):
		q = q[:len(q)-1] + "\\" + "u0025"
	}
	return q
}

var variableDeclRe = regexp.MustCompile(`(?m)^\s*variable\s+"([^"]+)"`)

// declaredVars are the variables any .tf file directly in folder already declares.
func declaredVars(content map[string]string, folder string) map[string]bool {
	out := map[string]bool{}
	prefix := folder + "/"
	for p, src := range content {
		rest, ok := strings.CutPrefix(p, prefix)
		if !ok || strings.Contains(rest, "/") || !strings.HasSuffix(rest, ".tf") {
			continue
		}
		for _, m := range variableDeclRe.FindAllStringSubmatch(src, -1) {
			out[m[1]] = true
		}
	}
	return out
}

// attributeNames lists the names assigned in a tfvars file. A file that does not parse is an
// error: appending to it would make it worse.
func attributeNames(path, src string) (map[string]bool, error) {
	out := map[string]bool{}
	if strings.TrimSpace(src) == "" {
		return out, nil
	}
	f, diags := hclsyntax.ParseConfig([]byte(src), path, hcl.InitialPos)
	if diags.HasErrors() {
		return nil, fmt.Errorf("cannot update %s: it does not parse: %s", path, diags.Error())
	}
	if body, ok := f.Body.(*hclsyntax.Body); ok {
		for name := range body.Attributes {
			out[name] = true
		}
	}
	return out, nil
}

// variableFiles adds what the used variables and the starter files need to content and status
// (the same maps Apply keeps). touched are the folders that receive a resource.
func variableFiles(s *ProjectSettings, used map[string]map[string]bool, touched map[string]bool, content, status map[string]string, repo []GeneratedFile) error {
	vars := s.varSet()
	set := func(path, text string) {
		_, had := content[path]
		content[path] = text
		if status[path] == "" {
			if had {
				status[path] = statusFor(repo, path)
			} else {
				status[path] = FileAdded
			}
		}
	}

	for _, folder := range sortedKeys(used) {
		names := make([]string, 0, len(used[folder]))
		for n := range used[folder] {
			names = append(names, n)
		}
		sort.Strings(names)

		// variables.tf: declare what the folder does not declare yet.
		declared := declaredVars(content, folder)
		var blocks []string
		for _, n := range names {
			if declared[n] {
				continue
			}
			var b strings.Builder
			// Aligned the way `terraform fmt` writes it, so a later fmt changes nothing.
			fmt.Fprintf(&b, "variable %q {\n", n)
			if d := vars[n].Description; d != "" {
				fmt.Fprintf(&b, "  type        = string\n  description = %s\n", hclString(d))
			} else {
				b.WriteString("  type = string\n")
			}
			b.WriteString("}\n")
			blocks = append(blocks, b.String())
		}
		if len(blocks) > 0 {
			path := pathOf(folder, "variables.tf")
			set(path, appendBlocks(content[path], strings.Join(blocks, "\n")))
		}

		// env/<env>.tfvars: the value for each environment that has one.
		envs := map[string]bool{}
		for _, n := range names {
			for env := range vars[n].Values {
				envs[env] = true
			}
		}
		for _, env := range sortedKeys(envs) {
			path := pathOf(folder, "env/"+env+".tfvars")
			have, err := attributeNames(path, content[path])
			if err != nil {
				return err
			}
			var lines []string
			for _, n := range names {
				if val, ok := vars[n].Values[env]; ok && !have[n] {
					lines = append(lines, fmt.Sprintf("%s = %s", n, hclString(val)))
				}
			}
			if len(lines) == 0 {
				continue
			}
			add := strings.Join(lines, "\n") + "\n"
			if cur := content[path]; cur != "" {
				if !strings.HasSuffix(cur, "\n") {
					cur += "\n"
				}
				add = cur + matchEOL(cur, add)
			}
			set(path, add)
		}
	}

	// Starter files: only where the folder does not have the file. The project's providers write
	// providers.tf the same way, unless a starter of that name is chosen explicitly.
	starters := s.Starters
	if pf := providersFile(s.Providers); pf != "" {
		explicit := false
		for _, st := range starters {
			explicit = explicit || st.Filename == "providers.tf"
		}
		if !explicit {
			starters = append(append([]StarterFile{}, starters...), StarterFile{Filename: "providers.tf", Content: pf})
		}
	}
	for _, folder := range sortedKeys(touched) {
		for _, st := range starters {
			path := pathOf(folder, st.Filename)
			if _, ok := content[path]; ok {
				continue
			}
			set(path, strings.ReplaceAll(st.Content, "{folder}", folder))
		}
	}
	return nil
}
