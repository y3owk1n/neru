//go:build linux

package linux

import (
	"path/filepath"
	"testing"
)

// TestSystemAdapter_Dirs_HonorXDGBaseDirectories pins the XDG Base Directory
// contract: an absolute env value wins, a relative value is ignored per the
// spec, and an unset variable falls back to the spec default under $HOME.
func TestSystemAdapter_Dirs_HonorXDGBaseDirectories(t *testing.T) {
	adapter := &SystemAdapter{}
	home := t.TempDir()
	t.Setenv("HOME", home)

	tests := []struct {
		name    string
		envVar  string
		envVal  string
		call    func() (string, error)
		want    string
		wantDef string
	}{
		{
			name:    "ConfigDir",
			envVar:  "XDG_CONFIG_HOME",
			envVal:  "/custom/config",
			call:    adapter.ConfigDir,
			want:    "/custom/config/neru",
			wantDef: filepath.Join(home, ".config", "neru"),
		},
		{
			name:    "UserDataDir",
			envVar:  "XDG_DATA_HOME",
			envVal:  "/custom/data",
			call:    adapter.UserDataDir,
			want:    "/custom/data/neru",
			wantDef: filepath.Join(home, ".local", "share", "neru"),
		},
		{
			name:    "LogDir",
			envVar:  "XDG_STATE_HOME",
			envVal:  "/custom/state",
			call:    adapter.LogDir,
			want:    filepath.Join("/custom/state", "neru", "log"),
			wantDef: filepath.Join(home, ".local", "state", "neru", "log"),
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name+" honors absolute env", func(t *testing.T) {
			t.Setenv(testCase.envVar, testCase.envVal)

			got, err := testCase.call()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != testCase.want {
				t.Errorf("got %q, want %q", got, testCase.want)
			}
		})

		t.Run(testCase.name+" ignores relative env", func(t *testing.T) {
			t.Setenv(testCase.envVar, "relative/path")

			got, err := testCase.call()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != testCase.wantDef {
				t.Errorf("got %q, want default %q", got, testCase.wantDef)
			}
		})

		t.Run(testCase.name+" defaults when unset", func(t *testing.T) {
			t.Setenv(testCase.envVar, "")

			got, err := testCase.call()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != testCase.wantDef {
				t.Errorf("got %q, want default %q", got, testCase.wantDef)
			}
		})
	}
}
