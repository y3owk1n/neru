package ipcctrl

import (
	"context"
	"slices"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain/action"
)

const (
	stepsThree      = "--steps=3"
	resetAction     = "reset"
	backspaceAction = "backspace"
	sampleModKey    = "shift"
)

// TestActionFlagSupport_CoversEveryParsedFlag is the guard that keeps flag
// validation fail-closed: a flag the parser understands but no action declares
// would be accepted everywhere and silently ignored.
func TestActionFlagSupport_CoversEveryParsedFlag(t *testing.T) {
	declared := map[string]bool{}

	for _, flags := range actionFlagSupport {
		for _, flag := range flags {
			declared[flag] = true
		}
	}

	for flag := range actionFlagSpecs {
		if !declared[flag] {
			t.Errorf(
				"flag %q is parsed but no action declares it in actionFlagSupport; "+
					"add it to the action(s) that accept it",
				flag,
			)
		}
	}
}

// TestActionFlagSupport_DeclaresOnlyParsedFlags catches the reverse typo: a
// table entry naming a flag the parser never produces would silently never
// match.
func TestActionFlagSupport_DeclaresOnlyParsedFlags(t *testing.T) {
	for name, flags := range actionFlagSupport {
		for _, flag := range flags {
			if _, ok := actionFlagSpecs[flag]; !ok {
				t.Errorf("action %q declares flag %q, which the parser does not parse", name, flag)
			}
		}
	}
}

// TestActionFlagSupport_NamesAreKnownActions keeps the table from accumulating
// entries for actions that no longer exist.
func TestActionFlagSupport_NamesAreKnownActions(t *testing.T) {
	for name := range actionFlagSupport {
		if !action.IsKnownName(action.Name(name)) {
			t.Errorf("actionFlagSupport has an entry for unknown action %q", name)
		}
	}
}

// TestActionFlagSupport_CoversEveryDispatchableAction lists the actions that
// reach flag parsing. An action missing from actionFlagSupport is not rejected
// (unknown names are left to the dispatcher), so it would accept every flag —
// this test is what forces a new action to declare its flags.
//
// feed and sleep are intentionally absent: both consume their raw arguments
// before flag parsing runs.
func TestActionFlagSupport_CoversEveryDispatchableAction(t *testing.T) {
	dispatchable := []action.Name{
		action.NameLeftClick, action.NameRightClick, action.NameMiddleClick,
		action.NameLeftMouseDown, action.NameLeftMouseUp,
		action.NameRightMouseDown, action.NameRightMouseUp,
		action.NameMiddleMouseDown, action.NameMiddleMouseUp,
		action.NameLeftMouseToggle, action.NameRightMouseToggle,
		action.NameMiddleMouseToggle,
		action.NameMouseDown, action.NameMouseUp, //nolint:staticcheck // still accepted
		action.NameMoveMouse, action.NameMoveMouseRelative, action.NameMoveMonitor,
		action.NameMoveCell,
		action.NameScroll,
		action.NameScrollUp, action.NameScrollDown,
		action.NameScrollLeft, action.NameScrollRight,
		action.NameGoTop, action.NameGoBottom,
		action.NamePageUp, action.NamePageDown,
		action.NameReset, action.NameBackspace,
		action.NameCycleHint, action.NameSearchHints,
		action.NameWaitForModeExit,
		action.NameSaveCursorPos, action.NameRestoreCursorPos,
		action.NameHideCursor, action.NameShowCursor,
	}

	for _, name := range dispatchable {
		if _, ok := actionFlagSupport[string(name)]; !ok {
			t.Errorf("action %q has no actionFlagSupport entry, so it accepts every flag", name)
		}
	}
}

// TestParseActionArgs_MarksEveryFlagPresent proves the parser cannot record a
// flag's value without also marking it present, which is what makes
// rejectUnsupportedFlags see it.
func TestParseActionArgs_MarksEveryFlagPresent(t *testing.T) {
	for flag, spec := range actionFlagSpecs {
		arg := flag
		if spec.takesValue {
			arg = flag + "=" + sampleFlagValue(flag)
		}

		parsed, parseErr := parseActionArgs([]string{arg})
		if parseErr {
			t.Errorf("parseActionArgs(%q) reported a parse error", arg)

			continue
		}

		if !slices.Contains(parsed.presentFlags(), flag) {
			t.Errorf("parseActionArgs(%q) did not mark %q present", arg, flag)
		}
	}
}

// sampleFlagValue returns a value the given flag accepts.
func sampleFlagValue(flag string) string {
	switch flag {
	case flagModifier:
		return sampleModKey
	case flagName:
		return "Display 1"
	case flagState:
		return stateDown
	case flagDirection:
		return dirLeftValue
	default:
		return "1"
	}
}

// TestRejectUnsupportedFlags_RejectsFlagsAnActionDoesNotDeclare walks the whole
// matrix: every action, every flag it does not declare.
func TestRejectUnsupportedFlags_RejectsFlagsAnActionDoesNotDeclare(t *testing.T) {
	for name, allowed := range actionFlagSupport {
		for flag, spec := range actionFlagSpecs {
			if slices.Contains(allowed, flag) {
				continue
			}

			arg := flag
			if spec.takesValue {
				arg = flag + "=" + sampleFlagValue(flag)
			}

			parsed, parseErr := parseActionArgs([]string{arg})
			if parseErr {
				t.Fatalf("parseActionArgs(%q) reported a parse error", arg)
			}

			resp := rejectUnsupportedFlags(name, parsed)
			if resp == nil {
				t.Errorf("action %q accepted undeclared flag %q", name, flag)

				continue
			}

			if resp.Code != ipc.CodeInvalidInput {
				t.Errorf("action %q flag %q: code = %q, want %q",
					name, flag, resp.Code, ipc.CodeInvalidInput)
			}

			if resp.Message == "" {
				t.Errorf("action %q flag %q: empty rejection message", name, flag)
			}
		}
	}
}

