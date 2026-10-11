package cli_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/buildinfo"
)

const (
	docsCLIPath       = "docs/reference/cli.md"
	mainDocsCLISuffix = "/main/docs/reference/cli.md"
)

func TestDocsURLUsesVersionTagOrMain(t *testing.T) {
	testCases := []struct {
		name       string
		version    string
		path       string
		wantSuffix string
	}{
		{
			name:       "empty version falls back to main",
			version:    "",
			path:       docsCLIPath,
			wantSuffix: mainDocsCLISuffix,
		},
		{
			name:       "non semver falls back to main",
			version:    "dev",
			path:       docsCLIPath,
			wantSuffix: mainDocsCLISuffix,
		},
		{
			name:       "valid release tag",
			version:    "v1.19.0",
			path:       docsCLIPath,
			wantSuffix: "/v1.19.0/docs/reference/cli.md",
		},
		{
			name:       "git describe with commits falls back to main",
			version:    "v1.19.0-3-gabcdef0",
			path:       "docs/reference/configuration.md",
			wantSuffix: "/main/docs/reference/configuration.md",
		},
		{
			name:       "git describe with commits and dirty falls back to main",
			version:    "v1.19.0-3-gabcdef0-dirty",
			path:       docsCLIPath,
			wantSuffix: mainDocsCLISuffix,
		},
		{
			name:       "git describe dirty state",
			version:    "v1.19.0-dirty",
			path:       docsCLIPath,
			wantSuffix: "/v1.19.0/docs/reference/cli.md",
		},
		{
			name:       "invalid semver segments",
			version:    "v1.2",
			path:       docsCLIPath,
			wantSuffix: mainDocsCLISuffix,
		},
		{
			name:       "non numeric segment",
			version:    "v1.2.x",
			path:       docsCLIPath,
			wantSuffix: mainDocsCLISuffix,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			url := buildinfo.DocsURL(testCase.path, testCase.version)
			if !strings.HasSuffix(url, testCase.wantSuffix) {
				t.Errorf("docs URL = %q, want suffix %q", url, testCase.wantSuffix)
			}
		})
	}
}

// TestRawURLUsesTheSameTreeAsDocsURL pins that a config's #:schema line names
// the schema from the tree its build's docs link to.
func TestRawURLUsesTheSameTreeAsDocsURL(t *testing.T) {
	testCases := []struct {
		version string
		want    string
	}{
		{
			"v1.19.0",
			"https://raw.githubusercontent.com/y3owk1n/neru/v1.19.0/schemas/config.schema.json",
		},
		{
			"v1.19.0-3-gabcdef0",
			"https://raw.githubusercontent.com/y3owk1n/neru/main/schemas/config.schema.json",
		},
	}
	for _, testCase := range testCases {
		got := buildinfo.RawURL("schemas/config.schema.json", testCase.version)
		if got != testCase.want {
			t.Errorf("RawURL(%q) = %q, want %q", testCase.version, got, testCase.want)
		}
	}
}
