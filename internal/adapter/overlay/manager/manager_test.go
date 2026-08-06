package manager_test

import (
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

// NoOpManager is what runs when there is no overlay to draw on: a headless CI
// job, and the GNOME Wayland path where NewSystemPort refuses to start. Nothing
// verified it, so a method that panicked or returned a non-zero value would
// only show up as a crash in the one environment nobody watches.
//
// Every method is called here. The point is not the return values — they are
// zero by definition — it is that the full surface is reachable without a
// display and without panicking.
func TestNoOpManager_AnswersEveryCallWithoutADisplay(t *testing.T) {
	noOp := &manager.NoOpManager{}

	// Lifecycle and visibility.
	noOp.Show()
	noOp.Hide()
	noOp.Clear()
	noOp.ClearCache()
	noOp.ResizeToActiveScreen()
	noOp.SetActiveScreenOrigin(image.Pt(100, 200))
	noOp.SwitchTo(manager.ModeHints)
	noOp.Flush()
	noOp.SetSharingType(true)
	noOp.SetHideUnmatched(true)
	noOp.UpdateGridMatches("ab")
	noOp.Destroy()

	// Drawing.
	noOp.DrawModeIndicator(1, 2)
	noOp.DrawStickyModifiersIndicator(1, 2, "⌘")
	noOp.DrawVirtualPointer(1, 2, 3, "#ffffff")
	noOp.HideHintSearchInput()

	// Indicators.
	noOp.ShowIndicator(ports.ModeIndicator)
	noOp.HideIndicator(ports.StickyModifiersIndicator)
	noOp.ResizeIndicatorToActiveScreen(ports.VirtualPointerIndicator)

	testGrid := domainGrid.NewGrid("abc", image.Rect(0, 0, 10, 10), zap.NewNop())

	err := noOp.DrawGrid(testGrid, "", grid.Style{})
	if err != nil {
		t.Errorf("DrawGrid returned %v, want nil", err)
	}

	noOp.ShowSubgrid(nil, grid.Style{})
}

func TestNoOpManager_ReportsEmptyState(t *testing.T) {
	noOp := &manager.NoOpManager{}

	if got := noOp.Mode(); got != manager.ModeIdle {
		t.Errorf("Mode() = %q, want %q", got, manager.ModeIdle)
	}

	if noOp.WaylandKeyboardChannel() != nil {
		t.Error("WaylandKeyboardChannel() is non-nil; the no-op manager has no keyboard")
	}

	if noOp.WindowPtr() != nil {
		t.Error("WindowPtr() is non-nil; the no-op manager has no window")
	}

	if noOp.HintOverlay() != nil {
		t.Error("HintOverlay() is non-nil")
	}

	if noOp.GridOverlay() != nil {
		t.Error("GridOverlay() is non-nil")
	}

	if noOp.ModeIndicatorOverlay() != nil {
		t.Error("ModeIndicatorOverlay() is non-nil")
	}

	if noOp.RecursiveGridOverlay() != nil {
		t.Error("RecursiveGridOverlay() is non-nil")
	}

	if noOp.StickyModifiersOverlay() != nil {
		t.Error("StickyModifiersOverlay() is non-nil")
	}

	if noOp.VirtualPointerOverlay() != nil {
		t.Error("VirtualPointerOverlay() is non-nil")
	}
}

// TestNoOpManager_DeclaresItselfHeadless pins the capability the component
// factory reads before it builds render overlays. An overlay manager that
// cannot render has to say so; if the no-op manager stopped declaring it, the
// factory would try to build overlays on a surface that does not exist.
func TestNoOpManager_DeclaresItselfHeadless(t *testing.T) {
	var noOp manager.Interface = &manager.NoOpManager{}

	reporter, ok := noOp.(manager.HeadlessReporter)
	if !ok {
		t.Fatal("the no-op manager does not implement HeadlessReporter")
	}

	if !reporter.Headless() {
		t.Error("Headless() = false; the no-op manager has no surface to render on")
	}
}

// TestNoOpManager_SubscriptionIsInert pins that a subscriber can be registered
// and removed. A caller that subscribes on a headless run must not be left
// holding an id it cannot unsubscribe.
func TestNoOpManager_SubscriptionIsInert(t *testing.T) {
	noOp := &manager.NoOpManager{}

	id := noOp.Subscribe(func(manager.StateChange) {
		t.Error("the no-op manager published a state change")
	})

	noOp.SwitchTo(manager.ModeGrid)
	noOp.Unsubscribe(id)
}

// TestStateChange_CarriesBothEnds pins the subscription payload. The fields are
// unexported so a subscriber cannot rewrite the transition it was handed, which
// makes the constructor and the two accessors the whole contract.
func TestStateChange_CarriesBothEnds(t *testing.T) {
	change := manager.NewStateChange(manager.ModeIdle, manager.ModeHints)

	if got := change.Prev(); got != manager.ModeIdle {
		t.Errorf("Prev() = %q, want %q", got, manager.ModeIdle)
	}

	if got := change.Next(); got != manager.ModeHints {
		t.Errorf("Next() = %q, want %q", got, manager.ModeHints)
	}
}

// TestModes_MatchTheDomainNames pins that the overlay's mode vocabulary is the
// domain's. They are separate constants, so a rename on one side that missed
// the other would silently stop matching.
func TestModes_MatchTheDomainNames(t *testing.T) {
	pairs := []struct {
		overlay manager.Mode
		domain  string
	}{
		{manager.ModeIdle, domain.ModeNameIdle},
		{manager.ModeHints, domain.ModeNameHints},
		{manager.ModeGrid, domain.ModeNameGrid},
		{manager.ModeScroll, domain.ModeNameScroll},
		{manager.ModeRecursiveGrid, domain.ModeNameRecursiveGrid},
		{manager.ModeMonitorSelect, domain.ModeNameMonitorSelect},
	}

	for _, pair := range pairs {
		if string(pair.overlay) != pair.domain {
			t.Errorf("overlay mode %q does not match domain name %q", pair.overlay, pair.domain)
		}
	}
}
