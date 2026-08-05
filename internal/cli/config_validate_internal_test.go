package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestRunConfigValidate_ReportsWhatLoadedAndWhatWillNotRun pins the reason a
// warning is told apart from a refusal at all: it does not stop the
// configuration loading, so unless this command prints it, it exists only in
// the daemon's log and never reaches the person checking their config.
func TestRunConfigValidate_ReportsWhatLoadedAndWhatWillNotRun(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		wantErr   bool
		wantLines []string
	}{
		{
			name:      "a clean configuration says only that it is valid",
			config:    "[hotkeys]\n\"Primary+Shift+K\" = \"hints --action left_click\"\n",
			wantLines: []string{"Configuration is valid"},
		},
		{
			name:   "an inert flag is named and the configuration still loads",
			config: "[hotkeys]\n\"Primary+Shift+K\" = \"grid --search\"\n",
			wantLines: []string{
				"Configuration is valid, with warnings:",
				"hotkeys.Primary+Shift+K: grid does not accept --search",
			},
		},
		{
			name:      "an unknown flag is refused",
			config:    "[hotkeys]\n\"Primary+Shift+K\" = \"hints --serach\"\n",
			wantErr:   true,
			wantLines: []string{"unknown flag: --serach"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")

			writeErr := os.WriteFile(path, []byte(testCase.config), 0o600)
			if writeErr != nil {
				t.Fatalf("WriteFile: %v", writeErr)
			}

			// The command reads the file named by the global --config flag,
			// which is the only way to point it at one; it is restored so the
			// next test finds it as it was.
			previous := configPath
			configPath = path

			defer func() { configPath = previous }()

			out := &bytes.Buffer{}
			cmd := &cobra.Command{}
			cmd.SetOut(out)
			cmd.SetErr(out)

			err := runConfigValidate(cmd)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("runConfigValidate() error = %v, wantErr = %v", err, testCase.wantErr)
			}

			for _, line := range testCase.wantLines {
				if !strings.Contains(out.String(), line) {
					t.Errorf("output = %q, want it to contain %q", out.String(), line)
				}
			}
		})
	}
}
