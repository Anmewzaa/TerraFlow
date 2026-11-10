package hcl

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// ProjectSettings is a project's own choices for the HCL TerraFlow writes (D-057). The
// zero value means "as before": no variables, no starter files, the catalog's own folders.
// It is data, never code: every name and path is validated before it is stored.
type ProjectSettings struct {
	// Layout says where a NEW resource goes by default. The generator itself does not read it:
	// the screen fills each node's folder and file from it when the node is created, and a person can
	// still change them per node. Stored here so every screen agrees.
	Layout Layout `json:"layout"`
	// Variables are declared once per project (org = aic). A value typed as {org} is written as
	// var.org, and variables.tf and env/<env>.tfvars get what they need.
	Variables []ProjectVariable `json:"variables,omitempty"`
	// Starters are files written to a folder that does not have them yet (providers.tf, backend.tf).
	Starters []StarterFile `json:"starters,omitempty"`
	// Providers are the cloud providers the project uses, each at a version constraint. They
	// decide which raw resources are offered, and write providers.tf to a new folder (D-058).
	Providers []ProjectProvider `json:"providers,omitempty"`
}

// Layout modes.
const (
	LayoutCatalog = "catalog" // each module's own folder and file (the default)
	LayoutSingle  = "single"  // every new resource in one folder and one file
)

// Layout is where new resources go by default.
type Layout struct {
	Mode string `json:"mode,omitempty"`
	// Folder and File are used by mode "single" (default folder "terraform", file "main.tf").
	Folder string `json:"folder,omitempty"`
	File   string `json:"file,omitempty"`
	// Files overrides the file by catalog name in mode "catalog" (resource-group -> rg.tf).
	Files map[string]string `json:"files,omitempty"`
}

// ProjectVariable is one project-wide value.
type ProjectVariable struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Values is the value per environment (dev, prod...). A variable with no value for an
	// environment gets no line in that environment's tfvars file.
	Values map[string]string `json:"values,omitempty"`
}

// StarterFile is a file written once to every folder that gets a resource and does not have it.
// {folder} in Content is replaced by the folder name (for a backend key).
type StarterFile struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

const (
	maxVariables    = 100
	maxStarters     = 6
	maxProviders    = 10
	maxStarterBytes = 4096
	maxValueLen     = 200
)

var (
	varNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,40}$`)
	catalogNameRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	// reservedVarNames are words Terraform does not accept as a variable name.
	reservedVarNames = map[string]bool{
		"source": true, "version": true, "providers": true, "count": true, "for_each": true,
		"lifecycle": true, "depends_on": true, "locals": true,
	}
)

// ParseSettings reads a settings document. An empty document is the zero value.
func ParseSettings(raw []byte) (*ProjectSettings, error) {
	var s ProjectSettings
	if len(strings.TrimSpace(string(raw))) == 0 {
		return &s, nil
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("settings are not valid: %w", err)
	}
	return &s, nil
}

// Validate lists every problem with the settings (nil when they are fine).
func (s *ProjectSettings) Validate() []string {
	var p []string
	add := func(f string, a ...any) { p = append(p, fmt.Sprintf(f, a...)) }

	switch s.Layout.Mode {
	case "", LayoutCatalog, LayoutSingle:
	default:
		add("layout.mode must be %q or %q", LayoutCatalog, LayoutSingle)
	}
	if s.Layout.Folder != "" {
		if strings.Contains(s.Layout.Folder, "/") || validFolder(s.Layout.Folder) != nil {
			add("layout.folder must be one plain folder name (a project's folders are one level deep)")
		}
	}
	if s.Layout.File != "" && validFilename(s.Layout.File) != nil {
		add("layout.file must be a file name ending in .tf")
	}
	if len(s.Layout.Files) > 100 {
		add("layout.files has too many entries (max 100)")
	}
	for name, file := range s.Layout.Files {
		if !catalogNameRe.MatchString(name) {
			add("layout.files: %q is not a catalog name", name)
		}
		if validFilename(file) != nil {
			add("layout.files[%s]: %q must be a file name ending in .tf", name, file)
		}
	}

	if len(s.Variables) > maxVariables {
		add("too many variables (max %d)", maxVariables)
	}
	seen := map[string]bool{}
	for i, v := range s.Variables {
		switch {
		case !varNamePattern.MatchString(v.Name):
			add("variables[%d]: %q is not a valid name (lower case letters, digits and _, starting with a letter)", i, v.Name)
		case reservedVarNames[v.Name]:
			add("variables[%d]: %q is reserved by Terraform", i, v.Name)
		case seen[v.Name]:
			add("variables[%d]: %q is declared twice", i, v.Name)
		}
		seen[v.Name] = true
		if len(v.Description) > 200 || strings.ContainsAny(v.Description, "\n\r") {
			add("variables[%d]: the description must be one line of at most 200 characters", i)
		}
		for env, val := range v.Values {
			if !ValidEnv(env) {
				add("variables[%d]: %q is not an environment name", i, env)
			}
			if len(val) > maxValueLen || strings.ContainsAny(val, "\n\r") {
				add("variables[%d]: the value for %s must be one line of at most %d characters", i, env, maxValueLen)
			}
		}
	}

	if len(s.Providers) > maxProviders {
		add("too many providers (max %d)", maxProviders)
	}
	pseen := map[string]bool{}
	for i, p := range s.Providers {
		switch {
		case !ValidProviderID(p.ID):
			add("providers[%d]: %q is not a provider id", i, p.ID)
		case pseen[p.ID]:
			add("providers[%d]: %q is listed twice", i, p.ID)
		}
		pseen[p.ID] = true
		if p.Version != "" && !ValidVersionConstraint(p.Version) {
			add("providers[%d]: the version %q must be one Terraform constraint such as ~> 4.0", i, p.Version)
		}
	}

	if len(s.Starters) > maxStarters {
		add("too many starter files (max %d)", maxStarters)
	}
	names := map[string]bool{}
	for i, f := range s.Starters {
		if validFilename(f.Filename) != nil {
			add("starters[%d]: %q must be a file name ending in .tf", i, f.Filename)
		}
		if names[f.Filename] {
			add("starters[%d]: %q is listed twice", i, f.Filename)
		}
		names[f.Filename] = true
		if len(f.Content) > maxStarterBytes {
			add("starters[%d]: the content is too long (max %d bytes)", i, maxStarterBytes)
			continue
		}
		if _, diags := hclsyntax.ParseConfig([]byte(strings.ReplaceAll(f.Content, "{folder}", "folder")), f.Filename, hcl.InitialPos); diags.HasErrors() {
			add("starters[%d]: %s is not valid HCL: %s", i, f.Filename, diags.Error())
		}
	}
	sort.Strings(p)
	return p
}

// varSet returns the declared variable names.
func (s *ProjectSettings) varSet() map[string]*ProjectVariable {
	m := map[string]*ProjectVariable{}
	if s == nil {
		return m
	}
	for i := range s.Variables {
		m[s.Variables[i].Name] = &s.Variables[i]
	}
	return m
}
