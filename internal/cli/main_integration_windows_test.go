//go:build integration && windows

package cli

import (
	"os"
	"testing"
)

// TestMain exits at once when this test binary runs with `launch`.
// TestServiceTask_RoundTrip registers a task that runs the binary with
// `launch`, the argument the task gives the real daemon. A test binary ignores
// that argument and runs every test in the package again, the round trip
// included. Each nested run registers and starts another task, and the runs
// leave tasks and processes behind.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "launch" {
		os.Exit(0)
	}

	os.Exit(m.Run())
}
