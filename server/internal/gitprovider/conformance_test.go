package gitprovider_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/gitprovidertest"
)

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func memoryFixture(t *testing.T) gitprovidertest.Fixture {
	m := gitprovider.NewMemory(gitprovidertest.Start)
	return gitprovidertest.Fixture{
		Source:  m,
		Branch:  m.Branch,
		Advance: func(c map[string]string) { m.Advance(c) },
	}
}

func localFixture(t *testing.T) gitprovidertest.Fixture {
	root := t.TempDir()
	needGit(t)
	run(t, root, "init", "-q", "-b", "main")
	project := filepath.Join(root, "iac")
	for p, c := range gitprovidertest.Start {
		full := filepath.Join(project, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run(t, root, "add", ".")
	run(t, root, "commit", "-q", "-m", "init")
	return gitprovidertest.Fixture{
		Source: gitprovider.Local{Dir: project},
		Branch: func(name string) map[string]string {
			names, err := gitOut(root, "ls-tree", "-r", "--name-only", name+":iac")
			if err != nil {
				return nil // no such branch
			}
			out := map[string]string{}
			for _, p := range strings.Fields(names) {
				content, err := gitOut(root, "show", name+":iac/"+p)
				if err != nil {
					t.Errorf("show %s: %v", p, err)
				}
				out[p] = content
			}
			return out
		},
	}
}

// gitOut runs git and returns stdout untrimmed, so file contents compare exactly.
func gitOut(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return string(out), err
}

func TestMemoryConforms(t *testing.T)         { gitprovidertest.Run(t, memoryFixture) }
func TestMemoryBranchesFromBase(t *testing.T) { gitprovidertest.RunBase(t, memoryFixture) }
func TestLocalConforms(t *testing.T)          { gitprovidertest.Run(t, localFixture) }
