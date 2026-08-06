package manager_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/domain"
)

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
