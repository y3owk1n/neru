//nolint:testpackage // Tests private sticky-indicator anchor selection behavior.
package modes

import (
	"image"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components"
	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
)

func TestStickyIndicatorAnchor_UsesGridSelectionWhenCursorFollowDisabled(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeGrid)

	handler := &Handler{
		appState: appState,
		logger:   zap.NewNop(),
		grid: &components.GridComponent{
			Context: &gridcomponent.Context{},
		},
	}

	handler.grid.Context.SetCursorFollowSelection(false)
	handler.grid.Context.SetSelectionPoint(image.Pt(40, 60))

	got := handler.stickyIndicatorAnchorLocked(image.Pt(10, 20))

	want := image.Pt(40, 60)
	if got != want {
		t.Fatalf("stickyIndicatorAnchor() = %v, want %v", got, want)
	}
}

func TestStickyIndicatorAnchor_UsesRecursiveGridSelectionWhenCursorFollowDisabled(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	handler := &Handler{
		appState: appState,
		logger:   zap.NewNop(),
		recursiveGrid: &components.RecursiveGridComponent{
			Context: &recursivegridcomponent.Context{},
		},
	}

	handler.recursiveGrid.Context.SetCursorFollowSelection(false)
	handler.recursiveGrid.Context.SetSelectionPoint(image.Pt(75, 25))

	got := handler.stickyIndicatorAnchorLocked(image.Pt(10, 20))

	want := image.Pt(75, 25)
	if got != want {
		t.Fatalf("stickyIndicatorAnchor() = %v, want %v", got, want)
	}
}

func TestStickyIndicatorAnchor_UsesCursorWhenGridFollowsSelection(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeGrid)

	handler := &Handler{
		appState: appState,
		logger:   zap.NewNop(),
		grid: &components.GridComponent{
			Context: &gridcomponent.Context{},
		},
	}

	handler.grid.Context.SetCursorFollowSelection(true)
	handler.grid.Context.SetSelectionPoint(image.Pt(40, 60))

	got := handler.stickyIndicatorAnchorLocked(image.Pt(10, 20))

	want := image.Pt(10, 20)
	if got != want {
		t.Fatalf("stickyIndicatorAnchor() = %v, want %v", got, want)
	}
}
