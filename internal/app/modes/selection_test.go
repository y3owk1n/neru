//nolint:testpackage // Tests private mode selection helpers.
package modes

import (
	"context"
	"image"
	"testing"

	"go.uber.org/zap"

	gridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
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
		logger:   zap.NewNop(),
		actionService: services.NewActionService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockOverlayPort{},
			&portmocks.MockSystemPort{},
			zap.NewNop(),
		),
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

func TestToggleCursorFollowSelection_MovesCursorToStoredGridSelectionWhenEnabling(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeGrid)

	var moved []image.Point

	handler := &Handler{
		appState: appState,
		logger:   zap.NewNop(),
		actionService: services.NewActionService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockOverlayPort{},
			&portmocks.MockSystemPort{
				MoveCursorToPointFunc: func(_ context.Context, point image.Point, _ bool) error {
					moved = append(moved, point)

					return nil
				},
			},
			zap.NewNop(),
		),
		grid: &components.GridComponent{
			Context: &gridcomponent.Context{},
		},
	}

	handler.grid.Context.SetSelectionPoint(image.Point{X: 120, Y: 240})

	enabled, supported := handler.ToggleCursorFollowSelection()
	if !supported || !enabled {
		t.Fatalf("ToggleCursorFollowSelection() = (%v, %v), want (true, true)", enabled, supported)
	}

	if len(moved) != 1 {
		t.Fatalf("ToggleCursorFollowSelection() moved cursor %d times, want 1", len(moved))
	}

	if moved[0] != (image.Point{X: 120, Y: 240}) {
		t.Fatalf("ToggleCursorFollowSelection() moved cursor to %v, want (120,240)", moved[0])
	}
}

func TestToggleCursorFollowSelection_DoesNotMoveCursorWhenDisabling(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeGrid)

	moveCount := 0

	handler := &Handler{
		appState: appState,
		logger:   zap.NewNop(),
		actionService: services.NewActionService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockOverlayPort{},
			&portmocks.MockSystemPort{
				MoveCursorToPointFunc: func(_ context.Context, _ image.Point, _ bool) error {
					moveCount++

					return nil
				},
			},
			zap.NewNop(),
		),
		grid: &components.GridComponent{
			Context: &gridcomponent.Context{},
		},
	}

	handler.grid.Context.SetCursorFollowSelection(true)
	handler.grid.Context.SetSelectionPoint(image.Point{X: 120, Y: 240})

	enabled, supported := handler.ToggleCursorFollowSelection()
	if !supported || enabled {
		t.Fatalf("ToggleCursorFollowSelection() = (%v, %v), want (false, true)", enabled, supported)
	}

	if moveCount != 0 {
		t.Fatalf(
			"ToggleCursorFollowSelection() moved cursor %d times while disabling, want 0",
			moveCount,
		)
	}
}

func TestToggleCursorFollowSelection_MovesCursorToStoredRecursiveGridSelectionWhenEnabling(
	t *testing.T,
) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeRecursiveGrid)

	var moved []image.Point

	handler := &Handler{
		appState: appState,
		logger:   zap.NewNop(),
		actionService: services.NewActionService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockOverlayPort{},
			&portmocks.MockSystemPort{
				MoveCursorToPointFunc: func(_ context.Context, point image.Point, _ bool) error {
					moved = append(moved, point)

					return nil
				},
			},
			zap.NewNop(),
		),
		recursiveGrid: &components.RecursiveGridComponent{
			Context: &recursivegridcomponent.Context{},
		},
	}

	handler.recursiveGrid.Context.SetSelectionPoint(image.Point{X: 33, Y: 66})

	enabled, supported := handler.ToggleCursorFollowSelection()
	if !supported || !enabled {
		t.Fatalf("ToggleCursorFollowSelection() = (%v, %v), want (true, true)", enabled, supported)
	}

	if len(moved) != 1 {
		t.Fatalf("ToggleCursorFollowSelection() moved cursor %d times, want 1", len(moved))
	}

	if moved[0] != (image.Point{X: 33, Y: 66}) {
		t.Fatalf("ToggleCursorFollowSelection() moved cursor to %v, want (33,66)", moved[0])
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

func TestSetCursorFollowSelection_ConvergesRatherThanFlipping(t *testing.T) {
	tests := []struct {
		name    string
		initial bool
		set     bool
	}{
		{name: "on from off", initial: false, set: true},
		// Setting the state it already holds must not flip it. This is the
		// whole difference between the setter and the toggle.
		{name: "on from on", initial: true, set: true},
		{name: "off from on", initial: true, set: false},
		{name: "off from off", initial: false, set: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			appState := state.NewAppState()
			appState.SetMode(domain.ModeHints)

			handler := &Handler{
				appState: appState,
				logger:   zap.NewNop(),
				hints: &components.HintsComponent{
					Context: &hintscomponent.Context{},
				},
			}

			handler.hints.Context.SetCursorFollowSelection(testCase.initial)

			enabled, supported := handler.SetCursorFollowSelection(testCase.set)
			if !supported {
				t.Fatal("SetCursorFollowSelection() expected success in hints mode")
			}

			if enabled != testCase.set {
				t.Fatalf("SetCursorFollowSelection(%t) = %t, want %t",
					testCase.set, enabled, testCase.set)
			}

			if got := handler.hints.Context.CursorFollowSelection(); got != testCase.set {
				t.Fatalf("context cursor follow = %t, want %t", got, testCase.set)
			}
		})
	}
}

func TestCursorFollowSelection_ReadsWithoutChanging(t *testing.T) {
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

	for range 2 {
		enabled, supported := handler.CursorFollowSelection()
		if !supported {
			t.Fatal("CursorFollowSelection() expected success in grid mode")
		}

		if !enabled {
			t.Fatal("CursorFollowSelection() = false, want the state left as it was read")
		}
	}
}

func TestCursorFollowSelection_ReportsUnsupportedWithoutAMode(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeIdle)

	handler := &Handler{
		appState: appState,
		logger:   zap.NewNop(),
		hints: &components.HintsComponent{
			Context: &hintscomponent.Context{},
		},
	}

	if _, supported := handler.CursorFollowSelection(); supported {
		t.Fatal("CursorFollowSelection() reported support in idle mode")
	}

	// Naming the state you want does not conjure a session to hold it.
	if _, supported := handler.SetCursorFollowSelection(true); supported {
		t.Fatal("SetCursorFollowSelection() reported support in idle mode")
	}
}
