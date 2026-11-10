package hcl

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Cloud providers (D-058). TerraFlow never calls a cloud: a provider here is only a name, a
// Terraform source address, the version constraints a team uses, and the block written to
// providers.tf. The code does not branch on a provider id: everything comes from the data.

var (
	providerIDRe = regexp.MustCompile(`^[a-z][a-z0-9]{1,30}$`)
	// A project's provider version is one Terraform constraint: "~> 4.0", ">= 3.100", "= 4.1.0", "4.0".
	versionConstraintRe = regexp.MustCompile(`^(~>|>=|<=|!=|=|>|<)?\s*\d+(\.\d+){0,2}$`)
	// A compatibility bound on a raw resource is major.minor: "3.0", "4.10".
	boundRe   = regexp.MustCompile(`^\d+\.\d+$`)
	numbersRe = regexp.MustCompile(`\d+`)
)

// ValidProviderID reports whether id can name a provider (azurerm, aws, google).
func ValidProviderID(id string) bool { return providerIDRe.MatchString(id) }

// ValidVersionConstraint reports whether v is a single Terraform version constraint.
func ValidVersionConstraint(v string) bool {
	return versionConstraintRe.MatchString(strings.TrimSpace(v))
}

// ValidBound reports whether b is a major.minor bound ("" is allowed: no bound).
func ValidBound(b string) bool { return b == "" || boundRe.MatchString(b) }

// ProviderOfType is the provider a resource type belongs to: the part before the first
// underscore (azurerm_resource_group -> azurerm). Terraform names resource types this way.
func ProviderOfType(resourceType string) string {
	if i := strings.Index(resourceType, "_"); i > 0 {
		return resourceType[:i]
	}
	return ""
}

// version splits "major.minor" (or a constraint such as "~> 4.0") into numbers; a missing minor is 0.
func version(s string) (major, minor int, ok bool) {
	nums := numbersRe.FindAllString(s, 3)
	if len(nums) == 0 {
		return 0, 0, false
	}
	major, _ = strconv.Atoi(nums[0])
	if len(nums) > 1 {
		minor, _ = strconv.Atoi(nums[1])
	}
	return major, minor, true
}

func less(aMaj, aMin, bMaj, bMin int) bool { return aMaj < bMaj || (aMaj == bMaj && aMin < bMin) }

// Compatible reports whether a raw resource available from min up to max (major.minor, either
// may be empty) can be used with the project's provider version constraint. The constraint's
// first version number is the baseline ("~> 4.0" -> 4.0): this is a help for choosing, not a
// guarantee, because TerraFlow does not read the provider's schema. A project with no version
// accepts everything.
func Compatible(projectConstraint, min, max string) bool {
	pMaj, pMin, ok := version(projectConstraint)
	if !ok {
		return true
	}
	if mMaj, mMin, ok := version(min); min != "" && ok && less(pMaj, pMin, mMaj, mMin) {
		return false
	}
	if xMaj, xMin, ok := version(max); max != "" && ok && less(xMaj, xMin, pMaj, pMin) {
		return false
	}
	return true
}

// ProjectProvider is a provider a project uses, at one version constraint. Source and Block
// are filled in by the server from the provider registry; a client cannot set them.
type ProjectProvider struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
	Source  string `json:"-"`
	Block   string `json:"-"`
}

// providersFile is the content of providers.tf for a project's providers: the required_providers
// block, then each provider's own block. Providers without a source are left out.
func providersFile(list []ProjectProvider) string {
	var req, blocks []string
	for _, p := range list {
		if p.Source == "" {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "    %s = {\n      source  = %s\n", p.ID, hclString(p.Source))
		if v := strings.TrimSpace(p.Version); v != "" {
			fmt.Fprintf(&b, "      version = %s\n", hclString(v))
		}
		b.WriteString("    }\n")
		req = append(req, b.String())
		if blk := strings.TrimSpace(p.Block); blk != "" {
			blocks = append(blocks, blk+"\n")
		}
	}
	if len(req) == 0 {
		return ""
	}
	out := "terraform {\n  required_providers {\n" + strings.Join(req, "") + "  }\n}\n"
	if len(blocks) > 0 {
		out += "\n" + strings.Join(blocks, "\n")
	}
	return out
}
