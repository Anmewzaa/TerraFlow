package catalog

import (
	"encoding/json"
	"strings"
	"testing"
)

const sampleVariables = `
variable "name" {
  type        = string
  description = "Name of the thing."
}

variable "replicas" {
  type    = number
  default = 2
}

variable "public" {
  type    = bool
  default = false
}

variable "tags" {
  type    = map(string)
  default = {}
}

variable "zones" {
  type    = list(string)
  default = ["1", "2"]
}

variable "rules" {
  type = list(object({
    port     = number
    protocol = optional(string, "tcp")
  }))
  default = []
}

variable "anything" {
  type    = any
  default = null
}

variable "computed" {
  type    = string
  default = local.x
}

output "ignored" { value = 1 }
`

func TestDraftFromVariablesIsAValidSchemaInDeclarationOrder(t *testing.T) {
	raw, notes, err := DraftFromVariables("my-widget", []byte(sampleVariables))
	if err != nil {
		t.Fatal(err)
	}
	if rep := Validate("my-widget", raw); !rep.Valid {
		t.Fatalf("a draft must pass the same check as an import: %+v\n%s", rep.Errors, raw)
	}
	var s struct {
		Title    string   `json:"title"`
		Required []string `json:"required"`
		Props    map[string]struct {
			Type    string          `json:"type"`
			Desc    string          `json:"description"`
			Default json.RawMessage `json:"default"`
			Items   json.RawMessage `json:"items"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Required) != 1 || s.Required[0] != "name" {
		t.Errorf("only a variable with no default is required: %v", s.Required)
	}
	want := map[string]string{"name": "string", "replicas": "number", "public": "boolean", "tags": "object",
		"zones": "array", "rules": "array", "anything": "string", "computed": "string"}
	for k, ty := range want {
		if s.Props[k].Type != ty {
			t.Errorf("%s: type %q, want %q", k, s.Props[k].Type, ty)
		}
	}
	if s.Props["name"].Desc != "Name of the thing." || string(s.Props["replicas"].Default) != "2" {
		t.Errorf("description and default are carried over: %+v", s.Props)
	}
	if len(s.Props["computed"].Default) != 0 || len(s.Props["anything"].Default) != 0 {
		t.Error("an expression or a null default is not written as a default")
	}
	if items := strings.Join(strings.Fields(string(s.Props["rules"].Items)), ""); !strings.Contains(items, `"required":["port"]`) {
		t.Errorf("an object's attributes without optional() are required: %s", s.Props["rules"].Items)
	}
	// The text keeps the order of the file, so the draft reads like the module.
	if i, j := strings.Index(string(raw), `"name"`), strings.Index(string(raw), `"replicas"`); i < 0 || i > j {
		t.Error("inputs must keep the order of variables.tf")
	}
	joined := strings.Join(notes, "\n")
	for _, w := range []string{`"anything"`, `"computed"`, "x-tf-module"} {
		if !strings.Contains(joined, w) {
			t.Errorf("notes should mention %s:\n%s", w, joined)
		}
	}
}

func TestDraftFromVariablesRefusesWhatItCannotRead(t *testing.T) {
	if _, _, err := DraftFromVariables("x-y", []byte(`variable "a" {`)); err == nil {
		t.Error("broken Terraform must be an error")
	}
	if _, _, err := DraftFromVariables("x-y", []byte(`locals { a = 1 }`)); err == nil {
		t.Error("text with no variable must be an error")
	}
}
