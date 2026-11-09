// Package envfile loads KEY=VALUE settings from a local file such as .env.local
// into the process environment, so `go run` works the same on Windows, macOS and
// Linux without a shell or `make`. It is for development: a deployment sets real
// environment variables, which always win over the file.
package envfile

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// Load reads path and sets every variable that is not already set in the
// environment. A missing file is not an error. It returns the names it set (never
// the values, which may be secrets).
func Load(path string) ([]string, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars, err := Parse(bufio.NewScanner(f))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var set []string
	for _, kv := range vars {
		if _, exists := os.LookupEnv(kv[0]); exists {
			continue // a real environment variable wins over the file
		}
		if err := os.Setenv(kv[0], kv[1]); err != nil {
			return set, err
		}
		set = append(set, kv[0])
	}
	return set, nil
}

// Parse turns lines into [name, value] pairs in file order. Rules:
//   - blank lines and lines starting with # are ignored
//   - an optional leading "export " is allowed
//   - a value may be wrapped in single or double quotes, which are removed; nothing
//     inside is interpreted, so a Windows path such as C:\Users\me\key.pem is kept as written
//   - an unquoted value ends at " #" (a comment) and is trimmed
//   - a line without "=" or with an invalid name is an error (the line number is
//     reported, never the content)
func Parse(sc *bufio.Scanner) ([][2]string, error) {
	var out [][2]string
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		name, value, ok := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !ok || !validName(name) {
			return nil, fmt.Errorf("line %d: expected NAME=value", n)
		}
		out = append(out, [2]string{name, unquote(strings.TrimSpace(value))})
	}
	return out, sc.Err()
}

func validName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') {
		if end := strings.IndexByte(v[1:], v[0]); end >= 0 {
			return v[1 : 1+end]
		}
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}
