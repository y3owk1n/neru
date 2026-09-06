package state_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// TestAppState_ModeName_NamesTheOpenDeclaredMode pins what a status report
// says about a declared mode: the declaration's name while one is open, the
// mode's own name otherwise, and the declared name gone the moment any other
// mode is entered.
func TestAppState_ModeName_NamesTheOpenDeclaredMode(t *testing.T) {
	t.Parallel()

	appState := state.NewAppState()

	if got := appState.ModeName(); got != domain.ModeNameIdle {
		t.Fatalf("ModeName() = %q at rest, want %q", got, domain.ModeNameIdle)
	}

	appState.SetCustomModeName("window")
	appState.SetMode(domain.ModeCustom)

	if got := appState.ModeName(); got != "window" {
		t.Fatalf("ModeName() = %q in a declared mode, want %q", got, "window")
	}

	if got := appState.CustomModeName(); got != "window" {
		t.Fatalf("CustomModeName() = %q, want %q", got, "window")
	}

	appState.SetMode(domain.ModeScroll)

	if got := appState.CustomModeName(); got != "" {
		t.Errorf("CustomModeName() = %q after entering scroll, want it cleared", got)
	}

	if got := appState.ModeName(); got != domain.ModeNameScroll {
		t.Errorf("ModeName() = %q in scroll, want %q", got, domain.ModeNameScroll)
	}
}
