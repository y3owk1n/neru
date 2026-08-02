package textinput_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/textinput"
	"github.com/y3owk1n/neru/internal/ports"
)

// frame is an arbitrary non-zero frame; nothing asserts on its values, it just
// has to be passed through.
var frame = ports.TextInputFrame{X: 10, Y: 20, Width: 200, Height: 30}

// TestAdapter_NilInputDegradesInsteadOfPanicking pins the adapter's contract
// for a nil TextInput. The composition root can leave it unset on platforms
// with no native field, and hint search must fall back to the event tap's key
// stream rather than crash.
func TestAdapter_NilInputDegradesInsteadOfPanicking(t *testing.T) {
	t.Parallel()

	adapter := textinput.NewAdapter(nil, nil)

	started, err := adapter.StartHintSearchSession(t.Context(), ports.TextInputCallbacks{}, frame)
	if err != nil {
		t.Fatalf("StartHintSearchSession() error = %v, want nil", err)
	}

	if started {
		t.Error("StartHintSearchSession() started = true, want false for a nil input")
	}

	err = adapter.StopHintSearchSession(t.Context())
	if err != nil {
		t.Errorf("StopHintSearchSession() error = %v, want nil", err)
	}
}
