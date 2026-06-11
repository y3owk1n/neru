package cli_test

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/cli"
)

const (
	docsCLIPath       = "docs/CLI.md"
	mainDocsCLISuffix = "/main/docs/CLI.md"
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
			path:       docsCLIPath,
			wantSuffix: "/v1.19.0/docs/CLI.md",
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
			url := cli.DocsURL(testCase.path, testCase.version)
			if !strings.HasSuffix(url, testCase.wantSuffix) {
				t.Errorf("docs URL = %q, want suffix %q", url, testCase.wantSuffix)
			}
		})
	}
}
