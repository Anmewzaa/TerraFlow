package hcl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// File change statuses returned by Apply.
const (
	FileAdded    = "added"
	FileModified = "modified"
)

// ChangedFile is a file that Apply created or changed. Content is the complete
// final file, so a caller can diff it against the repository or write it as is.
type ChangedFile struct {
	GeneratedFile
	Status string `json:"status"`
}

// Apply generates the canvas against the files that already exist in the
// repository, and never replaces what is there:
//
//   - new nodes are appended to the end of their file (or create the file);
//   - nodes with Existing set replace only that one module block;
//   - every other byte of every file stays exactly as it was.
//
// It fails, instead of overwriting, on a duplicate module label, on a data block
// whose arguments differ from the one already in the folder, on an edit whose
// target block is gone, and on any file that would not parse afterwards.
// Only files that changed are returned.
func Apply(input GenerateInput, repo []GeneratedFile) ([]ChangedFile, error) {
	prepared, err := prepareNodes(input.Nodes)
	if err != nil {
		return nil, err
	}
	var newNodes, editNodes []GenerateNode
	for _, n := range prepared {
		if n.Existing != nil {
			editNodes = append(editNodes, n)
		} else {
			newNodes = append(newNodes, n)
		}
	}

	content := make(map[string]string, len(repo))
	for _, f := range repo {
		content[pathOf(f.Folder, f.Filename)] = f.Content
	}
	status := map[string]string{}
	idx := scanRepo(repo)

	// skip: data blocks that must not be written again (already in the folder,
	// or written once for an earlier node in this run).
	skip := map[string]map[string]bool{}
	markData := func(folder string, pr PickRef) error {
		key := pr.DataType + "." + pr.DataName
		if skip[folder] == nil {
			skip[folder] = map[string]bool{}
		}
		if have, ok := idx.data[folder][key]; ok {
			for k, v := range pr.LookupArgs {
				if got, ok := have[k]; !ok || got != strings.TrimSpace(v) {
					return fmt.Errorf("data %q %q already exists in folder %q with a different %q; reuse it or pick another name", pr.DataType, pr.DataName, folder, k)
				}
			}
			skip[folder][key] = true
		}
		return nil
	}

	// 1. Edits: replace just the targeted module block.
	editsByFolder := map[string][]GenerateNode{}
	for _, n := range editNodes {
		ref := n.Existing
		if err := validFolder(ref.Folder); err != nil {
			return nil, err
		}
		if err := validFilename(ref.Filename); err != nil {
			return nil, err
		}
		path := pathOf(ref.Folder, ref.Filename)
		if idx.modules[ref.Folder] == nil {
			idx.modules[ref.Folder] = map[string]string{}
		}
		src, ok := content[path]
		if !ok {
			return nil, fmt.Errorf("cannot edit module %q: %s no longer exists in the repository", ref.Label, path)
		}
		name := n.InstanceName
		if name == "" {
			name = ref.Label
		}
		start, end, existingSource, err := findModuleBlock(path, src, ref.Label)
		if err != nil {
			return nil, err
		}
		if n.Source == "" {
			n.Source = existingSource // keep the repository's own source line
		}
		if name != ref.Label {
			if where, dup := idx.modules[ref.Folder][name]; dup {
				return nil, fmt.Errorf("cannot rename module %q to %q: that name is already used in %s", ref.Label, name, where)
			}
		}
		n.InstanceName = name
		repl := matchEOL(src, strings.TrimRight(generateModuleCalls([]GenerateNode{n}), "\n"))
		content[path] = src[:start] + repl + src[end:]
		status[path] = FileModified
		if name != ref.Label {
			delete(idx.modules[ref.Folder], ref.Label)
			idx.modules[ref.Folder][name] = path
		}
		for _, pr := range n.PickRefs {
			if err := markData(ref.Folder, pr); err != nil {
				return nil, err
			}
		}
		editsByFolder[ref.Folder] = append(editsByFolder[ref.Folder], n)
	}
	for _, folder := range sortedKeys(editsByFolder) {
		blocks := generateDataBlocks(editsByFolder[folder], skip[folder])
		if blocks == "" {
			continue
		}
		path := pathOf(folder, "data.tf")
		content[path] = appendBlocks(content[path], blocks)
		if status[path] == "" {
			status[path] = statusFor(repo, path)
		}
		for _, n := range editsByFolder[folder] {
			for _, pr := range n.PickRefs {
				skip[folder][pr.DataType+"."+pr.DataName] = true
			}
		}
	}

	// 2. New nodes: refuse duplicate labels, then append.
	seenNew := map[string]bool{}
	for _, n := range newNodes {
		if err := validFolder(n.TargetFolder); err != nil {
			return nil, err
		}
		if where, dup := idx.modules[n.TargetFolder][n.InstanceName]; dup {
			return nil, fmt.Errorf("module %q already exists in %s; edit that resource or choose another name", n.InstanceName, where)
		}
		k := n.TargetFolder + "|" + n.InstanceName
		if seenNew[k] {
			return nil, fmt.Errorf("two resources on the canvas are both named %q in folder %q", n.InstanceName, n.TargetFolder)
		}
		seenNew[k] = true
		for _, pr := range n.PickRefs {
			if err := markData(n.TargetFolder, pr); err != nil {
				return nil, err
			}
		}
	}
	out, err := generateFiles(newNodes, skip)
	if err != nil {
		return nil, err
	}
	for _, gf := range out.Files {
		path := pathOf(gf.Folder, gf.Filename)
		if have, ok := content[path]; ok {
			content[path] = appendBlocks(have, gf.Content)
			if status[path] == "" {
				status[path] = FileModified
			}
			continue
		}
		content[path] = gf.Content
		status[path] = FileAdded
	}

	// 3. Safety net: whatever we are about to hand back must still parse.
	paths := make([]string, 0, len(status))
	for p := range status {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	result := make([]ChangedFile, 0, len(paths))
	for _, p := range paths {
		if _, diags := hclsyntax.ParseConfig([]byte(content[p]), p, hcl.InitialPos); diags.HasErrors() {
			return nil, fmt.Errorf("generated HCL for %s is not valid: %s", p, diags.Error())
		}
		folder, name := splitPath(p)
		result = append(result, ChangedFile{
			GeneratedFile: GeneratedFile{Folder: folder, Filename: name, Content: content[p]},
			Status:        status[p],
		})
	}
	return result, nil
}

func pathOf(folder, filename string) string { return folder + "/" + filename }

func splitPath(p string) (folder, name string) {
	i := strings.LastIndex(p, "/")
	return p[:i], p[i+1:]
}

func statusFor(repo []GeneratedFile, path string) string {
	for _, f := range repo {
		if pathOf(f.Folder, f.Filename) == path {
			return FileModified
		}
	}
	return FileAdded
}

// matchEOL gives text the line endings the file already uses.
func matchEOL(file, text string) string {
	if strings.Contains(file, "\r\n") {
		return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", "\r\n")
	}
	return text
}

// appendBlocks adds blocks after existing content, separated by one blank line.
func appendBlocks(existing, add string) string {
	if existing == "" {
		return add
	}
	eol := "\n"
	if strings.Contains(existing, "\r\n") {
		eol = "\r\n"
	}
	if !strings.HasSuffix(existing, "\n") {
		existing += eol
	}
	return existing + eol + matchEOL(existing, add)
}

// findModuleBlock returns the byte range of `module "<label>" { ... }`.
// It also returns the block's literal `source` string ("" if not a literal).
func findModuleBlock(path, src, label string) (start, end int, source string, err error) {
	f, diags := hclsyntax.ParseConfig([]byte(src), path, hcl.InitialPos)
	if diags.HasErrors() {
		return 0, 0, "", fmt.Errorf("cannot edit %s: it does not parse: %s", path, diags.Error())
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return 0, 0, "", fmt.Errorf("cannot edit %s", path)
	}
	for _, b := range body.Blocks {
		if b.Type == "module" && len(b.Labels) == 1 && b.Labels[0] == label {
			if a, ok := b.Body.Attributes["source"]; ok {
				if v, d := a.Expr.Value(nil); !d.HasErrors() && v.Type() == cty.String && !v.IsNull() {
					source = v.AsString()
				}
			}
			return b.TypeRange.Start.Byte, b.CloseBraceRange.End.Byte, source, nil
		}
	}
	return 0, 0, "", fmt.Errorf("cannot edit module %q: it is no longer in %s (the repository changed since it was loaded)", label, path)
}

// repoIndex records what the repository already declares, per folder.
type repoIndex struct {
	modules map[string]map[string]string            // folder -> module label -> file
	data    map[string]map[string]map[string]string // folder -> "type.name" -> arg -> source text
}

// scanRepo is best effort: a file that does not parse contributes what could be
// read. Files that Apply edits are parsed strictly when it edits them.
func scanRepo(repo []GeneratedFile) repoIndex {
	idx := repoIndex{
		modules: map[string]map[string]string{},
		data:    map[string]map[string]map[string]string{},
	}
	for _, rf := range repo {
		f, _ := hclsyntax.ParseConfig([]byte(rf.Content), rf.Filename, hcl.InitialPos)
		if f == nil {
			continue
		}
		body, ok := f.Body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		if idx.modules[rf.Folder] == nil {
			idx.modules[rf.Folder] = map[string]string{}
			idx.data[rf.Folder] = map[string]map[string]string{}
		}
		for _, b := range body.Blocks {
			switch {
			case b.Type == "module" && len(b.Labels) == 1:
				idx.modules[rf.Folder][b.Labels[0]] = pathOf(rf.Folder, rf.Filename)
			case b.Type == "data" && len(b.Labels) == 2:
				args := map[string]string{}
				for k, a := range b.Body.Attributes {
					r := a.Expr.Range()
					if r.End.Byte <= len(rf.Content) && r.Start.Byte <= r.End.Byte {
						args[k] = strings.TrimSpace(rf.Content[r.Start.Byte:r.End.Byte])
					}
				}
				idx.data[rf.Folder][b.Labels[0]+"."+b.Labels[1]] = args
			}
		}
	}
	// Folders with no files yet still need non-nil maps for lookups.
	return idx
}
