package hcl

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// EvalContext builds an hcl.EvalContext for a folder, loading tfvars, variable
// defaults, and evaluating locals in order.
type EvalContext struct {
	Vars   map[string]cty.Value
	Locals map[string]cty.Value
	ctx    *hcl.EvalContext
}

var envPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$`)

// ValidEnv reports whether env is a plain environment name (dev, prod, uat-2).
// It becomes part of a file path (env/<env>.tfvars) and of a branch name, so
// anything with a separator, dot or control character is refused.
func ValidEnv(env string) bool { return envPattern.MatchString(env) }

// NewEvalContext creates an evaluation context for the given folder state and
// environment. It loads env/<env>.tfvars, applies variable defaults, then
// evaluates locals.
func NewEvalContext(fs *FolderState, env string) (*EvalContext, error) {
	ec := &EvalContext{
		Vars:   make(map[string]cty.Value),
		Locals: make(map[string]cty.Value),
	}

	if !ValidEnv(env) {
		return nil, fmt.Errorf("invalid environment name %q", env)
	}

	// 1. Load tfvars
	tfvarsPath := filepath.Join(fs.Dir, "env", env+".tfvars")
	if _, err := os.Stat(tfvarsPath); err == nil {
		attrs, err := ParseTFVars(tfvarsPath)
		if err != nil {
			return nil, fmt.Errorf("load tfvars: %w", err)
		}
		for name, attr := range attrs {
			val, diags := attr.Expr.Value(nil)
			if diags.HasErrors() {
				ec.Vars[name] = cty.DynamicVal
				continue
			}
			ec.Vars[name] = val
		}
	}

	// 2. Apply variable defaults for anything not in tfvars
	for _, v := range fs.Variables {
		if _, ok := ec.Vars[v.Name]; ok {
			continue
		}
		if v.Default != nil {
			val, diags := v.Default.Value(nil)
			if !diags.HasErrors() {
				ec.Vars[v.Name] = val
			} else {
				ec.Vars[v.Name] = cty.DynamicVal
			}
		} else {
			ec.Vars[v.Name] = cty.DynamicVal
		}
	}

	// 3. Build the HCL eval context with var.* and functions
	ec.ctx = &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"var": cty.ObjectVal(ec.Vars),
		},
		Functions: builtinFunctions(),
	}

	// 4. Evaluate locals iteratively (some locals depend on others)
	if err := ec.evalLocals(fs.Locals); err != nil {
		return nil, err
	}

	// Update context with locals
	ec.ctx.Variables["local"] = cty.ObjectVal(ec.Locals)

	return ec, nil
}

// evalLocals evaluates all locals blocks. Iterates multiple passes to handle
// inter-local dependencies.
func (ec *EvalContext) evalLocals(localsDefs []LocalsDef) error {
	pending := make(map[string]hcl.Expression)
	for _, ld := range localsDefs {
		for name, attr := range ld.Attrs {
			pending[name] = attr.Expr
		}
	}

	maxPasses := len(pending) + 1
	for pass := 0; pass < maxPasses && len(pending) > 0; pass++ {
		resolved := 0
		for name, expr := range pending {
			ec.ctx.Variables["local"] = cty.ObjectVal(ec.Locals)
			val, diags := expr.Value(ec.ctx)
			if !diags.HasErrors() && val.IsWhollyKnown() {
				ec.Locals[name] = val
				delete(pending, name)
				resolved++
			}
		}
		if resolved == 0 {
			break
		}
	}

	// Anything still pending is unknown
	for name := range pending {
		ec.Locals[name] = cty.DynamicVal
	}
	return nil
}

// Eval evaluates an expression in this folder's context.
func (ec *EvalContext) Eval(expr hcl.Expression) (cty.Value, bool) {
	ec.ctx.Variables["local"] = cty.ObjectVal(ec.Locals)
	val, diags := expr.Value(ec.ctx)
	if diags.HasErrors() || !val.IsWhollyKnown() {
		return cty.DynamicVal, false
	}
	return val, true
}

// EvalString evaluates an expression and returns its string value, or nil if unknown.
func (ec *EvalContext) EvalString(expr hcl.Expression) *string {
	val, ok := ec.Eval(expr)
	if !ok || val.Type() != cty.String {
		return nil
	}
	s := val.AsString()
	return &s
}

// ResolveInputs evaluates all attributes of a body and returns a map of
// resolved values. Unresolved values get {"$unknown": true}.
func (ec *EvalContext) ResolveInputs(body *hclsyntax.Body) map[string]interface{} {
	result := make(map[string]interface{})
	for name, attr := range body.Attributes {
		if name == "source" || name == "provider" || name == "providers" {
			continue
		}
		val, ok := ec.Eval(attr.Expr)
		if !ok {
			result[name] = map[string]interface{}{"$unknown": true}
			continue
		}
		result[name] = ctyToJSON(val)
	}
	return result
}

// ctyToJSON converts a cty.Value to a Go value suitable for JSON marshaling.
func ctyToJSON(v cty.Value) interface{} {
	if !v.IsKnown() {
		return map[string]interface{}{"$unknown": true}
	}
	if v.IsNull() {
		return nil
	}
	ty := v.Type()
	switch {
	case ty == cty.String:
		return v.AsString()
	case ty == cty.Number:
		bf := v.AsBigFloat()
		f, _ := bf.Float64()
		return f
	case ty == cty.Bool:
		return v.True()
	case ty.IsListType() || ty.IsTupleType() || ty.IsSetType():
		var items []interface{}
		for it := v.ElementIterator(); it.Next(); {
			_, val := it.Element()
			items = append(items, ctyToJSON(val))
		}
		return items
	case ty.IsMapType() || ty.IsObjectType():
		m := make(map[string]interface{})
		for it := v.ElementIterator(); it.Next(); {
			key, val := it.Element()
			m[key.AsString()] = ctyToJSON(val)
		}
		return m
	default:
		return map[string]interface{}{"$unknown": true}
	}
}

func builtinFunctions() map[string]function.Function {
	return map[string]function.Function{
		"merge":    stdlib.MergeFunc,
		"format":   stdlib.FormatFunc,
		"contains": stdlib.ContainsFunc,
		"lower":    stdlib.LowerFunc,
		"upper":    stdlib.UpperFunc,
		"length":   stdlib.LengthFunc,
		"tostring": stdlib.MakeToFunc(cty.String),
		"tolist":   stdlib.MakeToFunc(cty.List(cty.DynamicPseudoType)),
		"tomap":    stdlib.MakeToFunc(cty.Map(cty.DynamicPseudoType)),
		"tonumber": stdlib.MakeToFunc(cty.Number),
		"tobool":   stdlib.MakeToFunc(cty.Bool),
	}
}
