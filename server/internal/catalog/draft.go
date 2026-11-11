package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
)

// ordered is a JSON object that keeps the order its keys were added in, so a drafted
// schema reads in the order of the variables.tf it came from.
type ordered struct {
	keys []string
	vals map[string]any
}

func newOrdered() *ordered { return &ordered{vals: map[string]any{}} }

func (o *ordered) set(k string, v any) *ordered {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
	return o
}

func (o *ordered) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		vb, err := json.Marshal(o.vals[k])
		if err != nil {
			return nil, err
		}
		b.Write(kb)
		b.WriteByte(':')
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// DraftFromVariables turns the text of a module's variables.tf into a first draft of its
// schema: one input per variable, with its type, description and default; a variable with
// no default is required. The draft is a starting point for the schema editor, not a
// finished schema: the notes say what a person still has to decide (the module source,
// a type Terraform leaves open). Nothing is read but the text given.
func DraftFromVariables(name string, src []byte) (schema json.RawMessage, notes []string, err error) {
	file, diags := hclsyntax.ParseConfig(src, "variables.tf", hcl.InitialPos)
	if diags.HasErrors() {
		return nil, nil, fmt.Errorf("the text is not valid Terraform: %s", diags[0].Error())
	}
	body, _ := file.Body.(*hclsyntax.Body)
	props := newOrdered()
	required := []string{}
	order := 0
	for _, blk := range body.Blocks {
		if blk.Type != "variable" || len(blk.Labels) != 1 {
			continue
		}
		vname := blk.Labels[0]
		if !propNameRe.MatchString(vname) {
			notes = append(notes, fmt.Sprintf("variable %q was left out: its name cannot be an input name", vname))
			continue
		}
		order++
		ty := cty.String
		typed := false
		if attr, ok := blk.Body.Attributes["type"]; ok {
			if t, _, d := typeexpr.TypeConstraintWithDefaults(attr.Expr); !d.HasErrors() {
				ty, typed = t, true
			}
		}
		in := draftType(ty, 0, vname, &notes)
		if !typed {
			notes = append(notes, fmt.Sprintf("variable %q has no type; it was drafted as a string", vname))
		}
		p := newOrdered().set("type", in.vals["type"]).set("title", humanize(vname))
		if attr, ok := blk.Body.Attributes["description"]; ok {
			if v, d := attr.Expr.Value(nil); !d.HasErrors() && v.Type() == cty.String && !v.IsNull() {
				p.set("description", v.AsString())
			}
		}
		for _, k := range in.keys {
			if k != "type" {
				p.set(k, in.vals[k])
			}
		}
		if attr, ok := blk.Body.Attributes["default"]; !ok {
			required = append(required, vname)
		} else if v, d := attr.Expr.Value(nil); d.HasErrors() {
			notes = append(notes, fmt.Sprintf("the default of %q is an expression; it was left out", vname))
		} else if !v.IsNull() && v.IsWhollyKnown() {
			if b, err := ctyjson.Marshal(v, v.Type()); err == nil {
				p.set("default", json.RawMessage(b))
			}
		}
		p.set("x-tf-ui", map[string]any{"order": order})
		props.set(vname, p)
	}
	if len(props.keys) == 0 {
		return nil, nil, fmt.Errorf(`no variable "name" { ... } block was found in the text`)
	}
	root := newOrdered().
		set("title", humanize(strings.ReplaceAll(name, "-", "_"))).
		set("description", "").
		set("type", "object").
		set("x-tf-module", newOrdered().set("source", name).set("version", "v1").set("target_folder", name)).
		set("required", required).
		set("properties", props)
	notes = append(notes, "check x-tf-module: source is the module's address, target_folder the folder its calls are written to")
	out, err := json.MarshalIndent(root, "", "  ")
	return out, notes, err
}

// draftType maps a Terraform type constraint to the JSON Schema the form engine draws.
func draftType(t cty.Type, depth int, vname string, notes *[]string) *ordered {
	o := newOrdered()
	switch {
	case depth >= maxDepth-1:
		*notes = append(*notes, fmt.Sprintf("variable %q is nested too deeply; the inner part was drafted as a string", vname))
		return o.set("type", "string")
	case t == cty.String:
		return o.set("type", "string")
	case t == cty.Number:
		return o.set("type", "number")
	case t == cty.Bool:
		return o.set("type", "boolean")
	case t.IsListType() || t.IsSetType():
		return o.set("type", "array").set("items", draftType(t.ElementType(), depth+1, vname, notes))
	case t.IsMapType():
		return o.set("type", "object").set("additionalProperties", draftType(t.ElementType(), depth+1, vname, notes))
	case t.IsObjectType():
		props := newOrdered()
		req := []string{}
		atts := t.AttributeTypes()
		names := make([]string, 0, len(atts))
		for n := range atts {
			names = append(names, n)
		}
		sortStrings(names)
		for _, n := range names {
			if !propNameRe.MatchString(n) {
				continue
			}
			props.set(n, draftType(atts[n], depth+1, vname, notes))
			if !t.AttributeOptional(n) {
				req = append(req, n)
			}
		}
		o.set("type", "object")
		if len(props.keys) > 0 {
			o.set("properties", props)
			if len(req) > 0 {
				o.set("required", req)
			}
		}
		return o
	}
	*notes = append(*notes, fmt.Sprintf("variable %q has a type Terraform leaves open (any or a tuple); it was drafted as a string", vname))
	return o.set("type", "string")
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// humanize turns resource_group_name into "Resource group name".
func humanize(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "_", " "))
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
