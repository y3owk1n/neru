package modes

import (
	"image"
	"testing"

	"go.uber.org/zap"

	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

func TestStickyIndicatorAnchor_UsesGridSelectionWhenCursorFollowDisabled(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeGrid)

	handler := newHandlerWithState(handlerState{
		appState: appState,
		logger:   zap.NewNop(),
		grid: &components.GridComponent{
			Context: &gridcomponent.Context{},
		},
	})

	handler.grid.Context.SetCursorFollowSelection(false)
	handler.grid.Context.SetSelectionPoint(image.Pt(40, 60))

	got := handler.stickyIndicatorAnchor(image.Pt(10, 20))

	want := image.Pt(40, 60)
	if got != want {
		t.Fatalf("stickyIndicatorAnchor() = %v, want %v", got, want)
	}
}

func TestStickyIndicatorAnchor_UsesRecursiveGridSelectionWhenCursorFollowDisabled(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	handler := newHandlerWithState(handlerState{
		appState: appState,
		logger:   zap.NewNop(),
		recursiveGrid: &components.RecursiveGridComponent{
			Context: &recursivegridcomponent.Context{},
		},
	})

	handler.recursiveGrid.Context.SetCursorFollowSelection(false)
	handler.recursiveGrid.Context.SetSelectionPoint(image.Pt(75, 25))

	got := handler.stickyIndicatorAnchor(image.Pt(10, 20))

	want := image.Pt(75, 25)
	if got != want {
		t.Fatalf("stickyIndicatorAnchor() = %v, want %v", got, want)
	}
}

func TestStickyIndicatorAnchor_UsesCursorWhenGridFollowsSelection(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeGrid)

	handler := newHandlerWithState(handlerState{
		appState: appState,
		logger:   zap.NewNop(),
		grid: &components.GridComponent{
			Context: &gridcomponent.Context{},
		},
	})

	handler.grid.Context.SetCursorFollowSelection(true)
	handler.grid.Context.SetSelectionPoint(image.Pt(40, 60))

	got := handler.stickyIndicatorAnchor(image.Pt(10, 20))

	want := image.Pt(10, 20)
	if got != want {
		t.Fatalf("stickyIndicatorAnchor() = %v, want %v", got, want)
	}
}
