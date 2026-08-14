//go:build darwin

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDaemonStderrPath_ResolvesUnderTheUserOwnLogDirectory pins where the login
// agent's standard error lands. launchd expands nothing it reads out of a
// plist, so a "~" or a "$HOME" written into one is a literal directory name,
// and the shared fallback a wrong answer used to reach is world-readable.
func TestDaemonStderrPath_ResolvesUnderTheUserOwnLogDirectory(t *testing.T) {
	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		t.Skipf("no home directory on this machine: %v", homeErr)
	}

	got, err := daemonStderrPath()
	if err != nil {
		t.Fatalf("daemonStderrPath() error = %v", err)
	}

	want := filepath.Join(home, "Library", "Logs", "neru", "daemon.err.log")
	if got != want {
		t.Errorf("daemonStderrPath() = %q, want %q", got, want)
	}
}

// TestRenderPlist_SendsOutputOnlyToThePathItIsGiven pins the plist the agent is
// installed from: the stderr redirect names the file it was handed, nothing
// names a shared directory, and standard output is left unredirected, since the
// rotated log file already holds every log line the console core writes there.
func TestRenderPlist_SendsOutputOnlyToThePathItIsGiven(t *testing.T) {
	const (
		binPath    = "/Users/tester/bin/neru"
		stderrPath = "/Users/tester/Library/Logs/neru/daemon.err.log"
	)

	plist := renderPlist(binPath, stderrPath)

	testCases := []struct {
		name    string
		needle  string
		present bool
	}{
		{name: "runs the resolved binary", needle: binPath, present: true},
		{
			name:    "redirects stderr to the given file",
			needle:  "<key>StandardErrorPath</key>\n    <string>" + stderrPath + "</string>",
			present: true,
		},
		{name: "leaves stdout unredirected", needle: "StandardOutPath", present: false},
		{name: "names no shared temp directory", needle: "/tmp", present: false},
		{name: "leaves no placeholder behind", needle: "NERU_", present: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if strings.Contains(plist, testCase.needle) != testCase.present {
				t.Errorf(
					"rendered plist contains %q = %v, want %v:\n%s",
					testCase.needle,
					!testCase.present,
					testCase.present,
					plist,
				)
			}
		})
	}
}
