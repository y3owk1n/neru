//nolint:testpackage // Tests private IPC action parsing/dispatch helpers.
package app

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/domain/state"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

func TestParseActionArgs_StateAndToggleFlags(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantState     string
		wantHasState  bool
		wantUseToggle bool
		wantParseErr  bool
	}{
		{
			name:         "state with equals",
			args:         []string{flagStateDown},
			wantState:    stateDown,
			wantHasState: true,
		},
		{
			name:         "state with space",
			args:         []string{flagState, "up"},
			wantState:    "up",
			wantHasState: true,
		},
		{
			name:          "toggle",
			args:          []string{flagToggle},
			wantUseToggle: true,
		},
		{
			name:         "state without value",
			args:         []string{flagState},
			wantParseErr: true,
		},
		{
			name:         "state with empty value",
			args:         []string{flagState + "="},
			wantParseErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			parsed, parseErr := parseActionArgs(testCase.args)

			if parseErr != testCase.wantParseErr {
				t.Fatalf("parseActionArgs(%v) parseErr = %v, want %v",
					testCase.args, parseErr, testCase.wantParseErr)
			}

			if testCase.wantParseErr {
				return
			}

			if parsed.stateStr != testCase.wantState {
				t.Errorf("stateStr = %q, want %q", parsed.stateStr, testCase.wantState)
			}

			if parsed.hasState != testCase.wantHasState {
				t.Errorf("hasState = %v, want %v", parsed.hasState, testCase.wantHasState)
			}

			if parsed.useToggle != testCase.wantUseToggle {
				t.Errorf("useToggle = %v, want %v", parsed.useToggle, testCase.wantUseToggle)
			}
		})
	}
}

func TestResolveMouseButtonPhase(t *testing.T) {
	tests := []struct {
		name       string
		actionName string
		parsed     parsedActionArgs
		want       string
	}{
		{
			name:       "no flags leaves the action alone",
			actionName: leftClick,
			want:       "left_click",
		},
		{
			name:       "left down",
			actionName: leftClick,
			parsed:     parsedActionArgs{hasState: true, stateStr: stateDown},
			want:       "left_mouse_down",
		},
		{
			name:       "left up",
			actionName: leftClick,
			parsed:     parsedActionArgs{hasState: true, stateStr: "up"},
			want:       "left_mouse_up",
		},
		{
			name:       "right down",
			actionName: rightClick,
			parsed:     parsedActionArgs{hasState: true, stateStr: stateDown},
			want:       "right_mouse_down",
		},
		{
			name:       "middle up",
			actionName: "middle_click",
			parsed:     parsedActionArgs{hasState: true, stateStr: "up"},
			want:       "middle_mouse_up",
		},
		{
			name:       "right toggle",
			actionName: rightClick,
			parsed:     parsedActionArgs{useToggle: true},
			want:       "right_mouse_toggle",
		},
		{
			name:       "non-click action without flags is untouched",
			actionName: scrollUp,
			want:       "scroll_up",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, errResp := resolveMouseButtonPhase(testCase.actionName, testCase.parsed)
			if errResp != nil {
				t.Fatalf("resolveMouseButtonPhase() unexpected rejection: %q", errResp.Message)
			}

			if got != testCase.want {
				t.Errorf("resolveMouseButtonPhase() = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestResolveMouseButtonPhase_Rejections(t *testing.T) {
	tests := []struct {
		name        string
		actionName  string
		parsed      parsedActionArgs
		wantMessage string
	}{
		{
			name:        "state and toggle together",
			actionName:  leftClick,
			parsed:      parsedActionArgs{hasState: true, stateStr: stateDown, useToggle: true},
			wantMessage: "--state and --toggle cannot be used together",
		},
		{
			name:        "state on a press action",
			actionName:  "mouse_down",
			parsed:      parsedActionArgs{hasState: true, stateStr: "up"},
			wantMessage: msgStateOnlyOnClicks,
		},
		{
			name:        "toggle on a scroll action",
			actionName:  scrollUp,
			parsed:      parsedActionArgs{useToggle: true},
			wantMessage: msgStateOnlyOnClicks,
		},
		{
			name:        "state on move_mouse",
			actionName:  moveMouse,
			parsed:      parsedActionArgs{hasState: true, stateStr: stateDown},
			wantMessage: msgStateOnlyOnClicks,
		},
		{
			name:        "state on a click chain",
			actionName:  leftClick + "," + leftClick,
			parsed:      parsedActionArgs{hasState: true, stateStr: stateDown},
			wantMessage: msgStateOnlyOnClicks,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, errResp := resolveMouseButtonPhase(testCase.actionName, testCase.parsed)
			if errResp == nil {
				t.Fatal("resolveMouseButtonPhase() expected rejection, got none")
			}

			if errResp.Message != testCase.wantMessage {
				t.Errorf("message = %q, want %q", errResp.Message, testCase.wantMessage)
			}

			if errResp.Code != ipc.CodeInvalidInput {
				t.Errorf("code = %q, want %q", errResp.Code, ipc.CodeInvalidInput)
			}
		})
	}
}

func TestResolveMouseButtonPhase_RejectsUnknownState(t *testing.T) {
	_, errResp := resolveMouseButtonPhase(
		leftClick,
		parsedActionArgs{hasState: true, stateStr: "sideways"},
	)
	if errResp == nil {
		t.Fatal("resolveMouseButtonPhase(--state sideways) expected rejection, got none")
	}

	if errResp.Code != ipc.CodeInvalidInput {
		t.Errorf("code = %q, want %q", errResp.Code, ipc.CodeInvalidInput)
	}
}

func TestHandleAction_RejectsStateOnNonClickAction(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{scrollUp, flagStateDown},
	})

	if resp.Success {
		t.Fatal("handleAction(scroll_up --state down) expected rejection, got success")
	}

	if resp.Message != msgStateOnlyOnClicks {
		t.Fatalf("message = %q, want %q", resp.Message, msgStateOnlyOnClicks)
	}
}

func TestHandleAction_RejectsStateAndToggleTogether(t *testing.T) {
	controller := &IPCControllerActions{
		appState: state.NewAppState(),
		logger:   zap.NewNop(),
	}

	resp := controller.handleAction(context.Background(), ipc.Command{
		Action: actionCmd,
		Args:   []string{leftClick, flagStateDown, flagToggle},
	})

	if resp.Success {
		t.Fatal("handleAction(left_click --state down --toggle) expected rejection, got success")
	}

	if resp.Code != ipc.CodeInvalidInput {
		t.Fatalf("code = %q, want %q", resp.Code, ipc.CodeInvalidInput)
	}
}

func TestIsMouseButtonActionName(t *testing.T) {
	tests := []struct {
		actionName string
		want       bool
	}{
		{leftClick, true},
		{rightClick, true},
		{"middle_click", true},
		{"left_mouse_down", true},
		{"right_mouse_up", true},
		{"middle_mouse_toggle", true},
		{"mouse_down", true},
		{"mouse_up", true},
		{moveMouse, false},
		{"scroll", false},
		{fooStr, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.actionName, func(t *testing.T) {
			got := isMouseButtonActionName(testCase.actionName)
			if got != testCase.want {
				t.Errorf("isMouseButtonActionName(%q) = %v, want %v",
					testCase.actionName, got, testCase.want)
			}
		})
	}
}
