package manager_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
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
