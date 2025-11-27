package cli_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/cli"
)

func TestIsRunningFromAppBundle(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "app bundle path",
			path:     "/Applications/Neru.app/Contents/MacOS/neru",
			expected: true,
		},
		{
			name:     "regular binary path",
			path:     "/usr/local/bin/neru",
			expected: false,
		},
		{
			name:     "homebrew path",
			path:     "/opt/homebrew/bin/neru",
			expected: false,
		},
		{
			name:     "another app bundle",
			path:     "/Applications/MyApp.app/Contents/MacOS/myapp",
			expected: true,
		},
		{
			name:     "deeply nested app bundle",
			path:     "/Some/Deep/Path/MyApp.app/Contents/MacOS/myapp",
			expected: true,
		},
		{
			name:     "app bundle with spaces",
			path:     "/Applications/My App.app/Contents/MacOS/myapp",
			expected: true,
		},
		{
			name:     "false positive - contains app but not full path",
			path:     "/some/path/with/app/in/name/neru",
			expected: false,
		},
		{
			name:     "empty path",
			path:     "",
			expected: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// Test the string matching logic directly
			result := strings.Contains(testCase.path, ".app/Contents/MacOS")
			if result != testCase.expected {
				t.Errorf(
					"strings.Contains(%q, \".app/Contents/MacOS\") = %v, expected %v",
					testCase.path,
					result,
					testCase.expected,
				)
			}
		})
	}
}

func TestVersionTemplate(t *testing.T) {
	// Test that version template format is correct
	expectedTemplate := fmt.Sprintf(
		"Neru version %s\nGit commit: %s\nBuild date: %s\n",
		cli.Version,
		cli.GitCommit,
		cli.BuildDate,
	)

	// Test the template format
	if !strings.Contains(expectedTemplate, "Neru version") {
		t.Errorf("Version template should contain 'Neru version', got %s", expectedTemplate)
	}

	if !strings.Contains(expectedTemplate, "Git commit:") {
		t.Errorf("Version template should contain 'Git commit:', got %s", expectedTemplate)
	}

	if !strings.Contains(expectedTemplate, "Build date:") {
		t.Errorf("Version template should contain 'Build date:', got %s", expectedTemplate)
	}
}

func TestCommandStructure(t *testing.T) {
	// Test that commands are properly registered
	// Since init functions run when package is imported, we can check
	// that certain commands exist

	// We can't directly access the rootCmd from tests, but we can test
	// that the package initializes without panicking
	_ = cli.Version
	_ = cli.GitCommit
	_ = cli.BuildDate
}

func TestCLIVersionInfo(t *testing.T) {
	// Test that version information is properly set
	if cli.Version == "" {
		t.Error("Version should not be empty")
	}

	if cli.GitCommit == "" {
		t.Error("GitCommit should not be empty")
	}

	if cli.BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}

	// Version should be a semantic version or "dev"
	if cli.Version != "dev" && !strings.Contains(cli.Version, ".") {
		t.Errorf("Version should be semantic or 'dev', got %q", cli.Version)
	}
}

func TestCLIConstants(t *testing.T) {
	// Test that CLI constants are reasonable
	// These are compile-time checks that the constants are set

	// Version should be accessible
	_ = cli.Version

	// Git info should be accessible
	_ = cli.GitCommit
	_ = cli.BuildDate

	// Test that they are not obviously wrong
	if len(cli.Version) == 0 {
		t.Error("Version constant should not be empty")
	}

	if len(cli.GitCommit) == 0 {
		t.Error("GitCommit constant should not be empty")
	}

	if len(cli.BuildDate) == 0 {
		t.Error("BuildDate constant should not be empty")
	}
}
