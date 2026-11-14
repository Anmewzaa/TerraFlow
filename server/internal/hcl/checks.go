package hcl

import (
	"fmt"
	"regexp"
)

// Checks are a project's own policy rules (8.13), judged on top of the schema rules. A finding
// is a warning unless Strict is set; a value that cannot be worked out is never judged.
type Checks struct {
	// Naming is a regular expression per module name that the module's `name` must match.
	Naming map[string]string `json:"naming,omitempty"`
	// RequiredTags are tag names every resource that has tags must carry.
	RequiredTags []string `json:"required_tags,omitempty"`
	// AllowedRegions are the only locations the project may use.
	AllowedRegions []string `json:"allowed_regions,omitempty"`
	// RequiredInEnv says that a kind of resource must have an input set in an environment (a lock
	// in prod, for example).
	RequiredInEnv []EnvRequirement `json:"required_in_env,omitempty"`
	// MustMatch says that two environments must not differ in some inputs of a kind of resource.
	MustMatch []MustMatch `json:"must_match,omitempty"`
	// Strict makes the findings errors, which stop publish and approve.
	Strict bool `json:"strict,omitempty"`
}

// EnvRequirement: in Env, Module must have Field set.
type EnvRequirement struct {
	Env    string `json:"env"`
	Module string `json:"module"`
	Field  string `json:"field"`
}

// MustMatch: Module's Fields must be the same in From and To.
type MustMatch struct {
	From   string   `json:"from"`
	To     string   `json:"to"`
	Module string   `json:"module"`
	Fields []string `json:"fields"`
}

const (
	maxRules       = 30
	maxNamingRules = 50
	maxRegexLen    = 200
	maxTagNames    = 20
	maxRegions     = 30
)

var (
	tagNameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]{0,62}$`)
	regionRe  = regexp.MustCompile(`^[a-z][a-z0-9]{2,30}$`)
)

// Validate lists what is wrong with the checks; nil when they are fine.
func (c *Checks) Validate() []string {
	if c == nil {
		return nil
	}
	var p []string
	add := func(f string, a ...any) { p = append(p, fmt.Sprintf(f, a...)) }
	if len(c.Naming) > maxNamingRules {
		add("checks.naming has too many rules (max %d)", maxNamingRules)
	}
	for mod, re := range c.Naming {
		if !catalogNameRe.MatchString(mod) {
			add("checks.naming: %q is not a module name", mod)
		}
		if len(re) == 0 || len(re) > maxRegexLen {
			add("checks.naming[%s]: the pattern must be 1 to %d characters", mod, maxRegexLen)
		} else if _, err := regexp.Compile(re); err != nil {
			add("checks.naming[%s]: not a valid pattern: %v", mod, err)
		}
	}
	if len(c.RequiredTags) > maxTagNames {
		add("checks.required_tags has too many names (max %d)", maxTagNames)
	}
	for _, t := range c.RequiredTags {
		if !tagNameRe.MatchString(t) {
			add("checks.required_tags: %q is not a tag name", t)
		}
	}
	if len(c.RequiredInEnv) > maxRules || len(c.MustMatch) > maxRules {
		add("checks has too many environment rules (max %d of each)", maxRules)
	}
	for i, r := range c.RequiredInEnv {
		if !ValidEnv(r.Env) || !catalogNameRe.MatchString(r.Module) || !varNamePattern.MatchString(r.Field) {
			add("checks.required_in_env[%d]: needs an environment, a module name and an input name", i)
		}
	}
	for i, m := range c.MustMatch {
		if !ValidEnv(m.From) || !ValidEnv(m.To) || m.From == m.To || !catalogNameRe.MatchString(m.Module) || len(m.Fields) == 0 || len(m.Fields) > 20 {
			add("checks.must_match[%d]: needs two different environments, a module name and 1 to 20 input names", i)
			continue
		}
		for _, f := range m.Fields {
			if !varNamePattern.MatchString(f) {
				add("checks.must_match[%d]: %q is not an input name", i, f)
			}
		}
	}
	if len(c.AllowedRegions) > maxRegions {
		add("checks.allowed_regions has too many entries (max %d)", maxRegions)
	}
	for _, r := range c.AllowedRegions {
		if !regionRe.MatchString(r) {
			add("checks.allowed_regions: %q is not a region name such as southeastasia", r)
		}
	}
	return p
}
