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
// found, which is everything starting a search needs, over the overlay given.
func newHintSearchTestHandler(t *testing.T, overlayPort ports.OverlayPort) *Handler {
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

// TestHintsModeRefreshForThemeChange_RedrawsAnOpenSearchBox pins the half of a
// theme refresh a redraw of the labels alone would lose. On the backends that
// paint the search box onto the same surface as the labels (Linux and Windows),
// that redraw clears it, so a theme change mid-search would take the box off
// the screen while the search is still running and still taking keys.
func TestHintsModeRefreshForThemeChange_RedrawsAnOpenSearchBox(t *testing.T) {
	t.Parallel()

	searchDraws := 0
	overlayPort := &portmocks.MockOverlayPort{
		DrawHintSearchFunc: func(ports.HintSearch) error {
			searchDraws++

			return nil
		},
	}

	handler := newHintSearchTestHandler(t, overlayPort)
	mode := &HintsMode{handler: &handler.handlerState}

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

// TestHandleSearchInputKey_KeepsShiftedKeysAndSpaces pins the query text
// itself, which matching cannot. Search ignores case, so a capital typed as
// "Shift+c" and kept as "c" would still find the same elements. The taps name
// a key typed with Shift held this way, and a shifted digit or symbol is the
// character it types on a US layout.
func TestHandleSearchInputKey_KeepsShiftedKeysAndSpaces(t *testing.T) {
	t.Parallel()

	overlayPort := &portmocks.MockOverlayPort{}

	handler := newHintSearchTestHandler(t, overlayPort)

	handler.mu.Lock()
	defer handler.mu.Unlock()

	err := handler.startHintSearch(false)
	if err != nil {
		t.Fatalf("startHintSearch() error = %v", err)
	}

	for _, key := range []string{"Shift+c", "a", "Shift+Space", "shift+b", "Shift+1"} {
		handler.handleSearchInputKey(key)
	}

	if got := handler.hints.Context.SearchQuery(); got != "Ca B!" {
		t.Errorf("query = %q, want %q", got, "Ca B!")
	}
}
