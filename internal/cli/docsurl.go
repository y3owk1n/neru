package cli

import "strings"

const docsVersionSegments = 3

// DocsURL returns the documentation URL for a given path and version.
func DocsURL(path, version string) string {
	tag := extractDocsTag(version)
	if tag == "" {
		tag = "main"
	}

	return "https://github.com/y3owk1n/neru/blob/" + tag + "/" + path
}

func extractDocsTag(version string) string {
	if version == "" {
		return ""
	}

	if !strings.HasPrefix(version, "v") {
		return ""
	}

	if idx := strings.Index(version, "-"); idx != -1 {
		version = version[:idx]
	}

	parts := strings.Split(version[1:], ".")
	if len(parts) != docsVersionSegments {
		return ""
	}

	for _, part := range parts {
		if part == "" {
			return ""
		}

		for _, ch := range part {
			if ch < '0' || ch > '9' {
				return ""
			}
		}
	}

	return version
}
