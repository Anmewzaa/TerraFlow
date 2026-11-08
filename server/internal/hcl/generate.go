package hcl

import (
	"encoding/json"
	"fmt"
	"sort"
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

// Generate produces .tf files from canvas nodes.
// It groups nodes by target_folder and emits module calls + data blocks.
func Generate(input GenerateInput) (*GenerateOutput, error) {
	byFolder := map[string][]GenerateNode{}
	for _, n := range input.Nodes {
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

		moduleContent := generateModuleCalls(nodes)
		if moduleContent != "" {
			files = append(files, GeneratedFile{
				Folder:   folder,
				Filename: "main.tf",
				Content:  moduleContent,
			})
		}

		dataContent := generateDataBlocks(nodes)
		if dataContent != "" {
			files = append(files, GeneratedFile{
				Folder:   folder,
				Filename: "data.tf",
				Content:  dataContent,
			})
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

func generateDataBlocks(nodes []GenerateNode) string {
	seen := map[string]bool{}
	var blocks []string

	for _, n := range nodes {
		for _, pr := range n.PickRefs {
			key := pr.DataType + "." + pr.DataName
			if seen[key] {
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

func writeAttribute(b *strings.Builder, key string, val interface{}, indent int) {
	pad := strings.Repeat(" ", indent)
	switch v := val.(type) {
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
