//nolint:testpackage // Tests private IPC action parsing/dispatch helpers.
package app

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
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

	if resp.Message != "move_mouse requires --x and --y flags, --center, or --selection" {
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
