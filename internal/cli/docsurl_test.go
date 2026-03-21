package cli_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/cli"
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
			path:       "docs/CLI.md",
			wantSuffix: "/main/docs/CLI.md",
		},
		{
			name:       "non semver falls back to main",
			version:    "dev",
			path:       "docs/CLI.md",
			wantSuffix: "/main/docs/CLI.md",
		},
		{
			name:       "valid release tag",
			version:    "v1.19.0",
			path:       "docs/CLI.md",
			wantSuffix: "/v1.19.0/docs/CLI.md",
		},
		{
			name:       "git describe with commits",
			version:    "v1.19.0-3-gabcdef0",
			path:       "docs/CONFIGURATION.md",
			wantSuffix: "/v1.19.0/docs/CONFIGURATION.md",
		},
		{
			name:       "git describe dirty state",
			version:    "v1.19.0-dirty",
			path:       "docs/CLI.md",
			wantSuffix: "/v1.19.0/docs/CLI.md",
		},
		{
			name:       "invalid semver segments",
			version:    "v1.2",
			path:       "docs/CLI.md",
			wantSuffix: "/main/docs/CLI.md",
		},
		{
			name:       "non numeric segment",
			version:    "v1.2.x",
			path:       "docs/CLI.md",
			wantSuffix: "/main/docs/CLI.md",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			url := cli.DocsURL(testCase.path, testCase.version)
			if !strings.HasSuffix(url, testCase.wantSuffix) {
				t.Errorf("docs URL = %q, want suffix %q", url, testCase.wantSuffix)
			}
		})
	}
}