// TestRejectUnsupportedFlags_AcceptsDeclaredFlags is the other half: a declared
// flag must pass.
func TestRejectUnsupportedFlags_AcceptsDeclaredFlags(t *testing.T) {
	for name, allowed := range actionFlagSupport {
		for _, flag := range allowed {
			spec := actionFlagSpecs[flag]

			arg := flag
			if spec.takesValue {
				arg = flag + "=" + sampleFlagValue(flag)
			}

			parsed, parseErr := parseActionArgs([]string{arg})
			if parseErr {
				t.Fatalf("parseActionArgs(%q) reported a parse error", arg)
			}

			if resp := rejectUnsupportedFlags(name, parsed); resp != nil {
				t.Errorf("action %q rejected its own flag %q: %s", name, flag, resp.Message)
			}
		}
	}
}

// TestRejectUnsupportedFlags_ChainsIntersect checks that a comma-separated
// chain accepts only the flags every action in it accepts.
func TestRejectUnsupportedFlags_ChainsIntersect(t *testing.T) {
	// --state is a click flag, so it survives a chain of clicks.
	parsed, parseErr := parseActionArgs([]string{flagState + "=" + stateDown})
	if parseErr {
		t.Fatal("parseActionArgs(--state=down) reported a parse error")
	}

	if resp := rejectUnsupportedFlags("left_click,right_click", parsed); resp != nil {
		t.Errorf("chain of clicks rejected --state: %s", resp.Message)
	}

	// left_mouse_down does not take --state, so the intersection drops it.
	if resp := rejectUnsupportedFlags("left_click,left_mouse_down", parsed); resp == nil {
		t.Error("chain containing left_mouse_down accepted --state")
	}
}

// TestRejectUnsupportedFlags_IgnoresUnknownActions leaves an unknown action
// name to the dispatcher, which reports the name rather than its flags.
func TestRejectUnsupportedFlags_IgnoresUnknownActions(t *testing.T) {
	parsed, parseErr := parseActionArgs([]string{"--x=1"})
	if parseErr {
		t.Fatal("parseActionArgs(--x=1) reported a parse error")
	}

	if resp := rejectUnsupportedFlags("not_an_action", parsed); resp != nil {
		t.Errorf("unknown action produced a flag error: %s", resp.Message)
	}
}

// TestHandleAction_RejectsFlagsThatUsedToBeIgnored pins the drift this
// restructure fixes: these flags were silently accepted before, because each
// handler kept its own hand-written reject list.
func TestHandleAction_RejectsFlagsThatUsedToBeIgnored(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "backspace with steps", args: []string{backspaceAction, stepsThree}},
		{name: "backspace with toggle", args: []string{backspaceAction, flagToggle}},
		{name: "backspace with backward", args: []string{backspaceAction, flagBackward}},
		{name: "reset with bail", args: []string{resetAction, flagBail}},
		{name: "reset with direction", args: []string{resetAction, directionLeft}},
		{name: "cycle_hint with count", args: []string{"cycle_hint", "--count=2"}},
		{name: "hide_cursor with steps", args: []string{"hide_cursor", stepsThree}},
		{name: "save_cursor_pos with toggle", args: []string{"save_cursor_pos", flagToggle}},
		{name: "page_up with steps", args: []string{"page_up", stepsThree}},
		{name: "move_mouse with dx", args: []string{moveMouse, "--dx=10", "--dy=10"}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			controller := &ActionsHandler{logger: zap.NewNop()}

			resp := controller.handleAction(context.Background(), ipc.Command{
				Action: ActionCommand,
				Args:   testCase.args,
			})

			if resp.Success {
				t.Fatalf("handleAction(%v) expected rejection, got success", testCase.args)
			}

			if resp.Code != ipc.CodeInvalidInput {
				t.Fatalf("code = %q, want %q", resp.Code, ipc.CodeInvalidInput)
			}
		})
	}
}

// TestRejectUnsupportedFlags_ChainsDoNotMutateTheTable guards the aliasing
// hazard in chain handling: the table's flag slices are shared between actions,
// so narrowing them for a chain must work on a copy.
func TestRejectUnsupportedFlags_ChainsDoNotMutateTheTable(t *testing.T) {
	before := slices.Clone(actionFlagSupport[string(action.NameLeftClick)])

	for range 3 {
		parsed, parseErr := parseActionArgs([]string{flagState + "=" + stateDown})
		if parseErr {
			t.Fatal("parseActionArgs(--state=down) reported a parse error")
		}

		rejectUnsupportedFlags("left_click,left_mouse_down", parsed)
		rejectUnsupportedFlags("left_click,right_click", parsed)
	}

	after := actionFlagSupport[string(action.NameLeftClick)]
	if !slices.Equal(before, after) {
		t.Fatalf("chain validation mutated the shared table: %v -> %v", before, after)
	}

	if !slices.Equal(clickFlags, before) {
		t.Fatalf("chain validation mutated the shared clickFlags slice: %v", clickFlags)
	}
}
