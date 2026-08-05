//go:build integration && darwin

package textinput_test

import (
	"os"
	"runtime"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	"github.com/y3owk1n/neru/internal/adapter/textinput"
	"github.com/y3owk1n/neru/internal/ports"
)

// Pin the main thread during package init so TestMain still runs on it.
func init() {
	runtime.LockOSThread()
}

// TestMain services the macOS main run loop while the tests run.
//
// The native text field is created and torn down on the main queue. A plain
// `go test` binary drains nothing, so calling into the real TextInput without
// this harness deadlocks — which is why these assertions live behind the
// integration tag rather than in adapter_test.go.
func TestMain(m *testing.M) {
	os.Exit(darwin.RunMainLoopForTesting(m.Run))
}

// TestTextInput_StopIsIdempotent covers the teardown path: mode exit and
// cleanup both call Stop, so the second call must be harmless.
func TestTextInput_StopIsIdempotent(t *testing.T) {
	adapter := textinput.NewAdapter(textinput.NewTextInput(nil), nil)

	for attempt := range 2 {
		err := adapter.StopHintSearchSession(t.Context())
		if err != nil {
			t.Fatalf("StopHintSearchSession() attempt %d error = %v, want nil", attempt+1, err)
		}
	}
}

// requireDesktop skips unless this run opted into tests that drive the real
// desktop (cursor, keyboard, overlays). `just test-desktop` sets the variable;
// plain `just test` stays hands-off the machine.
func requireDesktop(t *testing.T) {
	t.Helper()

	if os.Getenv("NERU_DESKTOP_TESTS") == "" {
		t.Skip("skipping desktop-driving test; run `just test-desktop` to include it")
	}
}

// TestTextInput_StartThenStop exercises a full session against the real
// NSTextField overlay.
func TestTextInput_StartThenStop(t *testing.T) {
	requireDesktop(t)

	adapter := textinput.NewAdapter(textinput.NewTextInput(nil), nil)

	started, err := adapter.StartHintSearchSession(
		t.Context(),
		ports.TextInputCallbacks{},
		ports.TextInputFrame{X: 0, Y: 0, Width: 200, Height: 30},
	)
	if err != nil {
		t.Fatalf("StartHintSearchSession() error = %v, want nil", err)
	}

	t.Cleanup(func() {
		_ = adapter.StopHintSearchSession(t.Context())
	})

	if !started {
		t.Error("StartHintSearchSession() started = false, want true on macOS")
	}
}
