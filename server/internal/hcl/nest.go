package hcl

import (
	"fmt"
	"strings"
)

// Expr is an unquoted HCL expression (a reference such as module.vnet.subnet_ids["a"]).
type Expr string

// prepareNodes folds nested resources into their parent module's input map and
// resolves {"$expr": "..."} values into raw expressions.
//
// A node with NestField set is not a module call of its own: it becomes
// parent.Values[NestField][<its name>]. That is how a subnet lives inside a
// virtual network. The input slice is never modified.
func prepareNodes(in []GenerateNode) ([]GenerateNode, error) {
	byID := make(map[string]*GenerateNode, len(in))
	nodes := make([]GenerateNode, len(in))
	for i, n := range in {
		nodes[i] = n
		nodes[i].Values = cloneMap(n.Values)
		byID[n.ID] = &nodes[i]
	}

	// Fold children into parents first so expressions can see final parents.
	var out []GenerateNode
	for i := range nodes {
		n := &nodes[i]
		if n.NestField == "" {
			continue
		}
		parent := byID[n.ParentID]
		if parent == nil {
			return nil, fmt.Errorf("nested resource %q has no parent on the canvas: drag it into its parent resource", n.InstanceName)
		}
		if parent.NestField != "" {
			return nil, fmt.Errorf("nested resource %q: nesting more than one level is not supported", n.InstanceName)
		}
		key := nameOf(n)
		child := cloneMap(n.Values)
		delete(child, "name")
		m, _ := parent.Values[n.NestField].(map[string]interface{})
		if m == nil {
			m = map[string]interface{}{}
		}
		if _, dup := m[key]; dup {
			return nil, fmt.Errorf("duplicate %s entry %q in %q", n.NestField, key, parent.InstanceName)
		}
		m[key] = child
		parent.Values[n.NestField] = m
	}

	for i := range nodes {
		n := &nodes[i]
		if n.NestField != "" {
			continue
		}
		resolveExprs(n.Values, n, byID)
		out = append(out, *n)
	}
	return out, nil
}

func nameOf(n *GenerateNode) string {
	if s, ok := n.Values["name"].(string); ok && s != "" {
		return s
	}
	return n.InstanceName
}

// moduleAncestor is the nearest ancestor (or self) that emits a module call.
func moduleAncestor(n *GenerateNode, byID map[string]*GenerateNode) *GenerateNode {
	for cur, hops := n, 0; cur != nil && hops < 16; hops++ {
		if cur.NestField == "" {
			return cur
		}
		cur = byID[cur.ParentID]
	}
	return nil
}

// resolveExprs replaces {"$expr": "tmpl"} with an Expr. Placeholders:
// {parent_name} the parent node's name, {parent_instance} its instance name,
// {module} the instance name of the nearest ancestor module call.
func resolveExprs(v map[string]interface{}, n *GenerateNode, byID map[string]*GenerateNode) {
	for k, val := range v {
		switch t := val.(type) {
		case map[string]interface{}:
			if raw, ok := t["$expr"].(string); ok && len(t) == 1 {
				v[k] = Expr(expand(raw, n, byID))
			} else {
				resolveExprs(t, n, byID)
			}
		}
	}
}

func expand(tmpl string, n *GenerateNode, byID map[string]*GenerateNode) string {
	parent := byID[n.ParentID]
	pairs := []string{}
	if parent != nil {
		pairs = append(pairs, "{parent_name}", nameOf(parent), "{parent_instance}", parent.InstanceName)
		if mod := moduleAncestor(parent, byID); mod != nil {
			pairs = append(pairs, "{module}", mod.InstanceName)
		}
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}

func cloneMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		switch t := v.(type) {
		case map[string]interface{}:
			out[k] = cloneMap(t)
		default:
			out[k] = v
		}
	}
	return out
}
