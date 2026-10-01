package buildinfo

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

	// A build with commits past its tag, such as v1.2.3-4-gabcdef, links to
	// main. The tag may lack docs this build links to.
	version = strings.TrimSuffix(version, "-dirty")
	if strings.Contains(version, "-") {
		return ""
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
