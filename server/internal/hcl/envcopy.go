package hcl

import "strings"

// RenameEnvToken replaces the environment name in s where it stands as a whole word of a name
// (rg-dev-app, kv_dev, dev): the neighbours are the start or end of the text or one of - _ . /
// A name that merely contains the letters (device, devops) is left alone.
func RenameEnvToken(s, from, to string) string {
	if from == "" || from == to {
		return s
	}
	isSep := func(b byte) bool { return b == '-' || b == '_' || b == '.' || b == '/' }
	var out strings.Builder
	for i := 0; i < len(s); {
		if strings.HasPrefix(s[i:], from) &&
			(i == 0 || isSep(s[i-1])) && (i+len(from) == len(s) || isSep(s[i+len(from)])) {
			out.WriteString(to)
			i += len(from)
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

func renameValue(v any, from, to string) any {
	switch x := v.(type) {
	case string:
		if strings.Contains(x, "{") { // a project variable or a reference: its own value follows the environment
			return x
		}
		return RenameEnvToken(x, from, to)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = renameValue(e, from, to)
		}
		return out
	case map[string]any:
		if _, ok := x["$expr"]; ok {
			return x // an expression is copied as it is
		}
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = renameValue(e, from, to)
		}
		return out
	}
	return v
}

// CopyNodeToEnv makes a new node from one that edits a block of environment from, for
// environment to: it is no longer tied to the existing block, and the environment in its
// names follows the naming of the new environment (8.10).
func CopyNodeToEnv(n GenerateNode, from, to string) GenerateNode {
	out := n
	out.Existing = nil
	out.Source = n.Source // keeps the pinned version; the generator writes the same source line
	out.ID = "copy-" + n.TargetFolder + "-" + n.InstanceName
	out.Values = map[string]interface{}{}
	for k, v := range n.Values {
		out.Values[k] = renameValue(v, from, to)
	}
	return out
}
