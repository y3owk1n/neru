//nolint:testpackage
package logger

import (
	"path/filepath"
	"runtime"
	"testing"
)

const (
	goosWindows = "windows"
	goosDarwin  = "darwin"
)

func TestDefaultLogFilePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if runtime.GOOS == goosWindows {
		t.Setenv("USERPROFILE", home)
		t.Setenv("HOMEDRIVE", "")
		t.Setenv("HOMEPATH", "")
		t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	}

	got, err := defaultLogFilePath()
	if err != nil {
		t.Fatalf("DefaultLogFilePath() error = %v", err)
	}

	var want string

	switch runtime.GOOS {
	case goosDarwin:
		want = filepath.Join(home, "Library", "Logs", "neru", "app.log")
	case goosWindows:
		want = filepath.Join(home, "AppData", "Local", "neru", "log", "app.log")
	default:
		want = filepath.Join(home, ".local", "state", "neru", "log", "app.log")
	}

	if got != want {
		t.Fatalf("DefaultLogFilePath() = %q, want %q", got, want)
	}
}
