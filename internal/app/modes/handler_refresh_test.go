package modes

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/y3owk1n/neru/internal/app/components"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// newThemeChangeHintsHandler builds a handler with hints mode active and a
// collection on screen, which is all a theme refresh needs: the labels are not
// regenerated, only drawn again in the colors the overlay re-resolved.
func newThemeChangeHintsHandler(overlay *portmocks.MockOverlayPort) *Handler {
	handler := newHandlerWithState(handlerState{
		ctx:           context.Background(),
		logger:        zap.NewNop(),
		config:        &configpkg.Config{Hints: configpkg.HintsConfig{Enabled: true}},
		appState:      state.NewAppState(),
		cursorState:   state.NewCursorState(),
		modifierState: state.NewModifierState(),
		hints:         &components.HintsComponent{Context: &hintscomponent.Context{}},
		scroll:        &components.ScrollComponent{Context: &scroll.Context{}},
		overlayPort:   overlay,
	})

	handler.appState.SetMode(domain.ModeHints)

	_ = handler.hints.Context.SetHints(domainHint.NewCollection(nil))

	return handler
}

// TestRefreshActiveModeForThemeChange_ReadsTheModeUnderTheHandlerLock is the
// concurrency test the modes area guide asks of a locking change: it must fail
// under -race if the theme-change dispatch reads a mode's session outside h.mu.
//
// The app layer used to read the active mode unlocked and then call into a
// refresher that locked for itself. Now one hold covers the read, the selection
// and the work, and the session state on either side of it is the same state a
// concurrent mode exit clears: exiting hints resets the collection that the
// redraw reads. This guards that hold rather than the redraw's result.
//
// How often the two loops below actually meet is the scheduler's business, so
// the refresh is proven to reach a session once up front rather than by
// asserting on a count the scheduler decides.
func TestRefreshActiveModeForThemeChange_ReadsTheModeUnderTheHandlerLock(t *testing.T) {
	var redraws atomic.Int64

	handler := newThemeChangeHintsHandler(&portmocks.MockOverlayPort{})

	// A refresh reaches the session at all — asserted on its own, before the two
	// loops below, because whether either of them ever catches the other mid-round
	// is up to the scheduler. Without this the whole test could pass on a
	// dispatch that never redrew anything.
	if !handler.RefreshActiveModeForThemeChange() {
		t.Fatal("a theme refresh did not reach an open hints session")
	}

	const rounds = 200

	start := make(chan struct{})
	done := make(chan struct{})

	var waitGroup sync.WaitGroup

	waitGroup.Go(func() {
		<-start

		defer close(done)

		for range rounds {
			// Re-enter hints so the next exit has a session to tear down, the
			// way a repeating activation does. Both sides of the round are
			// locked writers — an activation and an exit — and the exit resets
			// the collection the refresh reads.
			handler.mu.Lock()
			handler.appState.SetMode(domain.ModeHints)
			_ = handler.hints.Context.SetHints(domainHint.NewCollection(nil))
			handler.mu.Unlock()

			handler.ExitMode()
		}
	})

	// The refresher runs flat out for as long as the mode is being entered and
	// exited, rather than for a round count of its own: under the race detector
	// the exit side is much the slower of the two.
	waitGroup.Go(func() {
		<-start

		for {
			select {
			case <-done:
				return
			default:
			}

			if handler.RefreshActiveModeForThemeChange() {
				redraws.Add(1)
			}
		}
	})

	close(start)
	waitGroup.Wait()

	t.Logf("%d of the concurrent refreshes found a session open", redraws.Load())

	if got := handler.appState.CurrentMode(); got != domain.ModeIdle {
		t.Fatalf("mode after the last exit = %v, want idle", got)
	}
}

// TestRefreshActiveModeForThemeChange_LeavesTheOverlayAloneWhileIdle pins the
// arm the mode map answers by having no entry: switching the system theme with
// nothing open must not put a frame on screen, which is what the deleted idle
// arm of the app-layer switch used to say.
func TestRefreshActiveModeForThemeChange_LeavesTheOverlayAloneWhileIdle(t *testing.T) {
	overlay := &portmocks.MockOverlayPort{}

	handler := newHandlerWithState(handlerState{
		ctx:         context.Background(),
		logger:      zap.NewNop(),
		appState:    state.NewAppState(),
		overlayPort: overlay,
	})

	if handler.RefreshActiveModeForThemeChange() {
		t.Error("an idle theme change reported a redraw")
	}

	if got := overlay.Frames(); len(got) != 0 {
		t.Fatalf("an idle theme change put %d frame(s) on screen, want none", len(got))
	}
}

// TestRefreshActiveModeForThemeChange_ScrollDeclinesAndSaysSo pins the other
// mode that carries this axis by not implementing it. Scroll draws nothing of
// its own, so a theme change leaves the screen alone — and because the axis is
// an effect rather than a getter, "the overlay stayed as it was" is answerable
// from the debug log instead of by reading the dispatch.
func TestRefreshActiveModeForThemeChange_ScrollDeclinesAndSaysSo(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)

	overlay := &portmocks.MockOverlayPort{}

	handler := newHandlerWithState(handlerState{
		ctx:         context.Background(),
		logger:      zap.New(core),
		appState:    state.NewAppState(),
		overlayPort: overlay,
	})

	handler.appState.SetMode(domain.ModeScroll)

	if handler.RefreshActiveModeForThemeChange() {
		t.Error("scroll mode reported a theme redraw")
	}

	if got := overlay.Frames(); len(got) != 0 {
		t.Fatalf("a theme change in scroll mode put %d frame(s) on screen, want none", len(got))
	}

	declined := logs.FilterLevelExact(zap.DebugLevel).All()
	if len(declined) != 1 {
		t.Fatalf("a theme change in scroll mode logged %d debug entries, want 1", len(declined))
	}

	fields := declined[0].ContextMap()
	if got := fields["extension"]; got != string(extensionThemeRefresh) {
		t.Errorf("declined effect named extension %v, want %q", got, extensionThemeRefresh)
	}

	if got, want := fields["mode"], domain.ModeString(domain.ModeScroll); got != want {
		t.Errorf("declined effect named mode %v, want %q", got, want)
	}
}
