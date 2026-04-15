package stickyindicator_test

import (
	"runtime"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	"github.com/y3owk1n/neru/internal/core/domain/action"
)

func TestModifierSymbolsString(t *testing.T) {
	// The display symbol for ModCmd is platform-dependent:
	// "⌘" on macOS, "❖" on Linux.
	cmdSym := "❖"
	if runtime.GOOS == "darwin" {
		cmdSym = "⌘"
	}

	tests := []struct {
		name string
		mods action.Modifiers
		want string
	}{
		{"none", 0, ""},
		{"cmd", action.ModCmd, cmdSym},
		{"shift", action.ModShift, "⇧"},
		{"alt", action.ModAlt, "⌥"},
		{"ctrl", action.ModCtrl, "⌃"},
		{"cmd+shift", action.ModCmd | action.ModShift, cmdSym + "⇧"},
		{"all", action.ModCmd | action.ModShift | action.ModAlt | action.ModCtrl, cmdSym + "⇧⌥⌃"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := stickyindicator.ModifierSymbolsString(testCase.mods)
			if got != testCase.want {
				t.Errorf(
					"ModifierSymbolsString(%v) = %q, want %q",
					testCase.mods,
					got,
					testCase.want,
				)
			}
		})
	}
}
