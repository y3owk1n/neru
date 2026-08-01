//go:build linux

//nolint:testpackage
package overlay

// This is an internal test: reaching the stub path needs a Manager with no
// backend attached, and the backend fields are unexported by design.

import (
	"image"
	"testing"

	"go.uber.org/zap"

	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/core/infra/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/core/infra/overlay/render/recursivegrid"
)

// The Linux overlay Manager dispatches every draw call to whichever backend
// initialized — X11 or wlroots layer-shell — and falls through to a
// CodeNotSupported stub when neither did. That happens on KWin and GNOME today,
// and on any headless or unrecognized session.
//
// The mode handler treats an overlay draw failure as a reason to abandon mode
// activation, and it distinguishes "this platform can't draw" from "drawing
// broke" via derrors.IsNotSupported. A stub that returned nil would leave the
// handler believing a mode was displayed: keyboard capture stays armed and the
// user is trapped in an invisible mode with no way to see what they are typing.
//
// A zero-value Manager is exactly the no-backend state — both backend fields
// nil, a valid zero renderMu — so these cases reach the stub without opening a
// display connection, which also keeps them safe on a headless CI runner.
// Deliberately not using Init(): it is a process-global sync.Once singleton
// that probes for a display.

// noBackendManager returns a Manager in the state a session with no usable
// display server produces.
func noBackendManager() *Manager {
	return &Manager{}
}

func TestLinuxOverlayManager_DrawCallsReportNotSupportedWithNoBackend(t *testing.T) {
	tests := []struct {
		name string
		call func(*Manager) error
	}{
		{
			name: "DrawHintsWithStyle",
			call: func(m *Manager) error {
				return m.DrawHintsWithStyle(nil, hints.StyleMode{})
			},
		},
		{
			name: "DrawGrid",
			call: func(m *Manager) error {
				return m.DrawGrid(nil, "", grid.Style{})
			},
		},
		{
			name: "DrawRecursiveGrid",
			call: func(m *Manager) error {
				return m.DrawRecursiveGrid(
					image.Rect(0, 0, 100, 100),
					0, "", 2, 2, "", 2, 2,
					recursivegrid.Style{},
					recursivegrid.VirtualPointerState{},
				)
			},
		},
		{
			name: "DrawMonitorSelect",
			call: func(m *Manager) error {
				return m.DrawMonitorSelect(nil, MonitorSelectStyle{})
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.call(noBackendManager())
			if err == nil {
				t.Fatalf(
					"%s returned nil with no backend attached; the mode handler would believe "+
						"the overlay was drawn and arm keyboard capture over an invisible mode",
					testCase.name,
				)
			}

			if !derrors.IsNotSupported(err) {
				t.Errorf("%s returned %v (code %q), want CodeNotSupported",
					testCase.name, err, derrors.GetCode(err))
			}
		})
	}
}

// TestLinuxOverlayManager_DrawCallsAreSafeWithRealArguments repeats the check
// with populated arguments, so a stub that only short-circuits on nil input
// cannot pass by accident.
func TestLinuxOverlayManager_DrawCallsAreSafeWithRealArguments(t *testing.T) {
	manager := noBackendManager()

	testGrid := domainGrid.NewGrid("abc", image.Rect(0, 0, 800, 600), zap.NewNop())

	err := manager.DrawGrid(testGrid, "ab", grid.Style{})
	if !derrors.IsNotSupported(err) {
		t.Errorf("DrawGrid with a real grid returned %v, want CodeNotSupported", err)
	}

	drawn := []*hints.Hint{
		hints.NewHint("aa", image.Point{X: 10, Y: 10}, image.Point{X: 20, Y: 10}, ""),
	}

	err = manager.DrawHintsWithStyle(drawn, hints.StyleMode{})
	if !derrors.IsNotSupported(err) {
		t.Errorf("DrawHintsWithStyle with real hints returned %v, want CodeNotSupported", err)
	}

	targets := []MonitorSelectTarget{{Label: "A", Bounds: image.Rect(0, 0, 800, 600)}}

	err = manager.DrawMonitorSelect(targets, MonitorSelectStyle{})
	if !derrors.IsNotSupported(err) {
		t.Errorf("DrawMonitorSelect with real targets returned %v, want CodeNotSupported", err)
	}
}

// TestLinuxOverlayManager_StubsAreRepeatable guards against a stub that reports
// NotSupported once and then changes its answer — the mode handler retries a
// draw on screen-change events, and an inconsistent answer would let a later
// attempt look like success.
func TestLinuxOverlayManager_StubsAreRepeatable(t *testing.T) {
	manager := noBackendManager()

	for i := range 3 {
		err := manager.DrawHintsWithStyle(nil, hints.StyleMode{})
		if !derrors.IsNotSupported(err) {
			t.Fatalf("DrawHintsWithStyle call %d returned %v, want CodeNotSupported every time",
				i+1, err)
		}
	}
}
