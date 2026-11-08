package hcl_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclwrite"
)

// samples/README.md promises every sample file parses and is terraform-fmt
// clean. Keep that true so the fixtures stay a trustworthy style reference.
func TestSamplesAreFormattedAndSynthetic(t *testing.T) {
	root := findSamplesDir(t)
	count := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".tf") {
			return err
		}
		count++
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		src := strings.ReplaceAll(string(b), "\r\n", "\n")
		rel, _ := filepath.Rel(root, path)
		if got := string(hclwrite.Format([]byte(src))); got != src {
			t.Errorf("%s is not terraform-fmt clean", rel)
		}
		// Samples are synthetic: no real subscription, tenant or bank names.
		for _, bad := range []string{"proj1", "project-1"} {
			if strings.Contains(src, bad) {
				t.Errorf("%s still mentions the old sample name %q", rel, bad)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count < 40 {
		t.Fatalf("only %d sample .tf files found", count)
	}
}
