package manager_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/ports"
)

// TestBase_SwitchTo_NotifiesOnRealTransitionsOnly pins the transition
// semantics every backend now shares: subscribers see each real mode change
// exactly once, and switching to the already-active mode publishes nothing.
// The three backends used to hand-roll this and had drifted on the
// equal-mode case.
func TestBase_SwitchTo_NotifiesOnRealTransitionsOnly(t *testing.T) {
	t.Parallel()

	base := manager.NewBase(nil)

	var got []manager.StateChange

	base.Subscribe(func(change manager.StateChange) {
		got = append(got, change)
	})

	base.SwitchTo(manager.ModeHints)
	base.SwitchTo(manager.ModeHints) // no-op: already active
	base.SwitchTo(manager.ModeIdle)

	if len(got) != 2 {
		t.Fatalf("subscriber saw %d transitions, want 2", len(got))
	}

	if got[0].Prev() != manager.ModeIdle || got[0].Next() != manager.ModeHints {
		t.Errorf("first transition = %s->%s, want idle->hints", got[0].Prev(), got[0].Next())
	}

	if got[1].Prev() != manager.ModeHints || got[1].Next() != manager.ModeIdle {
		t.Errorf("second transition = %s->%s, want hints->idle", got[1].Prev(), got[1].Next())
	}

	if base.Mode() != manager.ModeIdle {
		t.Errorf("Mode() = %s, want idle", base.Mode())
	}
}

// TestBase_Unsubscribe_StopsDelivery pins that an unsubscribed callback is
// never invoked again.
func TestBase_Unsubscribe_StopsDelivery(t *testing.T) {
	t.Parallel()

	base := manager.NewBase(nil)

	calls := 0
	id := base.Subscribe(func(manager.StateChange) { calls++ })

	base.SwitchTo(manager.ModeGrid)
	base.Unsubscribe(id)
	base.SwitchTo(manager.ModeIdle)

	if calls != 1 {
		t.Errorf("subscriber called %d times, want 1 (unsubscribed before second switch)", calls)
	}
}

// TestBase_IndicatorCallsAreSilentWithoutARenderComponent pins the guard that
// makes "was this indicator ever constructed" the manager's question rather
// than its callers'. A disabled indicator and a headless backend both leave
// the render component nil, and a nil *Overlay handed to an interface would
// pass every != nil check downstream before panicking on the first call.
func TestBase_IndicatorCallsAreSilentWithoutARenderComponent(t *testing.T) {
	t.Parallel()

	base := manager.NewBase(nil)

	for _, indicator := range []ports.Indicator{
		ports.ModeIndicator,
		ports.StickyModifiersIndicator,
		ports.VirtualPointerIndicator,
	} {
		base.ShowIndicator(indicator)
		base.HideIndicator(indicator)
		base.ResizeIndicatorToActiveScreen(indicator)
	}

	if got := base.ModeIndicatorOverlay(); got != nil {
		t.Errorf("ModeIndicatorOverlay() = %v, want nil with nothing registered", got)
	}
}

// TestBase_GridPointerCallsAreSilentWithoutARenderComponent is the same guard
// for the pointer a grid mode draws on its own surface: the mode names a mode,
// and whether there is a component behind it — a disabled mode, a backend
// whose grid overlay is a stub — is this package's question. A mode that draws
// no pointer of its own resolves to nothing at all.
func TestBase_GridPointerCallsAreSilentWithoutARenderComponent(t *testing.T) {
	t.Parallel()

	base := manager.NewBase(nil)

	for _, mode := range []manager.Mode{
		manager.ModeGrid,
		manager.ModeRecursiveGrid,
		manager.ModeHints,
		manager.ModeScroll,
		manager.ModeMonitorSelect,
		manager.ModeIdle,
	} {
		base.DrawGridPointer(mode, image.Pt(10, 20), manager.PointerAppearance{
			FontSize:  12,
			FillColor: "#ffffff",
		})
		base.HideGridPointer(mode)
	}

	if got := base.GridOverlay(); got != nil {
		t.Errorf("GridOverlay() = %v, want nil with nothing registered", got)
	}
}
