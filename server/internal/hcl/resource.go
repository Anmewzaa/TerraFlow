package hcl

import (
	"fmt"
	"regexp"
	"strings"
)

// KindResource marks a canvas node that is a raw provider resource
// (resource "azurerm_x" "name" { ... }) instead of a module call (D-040).
const KindResourceNode = "resource"

var (
	resourceTypePattern = regexp.MustCompile(`^[a-z][a-z0-9]*_[a-z0-9_]+$`)
	blockNamePattern    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
)

// ValidResourceType reports whether t looks like a provider resource type,
// such as azurerm_resource_group.
func ValidResourceType(t string) bool { return resourceTypePattern.MatchString(t) }

func (n GenerateNode) isResource() bool { return n.Kind == KindResourceNode }

// blockKey is how a block is told apart from the others in its folder:
// "<label>" for a module call, "<type>.<label>" for a resource. Terraform
// names cannot contain a dot, so the two never collide.
func blockKey(n GenerateNode) string {
	if n.isResource() {
		return n.ResourceType + "." + n.InstanceName
	}
	return n.InstanceName
}

func splitResourceLabel(label string) (rtype, name string) {
	rtype, name, _ = strings.Cut(label, ".")
	return rtype, name
}

func validateResourceNode(n GenerateNode) error {
	if !resourceTypePattern.MatchString(n.ResourceType) {
		return fmt.Errorf("resource %q: %q is not a provider resource type such as azurerm_resource_group", n.InstanceName, n.ResourceType)
	}
	if !blockNamePattern.MatchString(n.InstanceName) {
		return fmt.Errorf("resource of type %s: %q is not a valid Terraform name", n.ResourceType, n.InstanceName)
	}
	if n.NestField != "" || n.ParentID != "" {
		return fmt.Errorf("resource %q: a raw resource cannot be nested inside another node", n.InstanceName)
	}
	return nil
}

// writeResourceBlock renders one raw resource in the same style as a module call.
func writeResourceBlock(b *strings.Builder, n GenerateNode) {
	b.WriteString(fmt.Sprintf("resource %q %q {\n", n.ResourceType, n.InstanceName))
	for _, key := range sortedValueKeys(n.Values) {
		if key == "tags" {
			continue
		}
		writeAttribute(b, key, n.Values[key], 2)
	}
	if tags, ok := n.Values["tags"]; ok {
		writeAttribute(b, "tags", tags, 2)
	}
	b.WriteString("}\n")
}

// blocksValue is the value form of repeated nested blocks:
// {"$blocks": [ {attrs...}, {attrs...} ]} under key k writes `k { ... }` per item.
func blocksValue(v map[string]interface{}) ([]interface{}, bool) {
	if len(v) != 1 {
		return nil, false
	}
	items, ok := v["$blocks"].([]interface{})
	return items, ok
}

func writeBlocks(b *strings.Builder, key string, items []interface{}, indent int) {
	pad := strings.Repeat(" ", indent)
	for _, item := range items {
		attrs, _ := item.(map[string]interface{})
		b.WriteString(fmt.Sprintf("\n%s%s {\n", pad, key))
		for _, k := range sortedKeys(attrs) {
			writeAttribute(b, k, attrs[k], indent+2)
		}
		b.WriteString(fmt.Sprintf("%s}\n", pad))
	}
}
