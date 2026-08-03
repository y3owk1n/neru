package cli

import (
	"slices"
	"strings"
	"testing"
)

// The mode name, action and values these cases repeat.
const (
	modeHints    = "hints"
	modeGrid     = "grid"
	actLeftClick = "left_click"
	modCmd       = "cmd"
	stepScroll   = "scroll"
	cursorHold   = "hold"
)

// ipcArgs turns a mode command's flags into the request the daemon receives, so
// these cases are the contract between the two halves of every mode command.
// Until it was a function of its own the assembly ended in a socket write and
// could not be checked at all.

// hintsMode is a mode that declares every optional flag, so a case can set any
// of them.
func hintsMode() ModeConfig {
	return ModeConfig{
		Name:                     modeHints,
		SupportSearch:            true,
		SupportHideOnEmptySearch: true,
		SupportFiltering:         true,
		SupportStrategy:          true,
		SupportLabelDirection:    true,
		SupportDebug:             true,
		SupportSplitWord:         true,
		SupportZoomToDepth:       true,
	}
}

func TestIPCArgs_LeadsWithTheModeName(t *testing.T) {
	args := modeFlags{}.ipcArgs(hintsMode())

	if len(args) == 0 || args[0] != modeHints {
		t.Fatalf("args = %v, want the mode name first", args)
	}
}

// TestIPCArgs_OmitsUnsetFlags pins that an unset flag contributes nothing. The
// daemon distinguishes absent from empty, so sending "--role=" would not mean
// the same as sending nothing.
func TestIPCArgs_OmitsUnsetFlags(t *testing.T) {
	args := modeFlags{}.ipcArgs(hintsMode())

	if len(args) != 1 {
		t.Errorf("args = %v, want only the mode name", args)
	}
}

func TestIPCArgs_CarriesEachFlag(t *testing.T) {
	tests := []struct {
		name  string
		flags modeFlags
		want  string
	}{
		{"action", modeFlags{action: actLeftClick}, actLeftClick},
		{
			"modifier",
			modeFlags{action: actLeftClick, modifier: modCmd},
			"--modifier=cmd",
		},
		{"repeat", modeFlags{action: actLeftClick, repeat: true}, "--repeat"},
		{"toggle", modeFlags{toggle: true}, "--toggle"},
		{"search", modeFlags{search: true}, "--search"},
		{
			"hide on empty search",
			modeFlags{search: true, hideOnEmptySearch: true},
			"--hide-on-empty-search",
		},
		{"role", modeFlags{role: "AXButton"}, "--role=AXButton"},
		{"text", modeFlags{text: "OK"}, "--text=OK"},
		{"strategy", modeFlags{strategy: "vision"}, "--strategy=vision"},
		{"label direction", modeFlags{labelDirection: "reverse"}, "--label-direction=reverse"},
		{"split word", modeFlags{splitWord: true}, "--split-word"},
		{"zoom to depth", modeFlags{zoomToDepth: 3}, "--zoom-to-depth=3"},
		{
			"cursor selection mode",
			modeFlags{cursorSelectionMode: cursorHold},
			"--cursor-selection-mode=" + cursorHold,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			args := testCase.flags.ipcArgs(hintsMode())

			if !slices.Contains(args, testCase.want) {
				t.Errorf("args = %v, want to contain %q", args, testCase.want)
			}
		})
	}
}

// TestIPCArgs_RepeatsOnExitPerStep pins that each --on-exit step travels as its
// own argument rather than being joined, since a step may contain anything.
func TestIPCArgs_RepeatsOnExitPerStep(t *testing.T) {
	args := modeFlags{
		action:      actLeftClick,
		onExitSteps: []string{stepScroll, "exec echo hi"},
	}.ipcArgs(hintsMode())

	count := 0

	for _, arg := range args {
		if strings.HasPrefix(arg, "--on-exit=") {
			count++
		}
	}

	if count != 2 {
		t.Errorf("args = %v, want two --on-exit arguments", args)
	}
}

// TestIPCArgs_RespectsModeSupport pins that a flag a mode does not declare is not
// sent even when the value is set, which is what stops one mode's flags leaking
// into another's request.
func TestIPCArgs_RespectsModeSupport(t *testing.T) {
	plain := ModeConfig{Name: modeGrid}

	args := modeFlags{zoomToDepth: 3}.ipcArgs(plain)

	for _, arg := range args {
		if strings.HasPrefix(arg, "--zoom-to-depth") {
			t.Errorf("args = %v, want no --zoom-to-depth for a mode that does not support it", args)
		}
	}
}

// TestModeFlags_ValidateEnforcesFlagDependencies pins the rules the CLI checks before
// contacting the daemon, so a mistyped command fails immediately.
func TestModeFlags_ValidateEnforcesFlagDependencies(t *testing.T) {
	tests := []struct {
		name  string
		flags modeFlags
	}{
		{"repeat without action", modeFlags{repeat: true}},
		{"on-exit without action", modeFlags{onExitSteps: []string{stepScroll}}},
		{"hide-on-empty-search without search", modeFlags{hideOnEmptySearch: true}},
		{"modifier without action", modeFlags{modifier: modCmd}},
		{"unknown cursor selection mode", modeFlags{cursorSelectionMode: "sometimes"}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.flags.validate()
			if err == nil {
				t.Error("accepted; want a refusal before the daemon is contacted")
			}
		})
	}
}

// TestModeFlags_ValidateAcceptsAWellFormedCommand guards against the checks refusing
// everything, which the cases above would not catch on their own.
func TestModeFlags_ValidateAcceptsAWellFormedCommand(t *testing.T) {
	flags := modeFlags{
		action:              actLeftClick,
		modifier:            modCmd,
		repeat:              true,
		search:              true,
		hideOnEmptySearch:   true,
		cursorSelectionMode: cursorHold,
		onExitSteps:         []string{stepScroll},
	}

	err := flags.validate()
	if err != nil {
		t.Errorf("a well-formed command was refused: %v", err)
	}
}
