package catalog

import (
	"regexp"
	"strings"
)

var refPattern = regexp.MustCompile(`\?ref=([^&]+)`)

// VersionFromSource is the version a module source pins, e.g. "v1.1.0" from
// "git::https://host/x.git//modules/rg?ref=v1.1.0". It is "" when the source pins none.
func VersionFromSource(source string) string {
	if m := refPattern.FindStringSubmatch(source); m != nil {
		return m[1]
	}
	return ""
}

// ResolveVersion is the catalog version whose schema describes a module pinned at wanted in a
// repository: the same version, else the newest of the same major (v1 for v1.2.0), else the
// latest one. versions are in the order the catalog lists them (oldest first). The repository's
// own source line is kept when a block is written back, so this only chooses the form to show.
// It moved here from the browser (D-065): which schema applies is not the browser's decision.
func ResolveVersion(wanted string, versions []string, latest string) string {
	if len(versions) == 0 {
		return wanted
	}
	for _, v := range versions {
		if v == wanted {
			return wanted
		}
	}
	if major := majorOf(wanted); major != "" {
		same := ""
		for _, v := range versions {
			if v == "v"+major || strings.HasPrefix(v, "v"+major+".") {
				same = v
			}
		}
		if same != "" {
			return same
		}
	}
	if latest != "" {
		return latest
	}
	return wanted
}

func majorOf(v string) string {
	v = strings.TrimPrefix(v, "v")
	n := 0
	for n < len(v) && v[n] >= '0' && v[n] <= '9' {
		n++
	}
	return v[:n]
}
