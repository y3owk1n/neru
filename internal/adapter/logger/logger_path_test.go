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

	t.Setenv("XDG_STATE_HOME", "")

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

func TestDefaultLogFilePathWindowsFallback(t *testing.T) {
	if runtime.GOOS != goosWindows {
		t.Skip("Windows-only test")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	t.Setenv("LOCALAPPDATA", "")

	got, err := defaultLogFilePath()
	if err != nil {
		t.Fatalf("DefaultLogFilePath() fallback error = %v", err)
	}

	want := filepath.Join(home, "AppData", "Local", "neru", "log", "app.log")
	if got != want {
		t.Fatalf("DefaultLogFilePath() fallback = %q, want %q", got, want)
	}
}

// TestDefaultLogFilePathHonorsXDGStateHome pins that on platforms using the
// XDG fallback branch, an absolute XDG_STATE_HOME redirects the default log
// path, matching the Linux system adapter's LogDir.
func TestDefaultLogFilePathHonorsXDGStateHome(t *testing.T) {
	if runtime.GOOS == goosDarwin || runtime.GOOS == goosWindows {
		t.Skip("XDG branch applies to the non-darwin, non-windows default only")
	}

	t.Setenv("XDG_STATE_HOME", "/custom/state")

	got, err := defaultLogFilePath()
	if err != nil {
		t.Fatalf("DefaultLogFilePath() error = %v", err)
	}

	want := filepath.Join("/custom/state", "neru", "log", "app.log")
	if got != want {
		t.Fatalf("DefaultLogFilePath() = %q, want %q", got, want)
	}
}
