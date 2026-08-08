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

	box := image.Rect(410, 740, 610, 780)
	overlayPort := &portmocks.MockOverlayPort{
		HintSearchBoundsFunc: func(image.Rectangle) image.Rectangle { return box },
	}
	textInput := &portmocks.MockTextInputPort{Started: true}

	handler := newHintSearchTestHandler(t, overlayPort, textInput)

	handler.mu.Lock()
	err := handler.startHintSearch()
	handler.mu.Unlock()

	if err != nil {
		t.Fatalf("startHintSearch() error = %v", err)
	}

	if textInput.StartCount() != 1 {
		t.Fatalf("text input started %d times, want 1", textInput.StartCount())
	}

	want := ports.TextInputFrame{X: box.Min.X, Y: box.Min.Y, Width: box.Dx(), Height: box.Dy()}
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
	err := handler.startHintSearch()
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

	err := handler.startHintSearch()
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
