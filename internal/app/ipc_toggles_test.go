//nolint:testpackage // Tests the private toggle-state parsing and handlers.
package app

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

const (
	// toggleCommand is the command name the parse tests report failures against.
	toggleCommand = domain.CommandToggleScrollInvert

	stateOnArg  = flagState + "=" + toggleStateOn
	stateOffArg = flagState + "=" + toggleStateOff

	// blankArg stands in for whitespace a caller may pass by accident.
	blankArg = "   "
)

func TestParseToggleState_ResolvesEveryForm(t *testing.T) {
	t.Parallel()

	wantOn, wantOff := true, false

	tests := []struct {
		name string
		args []string
		want *bool
	}{
		{name: "no flag toggles", args: nil, want: nil},
		{name: "empty args toggle", args: []string{"", blankArg}, want: nil},
		{name: "inline on", args: []string{stateOnArg}, want: &wantOn},
		{name: "inline off", args: []string{stateOffArg}, want: &wantOff},
		{name: "separate on", args: []string{flagState, toggleStateOn}, want: &wantOn},
		{name: "separate off", args: []string{flagState, toggleStateOff}, want: &wantOff},
		{name: "explicit toggle", args: []string{flagState + "=" + toggleStateToggle}, want: nil},
		{name: "last one wins", args: []string{stateOnArg, stateOffArg}, want: &wantOff},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, errResponse := parseToggleState(toggleCommand, testCase.args)
			if errResponse != nil {
				t.Fatalf("parseToggleState(%v) = %+v, want success", testCase.args, errResponse)
			}

			switch {
			case testCase.want == nil && got != nil:
				t.Fatalf("parseToggleState(%v) = %t, want toggle", testCase.args, *got)
			case testCase.want != nil && got == nil:
				t.Fatalf("parseToggleState(%v) = toggle, want %t", testCase.args, *testCase.want)
			case testCase.want != nil && *got != *testCase.want:
				t.Fatalf("parseToggleState(%v) = %t, want %t", testCase.args, *got, *testCase.want)
			}
		})
	}
}

func TestParseToggleState_RejectsBadInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown value", args: []string{flagState + "=maybe"}},
		{name: "empty value", args: []string{flagState + "="}},
		// The value is missing, not the next flag: swallowing it would turn a
		// typo into a state change the caller never asked for.
		{name: "flag as value", args: []string{flagState, stateOnArg}},
		{name: "missing value at end", args: []string{flagState}},
		{name: "unknown flag", args: []string{"--force"}},
		{name: "bare argument", args: []string{"on"}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, errResponse := parseToggleState(toggleCommand, testCase.args)
			if errResponse == nil {
				t.Fatalf("parseToggleState(%v) = %v, want a rejection", testCase.args, got)
			}

			if errResponse.Success || errResponse.Code != ipc.CodeInvalidInput {
				t.Fatalf(
					"parseToggleState(%v) = %+v, want invalid input",
					testCase.args,
					errResponse,
				)
			}

			if !strings.Contains(errResponse.Message, toggleCommand) {
				t.Fatalf("message %q does not name the command", errResponse.Message)
			}
		})
	}
}

func TestHandleToggleScrollInvert_ConvergesOnRequestedState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		initial bool
		args    []string
		want    bool
	}{
		{name: "toggle from off", initial: false, args: nil, want: true},
		{name: "toggle from on", initial: true, args: nil, want: false},
		{name: "on from off", initial: false, args: []string{stateOnArg}, want: true},
		// The point of --state: asking for the state you are already in is not
		// a flip, so a repeated call does not undo itself.
		{name: "on from on", initial: true, args: []string{stateOnArg}, want: true},
		{name: "off from on", initial: true, args: []string{stateOffArg}, want: false},
		{name: "off from off", initial: false, args: []string{stateOffArg}, want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			appState := state.NewAppState()
			appState.SetScrollInverted(testCase.initial)

			handler := NewIPCControllerScroll(appState, nil, zap.NewNop())

			resp := handler.handleToggleScrollInvert(
				context.Background(),
				ipc.Command{Args: testCase.args},
			)
			if !resp.Success {
				t.Fatalf("handleToggleScrollInvert(%v) = %+v, want success", testCase.args, resp)
			}

			if got := appState.IsScrollInverted(); got != testCase.want {
				t.Fatalf("scroll inverted = %t, want %t", got, testCase.want)
			}

			assertToggleData(t, resp, "inverted", testCase.want)
		})
	}
}

func TestHandleToggleScrollInvert_RejectsBadState(t *testing.T) {
	t.Parallel()

	appState := state.NewAppState()
	appState.SetScrollInverted(true)

	handler := NewIPCControllerScroll(appState, nil, zap.NewNop())

	resp := handler.handleToggleScrollInvert(
		context.Background(),
		ipc.Command{Args: []string{flagState + "=maybe"}},
	)
	if resp.Success || resp.Code != ipc.CodeInvalidInput {
		t.Fatalf("handleToggleScrollInvert() = %+v, want invalid input", resp)
	}

	// A rejected command must not have changed anything on its way to failing.
	if !appState.IsScrollInverted() {
		t.Fatal("scroll inverted = false, want the state left untouched by a rejected command")
	}
}

func TestHandleToggleScreenShare_ConvergesOnRequestedState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		initial bool
		args    []string
		want    bool
	}{
		{name: "toggle from visible", initial: false, args: nil, want: true},
		{name: "toggle from hidden", initial: true, args: nil, want: false},
		// "on" is the reported state (hidden), not the visibility.
		{name: "on hides", initial: false, args: []string{stateOnArg}, want: true},
		{
			name:    "on from hidden",
			initial: true,
			args:    []string{flagState, toggleStateOn},
			want:    true,
		},
		{name: "off shows", initial: true, args: []string{stateOffArg}, want: false},
		{name: "off from visible", initial: false, args: []string{stateOffArg}, want: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			appState := state.NewAppState()
			appState.SetHiddenForScreenShare(testCase.initial)

			handler := NewIPCControllerOverlay(appState, zap.NewNop())

			resp := handler.handleToggleScreenShare(
				context.Background(),
				ipc.Command{Args: testCase.args},
			)
			if !resp.Success {
				t.Fatalf("handleToggleScreenShare(%v) = %+v, want success", testCase.args, resp)
			}

			if got := appState.IsHiddenForScreenShare(); got != testCase.want {
				t.Fatalf("hidden for screen share = %t, want %t", got, testCase.want)
			}

			assertToggleData(t, resp, "hidden", testCase.want)
		})
	}
}

// assertToggleData checks the boolean a toggle response reports back, which is
// what a caller reads to confirm the state it asked for.
func assertToggleData(t *testing.T, resp ipc.Response, key string, want bool) {
	t.Helper()

	data, ok := resp.Data.(map[string]bool)
	if !ok {
		t.Fatalf("response data = %T, want map[string]bool", resp.Data)
	}

	if got := data[key]; got != want {
		t.Fatalf("response data[%q] = %t, want %t", key, got, want)
	}
}
