package modes

import (
	"context"
	"image"
	"sync"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/y3owk1n/neru/internal/app/components"
	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/app/services"
	configpkg "github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
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

// The fixture values a screen-change refresh is driven with. They are named
// rather than repeated so that what a test is varying — the mode, the
// configuration gate — is the only thing that reads as significant.
const (
	// screenChangeFilterRole is a role a hint session was activated with:
	// exiting the mode resets it, which is the state a concurrent refresh
	// reads.
	screenChangeFilterRole = "button"

	// screenChangeGridCharacters labels the cells of a rebuilt grid.
	screenChangeGridCharacters = "ABCD"
)

// newScreenChangeHintsHandler builds a handler with hints mode active and
// enough wiring for a screen-change refresh to run end to end: a hint service
// to regenerate against, a hints context holding the session's flags, and the
// cursor and scroll state a mode exit resets.
func newScreenChangeHintsHandler(
	regenerations *atomic.Int64,
	overlay *portmocks.MockOverlayPort,
) *Handler {
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
		overlayPort:   overlay,
		hintService: services.NewHintService(
			accessibility,
			overlay,
			&portmocks.MockSystemPort{},
			nil,
			configpkg.HintsConfig{},
			zap.NewNop(),
			nil,
		),
	})

	handler.appState.SetMode(domain.ModeHints)
	handler.hints.Context.SetFilterRoles([]string{screenChangeFilterRole})

	return handler
}

// TestRefreshActiveModeForScreenChange_ReadsTheModeSessionUnderTheHandlerLock
// is the concurrency test the modes area guide asks of a locking change: it
// must fail under -race if the screen-change dispatch reads a mode's session
// outside h.mu.
//
// The app layer used to snapshot the active mode unlocked, hand that snapshot
// to whichever of three per-mode functions matched, and have each of them lock
// for itself and re-check the mode. Now one hold covers the read, the selection
// and the work — and the session state on either side of it is the same state a
// concurrent mode exit clears: exiting hints resets the filter roles and
// overrides that regenerating the labels reads.
//
// How often the two loops below actually meet is the scheduler's business, so
// the refresh is proven to reach a session up front rather than by asserting on
// a count the scheduler decides.
func TestRefreshActiveModeForScreenChange_ReadsTheModeSessionUnderTheHandlerLock(t *testing.T) {
	var regenerations atomic.Int64

	handler := newScreenChangeHintsHandler(&regenerations, &portmocks.MockOverlayPort{})

	// A refresh reaches the session at all — asserted on its own, before the two
	// loops below, because whether either of them ever catches the other
	// mid-round is up to the scheduler. Without this the whole test could pass
	// on a dispatch that never reached a session.
	handler.RefreshActiveModeForScreenChange(context.Background())

	if regenerations.Load() == 0 {
		t.Fatal("a screen-change refresh did not reach an open hints session")
	}

	regenerations.Store(0)

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
			// the session flags the refresh reads.
			handler.mu.Lock()
			handler.appState.SetMode(domain.ModeHints)
			handler.hints.Context.SetFilterRoles([]string{screenChangeFilterRole})
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

			handler.RefreshActiveModeForScreenChange(context.Background())
		}
	})

	close(start)
	waitGroup.Wait()

	// Reported rather than asserted: the exit side re-locks the moment it
	// unlocks, so how many of the concurrent refreshes find a session open is
	// the scheduler's decision. Asserting on it makes the test flaky rather
	// than strict — the guarantee that a refresh reaches a session at all is
	// the one made before the loops started.
	t.Logf("%d of the concurrent refreshes reached an open session", regenerations.Load())

	if got := handler.appState.CurrentMode(); got != domain.ModeIdle {
		t.Fatalf("mode after the last exit = %v, want idle", got)
	}
}

// TestRefreshActiveModeForScreenChange_LeavesTheOverlayAloneWhileIdle pins the
// arm the dispatch answers before it looks at any mode: a display change with
// nothing open must leave the overlay alone.
//
// The caller resizes the overlay itself when this reports the resize is still
// owed, and resizing is what brings the overlay up — so reporting it here would
// flash an overlay at a user who only plugged a monitor in.
func TestRefreshActiveModeForScreenChange_LeavesTheOverlayAloneWhileIdle(t *testing.T) {
	overlay := &portmocks.MockOverlayPort{}

	handler := newHandlerWithState(handlerState{
		ctx:         context.Background(),
		logger:      zap.NewNop(),
		appState:    state.NewAppState(),
		overlayPort: overlay,
	})

	if handler.RefreshActiveModeForScreenChange(context.Background()) {
		t.Error("an idle screen change asked for the overlay to be resized")
	}

	if got := overlay.Frames(); len(got) != 0 {
		t.Fatalf("an idle screen change put %d frame(s) on screen, want none", len(got))
	}
}

