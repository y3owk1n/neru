package app

// The hotkey binder is built in a phase that runs before the phase building the
// event tap, so everything it is handed has to survive the fields it reads being
// nil. This is the one that bit: `app.eventTap.SetHotkeys` as a method value is
// evaluated where it is written, which on a nil interface is a segfault during
// startup — before the daemon has logged anything to diagnose it from, and
// invisible to every journey, because the simulation harness injects an event tap
// through WithEventTap and so never has a nil one here.

import (
	"slices"
	"testing"

	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

func TestHotkeyBinderDeps_PublishSurvivesAnEventTapThatDoesNotExistYet(t *testing.T) {
	t.Parallel()

	// The App as this phase finds it: nothing phase 8 owns has been built.
	app := &App{}

	deps := hotkeyBinderDeps(app)

	if deps.PublishRegisteredHotkeys == nil {
		t.Fatal("no publish hook; the taps would keep the configured table")
	}

	// Registration can run before the tap exists — a systray toggle or a failed
	// reload both reach it — so the hook has to answer to that, not just be built
	// without panicking.
	deps.PublishRegisteredHotkeys([]string{"Ctrl+G"})
}

// And once the tap exists, what registration took reaches it.
func TestHotkeyBinderDeps_PublishReachesTheEventTap(t *testing.T) {
	t.Parallel()

	tap := &portmocks.MockEventTapPort{}
	app := &App{eventTap: tap}

	want := []string{"Ctrl+G", "Alt+J"}

	hotkeyBinderDeps(app).PublishRegisteredHotkeys(want)

	if got := tap.Hotkeys(); !slices.Equal(got, want) {
		t.Errorf("the tap was told %v, want %v", got, want)
	}
}
