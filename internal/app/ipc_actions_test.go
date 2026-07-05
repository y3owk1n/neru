//nolint:testpackage // Tests private IPC action parsing/dispatch helpers.
package app

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
	portmocks "github.com/y3owk1n/neru/internal/core/ports/mocks"
)

const (
	moveMonitor     = "move_monitor"
	moveMouse       = "move_mouse"
	fooStr          = "foo"
	waitForModeExit = "wait_for_mode_exit"
)

func TestParseActionArgs_MoveMouseFlags(t *testing.T) {
	parsed, parseErr := parseActionArgs([]string{flagCenter, "--x=100", "--y=200"})
	if parseErr {
		t.Fatal("parseActionArgs() unexpected parse error")
	}

	if !parsed.hasCenter {
		t.Fatal("parseActionArgs() expected hasCenter to be true")
	}

	if parsed.xVal != 100 {
		t.Fatalf("parseActionArgs() expected xVal to be 100, got %d", parsed.xVal)
	}

	if parsed.yVal != 200 {
		t.Fatalf("parseActionArgs() expected yVal to be 200, got %d", parsed.yVal)
	}
}

func TestParseActionArgs_MoveMouseWindowFlag(t *testing.T) {
	parsed, parseErr := parseActionArgs([]string{flagWindow, "--x=50", "--y=50"})
	if parseErr {
		t.Fatal("parseActionArgs() unexpected parse error")
	}

	if !parsed.hasWindow {
		t.Fatal("parseActionArgs() expected hasWindow to be true")
	}

	if parsed.xVal != 50 {
		t.Fatalf("parseActionArgs() expected xVal to be 50, got %d", parsed.xVal)
	}

	if parsed.yVal != 50 {
		t.Fatalf("parseActionArgs() expected yVal to be 50, got %d", parsed.yVal)
	}
}

func TestParseActionArgs_SelectionFlag(t *testing.T) {
	parsed, parseErr := parseActionArgs([]string{flagSelection})
	if parseErr {
		t.Fatal("parseActionArgs() unexpected parse error for --selection")
	}

	if !parsed.useSelection {
		t.Fatal("parseActionArgs() expected useSelection to be true")
	}
}

func TestParseActionArgs_PreviousFlag(t *testing.T) {
	parsed, parseErr := parseActionArgs([]string{flagPrevious})
	if parseErr {
		t.Fatal("parseActionArgs() unexpected parse error for --previous")
	}

	if !parsed.usePrevious {
		t.Fatal("parseActionArgs() expected usePrevious to be true")
	}
}

func TestHandleAction_MoveMonitorRejectsUnsupportedFlags_X(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{moveMonitor, "--x=100"},
	})

	if resp.Success {
		t.Fatal("handleAction(move_monitor --x) expected failure")
	}

	if resp.Message != msgMoveMonitorDoesNotSupportTheseFlags {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestHandleAction_MoveMonitorRejectsUnsupportedFlags(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{moveMonitor, "--selection"},
	})

	if resp.Success {
		t.Fatal("handleAction(move_monitor --selection) expected failure")
	}

	if resp.Message != msgMoveMonitorDoesNotSupportTheseFlags {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestParseActionArgs_BailFlag(t *testing.T) {
	parsed, parseErr := parseActionArgs([]string{flagBail})
	if parseErr {
		t.Fatal("parseActionArgs() unexpected parse error for --bail")
	}

	if !parsed.useBail {
		t.Fatal("parseActionArgs() expected useBail to be true")
	}
}

func TestHandleAction_RejectsBailOnNonWaitForModeExit(t *testing.T) {
	controller := &IPCControllerActions{logger: zap.NewNop()}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{"left_click", flagBail},
	})

	if resp.Success {
		t.Fatal("handleAction(left_click --bail) expected rejection, got success")
	}

	if resp.Code != ipc.CodeInvalidInput {
		t.Fatalf(
			"handleAction(left_click --bail) code = %q, want %q",
			resp.Code,
			ipc.CodeInvalidInput,
		)
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

func TestParseActionArgs_StepsFlagEqualsForm(t *testing.T) {
	parsed, parseErr := parseActionArgs([]string{"--steps=100"})
	if parseErr {
		t.Fatal("parseActionArgs() unexpected parse error for --steps=100")
	}

	if !parsed.hasSteps {
		t.Fatal("parseActionArgs() expected hasSteps to be true")
	}

	if parsed.stepsOverride != 100 {
		t.Fatalf("parseActionArgs() expected stepsOverride to be 100, got %d", parsed.stepsOverride)
	}
}

func TestParseActionArgs_StepsFlagSpaceForm(t *testing.T) {
	parsed, parseErr := parseActionArgs([]string{"--steps", "200"})
	if parseErr {
		t.Fatal("parseActionArgs() unexpected parse error for --steps 200")
	}

	if !parsed.hasSteps {
		t.Fatal("parseActionArgs() expected hasSteps to be true")
	}

	if parsed.stepsOverride != 200 {
		t.Fatalf("parseActionArgs() expected stepsOverride to be 200, got %d", parsed.stepsOverride)
	}
}

func TestParseActionArgs_StepsFlagRejectsZero(t *testing.T) {
	_, parseErr := parseActionArgs([]string{"--steps=0"})
	if !parseErr {
		t.Fatal("parseActionArgs() expected parse error for --steps=0")
	}
}

func TestParseActionArgs_StepsFlagRejectsNegative(t *testing.T) {
	_, parseErr := parseActionArgs([]string{"--steps=-10"})
	if !parseErr {
		t.Fatal("parseActionArgs() expected parse error for --steps=-10")
	}
}

func TestParseActionArgs_ModifierCSV(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantModStr string
	}{
		{
			name:       "single modifier",
			args:       []string{"--modifier=cmd"},
			wantModStr: "cmd",
		},
		{
			name:       "comma-separated modifiers",
			args:       []string{"--modifier=cmd,shift"},
			wantModStr: "cmd,shift",
		},
		{
			name:       "all modifiers",
			args:       []string{"--modifier=cmd,shift,alt,ctrl"},
			wantModStr: "cmd,shift,alt,ctrl",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			parsed, parseErr := parseActionArgs(testCase.args)
			if parseErr {
				t.Fatalf("parseActionArgs() unexpected parse error: %v", parseErr)
			}

			if parsed.modifierStr != testCase.wantModStr {
				t.Fatalf(
					"parseActionArgs() expected modifierStr %q, got %q",
					testCase.wantModStr,
					parsed.modifierStr,
				)
			}
		})
	}
}

