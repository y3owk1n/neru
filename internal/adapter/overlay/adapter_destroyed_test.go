package overlay_test

import (
	"context"
	"image"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay"
	rendergrid "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	renderhints "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	renderrecursivegrid "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

// blockedDestroyTimeout bounds a Destroy that must return. A second caller
// waits on the first one's teardown, so a teardown that never publishes parks
// it for good and giving up on the call is the only way to observe that.
const blockedDestroyTimeout = 2 * time.Second

// countingManager is the backend as this file needs to see it: every call the
// adapter can make on one is recorded, so a test can say the backend was not
// reached at all rather than naming the method it expected.
//
// It declares the two optional capabilities as well — the keyboard grab and
// the capability report — because the port answers both by asking the backend,
// and a fake that declined them would make those two rows of the table below
// pass without the guard.
type countingManager struct {
	headlessManager

	mu    sync.Mutex
	calls int
}

var (
	_ overlay.ManagerInterface          = (*countingManager)(nil)
	_ overlay.CapabilityReporter        = (*countingManager)(nil)
	_ overlay.KeyboardCaptureController = (*countingManager)(nil)
)

func (m *countingManager) Show()                               { m.hit() }
func (m *countingManager) Hide()                               { m.hit() }
func (m *countingManager) Clear()                              { m.hit() }
func (m *countingManager) ClearCache()                         { m.hit() }
func (m *countingManager) ResizeToActiveScreen()               { m.hit() }
func (m *countingManager) SetActiveScreenOrigin(_ image.Point) { m.hit() }
func (m *countingManager) SwitchTo(_ overlay.Mode)             { m.hit() }
func (m *countingManager) Destroy()                            { m.hit() }

func (m *countingManager) Mode() overlay.Mode                           { m.hit(); return overlay.ModeIdle }
func (m *countingManager) HideHintSearchInput()                         { m.hit() }
func (m *countingManager) DrawModeIndicator(_, _ int)                   { m.hit() }
func (m *countingManager) UpdateGridMatches(_ string)                   { m.hit() }
func (m *countingManager) SetHideUnmatched(_ bool)                      { m.hit() }
func (m *countingManager) HideGridPointer(_ overlay.Mode)               { m.hit() }
func (m *countingManager) Flush()                                       { m.hit() }
func (m *countingManager) SetSharingType(_ bool)                        { m.hit() }
func (m *countingManager) SetKeyboardCaptureEnabled(_ bool)             { m.hit() }
func (m *countingManager) ShowIndicator(_ ports.Indicator)              { m.hit() }
func (m *countingManager) HideIndicator(_ ports.Indicator)              { m.hit() }
func (m *countingManager) DrawVirtualPointer(_, _ int, _ int, _ string) { m.hit() }

func (m *countingManager) ResizeIndicatorToActiveScreen(_ ports.Indicator) { m.hit() }

func (m *countingManager) DrawStickyModifiersIndicator(_, _ int, _ string) { m.hit() }

func (m *countingManager) DrawMouseActionIndicator(
	_ image.Point,
	_ ports.MouseActionIndicatorStyle,
) {
	m.hit()
}

func (m *countingManager) ConfigureComponents(_ *config.Config, _ overlay.PointerAppearance) {
	m.hit()
}

func (m *countingManager) DrawHintsWithStyle(
	_ []*renderhints.Hint,
	_ renderhints.StyleMode,
) error {
	m.hit()

	return nil
}

func (m *countingManager) DrawHintSearchInput(
	_ string,
	_ int,
	_ renderhints.SearchInputFrame,
	_ renderhints.SearchInputStyle,
) error {
	m.hit()

	return nil
}

func (m *countingManager) DrawGrid(_ *domainGrid.Grid, _ string, _ rendergrid.Style) error {
	m.hit()

	return nil
}

func (m *countingManager) DrawRecursiveGrid(
	_ image.Rectangle,
	_ int,
	_ string,
	_ domain.GridDimensions,
	_ string,
	_ domain.GridDimensions,
	_ renderrecursivegrid.Style,
	_ renderrecursivegrid.VirtualPointerState,
) error {
	m.hit()

	return nil
}

func (m *countingManager) ShowSubgrid(
	_ *domainGrid.Cell,
	_ rendergrid.Style,
	_ renderrecursivegrid.VirtualPointerState,
) {
	m.hit()
}

func (m *countingManager) DrawGridPointer(
	_ overlay.Mode,
	_ image.Point,
	_ overlay.PointerAppearance,
) {
	m.hit()
}

func (m *countingManager) OverlayCapabilities() ports.FeatureCapability {
	m.hit()

	return ports.FeatureCapability{
		Status: ports.FeatureStatusSupported,
		Detail: "counting overlay available",
	}
}

func (m *countingManager) hit() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls++
}

