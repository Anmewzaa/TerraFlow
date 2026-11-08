package hcl

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// GenerateInput is the canvas state used by the HCL generator.
type GenerateInput struct {
	Nodes []GenerateNode `json:"nodes"`
}

// GenerateNode is a module instance placed on the canvas by the developer.
type GenerateNode struct {
	ID            string                 `json:"id"`
	ModuleName    string                 `json:"module_name"`
	ModuleVersion string                 `json:"module_version"`
	InstanceName  string                 `json:"instance_name"`
	TargetFolder  string                 `json:"target_folder"`
	Source        string                 `json:"source"`
	Values        map[string]interface{} `json:"values"`
	PickRefs      []PickRef              `json:"pick_refs"`
	// ParentID/NestField: when NestField is set the node is folded into the
	// parent node's input map instead of becoming a module call (e.g. a subnet).
	ParentID  string `json:"parent_id,omitempty"`
	NestField string `json:"nest_field,omitempty"`
	// Filename is the .tf file the module call is written to ("" = main.tf).
	Filename string `json:"filename,omitempty"`
	// Existing marks a node that edits a module call already in the repository.
	// Folder, file and label are fixed by the repository, not by the fields above.
	Existing *ExistingRef `json:"existing,omitempty"`
}

// ExistingRef locates a module call that already exists in the repository.
type ExistingRef struct {
	Folder   string `json:"folder"`
	Filename string `json:"filename"`
	Label    string `json:"label"`
}

// PickRef records that a field value came from picking an existing resource.
type PickRef struct {
	Field      string `json:"field"`
	ModuleType string `json:"module_type"`
	Output     string `json:"output"`
	DataName   string `json:"data_name"`
	DataType   string `json:"data_type"`
	LookupKey  string `json:"lookup_key"`
	LookupArgs map[string]string `json:"lookup_args"`
}

// GenerateOutput holds the generated .tf content per folder.
type GenerateOutput struct {
	Files []GeneratedFile `json:"files"`
}