// TestRefreshActiveModeForScreenChange_ScrollDeclinesAndSaysSo pins the mode
// that carries this axis by not implementing it. Scroll holds no drawing built
// for the bounds that changed, so there is nothing to rebuild — but it is on
// screen, so the overlay is still resized for it, which is what the app layer's
// three per-mode functions used to say by all returning false.
//
// Because the axis is an effect rather than a getter, "my mode rebuilt nothing"
// is answerable from the debug log instead of by reading the dispatch.
func TestRefreshActiveModeForScreenChange_ScrollDeclinesAndSaysSo(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)

	overlay := &portmocks.MockOverlayPort{}

	handler := newHandlerWithState(handlerState{
		ctx:         context.Background(),
		logger:      zap.New(core),
		appState:    state.NewAppState(),
		overlayPort: overlay,
	})

	handler.appState.SetMode(domain.ModeScroll)

	if !handler.RefreshActiveModeForScreenChange(context.Background()) {
		t.Error("a screen change in scroll mode left the overlay sized for the old display")
	}

	if got := overlay.Frames(); len(got) != 0 {
		t.Fatalf("a screen change in scroll mode put %d frame(s) on screen, want none", len(got))
	}

	declined := logs.FilterLevelExact(zap.DebugLevel).All()
	if len(declined) != 1 {
		t.Fatalf("a screen change in scroll mode logged %d debug entries, want 1", len(declined))
	}

	fields := declined[0].ContextMap()
	if got := fields["extension"]; got != string(extensionScreenRefresh) {
		t.Errorf("declined effect named extension %v, want %q", got, extensionScreenRefresh)
	}

	if got, want := fields["mode"], domain.ModeString(domain.ModeScroll); got != want {
		t.Errorf("declined effect named mode %v, want %q", got, want)
	}
}

// TestRefreshActiveModeForScreenChange_GridDisabledInConfigIsNotRefreshed pins
// the per-mode configuration gate that used to sit in the app layer, one copy
// per mode.
//
// A user who has switched grid off in configuration while a grid session is
// still open must not have it rebuilt underneath them; the overlay is left for
// the caller to resize, exactly as the app-layer gate left it.
func TestRefreshActiveModeForScreenChange_GridDisabledInConfigIsNotRefreshed(t *testing.T) {
	for _, testCase := range []struct {
		name            string
		enabled         bool
		wantNeedsResize bool
		wantFrames      int
	}{
		{name: "grid enabled", enabled: true, wantNeedsResize: false, wantFrames: 1},
		{name: "grid disabled", enabled: false, wantNeedsResize: true, wantFrames: 0},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			overlay := &portmocks.MockOverlayPort{}

			// The grid a session holds lives behind a pointer the component
			// owns, which is what rebuilding it writes through.
			var gridInstance *domainGrid.Grid

			gridContext := &gridcomponent.Context{}
			gridContext.SetGridInstance(&gridInstance)

			handler := newHandlerWithState(handlerState{
				ctx:      context.Background(),
				logger:   zap.NewNop(),
				appState: state.NewAppState(),
				config: &configpkg.Config{
					Grid: configpkg.GridConfig{
						Enabled:    testCase.enabled,
						Characters: screenChangeGridCharacters,
					},
				},
				grid:         &components.GridComponent{Context: gridContext},
				overlayPort:  overlay,
				screenBounds: image.Rect(0, 0, 100, 100),
			})

			handler.appState.SetMode(domain.ModeGrid)

			got := handler.RefreshActiveModeForScreenChange(context.Background())
			if got != testCase.wantNeedsResize {
				t.Errorf(
					"screen change reported the overlay needs a resize = %t, want %t",
					got, testCase.wantNeedsResize,
				)
			}

			if frames := overlay.Frames(); len(frames) != testCase.wantFrames {
				t.Fatalf(
					"screen change put %d frame(s) on screen, want %d",
					len(frames), testCase.wantFrames,
				)
			}
		})
	}
}
