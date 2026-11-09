package gitprovider

import (
	"fmt"
	"path"
	"strings"
)

// ValidatePath checks a file path that will be written below a project folder and
// returns it cleaned (slash-separated). It refuses anything that could leave the
// folder or touch git's own data, on every operating system: empty paths, absolute
// paths, drive prefixes, backslashes, "." and ".." segments, NUL, and ".git".
func ValidatePath(p string) (string, error) {
	bad := func() (string, error) { return "", fmt.Errorf("%w: %q", ErrUnsafePath, p) }
	if p == "" || strings.ContainsAny(p, "\\\x00") || strings.HasPrefix(p, "/") || (len(p) >= 2 && p[1] == ':') {
		return bad()
	}
	clean := path.Clean(p)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return bad()
	}
	for _, seg := range strings.Split(clean, "/") {
		if strings.EqualFold(seg, ".git") {
			return bad()
		}
	}
	return clean, nil
}
