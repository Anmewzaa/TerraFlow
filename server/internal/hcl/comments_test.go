package hcl

import (
	"strings"
	"testing"
)

const commentedFile = `module "rg_app" {
  source = "` + rgSrc + `"

  # Keep this name in step with the naming policy.
  # (two lines on purpose)
  name     = local.rg_names.app
  location = var.location # never change the region
  // the lock is off outside prod
  lock_level = var.environment == "prod" ? "CanNotDelete" : ""
  tags       = local.common_tags # shared tags
  # about an argument that is removed below
  old_flag = true
  # dangling comment at the end
}
`

func TestEditKeepsCommentsOfTheArgumentsThatSurvive(t *testing.T) {
	n := newRG("rg_app")
	n.Values = map[string]interface{}{"name": "rg-edited", "location": "japaneast"}
	n.Existing = &ExistingRef{Folder: "common", Filename: "rg.tf", Label: "rg_app"}
	repo := []GeneratedFile{{Folder: "common", Filename: "rg.tf", Content: commentedFile}}

	out, err := Apply(GenerateInput{Nodes: []GenerateNode{n}}, repo)
	if err != nil {
		t.Fatal(err)
	}
	got := out[0].Content
	for _, want := range []string{
		"# Keep this name in step with the naming policy.\n  # (two lines on purpose)\n  name",
		`"japaneast" # never change the region`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, gone := range []string{"about an argument that is removed", "dangling comment"} {
		if strings.Contains(got, gone) {
			t.Errorf("%q should be dropped with its argument or the block end:\n%s", gone, got)
		}
	}
	if strings.Count(got, "never change the region") != 1 {
		t.Errorf("a comment was duplicated:\n%s", got)
	}
}

func TestEditingTwiceDoesNotMultiplyComments(t *testing.T) {
	n := newRG("rg_app")
	n.Existing = &ExistingRef{Folder: "common", Filename: "rg.tf", Label: "rg_app"}
	repo := []GeneratedFile{{Folder: "common", Filename: "rg.tf", Content: commentedFile}}
	once, err := Apply(GenerateInput{Nodes: []GenerateNode{n}}, repo)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := Apply(GenerateInput{Nodes: []GenerateNode{n}}, []GeneratedFile{{Folder: "common", Filename: "rg.tf", Content: once[0].Content}})
	if err != nil {
		t.Fatal(err)
	}
	if once[0].Content != twice[0].Content {
		t.Errorf("a second identical edit changed the file:\n%s\n---\n%s", once[0].Content, twice[0].Content)
	}
}

func TestEditKeepsCommentsWithWindowsLineEndings(t *testing.T) {
	n := newRG("rg_app")
	n.Existing = &ExistingRef{Folder: "common", Filename: "rg.tf", Label: "rg_app"}
	crlf := strings.ReplaceAll(commentedFile, "\n", "\r\n")
	out, err := Apply(GenerateInput{Nodes: []GenerateNode{n}}, []GeneratedFile{{Folder: "common", Filename: "rg.tf", Content: crlf}})
	if err != nil {
		t.Fatal(err)
	}
	got := out[0].Content
	if !strings.Contains(got, "# never change the region\r\n") || strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Errorf("line endings or comment lost:\n%q", got)
	}
}

func TestCarryCommentsLeavesABlockWithoutCommentsAlone(t *testing.T) {
	const a = "module \"x\" {\n  source = \"s\"\n  name = 1\n}"
	const b = "module \"x\" {\n  source = \"s\"\n  name = 2\n}"
	if got := carryComments(a, b); got != b {
		t.Errorf("got %q", got)
	}
	if got := carryComments("not hcl {", b); got != b {
		t.Errorf("an unparsable old block must leave the new one as it is: %q", got)
	}
}