func TestParseCSV(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty",
			input: "",
			want:  nil,
		},
		{
			name:  "single value",
			input: fooStr,
			want:  []string{fooStr},
		},
		{
			name:  "comma-separated",
			input: fooStr + ",bar,baz",
			want:  []string{fooStr, "bar", "baz"},
		},
		{
			name:  "trailing comma",
			input: fooStr + ",",
			want:  []string{fooStr, ""},
		},
		{
			name:  "leading comma",
			input: ",foo",
			want:  []string{"", fooStr},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := parseCSV(testCase.input)

			if len(got) != len(testCase.want) {
				t.Fatalf("parseCSV() returned %d elements, want %d", len(got), len(testCase.want))
			}

			for i := range got {
				if got[i] != testCase.want[i] {
					t.Fatalf("parseCSV()[%d] = %q, want %q", i, got[i], testCase.want[i])
				}
			}
		})
	}
}

func TestHandleAction_MoveMouseWithoutTargetingOrSelectionErrors(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{moveMouse},
	})

	if resp.Success {
		t.Fatal("handleAction(move_mouse) expected failure without explicit target or selection")
	}

	if resp.Message != "move_mouse requires --x and --y flags, --center, --window, active selection, or --bare" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestHandleAction_MoveMouseSelectionWithoutActiveSelectionErrors(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{moveMouse, flagSelection},
	})

	if resp.Success {
		t.Fatal("handleAction(move_mouse --selection) expected failure without active selection")
	}

	if resp.Message != msgSelectionRequiresActiveSelection {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestHandleAction_MoveMouseRejectsCenterAndWindow(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{moveMouse, flagCenter, flagWindow},
	})

	if resp.Success {
		t.Fatal("handleAction(move_mouse --center --window) expected failure")
	}

	if resp.Message != "--center and --window cannot be used together" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestHandleAction_MoveMouseRejectsWindowAndDX(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{moveMouse, flagWindow, "--dx=10", "--dy=10"},
	})

	if resp.Success {
		t.Fatal("handleAction(move_mouse --window --dx) expected failure")
	}

	if resp.Message != "use either --window or --dx/--dy, not both" {
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
			&portmocks.MockSystemPort{},
			config.ScrollConfig{ScrollStep: 10, ScrollStepHalf: 20, ScrollStepFull: 30},
			zap.NewNop(),
		),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{"scroll_down", flagSelection},
	})

	if resp.Success {
		t.Fatal("handleAction(scroll_down --selection) expected failure without active selection")
	}

	if resp.Message != msgSelectionRequiresActiveSelection {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestHandleAction_PreviousRejectedOnNonMoveMonitor(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{"left_click", flagPrevious},
	})

	if resp.Success {
		t.Fatal("handleAction(left_click --previous) expected failure")
	}

	if resp.Message != "--previous and --name are only supported with move_monitor" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestHandleAction_NameRejectedOnNonMoveMonitor(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{"reset", "--name=DELL"},
	})

	if resp.Success {
		t.Fatal("handleAction(reset --name) expected failure")
	}

	if resp.Message != "reset does not support action flags" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestHandleAction_PreviousRejectedOnScrollAction(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
		scrollService: services.NewScrollService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockOverlayPort{},
			&portmocks.MockSystemPort{},
			config.ScrollConfig{ScrollStep: 10, ScrollStepHalf: 20, ScrollStepFull: 30},
			zap.NewNop(),
		),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{"scroll_down", flagPrevious},
	})

	if resp.Success {
		t.Fatal("handleAction(scroll_down --previous) expected failure")
	}

	if resp.Message != "scroll actions do not support --x/--y/--dx/--dy/--center/--name/--modifier/--previous flags" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestParseActionArgs_NameFlag(t *testing.T) {
	parsed, parseErr := parseActionArgs([]string{flagName + "=DELL U2720Q"})
	if parseErr {
		t.Fatal("parseActionArgs() unexpected parse error for --name")
	}

	if !parsed.hasMonitorName {
		t.Fatal("parseActionArgs() expected hasMonitorName to be true")
	}

	if parsed.monitorName != "DELL U2720Q" {
		t.Fatalf(
			"parseActionArgs() expected monitorName to be 'DELL U2720Q', got %q",
			parsed.monitorName,
		)
	}
}

