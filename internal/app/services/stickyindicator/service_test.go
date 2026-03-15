package stickyindicator_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/app/services/stickyindicator"
	"github.com/y3owk1n/neru/internal/core/domain/action"
)

func TestModifierSymbolsString(t *testing.T) {
	tests := []struct {
		name string
		mods action.Modifiers
		want string
	}{
		{"none", 0, ""},
		{"cmd", action.ModCmd, "⌘"},
		{"shift", action.ModShift, "⇧"},
		{"alt", action.ModAlt, "⌥"},
		{"ctrl", action.ModCtrl, "⌃"},
		{"cmd+shift", action.ModCmd | action.ModShift, "⌘⇧"},
		{"all", action.ModCmd | action.ModShift | action.ModAlt | action.ModCtrl, "⌘⇧⌥⌃"},
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