// reached reports how many times the backend has been called in all.
func (m *countingManager) reached() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.calls
}

// portCall is one call a caller can make on ports.OverlayPort, named the way a
// caller makes it.
type portCall struct {
	name string
	call func(ports.OverlayPort)
}

// everyPortCall is the whole of ports.OverlayPort bar two: Destroy, which is
// the subject rather than a case, and HintSearchBounds, which answers from the
// resolved Style and reaches no backend at all — what its guard changes is the
// answer, which the test below asserts instead.
//
// Every call here has to reach the backend while the overlay is alive and none
// of them may reach it afterwards, which is what makes the table worth having:
// a case that reached nothing either way would pass the guard without
// exercising it.
func everyPortCall() []portCall {
	screen := image.Rect(0, 0, 1920, 1080)

	return []portCall{
		{"Health", func(p ports.OverlayPort) { _ = p.Health(context.Background()) }},
		{"ShowFrame", func(p ports.OverlayPort) {
			_ = p.ShowFrame(context.Background(), ports.ScrollFrame{})
		}},
		{"RedrawFrame", func(p ports.OverlayPort) {
			_ = p.RedrawFrame(context.Background(), ports.GridFrame{Grid: destroyedTestGrid()})
		}},
		{"ClearFrame", func(p ports.OverlayPort) { _ = p.ClearFrame(context.Background()) }},
		{"SetActiveScreen", func(p ports.OverlayPort) { p.SetActiveScreen(screen) }},
		{"DrawHintSearch", func(p ports.OverlayPort) {
			_ = p.DrawHintSearch(ports.HintSearch{Screen: screen})
		}},
		{"HideHintSearch", func(p ports.OverlayPort) { p.HideHintSearch() }},
		{"UpdateGridMatches", func(p ports.OverlayPort) { p.UpdateGridMatches("a") }},
		{"SetGridHideUnmatched", func(p ports.OverlayPort) { p.SetGridHideUnmatched(true) }},
		{"ShowGridSubgrid", func(p ports.OverlayPort) {
			p.ShowGridSubgrid(&domainGrid.Cell{}, ports.GridPointer{})
		}},
		{"UpdateGridPointer", func(p ports.OverlayPort) {
			p.UpdateGridPointer(domain.ModeGrid, ports.GridPointer{Visible: true})
		}},
		{"DrawModeIndicator", func(p ports.OverlayPort) { p.DrawModeIndicator(1, 2) }},
		{"DrawStickyModifiersIndicator", func(p ports.OverlayPort) {
			p.DrawStickyModifiersIndicator(1, 2, "⌘")
		}},
		{"DrawVirtualPointer", func(p ports.OverlayPort) { p.DrawVirtualPointer(1, 2) }},
		{"DrawMouseActionIndicator", func(p ports.OverlayPort) {
			p.DrawMouseActionIndicator(image.Pt(1, 2), ports.MouseActionIndicatorStyle{})
		}},
		{"ShowIndicator", func(p ports.OverlayPort) { p.ShowIndicator(ports.ModeIndicator) }},
		{"HideIndicator", func(p ports.OverlayPort) { p.HideIndicator(ports.ModeIndicator) }},
		{"ResizeIndicatorToActiveScreen", func(p ports.OverlayPort) {
			p.ResizeIndicatorToActiveScreen(ports.ModeIndicator)
		}},
		{"Flush", func(p ports.OverlayPort) { p.Flush() }},
		{"IsVisible", func(p ports.OverlayPort) { _ = p.IsVisible() }},
		{"Refresh", func(p ports.OverlayPort) { _ = p.Refresh(context.Background()) }},
		{"ApplyConfig", func(p ports.OverlayPort) { p.ApplyConfig(config.DefaultConfig()) }},
		{"RefreshStyles", func(p ports.OverlayPort) { p.RefreshStyles() }},
		{"SetHiddenInScreenShare", func(p ports.OverlayPort) { p.SetHiddenInScreenShare(true) }},
		{
			"SetKeyboardCaptureEnabled",
			func(p ports.OverlayPort) { p.SetKeyboardCaptureEnabled(true) },
		},
	}
}

// destroyedPort builds the adapter over a counting backend, with a real
// resolver so the two notifications a reload owes the overlay reach the
// backend the way they do in the daemon.
func destroyedPort(manager overlay.ManagerInterface) ports.OverlayPort {
	return overlay.NewAdapter(
		manager,
		overlay.NewStyleResolver(manager, config.DefaultConfig(), nil, zap.NewNop()),
		zap.NewNop(),
	)
}

