//go:build linux && cgo

package textinput

import (
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/ports"
)

// TestTextInput_StartHintSearchSession_FallsBackWithoutACompositor pins the
// degrade the handler depends on. A field that cannot open has to report
// started == false and no error, so hint search keeps reading the key stream
// instead of switching the event tap off with nothing to type into.
func TestTextInput_StartHintSearchSession_FallsBackWithoutACompositor(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "neru-test-no-such-compositor")

	input := NewTextInput(zap.NewNop())

	started, err := input.StartHintSearchSession(
		t.Context(),
		ports.TextInputCallbacks{},
		ports.TextInputFrame{X: 0, Y: 0, Width: 200, Height: 30},
	)
	if err != nil {
		t.Fatalf("StartHintSearchSession() error = %v", err)
	}

	if started {
		t.Fatal("StartHintSearchSession() started a field with no compositor to show it")
	}

	err = input.StopHintSearchSession(t.Context())
	if err != nil {
		t.Fatalf("StopHintSearchSession() error = %v", err)
	}
}
