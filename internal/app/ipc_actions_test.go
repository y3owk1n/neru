//nolint:testpackage // Tests private IPC action parsing/dispatch helpers.
package app

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	portmocks "github.com/y3owk1n/neru/internal/core/ports/mocks"
)

func TestParseActionArgs_MoveMouseFlags(t *testing.T) {
	parsed, parseErr := parseActionArgs([]string{"--center", "--monitor=Built-in Retina Display"})
	if parseErr {
		t.Fatal("parseActionArgs() unexpected parse error")
	}

	if !parsed.hasCenter {
		t.Fatal("parseActionArgs() expected hasCenter to be true")
	}

	if !parsed.hasMonitor {
		t.Fatal("parseActionArgs() expected hasMonitor to be true")
	}
}

func TestParseActionArgs_SelectionFlag(t *testing.T) {
	parsed, parseErr := parseActionArgs([]string{"--selection"})
	if parseErr {
		t.Fatal("parseActionArgs() unexpected parse error for --selection")
	}

	if !parsed.useSelection {
		t.Fatal("parseActionArgs() expected useSelection to be true")
	}
}

func TestParseActionArgs_PreviousFlag(t *testing.T) {
	parsed, parseErr := parseActionArgs([]string{"--previous"})
	if parseErr {
		t.Fatal("parseActionArgs() unexpected parse error for --previous")
	}

	if !parsed.usePrevious {
		t.Fatal("parseActionArgs() expected usePrevious to be true")
	}
}

func TestHandleAction_PreviousRejectedOnNonMoveMonitor(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: "action",
		Args:   []string{"left_click", "--previous"},
	})

	if resp.Success {
		t.Fatal("handleAction(left_click --previous) expected failure")
	}

	if resp.Message != "--previous is only supported with move_monitor" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestHandleAction_MoveMonitorRejectsMonitorFlag(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: "action",
		Args:   []string{"move_monitor", "--monitor=foo"},
	})

	if resp.Success {
		t.Fatal("handleAction(move_monitor --monitor) expected failure")
	}

	if resp.Message != "move_monitor only supports --previous; use move_mouse --center --monitor to target a named display" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestHandleAction_MoveMonitorRejectsUnsupportedFlags(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: "action",
		Args:   []string{"move_monitor", "--selection"},
	})

	if resp.Success {
		t.Fatal("handleAction(move_monitor --selection) expected failure")
	}

	if resp.Message != "move_monitor only supports --previous; use move_mouse --center --monitor to target a named display" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestParseActionArgs_BareFlag(t *testing.T) {
	parsed, parseErr := parseActionArgs([]string{"--bare"})
	if parseErr {
		t.Fatal("parseActionArgs() unexpected parse error for --bare")
	}

	if !parsed.useBare {
		t.Fatal("parseActionArgs() expected useBare to be true")
	}
}

func TestHandleAction_MoveMouseWithoutTargetingOrSelectionErrors(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: "action",
		Args:   []string{"move_mouse"},
	})

	if resp.Success {
		t.Fatal("handleAction(move_mouse) expected failure without explicit target or selection")
	}

	if resp.Message != "move_mouse requires --x and --y flags, --center, active selection, or --bare" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestHandleAction_MoveMouseSelectionWithoutActiveSelectionErrors(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: "action",
		Args:   []string{"move_mouse", "--selection"},
	})

	if resp.Success {
		t.Fatal("handleAction(move_mouse --selection) expected failure without active selection")
	}

	if resp.Message != "--selection requires an active mode selection" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestHandleAction_ScrollSelectionWithoutActiveSelectionErrors(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
		scrollService: services.NewScrollService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockOverlayPort{},
			&portmocks.SystemMock{},
			config.ScrollConfig{ScrollStep: 10, ScrollStepHalf: 20, ScrollStepFull: 30},
			zap.NewNop(),
		),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: "action",
		Args:   []string{"scroll_down", "--selection"},
	})

	if resp.Success {
		t.Fatal("handleAction(scroll_down --selection) expected failure without active selection")
	}

	if resp.Message != "--selection requires an active mode selection" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestShouldClearSelectionAfterMoveMouse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		parsed           parsedActionArgs
		targetsSelection bool
		want             bool
	}{
		{
			name:             "relative move clears selection",
			parsed:           parsedActionArgs{hasDX: true, hasDY: true},
			targetsSelection: false,
			want:             true,
		},
		{
			name:             "absolute move clears selection",
			parsed:           parsedActionArgs{hasX: true, hasY: true},
			targetsSelection: false,
			want:             true,
		},
		{
			name:             "center move clears selection",
			parsed:           parsedActionArgs{hasCenter: true},
			targetsSelection: false,
			want:             true,
		},
		{
			name:             "monitor center move clears selection",
			parsed:           parsedActionArgs{hasCenter: true, hasMonitor: true},
			targetsSelection: false,
			want:             true,
		},
		{
			name:             "bare move clears selection",
			parsed:           parsedActionArgs{useBare: true},
			targetsSelection: false,
			want:             true,
		},
		{
			name:             "selection targeted move preserves selection",
			parsed:           parsedActionArgs{useSelection: true},
			targetsSelection: true,
			want:             false,
		},
		{
			name:             "default selection resolved move preserves selection",
			parsed:           parsedActionArgs{},
			targetsSelection: true,
			want:             false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := shouldClearSelectionAfterMoveMouse(testCase.parsed, testCase.targetsSelection)
			if got != testCase.want {
				t.Fatalf("shouldClearSelectionAfterMoveMouse() = %v, want %v", got, testCase.want)
			}
		})
	}
}
