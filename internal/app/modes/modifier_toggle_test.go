//nolint:testpackage // Tests internal function parseModifierEvent
package modes

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/action"
)

func TestParseModifierEvent(t *testing.T) {
	tests := []struct {
		key      string
		wantMod  action.Modifiers
		wantDown bool
		wantOk   bool
	}{
		// Down events
		{"__modifier_shift_down", action.ModShift, true, true},
		{"__modifier_cmd_down", action.ModCmd, true, true},
		{"__modifier_alt_down", action.ModAlt, true, true},
		{"__modifier_ctrl_down", action.ModCtrl, true, true},
		{"__modifier_CMD_down", action.ModCmd, true, true},
		{"__modifier_Shift_down", action.ModShift, true, true},
		// Up events
		{"__modifier_shift_up", action.ModShift, false, true},
		{"__modifier_cmd_up", action.ModCmd, false, true},
		{"__modifier_alt_up", action.ModAlt, false, true},
		{"__modifier_ctrl_up", action.ModCtrl, false, true},
		{"__modifier_CMD_up", action.ModCmd, false, true},
		// Invalid
		{"__modifier_shift", 0, false, false},
		{"__modifier_cmd", 0, false, false},
		{"__modifier_foo_down", 0, false, false},
		{"__modifier_foo_up", 0, false, false},
		{"__modifier", 0, false, false},
		{"shift", 0, false, false},
		{"cmd", 0, false, false},
		{"", 0, false, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.key, func(t *testing.T) {
			mod, isDown, ok := parseModifierEvent(testCase.key)
			if ok != testCase.wantOk {
				t.Errorf(
					"parseModifierEvent(%q) ok = %v, want %v",
					testCase.key,
					ok,
					testCase.wantOk,
				)

				return
			}

			if mod != testCase.wantMod {
				t.Errorf(
					"parseModifierEvent(%q) mod = %v, want %v",
					testCase.key,
					mod,
					testCase.wantMod,
				)
			}

			if isDown != testCase.wantDown {
				t.Errorf(
					"parseModifierEvent(%q) isDown = %v, want %v",
					testCase.key,
					isDown,
					testCase.wantDown,
				)
			}
		})
	}
}
