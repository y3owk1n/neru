package modes

import (
	"context"
	"image"
	"sync"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/app/services"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// targetDisplay is the display a monitor move lands the cursor on in these
// tests. Its contents do not matter; only that it is not where the mode was.
var targetDisplay = image.Rect(1920, 0, 3840, 1080)

// newMonitorMoveHintsHandler builds a handler with hints mode active and
// enough wiring for the monitor-move refresh to run end to end: a hint service
// to regenerate against, a hints context holding the session's flags, and the
// cursor and scroll state a mode exit resets.
func newMonitorMoveHintsHandler(regenerations *atomic.Int64) *Handler {
	accessibility := &portmocks.MockAccessibilityPort{
		// The first thing regenerating the labels does is ask what is
		// focused, so this counts the refreshes that reached the session.
		FocusedAppBundleIDFunc: func(context.Context) (string, error) {
			regenerations.Add(1)

			return "", nil
		},
	}

	handler := newHandlerWithState(handlerState{
		ctx:           context.Background(),
		logger:        zap.NewNop(),
		config:        &configpkg.Config{Hints: configpkg.HintsConfig{Enabled: true}},
		appState:      state.NewAppState(),
		cursorState:   state.NewCursorState(),
		modifierState: state.NewModifierState(),
		hints:         &components.HintsComponent{Context: &hintscomponent.Context{}},
		scroll:        &components.ScrollComponent{Context: &scroll.Context{}},
		hintService: services.NewHintService(
			accessibility,
			&portmocks.MockOverlayPort{},
			&portmocks.MockSystemPort{},
			nil,
			configpkg.HintsConfig{},
			zap.NewNop(),
			nil,
		),
	})

	handler.appState.SetMode(domain.ModeHints)
	handler.hints.Context.SetFilterRoles([]string{"button"})

	return handler
}

// TestRefreshActiveModeOnNewScreen_ReadsTheModeSessionUnderTheHandlerLock is
// the concurrency test the modes area guide asks of a locking change: it must
// fail under -race if the monitor-move dispatch reads a mode's session state
// outside h.mu.
//
// A monitor move races a mode exit by construction — the warp is animated and
// the user can press escape during it — and the two touch the same fields.
// Exiting hints resets the session flags (filter roles, strategy override) in a
// locked section; putting hints back on the new display reads exactly those
// flags to regenerate the labels with the same filters. While the dispatch ran
// unlocked and each arm locked for itself, the read happened before any lock
// was taken and the race detector saw it. Dispatching under one hold is what
// closes it, so this test is the guard on that hold rather than on the
// refresh's result.
func TestRefreshActiveModeOnNewScreen_ReadsTheModeSessionUnderTheHandlerLock(t *testing.T) {
	var regenerations atomic.Int64

	handler := newMonitorMoveHintsHandler(&regenerations)

	const rounds = 200

	start := make(chan struct{})

	var waitGroup sync.WaitGroup

	waitGroup.Go(func() {
		<-start

		for range rounds {
			// Re-enter hints so the next exit has a session to tear down,
			// the way a repeating activation does. Exiting is the locked
			// writer: it resets the session flags the refresh reads.
			handler.appState.SetMode(domain.ModeHints)
			handler.ExitMode()
		}
	})

	waitGroup.Go(func() {
		<-start

		for range rounds {
			handler.refreshActiveModeForMonitorMove(context.Background(), targetDisplay)
		}
	})

	close(start)
	waitGroup.Wait()

	// Without this the test could pass by never refreshing anything: the
	// window it guards only exists while a refresh is reading the session a
	// concurrent exit is clearing.
	if regenerations.Load() == 0 {
		t.Fatal("no monitor-move refresh reached the hints session, so nothing raced")
	}

	if got := handler.appState.CurrentMode(); got != domain.ModeIdle {
		t.Fatalf("mode after the last exit = %v, want idle", got)
	}
}

// TestRefreshActiveModeOnNewScreen_LeavesTheOverlayAloneWhileIdle pins the arm
// the mode map answers by having no entry: a move with nothing open must not
// bring a frame back up, which is what the deleted idle switch arm used to say.
func TestRefreshActiveModeOnNewScreen_LeavesTheOverlayAloneWhileIdle(t *testing.T) {
	overlay := &portmocks.MockOverlayPort{}

	handler := newHandlerWithState(handlerState{
		ctx:         context.Background(),
		logger:      zap.NewNop(),
		appState:    state.NewAppState(),
		overlayPort: overlay,
	})

	handler.refreshActiveModeForMonitorMove(context.Background(), targetDisplay)

	if got := overlay.Frames(); len(got) != 0 {
		t.Fatalf("an idle monitor move put %d frame(s) on screen, want none", len(got))
	}
}
