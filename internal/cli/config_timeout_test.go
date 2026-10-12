package cli_test

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain"
)

// slowDaemon serves this user's endpoint from a directory the test owns,
// answering ping at once and everything else only after delay.
func slowDaemon(t *testing.T, delay time.Duration) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("the Windows pipe name cannot be moved to a test's own")
	}

	//nolint:usetesting // t.TempDir names its directory after the test, which is
	// far too long for a socket path.
	dir, err := os.MkdirTemp("", "neru")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("TMPDIR", dir)
	t.Setenv("XDG_RUNTIME_DIR", dir)

	server, err := ipc.NewServer(func(ctx context.Context, cmd ipc.Command) ipc.Response {
		if cmd.Action != domain.CommandPing {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
			}
		}

		return ipc.Response{Success: true, Code: ipc.CodeOK}
	}, nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	server.Start()

	t.Cleanup(func() { _ = server.Stop() })
}

// TestConfigCommands_HonourTheTimeout pins that config set, config reset and
// config dump give up after --timeout, as every other command does, rather
// than after a fixed wait of their own.
func TestConfigCommands_HonourTheTimeout(t *testing.T) {
	slowDaemon(t, 3*time.Second)

	for _, args := range [][]string{
		{cliTestConfig, "set", "hints.strategy", "axtree"},
		{cliTestConfig, "reset", "hints.strategy"},
		{cliTestConfig, "dump"},
	} {
		t.Run(strings.Join(args[:2], " "), func(t *testing.T) {
			start := time.Now()

			_, _, err := executeRoot(t, append(args, "--timeout", "1")...)
			if err == nil {
				t.Fatalf("neru %v succeeded against a daemon slower than --timeout", args)
			}

			if strings.Contains(err.Error(), "not running") {
				t.Fatalf("neru %v = %v, want a timeout from a running daemon", args, err)
			}

			if elapsed := time.Since(start); elapsed > 2500*time.Millisecond {
				t.Errorf("neru %v gave up after %s, want about the 1s --timeout", args, elapsed)
			}
		})
	}
}
