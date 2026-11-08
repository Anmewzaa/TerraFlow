package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Change is one difference between two versions of a module schema.
type Change struct {
	Kind     string `json:"kind"` // added | removed | changed
	Path     string `json:"path"`
	Detail   string `json:"detail"`
	Breaking bool   `json:"breaking"`
}

func decode(raw []byte) map[string]any {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if dec.Decode(&m) != nil {
		return nil
	}
	return m
}

func stringSet(v any) map[string]bool {
	out := map[string]bool{}
	list, _ := v.([]any)
	for _, x := range list {
		if s, ok := x.(string); ok {
			out[s] = true
		}
	}
	return out
}

// Compare lists what changed between two versions of a schema. A change is
// breaking when a project that used the old version could stop working or must
// fill in something new: an input was removed, its type changed, an enum value
// was dropped, or an input became required.
func Compare(oldRaw, newRaw []byte) []Change {
	oldS, newS := decode(oldRaw), decode(newRaw)
	if oldS == nil || newS == nil {
		return []Change{}
	}
	changes := []Change{}
	compareProps("/properties", oldS, newS, &changes)
	sort.SliceStable(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes
}

func compareProps(path string, oldS, newS map[string]any, out *[]Change) {
	oldP, _ := oldS["properties"].(map[string]any)
	newP, _ := newS["properties"].(map[string]any)
	oldReq, newReq := stringSet(oldS["required"]), stringSet(newS["required"])

	names := map[string]bool{}
	for n := range oldP {
		names[n] = true
	}
	for n := range newP {
		names[n] = true
	}
	for name := range names {
		p := path + "/" + name
		o, inOld := oldP[name].(map[string]any)
		n, inNew := newP[name].(map[string]any)
		switch {
		case inOld && !inNew:
			*out = append(*out, Change{"removed", p, "input removed", true})
		case !inOld && inNew:
			req := newReq[name]
			detail := "input added"
			if req {
				detail = "input added and required"
			}
			*out = append(*out, Change{"added", p, detail, req})
		default:
			compareInput(p, o, n, oldReq[name], newReq[name], out)
		}
	}
}

func compareInput(p string, o, n map[string]any, wasReq, isReq bool, out *[]Change) {
	if wasReq != isReq {
		*out = append(*out, Change{"changed", p, fmt.Sprintf("required: %v -> %v", wasReq, isReq), isReq})
	}
	if ot, nt := typeString(o), typeString(n); ot != nt {
		*out = append(*out, Change{"changed", p + "/type", fmt.Sprintf("type: %s -> %s", ot, nt), true})
	}
	if od, nd := compact(o["default"]), compact(n["default"]); od != nd {
		*out = append(*out, Change{"changed", p + "/default", fmt.Sprintf("default: %s -> %s", od, nd), false})
	}
	oe, ne := enumSet(o["enum"]), enumSet(n["enum"])
	if len(oe) > 0 || len(ne) > 0 {
		var dropped, added []string
		for v := range oe {
			if !ne[v] {
				dropped = append(dropped, v)
			}
		}
		for v := range ne {
			if !oe[v] {
				added = append(added, v)
			}
		}
		sort.Strings(dropped)
		sort.Strings(added)
		if len(dropped) > 0 {
			*out = append(*out, Change{"changed", p + "/enum", "enum values removed: " + strings.Join(dropped, ", "), true})
		}
		if len(added) > 0 {
			*out = append(*out, Change{"changed", p + "/enum", "enum values added: " + strings.Join(added, ", "), false})
		}
	}
	if _, ok := o["properties"].(map[string]any); ok {
		compareProps(p+"/properties", o, n, out)
	} else if _, ok := n["properties"].(map[string]any); ok {
		compareProps(p+"/properties", o, n, out)
	}
}

func typeString(sch map[string]any) string {
	t, _ := typesOf(sch)
	sort.Strings(t)
	return strings.Join(t, "|")
}

func enumSet(v any) map[string]bool {
	out := map[string]bool{}
	list, _ := v.([]any)
	for _, x := range list {
		out[compact(x)] = true
	}
	return out
}
