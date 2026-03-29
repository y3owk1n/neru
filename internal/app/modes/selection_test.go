//nolint:testpackage // Tests private mode selection helpers.
package modes

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/app/components"
	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
)

func TestCurrentSelectionPoint_UsesActiveModeContext(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	handler := &Handler{
		appState: appState,
		hints:    &components.HintsComponent{},
		grid: &components.GridComponent{
			Context: &gridcomponent.Context{},
		},
		recursiveGrid: &components.RecursiveGridComponent{
			Context: &recursivegridcomponent.Context{},
		},
	}

	handler.grid.Context.SetSelectionPoint(image.Point{X: 10, Y: 20})
	handler.recursiveGrid.Context.SetSelectionPoint(image.Point{X: 30, Y: 40})

	got, ok := handler.CurrentSelectionPoint()
	if !ok {
		t.Fatal("CurrentSelectionPoint() expected selection")
	}

	if got != (image.Point{X: 30, Y: 40}) {
		t.Fatalf("CurrentSelectionPoint() = %v, want (30,40)", got)
	}
}

func TestToggleCursorFollowSelection_UpdatesOnlySupportedModes(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeGrid)

	handler := &Handler{
		appState: appState,
		hints: &components.HintsComponent{
			Context: &hintscomponent.Context{},
		},
		grid: &components.GridComponent{
			Context: &gridcomponent.Context{},
		},
	}

	enabled, supported := handler.ToggleCursorFollowSelection()
	if !supported {
		t.Fatal("ToggleCursorFollowSelection() expected success in grid mode")
	}

	if !enabled {
		t.Fatal("ToggleCursorFollowSelection() expected cursor_follow_selection to become enabled")
	}

	appState.SetMode(domain.ModeHints)

	enabled, supported = handler.ToggleCursorFollowSelection()
	if !supported {
		t.Fatal("ToggleCursorFollowSelection() expected success in hints mode")
	}

	if !enabled {
		t.Fatal(
			"ToggleCursorFollowSelection() expected cursor_follow_selection to become enabled in hints mode",
		)
	}
}

func TestClearCurrentSelectionPoint_ClearsOnlyActiveModeSelection(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	handler := &Handler{
		appState: appState,
		grid: &components.GridComponent{
			Context: &gridcomponent.Context{},
		},
		recursiveGrid: &components.RecursiveGridComponent{
			Context: &recursivegridcomponent.Context{},
		},
	}

	handler.grid.Context.SetSelectionPoint(image.Point{X: 10, Y: 20})
	handler.recursiveGrid.Context.SetSelectionPoint(image.Point{X: 30, Y: 40})

	cleared := handler.ClearCurrentSelectionPoint()
	if !cleared {
		t.Fatal("ClearCurrentSelectionPoint() expected success in recursive-grid mode")
	}

	if _, ok := handler.recursiveGrid.Context.SelectionPoint(); ok {
		t.Fatal("ClearCurrentSelectionPoint() expected recursive-grid selection to be cleared")
	}

	if got, ok := handler.grid.Context.SelectionPoint(); !ok || got != (image.Point{X: 10, Y: 20}) {
		t.Fatalf("grid selection = %v, %v; want (10,20), true", got, ok)
	}
}
