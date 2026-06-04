package logger_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/logger"
)

func TestDefaultLogFilePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
		t.Setenv("HOMEDRIVE", "")
		t.Setenv("HOMEPATH", "")
		t.Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
	}

	got, err := logger.DefaultLogFilePath()
	if err != nil {
		t.Fatalf("DefaultLogFilePath() error = %v", err)
	}

	var want string

	switch runtime.GOOS {
	case "darwin":
		want = filepath.Join(home, "Library", "Logs", "neru", "app.log")
	case "windows":
		want = filepath.Join(home, "AppData", "Local", "neru", "log", "app.log")
	default:
		want = filepath.Join(home, ".local", "state", "neru", "log", "app.log")
	}

	if got != want {
		t.Fatalf("DefaultLogFilePath() = %q, want %q", got, want)
	}
}
