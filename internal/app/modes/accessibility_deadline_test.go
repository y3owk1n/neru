package modes

// The element walk is bounded on every path that runs it under h.mu. h.mu
// serializes key handling, so a walk of a tree that stops answering — a hung
// application in the frontmost position — costs whatever it costs, and without
// a deadline that is unbounded: the keyboard stops with it.
//
// These pin the bound at each call site rather than the behavior of any one
// refresh. What they assert is the deadline reaching the port, not the timeout
// expiring, so they cost nothing in wall clock.
//
// Asserting on what the port is handed is deliberate: a deadline is only as
// good as the callee's willingness to read it, which is the accessibility
// client's business, not the handler's. What the handler can be held to is the
// budget it sets. The area guide's deadline rule says which calls read one —
// the walk does, which is why it is the walk that is pinned here.

import (
	"context"
	"image"
	"sync"
	"testing"
	"time"

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

// capturedDeadline records the deadline carried by the context an
// accessibility call was made with, and whether it carried one at all.
//
// The walk fans out across goroutines, so the port can be reached from more
// than one; the mutex keeps this an assertion failure rather than a -race
// failure if it ever is.
type capturedDeadline struct {
	mu     sync.Mutex
	budget time.Duration
	bound  bool
	called bool
}

// capture records what ctx allows the call it was passed to.
func (c *capturedDeadline) capture(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.called = true

	deadline, ok := ctx.Deadline()
	c.bound = ok

	if ok {
		c.budget = time.Until(deadline)
	}
}

// assertBoundedBy fails unless the call happened and its context allowed at
// most want, with enough left to be that budget rather than a smaller one
// inherited from somewhere else.
func (c *capturedDeadline) assertBoundedBy(t *testing.T, what string, want time.Duration) {
	t.Helper()

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.called {
		t.Fatalf("%s never reached the accessibility port, so nothing was measured", what)
	}

	if !c.bound {
		t.Fatalf("%s ran with no deadline: a tree that never answers holds h.mu, "+
			"and with it every keystroke, for as long as it takes", what)
	}

	if c.budget > want {
		t.Errorf("%s was allowed %v, want at most %v", what, c.budget, want)
	}

	// A deadline far shorter than the budget means it came from somewhere
	// other than the bound under test, which would pin the wrong thing. The
	// floor scales with the budget so it keeps its teeth on a short one.
	if floor := want / 2; c.budget < floor {
		t.Errorf("%s was allowed only %v, want about %v", what, c.budget, want)
	}
}

// newBoundedWalkHintsHandler builds a handler with hints mode active and the
// wiring a hint regeneration needs, reporting what the accessibility port was
// allowed when the walk reached it.
func newBoundedWalkHintsHandler(walk *capturedDeadline, cfg *configpkg.Config) *Handler {
	accessibility := &portmocks.MockAccessibilityPort{
		// Regenerating the labels asks what is focused first, so this is the
		// context the walk carries.
		FocusedAppBundleIDFunc: func(ctx context.Context) (string, error) {
			walk.capture(ctx)

			return "", nil
		},
	}

	handler := newHandlerWithState(handlerState{
		ctx:           context.Background(),
		logger:        zap.NewNop(),
		config:        cfg,
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

	return handler
}

// hintsEnabledConfig is the configuration a hint regeneration needs and
// nothing more.
func hintsEnabledConfig() *configpkg.Config {
	return &configpkg.Config{Hints: configpkg.HintsConfig{Enabled: true}}
}

// TestRefreshActiveModeForScreenChange_BoundsTheAccessibilityWalk covers the
// display-change refresh, which regenerates the labels from the accessibility
// tree under h.mu.
//
// The context a screen change arrives with is the application's own and
// carries no deadline of its own, so the bound has to be applied here. Without
// it, plugging in a monitor while an unresponsive application is frontmost
// stops the keyboard for as long as that application takes to answer.
func TestRefreshActiveModeForScreenChange_BoundsTheAccessibilityWalk(t *testing.T) {
	var walk capturedDeadline

	handler := newBoundedWalkHintsHandler(&walk, hintsEnabledConfig())

	handler.RefreshActiveModeForScreenChange(context.Background())

	walk.assertBoundedBy(t, "the screen-change hint walk", HintTimeout)
}

// TestRefreshActiveModeForMonitorMove_BoundsTheAccessibilityWalk covers the
// same walk on the monitor-move path, which was given this bound when it moved
// under one lock hold but never had it pinned.
func TestRefreshActiveModeForMonitorMove_BoundsTheAccessibilityWalk(t *testing.T) {
	var walk capturedDeadline

	handler := newBoundedWalkHintsHandler(&walk, hintsEnabledConfig())

	handler.refreshActiveModeForMonitorMove(context.Background(), image.Rect(1920, 0, 3840, 1080))

	walk.assertBoundedBy(t, "the monitor-move hint walk", HintTimeout)
}
