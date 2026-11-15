package modes

import (
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// cursorHidingSystem is a SystemPort that can hide the system cursor, and
// records each call it gets.
type cursorHidingSystem struct {
	portmocks.MockSystemPort

	calls []string
}

var _ ports.SystemCursorHider = (*cursorHidingSystem)(nil)

func (s *cursorHidingSystem) HideSystemCursor()   { s.calls = append(s.calls, "hide") }
func (s *cursorHidingSystem) ShowSystemCursor()   { s.calls = append(s.calls, "show") }
func (s *cursorHidingSystem) RehideSystemCursor() { s.calls = append(s.calls, "rehide") }

// TestHandler_HideSystemCursor_HidesThroughTheSystemPort checks that hiding
// and showing the cursor reach the platform once each, however often they are
// asked, and that a cursor the system revealed is hidden again.
func TestHandler_HideSystemCursor_HidesThroughTheSystemPort(t *testing.T) {
	system := &cursorHidingSystem{}
	handler := newHandlerWithState(handlerState{system: system, appState: state.NewAppState()})

	if !handler.CursorVisibilitySupported() {
		t.Fatal("CursorVisibilitySupported() = false on a system that can hide the cursor")
	}

	handler.HideSystemCursor()
	handler.HideSystemCursor()
	handler.rehideSystemCursor()
	handler.ShowSystemCursor()
	handler.ShowSystemCursor()

	want := []string{"hide", "rehide", "show"}
	if !slices.Equal(system.calls, want) {
		t.Fatalf("system cursor calls = %v, want %v", system.calls, want)
	}
}

// TestHandler_CursorVisibilitySupported_WithoutAHidingSystem checks that a
// platform whose system port cannot hide the cursor says so, which is what
// makes hide_cursor and show_cursor answer CodeNotSupported there.
func TestHandler_CursorVisibilitySupported_WithoutAHidingSystem(t *testing.T) {
	handler := newHandlerWithState(handlerState{system: &portmocks.MockSystemPort{}})

	if handler.CursorVisibilitySupported() {
		t.Fatal("CursorVisibilitySupported() = true on a system that cannot hide the cursor")
	}
}
