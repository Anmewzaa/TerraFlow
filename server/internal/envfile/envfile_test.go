package envfile

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func parse(t *testing.T, text string) ([][2]string, error) {
	t.Helper()
	return Parse(bufio.NewScanner(strings.NewReader(text)))
}

func TestParse(t *testing.T) {
	text := "\ufeff# a comment\n" +
		"\n" +
		"TF_A=plain\n" +
		"export TF_B = spaced value  \n" +
		"TF_C=\"double quoted # not a comment\"\n" +
		"TF_D='single quoted'\n" +
		"TF_E=value # trailing comment\n" +
		"TF_F=\n" +
		"TF_G=C:\\Users\\me\\.terraflow\\key.pem\n" +
		"TF_H=\"C:\\Users\\me\\key.pem\"\n" +
		"TF_I=a=b=c\n" +
		"  TF_J=indented\n"
	got, err := parse(t, text)
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]string{
		{"TF_A", "plain"},
		{"TF_B", "spaced value"},
		{"TF_C", "double quoted # not a comment"},
		{"TF_D", "single quoted"},
		{"TF_E", "value"},
		{"TF_F", ""},
		{"TF_G", `C:\Users\me\.terraflow\key.pem`},
		{"TF_H", `C:\Users\me\key.pem`},
		{"TF_I", "a=b=c"},
		{"TF_J", "indented"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestParseRejectsBadLinesWithoutEchoingThem(t *testing.T) {
	for _, bad := range []string{"just words", "1BAD=x", "BAD-NAME=x", "=x", "BAD NAME=x"} {
		_, err := parse(t, "TF_OK=1\n"+bad+"\n")
		if err == nil {
			t.Errorf("%q must be an error", bad)
			continue
		}
		if !strings.Contains(err.Error(), "line 2") {
			t.Errorf("%q: the error must name the line: %v", bad, err)
		}
		if strings.Contains(err.Error(), "BAD") || strings.Contains(err.Error(), "words") {
			t.Errorf("%q: the error must not repeat the content: %v", bad, err)
		}
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.local")
	content := "TF_ENVFILE_NEW=from-file\nTF_ENVFILE_SET=from-file\nTF_ENVFILE_SECRET=s3cr3t\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TF_ENVFILE_SET", "from-environment")
	os.Unsetenv("TF_ENVFILE_NEW")
	os.Unsetenv("TF_ENVFILE_SECRET")
	t.Cleanup(func() { os.Unsetenv("TF_ENVFILE_NEW"); os.Unsetenv("TF_ENVFILE_SECRET") })

	set, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TF_ENVFILE_NEW"); got != "from-file" {
		t.Errorf("a new variable must be set from the file, got %q", got)
	}
	if got := os.Getenv("TF_ENVFILE_SET"); got != "from-environment" {
		t.Errorf("a real environment variable must win over the file, got %q", got)
	}
	if !reflect.DeepEqual(set, []string{"TF_ENVFILE_NEW", "TF_ENVFILE_SECRET"}) {
		t.Errorf("returned names: %v", set)
	}
	for _, name := range set {
		if strings.Contains(name, "s3cr3t") {
			t.Error("Load must return names only, never values")
		}
	}
}

func TestLoadMissingFileIsNotAnError(t *testing.T) {
	set, err := Load(filepath.Join(t.TempDir(), "nope"))
	if err != nil || len(set) != 0 {
		t.Fatalf("got %v %v", set, err)
	}
}

func TestLoadBadFileReportsTheFileAndLineOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env.local")
	if err := os.WriteFile(path, []byte("TF_OK=1\nsecret-without-equals\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "line 2") || strings.Contains(err.Error(), "secret-without-equals") {
		t.Fatalf("got %v", err)
	}
}
