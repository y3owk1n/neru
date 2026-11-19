//go:build integration && !windows

package ipc_test

import (
	"fmt"
	"os"
	"testing"
)

// TestMain moves the endpoint into a directory of the run's own. NewServer
// binds DaemonPath and clears this user's socket there first, so a run that
// shares the real path unlinks a running daemon's socket and leaves the CLI
// unable to reach it until the daemon restarts.
func TestMain(m *testing.M) {
	// t.TempDir is not available here, and its names are too long for a
	// socket path anyway.
	dir, err := os.MkdirTemp("", "neru")
	if err != nil {
		fmt.Fprintln(os.Stderr, "MkdirTemp:", err)
		os.Exit(1)
	}

	for _, key := range []string{"TMPDIR", "XDG_RUNTIME_DIR"} {
		setErr := os.Setenv(key, dir)
		if setErr != nil {
			fmt.Fprintln(os.Stderr, "Setenv:", setErr)
			os.Exit(1)
		}
	}

	code := m.Run()

	_ = os.RemoveAll(dir)

	os.Exit(code)
}
