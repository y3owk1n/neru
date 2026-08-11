//go:build linux

package linux

import (
	"image"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
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
					0, "", domain.GridDimensions{Rows: 2, Cols: 2},
					"", domain.GridDimensions{Rows: 2, Cols: 2},
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
		{
			name: "DrawHintSearchInput",
			call: func(m *Manager) error {
				return m.DrawHintSearchInput(
					"sav", 1,
					hints.NewSearchInputFrame(image.Pt(10, 10), 200),
					hints.SearchInputStyle{},
				)
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

// TestLinuxOverlayManager_HintSearchInputRefusesWhenNothingWasPainted pins what
// the refusal means now that both backends draw the badge. It used to mean
// "unimplemented here", and attaching X11 or wlroots changed nothing; it now
// means the draw put no pixels anywhere — a backend pointer whose native handle
// is closed or was never opened, which is the state a torn-down session leaves.
//
// Answering nil there is the failure the old body was written against from the
// other end: the mode handler would believe a query and a match count were on
// screen with nothing on it. What a live surface answers is
// TestLinuxOverlayManager_DrawHintSearchInput_PaintsTheQueryOnTheActiveScreen.
func TestLinuxOverlayManager_HintSearchInputRefusesWhenNothingWasPainted(t *testing.T) {
	tests := map[string]*Manager{
		"x11 attached, handle closed": {backend: linuxOverlayBackendX11, x11: &x11Overlay{}},
		"wlroots attached, handle closed": {
			backend: linuxOverlayBackendWaylandWlroots,
			wlroots: &wlrootsOverlay{},
		},
	}

	for name, mgr := range tests {
		t.Run(name, func(t *testing.T) {
			err := mgr.DrawHintSearchInput(
				"sav", 1,
				hints.NewSearchInputFrame(image.Pt(10, 10), 200),
				hints.SearchInputStyle{},
			)
			if !derrors.IsNotSupported(err) {
				t.Errorf("DrawHintSearchInput returned %v (code %q), want CodeNotSupported",
					err, derrors.GetCode(err))
			}
		})
	}
}

// TestLinuxOverlayManager_NoBackendDeclaresItselfHeadless pins the other half
// of the no-backend contract: a Manager with no surface has to say so, because
// its own component build reads that declaration before it builds the render
// overlays, and it must not hand the app components this Manager has no way to
// draw.
//
// The nil case follows the package's nil-guarded-delegate rule rather than
// blessing a typed nil: NewOverlayManager returns a nil *Manager when no
// display is detected, and the accessor that wraps it does not yet guard that,
// so Headless can be reached on a nil receiver and must answer rather than
// panic.
func TestLinuxOverlayManager_NoBackendDeclaresItselfHeadless(t *testing.T) {
	tests := map[string]*Manager{
		"no backend attached": noBackendManager(),
		"nil manager":         nil,
	}

	for name, mgr := range tests {
		t.Run(name, func(t *testing.T) {
			var overlayManager manager.Interface = mgr

			reporter, ok := overlayManager.(manager.HeadlessReporter)
			if !ok {
				t.Fatal("the Linux manager does not implement HeadlessReporter")
			}

			if !reporter.Headless() {
				t.Error("Headless() = false; there is no surface for an overlay to draw on")
			}

			// The declaration has to be what building reads. A backend that
			// re-derived it — from a window handle it can see, say — would
			// still pass the assertion above and then hand the app components
			// it has no surface to draw.
			built, err := overlayManager.BuildComponents(config.DefaultConfig(), nil)
			if err != nil {
				t.Fatalf("BuildComponents() error = %v, want nil", err)
			}

			if built != (manager.Components{}) {
				t.Errorf(
					"BuildComponents() = %+v, want nothing built: the manager declared itself headless",
					built,
				)
			}
		})
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

// TestLinuxOverlayManager_HideIndicatorErasesTheBadgeItPainted pins what
// hiding an indicator costs here: a Linux indicator is a badge painted onto
// the one shared overlay surface, not a window of its own, so hiding it means
// forgetting the rectangle it occupied. Without that an indicator turned off
// mid-session would stay on screen until the whole overlay is hidden, which is
// the mode ending rather than the indicator being turned off.
func TestLinuxOverlayManager_HideIndicatorErasesTheBadgeItPainted(t *testing.T) {
	painted := image.Rect(10, 10, 60, 30)

	tests := map[string]struct {
		indicator ports.Indicator
		paint     func(*Manager)
		stillOn   func(*Manager) bool
	}{
		"mode indicator": {
			indicator: ports.ModeIndicator,
			paint: func(m *Manager) {
				m.modeIndicatorBadgeVisible = true
				m.modeIndicatorBadgeRect = painted
			},
			stillOn: func(m *Manager) bool { return m.modeIndicatorBadgeVisible },
		},
		"sticky modifiers indicator": {
			indicator: ports.StickyModifiersIndicator,
			paint: func(m *Manager) {
				m.stickyBadgeVisible = true
				m.stickyBadgeRect = painted
			},
			stillOn: func(m *Manager) bool { return m.stickyBadgeVisible },
		},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			mgr := noBackendManager()
			testCase.paint(mgr)

			mgr.HideIndicator(testCase.indicator)

			if testCase.stillOn(mgr) {
				t.Error("HideIndicator left the badge painted on the shared overlay")
			}
		})
	}
}

// TestLinuxOverlayManager_HidingTheVirtualPointerTakesNoRenderLock pins the
// early return in HideIndicator, which is otherwise indistinguishable from the
// no-op switch case below it.
//
// The virtual pointer is hidden from the cursor-visibility path, which runs
// with the mode handler's lock held (`internal/app/modes/AGENTS.md`). It draws
// into its own surface, so there is no rectangle on the shared overlay to
// erase — and taking renderMu, which every draw contends for, would stall the
// handler for a case body that does nothing.
func TestLinuxOverlayManager_HidingTheVirtualPointerTakesNoRenderLock(t *testing.T) {
	mgr := noBackendManager()

	mgr.renderMu.Lock()
	defer mgr.renderMu.Unlock()

	done := make(chan struct{})

	go func() {
		defer close(done)

		mgr.HideIndicator(ports.VirtualPointerIndicator)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("HideIndicator blocked on renderMu while hiding the virtual pointer")
	}
}
