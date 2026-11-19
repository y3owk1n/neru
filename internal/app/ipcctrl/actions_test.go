package ipcctrl

import (
	"context"
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

const (
	moveMonitor     = "move_monitor"
	moveMouse       = "move_mouse"
	leftClick       = "left_click"
	rightClick      = "right_click"
	scrollUp        = "scroll_up"
	stateDown       = "down"
	flagStateDown   = flagState + "=" + stateDown
	fooStr          = "foo"
	waitForModeExit = "wait_for_mode_exit"
)

// TestParseActionArgs_StoresEachFlagsValue pins where each flag lands. Presence
// is checked separately (TestParseActionArgs_MarksEveryFlagPresent), so this
// compares every field to catch a flag written to the wrong one.
func TestParseActionArgs_StoresEachFlagsValue(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantParseErr bool
		want         parsedActionArgs
	}{
		{
			name: "direction and count, equals form",
			args: []string{directionLeft, "--count=3"},
			want: parsedActionArgs{
				directionStr: dirLeftValue, hasDirection: true, countVal: 3, hasCount: true,
			},
		},
		{
			name: "direction and count, space form",
			args: []string{"--direction", "up", "--count", "2"},
			want: parsedActionArgs{
				directionStr: "up", hasDirection: true, countVal: 2, hasCount: true,
			},
		},
		{
			name: "count is optional",
			args: []string{"--direction=right"},
			want: parsedActionArgs{directionStr: "right", hasDirection: true},
		},
		{
			name: "center and coordinates",
			args: []string{flagCenter, "--x=100", "--y=200"},
			want: parsedActionArgs{hasCenter: true, xVal: 100, hasX: true, yVal: 200, hasY: true},
		},
		{name: "window", args: []string{flagWindow}, want: parsedActionArgs{hasWindow: true}},
		{
			name: "selection",
			args: []string{flagSelection},
			want: parsedActionArgs{useSelection: true},
		},
		{name: "previous", args: []string{flagPrevious}, want: parsedActionArgs{usePrevious: true}},
		{name: "bail", args: []string{flagBail}, want: parsedActionArgs{useBail: true}},
		{name: "bare", args: []string{"--bare"}, want: parsedActionArgs{useBare: true}},
		{
			name: "steps, equals form",
			args: []string{"--steps=100"},
			want: parsedActionArgs{stepsOverride: 100, hasSteps: true},
		},
		{
			name: "steps, space form",
			args: []string{"--steps", "200"},
			want: parsedActionArgs{stepsOverride: 200, hasSteps: true},
		},
		{
			name: "modifier list is kept verbatim",
			args: []string{"--modifier=cmd,shift"},
			want: parsedActionArgs{modifierStr: "cmd,shift"},
		},
		{
			name: "name, equals form",
			args: []string{flagName + "=DELL U2720Q"},
			want: parsedActionArgs{monitorName: "DELL U2720Q", hasMonitorName: true},
		},
		{
			name: "name, space form",
			args: []string{flagName, builtInRetinaDisplay},
			want: parsedActionArgs{monitorName: builtInRetinaDisplay, hasMonitorName: true},
		},
		{name: "empty direction", args: []string{"--direction="}, wantParseErr: true},
		{name: "missing direction value", args: []string{"--direction"}, wantParseErr: true},
		{name: "non-numeric count", args: []string{"--count=many"}, wantParseErr: true},
		{name: "zero count", args: []string{"--count=0"}, wantParseErr: true},
		{name: "negative count", args: []string{"--count=-2"}, wantParseErr: true},
		{name: "zero steps", args: []string{"--steps=0"}, wantParseErr: true},
		{name: "negative steps", args: []string{"--steps=-10"}, wantParseErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			parsed, parseErr := parseActionArgs(testCase.args)

			if parseErr != testCase.wantParseErr {
				t.Fatalf(
					"parseActionArgs(%v) parseErr = %v, want %v",
					testCase.args,
					parseErr,
					testCase.wantParseErr,
				)
			}

			if testCase.wantParseErr {
				return
			}

			parsed.present = nil

			if !reflect.DeepEqual(parsed, testCase.want) {
				t.Errorf(
					"parseActionArgs(%v) = %+v, want %+v",
					testCase.args,
					parsed,
					testCase.want,
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

func TestHandleAction_MoveMouseSelectionWithoutActiveSelectionErrors(t *testing.T) {
	controller := &ActionsHandler{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: ActionCommand,
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
	controller := &ActionsHandler{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: ActionCommand,
		Args:   []string{moveMouse, flagCenter, flagWindow},
	})

	if resp.Success {
		t.Fatal("handleAction(move_mouse --center --window) expected failure")
	}

	if resp.Message != "--center and --window cannot be used together" {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

func TestHandleAction_ScrollSelectionWithoutActiveSelectionErrors(t *testing.T) {
	controller := &ActionsHandler{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
		scrollService: services.NewScrollService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockMousePort{},
			&portmocks.MockSystemPort{},
			config.ScrollConfig{ScrollStep: 10, ScrollStepHalf: 20, ScrollStepFull: 30},
			zap.NewNop(),
		),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: ActionCommand,
		Args:   []string{"scroll_down", flagSelection},
	})

	if resp.Success {
		t.Fatal("handleAction(scroll_down --selection) expected failure without active selection")
	}

	if resp.Message != msgSelectionRequiresActiveSelection {
		t.Fatalf("unexpected error message: %q", resp.Message)
	}
}

// scrollControllerWithModifierCapture builds a controller whose scroll service
// records the modifier set that reached the mouse port.
func scrollControllerWithModifierCapture(
	got *action.Modifiers,
) *ActionsHandler {
	mouse := &portmocks.MockMousePort{
		ScrollFunc: func(_ context.Context, _, _ int, modifiers action.Modifiers) error {
			*got = modifiers

			return nil
		},
	}

	return &ActionsHandler{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
		scrollService: services.NewScrollService(
			&portmocks.MockAccessibilityPort{},
			mouse,
			&portmocks.MockSystemPort{},
			config.ScrollConfig{ScrollStep: 10, ScrollStepHalf: 20, ScrollStepFull: 30},
			zap.NewNop(),
		),
	}
}

// TestHandleAction_ScrollAcceptsModifier pins issue #1448. Every scroll
// sub-action takes --modifier and hands the parsed set to the scroll service,
// which is what makes ctrl + scroll_up zoom instead of pan.
func TestHandleAction_ScrollAcceptsModifier(t *testing.T) {
	scrollActions := []string{
		scrollUp, scrollDown, "scroll_left", "scroll_right",
		"go_top", "go_bottom", "page_up", "page_down",
	}

	for _, name := range scrollActions {
		t.Run(name, func(t *testing.T) {
			var got action.Modifiers

			controller := scrollControllerWithModifierCapture(&got)

			resp := controller.handleAction(context.Background(), ipc.Command{
				Action: ActionCommand,
				Args:   []string{name, flagModifier + "=ctrl,shift"},
			})

			if !resp.Success {
				t.Fatalf("handleAction(%s --modifier ctrl,shift) failed: %s", name, resp.Message)
			}

			if want := action.ModCtrl | action.ModShift; got != want {
				t.Errorf("port received modifiers %q, want %q", got, want)
			}
		})
	}
}

// TestHandleAction_ScrollRejectsUnknownModifier keeps a typo loud. An
// unparseable modifier must refuse the action rather than scroll without it.
func TestHandleAction_ScrollRejectsUnknownModifier(t *testing.T) {
	var got action.Modifiers

	controller := scrollControllerWithModifierCapture(&got)

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: ActionCommand,
		Args:   []string{scrollUp, flagModifier + "=hyper"},
	})

	if resp.Success {
		t.Fatal("handleAction(scroll_up --modifier hyper) expected refusal")
	}

	if resp.Code != ipc.CodeInvalidInput {
		t.Fatalf("code = %q, want %q", resp.Code, ipc.CodeInvalidInput)
	}

	if got != 0 {
		t.Errorf("a refused modifier still reached the port as %q", got)
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

	controller := &ActionsHandler{
		appState: appState,
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: ActionCommand,
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

	controller := &ActionsHandler{
		appState: appState,
		logger:   zap.NewNop(),
	}

	respCh := make(chan ipc.Response, 1)

	go func() {
		respCh <- controller.handleAction(context.Background(), ipc.Command{
			Action: ActionCommand,
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

func TestHandleAction_ChainRejectsNonPointerActions(t *testing.T) {
	controller := &ActionsHandler{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: ActionCommand,
		Args:   []string{leftClick + ",feed"},
	})
	if resp.Success {
		t.Fatal("expected chain validation to fail for feed action")
	}

	resp = controller.handleAction(context.Background(), ipc.Command{
		Action: ActionCommand,
		Args:   []string{leftClick + ",sleep"},
	})
	if resp.Success {
		t.Fatal("expected chain validation to fail for sleep action")
	}

	resp = controller.handleAction(context.Background(), ipc.Command{
		Action: ActionCommand,
		Args:   []string{leftClick + ",cycle_hint"},
	})
	if resp.Success {
		t.Fatal("expected chain validation to fail for cycle_hint action")
	}
}

// builtInRetinaDisplay is the display name the action tests use when they need
// a screen that exists on every Mac.
const builtInRetinaDisplay = "Built-in Retina Display"