// TestAdapterDestroy_StopsEveryPortCallFromReachingTheBackend is the guard the
// shutdown ordering no longer has to carry alone (#1515): once the overlay has
// been released, nothing a caller still holding the port does may reach the
// backend that was freed. The event tap's drain is the caller that matters —
// it delivers a queued key into the mode handler while the daemon is tearing
// itself down — but the rule is stated for every call, because a guard that
// covers only the paths reachable today is a guard the next call site
// silently escapes.
func TestAdapterDestroy_StopsEveryPortCallFromReachingTheBackend(t *testing.T) {
	t.Parallel()

	for _, testCase := range everyPortCall() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			manager := &countingManager{}
			port := destroyedPort(manager)

			testCase.call(port)

			alive := manager.reached()
			if alive == 0 {
				t.Fatalf("%s never reached the backend while the overlay was alive", testCase.name)
			}

			port.Destroy()

			afterDestroy := manager.reached()

			testCase.call(port)

			if got := manager.reached(); got != afterDestroy {
				t.Errorf(
					"%s reached the backend %d more times after Destroy, want 0",
					testCase.name, got-afterDestroy,
				)
			}
		})
	}
}

// TestAdapterDestroy_AnswersACallerThatRacedTheTeardown pins what the guarded
// calls answer, which is not the same question as whether they reached the
// backend. A mode exiting into a shutdown is a race the shutdown already won,
// not a failure the user needs told about, so the drawing calls report success
// the way the event tap adapter's Enable and Disable do — while the two that
// report screen state have to say there is none, because there is not.
func TestAdapterDestroy_AnswersACallerThatRacedTheTeardown(t *testing.T) {
	t.Parallel()

	manager := &countingManager{}
	port := destroyedPort(manager)

	port.Destroy()

	showErr := port.ShowFrame(context.Background(), ports.ScrollFrame{})
	if showErr != nil {
		t.Errorf("ShowFrame() after Destroy = %v, want nil", showErr)
	}

	redrawErr := port.RedrawFrame(
		context.Background(),
		ports.GridFrame{Grid: destroyedTestGrid()},
	)
	if redrawErr != nil {
		t.Errorf("RedrawFrame() after Destroy = %v, want nil", redrawErr)
	}

	clearErr := port.ClearFrame(context.Background())
	if clearErr != nil {
		t.Errorf("ClearFrame() after Destroy = %v, want nil", clearErr)
	}

	searchErr := port.DrawHintSearch(ports.HintSearch{})
	if searchErr != nil {
		t.Errorf("DrawHintSearch() after Destroy = %v, want nil", searchErr)
	}

	refreshErr := port.Refresh(context.Background())
	if refreshErr != nil {
		t.Errorf("Refresh() after Destroy = %v, want nil", refreshErr)
	}

	healthErr := port.Health(context.Background())
	if !derrors.IsNotSupported(healthErr) {
		t.Errorf("Health() after Destroy = %v, want a CodeNotSupported error", healthErr)
	}

	if port.IsVisible() {
		t.Error("IsVisible() after Destroy = true, want false: nothing is on screen")
	}

	if bounds := port.HintSearchBounds(image.Rect(0, 0, 1920, 1080)); !bounds.Empty() {
		t.Errorf("HintSearchBounds() after Destroy = %v, want the empty rectangle", bounds)
	}
}

// TestAdapterDestroy_IsSafeToCallTwice covers the postcondition a second
// caller needs: the overlay is released when Destroy returns, whoever ran the
// teardown, and the backend is not released twice. Only App.Cleanup calls it
// today, under a sync.Once — the guarantee is for the caller a later change
// adds, which is the same reason the guard above exists.
func TestAdapterDestroy_IsSafeToCallTwice(t *testing.T) {
	t.Parallel()

	manager := &countingManager{}
	port := destroyedPort(manager)

	port.Destroy()

	released := manager.reached()

	done := make(chan struct{})

	go func() {
		defer close(done)

		port.Destroy()
	}()

	select {
	case <-done:
	case <-time.After(blockedDestroyTimeout):
		t.Fatal("a second Destroy did not return")
	}

	if got := manager.reached(); got != released {
		t.Errorf("a second Destroy released the backend again (%d calls, want %d)", got, released)
	}
}

// destroyedTestGrid is the smallest grid a redraw can carry.
func destroyedTestGrid() *domainGrid.Grid {
	return domainGrid.NewGridWithLabels("abcd", "", "", image.Rect(0, 0, 400, 300), zap.NewNop())
}
