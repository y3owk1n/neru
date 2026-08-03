//go:build linux

//nolint:testpackage
package linux

import (
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/derrors"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

// noBackendManager returns a Manager in the state a session with no usable
// display server produces.
func noBackendManager() *Manager {
	return &Manager{}
}

// With no backend initialized (KWin, GNOME, headless), every Manager draw
// call must report CodeNotSupported. A stub returning nil would leave the mode
// handler believing a mode is displayed — keyboard capture armed, the user
// trapped in an invisible mode. A zero-value Manager is exactly the no-backend
// state, so these cases run headless; Init() is deliberately avoided, being a
// process-global singleton that probes for a display.
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
				return m.DrawMonitorSelect(nil, manager.MonitorSelectStyle{})
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
	mgr := noBackendManager()

	testGrid := domainGrid.NewGrid("abc", image.Rect(0, 0, 800, 600), zap.NewNop())

	err := mgr.DrawGrid(testGrid, "ab", grid.Style{})
	if !derrors.IsNotSupported(err) {
		t.Errorf("DrawGrid with a real grid returned %v, want CodeNotSupported", err)
	}

	drawn := []*hints.Hint{
		hints.NewHint("aa", image.Point{X: 10, Y: 10}, image.Point{X: 20, Y: 10}, ""),
	}

	err = mgr.DrawHintsWithStyle(drawn, hints.StyleMode{})
	if !derrors.IsNotSupported(err) {
		t.Errorf("DrawHintsWithStyle with real hints returned %v, want CodeNotSupported", err)
	}

	targets := []manager.MonitorSelectTarget{{Label: "A", Bounds: image.Rect(0, 0, 800, 600)}}

	err = mgr.DrawMonitorSelect(targets, manager.MonitorSelectStyle{})
	if !derrors.IsNotSupported(err) {
		t.Errorf("DrawMonitorSelect with real targets returned %v, want CodeNotSupported", err)
	}
}

// TestLinuxOverlayManager_StubsAreRepeatable guards against a stub that reports
// NotSupported once and then changes its answer — the mode handler retries a
// draw on screen-change events, and an inconsistent answer would let a later
// attempt look like success.
func TestLinuxOverlayManager_StubsAreRepeatable(t *testing.T) {
	mgr := noBackendManager()

	for i := range 3 {
		err := mgr.DrawHintsWithStyle(nil, hints.StyleMode{})
		if !derrors.IsNotSupported(err) {
			t.Fatalf("DrawHintsWithStyle call %d returned %v, want CodeNotSupported every time",
				i+1, err)
		}
	}
}
