package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestPrintClickableRolesCheck covers the value that feeds the `neru doctor`
// exit status. A config that loads but selects no role on this platform cannot
// produce a hint, so it has to report as unhealthy rather than merely print a
// warning that a scripted health check would ignore.
func TestPrintClickableRolesCheck(t *testing.T) {
	// Roles that belong to a platform other than the one running the test.
	foreign := map[string]string{
		"darwin":  `["atspi:push button", "uia:Button"]`,
		"linux":   `["ax:AXButton", "uia:Button"]`,
		"windows": `["ax:AXButton", "atspi:push button"]`,
	}

	tests := []struct {
		name        string
		roles       string
		wantUsable  bool
		wantMessage string
	}{
		{
			name:        "roles that apply here are healthy",
			roles:       `["button", "link"]`,
			wantUsable:  true,
			wantMessage: "native roles on",
		},
		{
			name:        "roles for other platforms only",
			roles:       foreign[runtime.GOOS],
			wantUsable:  false,
			wantMessage: "hints would be empty",
		},
		{
			name:        "config that fails validation",
			roles:       `["AXButton"]`,
			wantUsable:  false,
			wantMessage: "config invalid",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.roles == "" {
				t.Skipf("no foreign role set defined for %s", runtime.GOOS)
			}

			path := filepath.Join(t.TempDir(), "config.toml")

			contents := `
[hints]
enabled = true
hint_characters = "asdf"
clickable_roles = ` + testCase.roles + `
`

			err := os.WriteFile(path, []byte(contents), 0o600)
			if err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			previous := configPath
			configPath = path

			t.Cleanup(func() { configPath = previous })

			var out bytes.Buffer

			cmd := &cobra.Command{}
			cmd.SetOut(&out)

			got := printClickableRolesCheck(cmd, doctorConfigLoad())
			if got != testCase.wantUsable {
				t.Errorf(
					"printClickableRolesCheck() = %v, want %v\noutput:\n%s",
					got, testCase.wantUsable, out.String(),
				)
			}

			if !strings.Contains(out.String(), testCase.wantMessage) {
				t.Errorf(
					"output does not mention %q:\n%s",
					testCase.wantMessage, out.String(),
				)
			}
		})
	}
}
