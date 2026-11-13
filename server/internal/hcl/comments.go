package hcl

import (
	"sort"
	"strings"

	hcl "github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// carryComments keeps the comments a person wrote inside a block that the canvas
// edits (D-064). An edit regenerates the whole block (D-036), so without this the
// comments between its arguments would vanish from the diff.
//
// Only comments tied to a top-level argument travel: the comment lines directly above
// it and a comment at the end of its last line. They are put back on the argument of
// the same name in the new block. A comment of an argument that was removed is dropped
// with it, and so are comments inside nested blocks (those are regenerated) and
// comments that stand alone at the end of the block. Both texts use "\n" lines.
// When anything is unclear the new block is returned unchanged.
func carryComments(oldBlock, newBlock string) string {
	old := topLevelArgs(oldBlock)
	if len(old) == 0 {
		return newBlock
	}
	oldLines := strings.Split(oldBlock, "\n")
	type carried struct {
		above    []string
		trailing string
	}
	byName := map[string]carried{}
	for _, a := range old {
		c := carried{trailing: trailingComment(oldBlock, a)}
		for i := a.startLine - 2; i >= 0 && i >= a.floorLine; i-- {
			t := strings.TrimSpace(oldLines[i])
			if !isLineComment(t) {
				break
			}
			c.above = append([]string{t}, c.above...)
		}
		if len(c.above) > 0 || c.trailing != "" {
			byName[a.name] = c
		}
	}
	if len(byName) == 0 {
		return newBlock
	}

	fresh := topLevelArgs(newBlock)
	if len(fresh) == 0 {
		return newBlock
	}
	lines := strings.Split(newBlock, "\n")
	sort.Slice(fresh, func(i, j int) bool { return fresh[i].startLine > fresh[j].startLine }) // bottom up
	for _, a := range fresh {
		c, ok := byName[a.name]
		if !ok {
			continue
		}
		if c.trailing != "" && !strings.Contains(lines[a.endLine-1], "#") && !strings.Contains(lines[a.endLine-1], "//") {
			lines[a.endLine-1] = strings.TrimRight(lines[a.endLine-1], " \t") + " " + c.trailing
		}
		if len(c.above) > 0 && (a.startLine < 2 || !isLineComment(strings.TrimSpace(lines[a.startLine-2]))) {
			indent := lines[a.startLine-1][:len(lines[a.startLine-1])-len(strings.TrimLeft(lines[a.startLine-1], " \t"))]
			add := make([]string, len(c.above))
			for i, t := range c.above {
				add[i] = indent + t
			}
			lines = append(lines[:a.startLine-1], append(add, lines[a.startLine-1:]...)...)
		}
	}
	out := strings.Join(lines, "\n")
	if _, diags := hclsyntax.ParseConfig([]byte(out), "block.tf", hcl.InitialPos); diags.HasErrors() {
		return newBlock
	}
	return out
}

type blockArg struct {
	name               string
	startLine, endLine int // 1-based
	endByte            int
	floorLine          int // comments above may not reach up to or past this line index (0-based)
}

// topLevelArgs lists the arguments written directly in the block, in source order.
func topLevelArgs(block string) []blockArg {
	f, diags := hclsyntax.ParseConfig([]byte(block), "block.tf", hcl.InitialPos)
	if diags.HasErrors() {
		return nil
	}
	file, ok := f.Body.(*hclsyntax.Body)
	if !ok || len(file.Blocks) != 1 {
		return nil
	}
	body := file.Blocks[0].Body
	var out []blockArg
	for name, a := range body.Attributes {
		out = append(out, blockArg{name: name, startLine: a.SrcRange.Start.Line, endLine: a.SrcRange.End.Line, endByte: a.SrcRange.End.Byte})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].startLine < out[j].startLine })
	// A comment above an argument may not reach back past the previous argument or the
	// opening line of the block.
	prev := file.Blocks[0].OpenBraceRange.End.Line // 1-based line of "{"
	for i := range out {
		out[i].floorLine = prev // 0-based index prev is the line after "{"
		prev = out[i].endLine
	}
	return out
}

// trailingComment is the comment after the argument on its last line, if any.
func trailingComment(block string, a blockArg) string {
	rest := block[a.endByte:]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[:nl]
	}
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "#") || strings.HasPrefix(rest, "//") {
		return rest
	}
	return ""
}

func isLineComment(t string) bool {
	return strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//")
}