func TestParseActionArgs_NameFlagSpaceForm(t *testing.T) {
	parsed, parseErr := parseActionArgs([]string{flagName, builtInRetinaDisplay})
	if parseErr {
		t.Fatal("parseActionArgs() unexpected parse error for --name space form")
	}

	if !parsed.hasMonitorName {
		t.Fatal("parseActionArgs() expected hasMonitorName to be true")
	}

	if parsed.monitorName != builtInRetinaDisplay {
		t.Fatalf(
			"parseActionArgs() expected monitorName to be 'Built-in Retina Display', got %q",
			parsed.monitorName,
		)
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
			name:             "window move clears selection",
			parsed:           parsedActionArgs{hasWindow: true},
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

func TestParseSleepDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantValid bool
		wantNS    time.Duration
	}{
		{
			name:      "bare number seconds",
			input:     "0.2",
			wantValid: true,
			wantNS:    200 * time.Millisecond,
		},
		{
			name:      "integer seconds",
			input:     "1",
			wantValid: true,
			wantNS:    1 * time.Second,
		},
		{
			name:      "float seconds",
			input:     "1.5",
			wantValid: true,
			wantNS:    1500 * time.Millisecond,
		},
		{
			name:      "milliseconds",
			input:     "500ms",
			wantValid: true,
			wantNS:    500 * time.Millisecond,
		},
		{
			name:      "seconds with s suffix",
			input:     "2s",
			wantValid: true,
			wantNS:    2 * time.Second,
		},
		{
			name:      "empty string",
			input:     "",
			wantValid: false,
			wantNS:    0,
		},
		{
			name:      "zero duration",
			input:     "0",
			wantValid: false,
			wantNS:    0,
		},
		{
			name:      "negative duration",
			input:     "-1s",
			wantValid: false,
			wantNS:    0,
		},
		{
			name:      "invalid string",
			input:     "invalid",
			wantValid: false,
			wantNS:    0,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseSleepDuration(testCase.input)
			if testCase.wantValid && err != nil {
				t.Fatalf("parseSleepDuration(%q) unexpected error: %v", testCase.input, err)
			}

			if !testCase.wantValid && err == nil {
				t.Fatalf("parseSleepDuration(%q) expected error, got nil", testCase.input)
			}

			if got != testCase.wantNS {
				t.Fatalf(
					"parseSleepDuration(%q) = %v, want %v",
					testCase.input,
					got,
					testCase.wantNS,
				)
			}
		})
	}
}

func TestHandleAction_WaitForModeExitBail_NoSelection(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeIdle)

	controller := &IPCControllerActions{
		appState: appState,
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{waitForModeExit, flagBail},
	})

	if resp.Success {
		t.Fatal("handleAction(wait_for_mode_exit --bail) expected bail when no selection was made")
	}

	if resp.Code != ipc.CodeChainBail {
		t.Fatalf("response code = %q, want %q", resp.Code, ipc.CodeChainBail)
	}
}

func TestHandleAction_WaitForModeExitBail_WithSelection(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeMonitorSelect)

	controller := &IPCControllerActions{
		appState: appState,
		logger:   zap.NewNop(),
	}

	respCh := make(chan ipc.Response, 1)

	go func() {
		respCh <- controller.handleAction(context.Background(), ipc.Command{
			Action: actionCmd,
			Args:   []string{waitForModeExit, flagBail},
		})
	}()

	time.Sleep(50 * time.Millisecond)

	appState.SetModeExitReason(state.ModeExitReasonCompleted)
	appState.SetMode(domain.ModeIdle)

	select {
	case resp := <-respCh:
		if !resp.Success {
			t.Fatalf("expected success after selection, got: %s", resp.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for wait_for_mode_exit --bail")
	}
}

func TestHandleAction_WaitForModeExitBail_StaleReasonCleared(t *testing.T) {
	appState := state.NewAppState()
	appState.SetMode(domain.ModeIdle)
	appState.SetModeExitReason(state.ModeExitReasonCompleted)
	appState.ConsumeModeExitReason()

	controller := &IPCControllerActions{
		appState: appState,
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{waitForModeExit, flagBail},
	})

	if resp.Success {
		t.Fatal("handleAction(wait_for_mode_exit --bail) expected bail after stale reason consumed")
	}

	if resp.Code != ipc.CodeChainBail {
		t.Fatalf("response code = %q, want %q", resp.Code, ipc.CodeChainBail)
	}
}