// GeneratedFile is a single .tf file to write.
type GeneratedFile struct {
	Folder   string `json:"folder"`
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// DefaultFilename is used when a node does not name its file.
const DefaultFilename = "main.tf"

var filenamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}\.tf$`)

func validFilename(name string) error {
	if !filenamePattern.MatchString(name) || strings.Contains(name, "..") {
		return fmt.Errorf("invalid file name %q: use letters, digits, - _ . and end with .tf", name)
	}
	return nil
}

func validFolder(folder string) error {
	if folder == "" || strings.Contains(folder, "..") || strings.ContainsAny(folder, `\:`) || strings.HasPrefix(folder, "/") {
		return fmt.Errorf("invalid folder %q", folder)
	}
	return nil
}

// Generate produces .tf files from canvas nodes.
// It groups nodes by target_folder and emits module calls + data blocks.
func Generate(input GenerateInput) (*GenerateOutput, error) {
	prepared, err := prepareNodes(input.Nodes)
	if err != nil {
		return nil, err
	}
	// Nodes that edit an existing module call are applied by Apply, not emitted
	// as new blocks.
	newNodes := make([]GenerateNode, 0, len(prepared))
	for _, n := range prepared {
		if n.Existing == nil {
			newNodes = append(newNodes, n)
		}
	}
	return generateFiles(newNodes, nil)
}

// generateFiles renders already-prepared nodes into per-folder .tf files.
// skip holds data blocks (folder -> "type.name") that must not be emitted again.
func generateFiles(prepared []GenerateNode, skip map[string]map[string]bool) (*GenerateOutput, error) {
	byFolder := map[string][]GenerateNode{}
	for _, n := range prepared {
		byFolder[n.TargetFolder] = append(byFolder[n.TargetFolder], n)
	}

	var files []GeneratedFile

	folders := make([]string, 0, len(byFolder))
	for f := range byFolder {
		folders = append(folders, f)
	}
	sort.Strings(folders)

	for _, folder := range folders {
		nodes := byFolder[folder]
		if err := validFolder(folder); err != nil {
			return nil, err
		}

		// Module calls go to the file each node asks for (default main.tf), so a
		// team can keep one file per resource kind (rg.tf, kv.tf, ...).
		byFile := map[string][]GenerateNode{}
		for _, n := range nodes {
			name := n.Filename
			if name == "" {
				name = DefaultFilename
			}
			if err := validFilename(name); err != nil {
				return nil, fmt.Errorf("%q: %w", n.InstanceName, err)
			}
			byFile[name] = append(byFile[name], n)
		}
		names := make([]string, 0, len(byFile))
		for name := range byFile {
			names = append(names, name)
		}
		sort.Strings(names)

		start := len(files)
		for _, name := range names {
			if content := generateModuleCalls(byFile[name]); content != "" {
				files = append(files, GeneratedFile{Folder: folder, Filename: name, Content: content})
			}
		}

		if dataContent := generateDataBlocks(nodes, skip[folder]); dataContent != "" {
			merged := false
			for i := start; i < len(files); i++ {
				if files[i].Filename == "data.tf" {
					files[i].Content += "\n" + dataContent
					merged = true
				}
			}
			if !merged {
				files = append(files, GeneratedFile{Folder: folder, Filename: "data.tf", Content: dataContent})
			}
		}
	}

	return &GenerateOutput{Files: files}, nil
}

func generateModuleCalls(nodes []GenerateNode) string {
	var b strings.Builder
	for i, n := range nodes {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("module %q {\n", n.InstanceName))
		b.WriteString(fmt.Sprintf("  source = %q\n", n.Source))
		b.WriteString("\n")

		pickRefFields := map[string]PickRef{}
		for _, pr := range n.PickRefs {
			pickRefFields[pr.Field] = pr
		}

		attrs := sortedValueKeys(n.Values)
		for _, key := range attrs {
			if key == "tags" {
				continue
			}
			if pr, ok := pickRefFields[key]; ok {
				b.WriteString(fmt.Sprintf("  %-24s = data.%s.%s.%s\n", key, pr.DataType, pr.DataName, pr.Output))
				continue
			}
			writeAttribute(&b, key, n.Values[key], 2)
		}

		if tags, ok := n.Values["tags"]; ok {
			writeAttribute(&b, "tags", tags, 2)
		}

		b.WriteString("}\n")
	}
	return b.String()
}

func generateDataBlocks(nodes []GenerateNode, skip map[string]bool) string {
	seen := map[string]bool{}
	var blocks []string

	for _, n := range nodes {
		for _, pr := range n.PickRefs {
			key := pr.DataType + "." + pr.DataName
			if seen[key] || skip[key] {
				continue
			}
			seen[key] = true

			var b strings.Builder
			b.WriteString(fmt.Sprintf("# Cross-folder reference\n"))
			b.WriteString(fmt.Sprintf("data %q %q {\n", pr.DataType, pr.DataName))
			args := sortedKeys(pr.LookupArgs)
			for _, k := range args {
				b.WriteString(fmt.Sprintf("  %-24s = %s\n", k, pr.LookupArgs[k]))
			}
			b.WriteString("}\n")
			blocks = append(blocks, b.String())
		}
	}
	return strings.Join(blocks, "\n")
}

var plainKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// hclKey quotes map keys that are not plain identifiers ("snet-app" = {...}),
// matching the team's style in samples/.
func hclKey(k string) string {
	if plainKey.MatchString(k) {
		return k
	}
	return strconv.Quote(k)
}

func writeAttribute(b *strings.Builder, key string, val interface{}, indent int) {
	key = hclKey(key)
	pad := strings.Repeat(" ", indent)
	switch v := val.(type) {
	case Expr:
		b.WriteString(fmt.Sprintf("%s%-24s = %s\n", pad, key, string(v)))
	case map[string]interface{}:
		if len(v) == 0 {
			b.WriteString(fmt.Sprintf("%s%-24s = {}\n", pad, key))
			return
		}
		b.WriteString(fmt.Sprintf("\n%s%s = {\n", pad, key))
		keys := sortedKeys(v)
		for _, k := range keys {
			writeAttribute(b, k, v[k], indent+2)
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	case []interface{}:
		items := make([]string, 0, len(v))
		for _, item := range v {
			items = append(items, formatValue(item))
		}
		b.WriteString(fmt.Sprintf("%s%-24s = [%s]\n", pad, key, strings.Join(items, ", ")))
	default:
		b.WriteString(fmt.Sprintf("%s%-24s = %s\n", pad, key, formatValue(val)))
	}
}

func formatValue(val interface{}) string {
	switch v := val.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case json.Number:
		return v.String()
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	case Expr:
		return string(v)
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedValueKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
