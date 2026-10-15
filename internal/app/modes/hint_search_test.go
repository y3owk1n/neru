package modes

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
	domainhint "github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// newHintSearchTestHandler builds a handler sitting in hints mode with one hint
// found, which is everything starting a search needs, over the overlay and text
// input given.
func newHintSearchTestHandler(
	t *testing.T,
	overlayPort ports.OverlayPort,
	textInput ports.TextInputPort,
) *Handler {
	t.Helper()

	appState := state.NewAppState()
	appState.SetMode(domain.ModeHints)

	handler := newHandlerWithState(handlerState{
		ctx:      context.Background(),
		logger:   zap.NewNop(),
		appState: appState,
		hints: &components.HintsComponent{
			Context: &hintscomponent.Context{},
		},
		overlayPort: overlayPort,
		textInput:   textInput,
		modes:       map[domain.Mode]Mode{},
	})

	elem, elemErr := element.NewElement("search", image.Rect(0, 0, 20, 20), element.RoleButton)
	if elemErr != nil {
		t.Fatalf("NewElement() error = %v", elemErr)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	handler.hints.Context.SetManager(domainhint.NewManager(handler.logger, &handler.mu))

	setErr := handler.hints.Context.SetHints(
		domainhint.NewCollection([]*domainhint.Interface{mustNewModeHint("AA", elem)}),
	)
	if setErr != nil {
		t.Fatalf("SetHints() error = %v", setErr)
	}

	return handler
}

// TestStartHintSearch_PlacesTheTextInputOverTheDrawnBox is the ordinary case:
// the overlay says where it put the search box and the platform's input field
// is placed over exactly that rectangle, rather than deriving the placement a
// second time here.
func TestStartHintSearch_PlacesTheTextInputOverTheDrawnBox(t *testing.T) {
	t.Parallel()

	// The box is relative to the active screen, here a second monitor, and
	// the frame carries that screen so a platform field can place itself.
	box := image.Rect(410, 740, 610, 780)
	screen := image.Rect(1920, 0, 3840, 1080)
	overlayPort := &portmocks.MockOverlayPort{
		HintSearchBoundsFunc: func(image.Rectangle) image.Rectangle { return box },
	}
	textInput := &portmocks.MockTextInputPort{Started: true}

	handler := newHintSearchTestHandler(t, overlayPort, textInput)

	handler.mu.Lock()
	handler.screenBounds = screen
	err := handler.startHintSearch(false)
	handler.mu.Unlock()

	if err != nil {
		t.Fatalf("startHintSearch() error = %v", err)
	}

	if textInput.StartCount() != 1 {
		t.Fatalf("text input started %d times, want 1", textInput.StartCount())
	}

	want := ports.TextInputFrame{
		X: box.Min.X, Y: box.Min.Y, Width: box.Dx(), Height: box.Dy(), Screen: screen,
	}
	if got := textInput.Frame(); got != want {
		t.Errorf("text input frame = %+v, want %+v", got, want)
	}
}

// TestStartHintSearch_NoBoxOnScreenStartsNoTextInput pins the degradation that
// comes with the overlay being able to refuse a search input it cannot place
// (#1329). An empty rectangle means nothing was drawn, and the platform's field
// is not sized from the box it sits over — macOS substitutes a 16x16 one at the
// screen origin — so starting the session anyway would hand the keyboard to an
// invisible input in a corner while the event tap is switched off. Search still
// works: the query arrives through the key stream, the way it does on every
// overlay that draws no search box at all.
func TestStartHintSearch_NoBoxOnScreenStartsNoTextInput(t *testing.T) {
	t.Parallel()

	overlayPort := &portmocks.MockOverlayPort{
		HintSearchBoundsFunc: func(image.Rectangle) image.Rectangle { return image.Rectangle{} },
	}
	textInput := &portmocks.MockTextInputPort{Started: true}

	handler := newHintSearchTestHandler(t, overlayPort, textInput)

	handler.mu.Lock()
	err := handler.startHintSearch(false)
	searchActive := handler.hints.Context.SearchActive()
	textInputActive := handler.hintSearchTextInputActive
	handler.mu.Unlock()

	if err != nil {
		t.Fatalf("startHintSearch() error = %v", err)
	}

	if textInput.StartCount() != 0 {
		t.Errorf(
			"text input started %d times with no search box on screen, want 0",
			textInput.StartCount(),
		)
	}

	if textInputActive {
		t.Error("the handler recorded an active text input session that was never started")
	}

	// The search itself still opened — only the native field was skipped.
	if !searchActive {
		t.Error("hint search is not active; the key stream has nothing to filter")
	}
}

// TestStartHintSearch_NoBoxOnScreenGivesTheKeyboardBack pins the half of that
// degradation the user would notice. Starting a search stops any live session
// while deliberately leaving the event tap off, because the session about to
// start wants it off. When no session starts, the tap has to come back on: it
// is the only thing left that can deliver a key, and hints mode with the
// keyboard switched off and nothing listening is indistinguishable from a hang.
func TestStartHintSearch_NoBoxOnScreenGivesTheKeyboardBack(t *testing.T) {
	t.Parallel()

	enables := 0
	eventTap := &portmocks.MockEventTapPort{
		EnableFunc: func(context.Context) error {
			enables++

			return nil
		},
	}

	overlayPort := &portmocks.MockOverlayPort{
		HintSearchBoundsFunc: func(image.Rectangle) image.Rectangle { return image.Rectangle{} },
	}

	handler := newHintSearchTestHandler(
		t,
		overlayPort,
		&portmocks.MockTextInputPort{Started: true},
	)

	handler.mu.Lock()
	// The state a live session leaves behind: the tap is off and the handler
	// knows it turned it off.
	handler.eventTap = eventTap
	handler.hintSearchTextInputActive = true
	handler.hintSearchEventTapDisabled = true

	err := handler.startHintSearch(false)
	stillDisabled := handler.hintSearchEventTapDisabled
	handler.mu.Unlock()

	if err != nil {
		t.Fatalf("startHintSearch() error = %v", err)
	}

	if enables != 1 {
		t.Errorf("event tap enabled %d times, want 1: the keyboard is still switched off", enables)
	}

	if stillDisabled {
		t.Error("the handler still believes it owes the event tap a re-enable")
	}
}

// TestHintsModeRefreshForThemeChange_RedrawsAnOpenSearchBox pins the half of a
// theme refresh a redraw of the labels alone would lose. On the backends that
// paint the search box onto the same surface as the labels — Linux and Windows
// — that redraw clears it, so a theme change mid-search would take the box off
// the screen while the search is still running and still taking keys.
func TestHintsModeRefreshForThemeChange_RedrawsAnOpenSearchBox(t *testing.T) {
	t.Parallel()

	searchDraws := 0
	overlayPort := &portmocks.MockOverlayPort{
		DrawHintSearchFunc: func(ports.HintSearch) error {
			searchDraws++

			return nil
		},
		HintSearchBoundsFunc: func(image.Rectangle) image.Rectangle { return image.Rectangle{} },
	}

	handler := newHintSearchTestHandler(t, overlayPort, &portmocks.MockTextInputPort{})
	mode := &HintsMode{baseMode: baseMode{handler: &handler.handlerState}}

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if !mode.RefreshForThemeChange() {
		t.Fatal("RefreshForThemeChange() = false with hints on screen")
	}

	if searchDraws != 0 {
		t.Errorf("the search box was drawn %d times with no search open, want 0", searchDraws)
	}

	handler.hints.Context.SetSearchActive(true)

	if !mode.RefreshForThemeChange() {
		t.Fatal("RefreshForThemeChange() = false with hints on screen")
	}

	if searchDraws != 1 {
		t.Errorf(
			"the search box was drawn %d times after a theme change, want 1: "+
				"the label redraw cleared it",
			searchDraws,
		)
	}
}

// TestStartHintSearch_IgnoresCallbacksFromAStoppedSession pins the session
// token. A platform field can queue a callback just before it stops, and that
// callback runs once the handler lock is free, possibly after a new search
// started. A stale cancel must not close the new search, and a stale query
// must not replace its text.
func TestStartHintSearch_IgnoresCallbacksFromAStoppedSession(t *testing.T) {
	t.Parallel()

	overlayPort := &portmocks.MockOverlayPort{
		HintSearchBoundsFunc: func(image.Rectangle) image.Rectangle {
			return image.Rect(0, 0, 200, 30)
		},
	}
	textInput := &portmocks.MockTextInputPort{Started: true}

	handler := newHintSearchTestHandler(t, overlayPort, textInput)

	handler.mu.Lock()
	err := handler.startHintSearch(false)
	handler.mu.Unlock()

	if err != nil {
		t.Fatalf("first startHintSearch() error = %v", err)
	}

	stale := textInput.Callbacks()

	handler.mu.Lock()
	err = handler.startHintSearch(false)
	handler.hints.Context.SetSearchQuery("new")
	handler.mu.Unlock()

	if err != nil {
		t.Fatalf("second startHintSearch() error = %v", err)
	}

	stale.OnQueryChanged("old")
	stale.OnCancel()

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if !handler.hints.Context.SearchActive() {
		t.Error("a callback from the stopped session closed the new search")
	}

	if got := handler.hints.Context.SearchQuery(); got != "new" {
		t.Errorf("query = %q, want %q", got, "new")
	}
}

// TestHandleSearchInputKey_KeepsShiftedLettersAndSpaces pins the query text
// itself, which matching cannot: search ignores case, so a capital typed as
// "Shift+c" and kept as "c" would still find the same elements. The Linux and
// Windows taps name a key typed with Shift held this way.
func TestHandleSearchInputKey_KeepsShiftedLettersAndSpaces(t *testing.T) {
	t.Parallel()

	overlayPort := &portmocks.MockOverlayPort{
		HintSearchBoundsFunc: func(image.Rectangle) image.Rectangle { return image.Rectangle{} },
	}

	handler := newHintSearchTestHandler(t, overlayPort, &portmocks.MockTextInputPort{})

	handler.mu.Lock()
	defer handler.mu.Unlock()

	err := handler.startHintSearch(false)
	if err != nil {
		t.Fatalf("startHintSearch() error = %v", err)
	}

	for _, key := range []string{"Shift+c", "a", "Shift+Space", "shift+b", "Shift+1"} {
		handler.handleSearchInputKey(key)
	}

	// The shifted symbol depends on the layout, so it stays dropped.
	if got := handler.hints.Context.SearchQuery(); got != "Ca B" {
		t.Errorf("query = %q, want %q", got, "Ca B")
	}
}

// TestStartHintSearch_ReplacementThatFailsGivesTheKeyboardBack is the same
// promise for a restarted search. The live session is stopped with the tap
// left off for its replacement, and when the replacement field does not
// start, the tap has to come back on.
func TestStartHintSearch_ReplacementThatFailsGivesTheKeyboardBack(t *testing.T) {
	t.Parallel()

	enables := 0
	eventTap := &portmocks.MockEventTapPort{
		EnableFunc: func(context.Context) error {
			enables++

			return nil
		},
	}

	overlayPort := &portmocks.MockOverlayPort{
		HintSearchBoundsFunc: func(image.Rectangle) image.Rectangle {
			return image.Rect(0, 0, 200, 30)
		},
	}

	handler := newHintSearchTestHandler(
		t,
		overlayPort,
		&portmocks.MockTextInputPort{Started: false},
	)

	handler.mu.Lock()
	handler.eventTap = eventTap
	handler.hintSearchTextInputActive = true
	handler.hintSearchEventTapDisabled = true

	err := handler.startHintSearch(false)
	stillDisabled := handler.hintSearchEventTapDisabled
	handler.mu.Unlock()

	if err != nil {
		t.Fatalf("startHintSearch() error = %v", err)
	}

	if enables != 1 {
		t.Errorf("event tap enabled %d times, want 1: the keyboard is still switched off", enables)
	}

	if stillDisabled {
		t.Error("the handler still believes it owes the event tap a re-enable")
	}
}
