//nolint:testpackage // Tests internal function parseModifierToggleKey
package modes

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/action"
)

func TestParseModifierToggleKey(t *testing.T) {
	tests := []struct {
		key     string
		wantMod action.Modifiers
		wantOk  bool
	}{
		{"__modifier_shift", action.ModShift, true},
		{"__modifier_cmd", action.ModCmd, true},
		{"__modifier_alt", action.ModAlt, true},
		{"__modifier_ctrl", action.ModCtrl, true},
		{"__modifier_CMD", action.ModCmd, true},
		{"__modifier_Shift", action.ModShift, true},
		{"__modifier_CMD", action.ModCmd, true},
		{"__modifier_ALT", action.ModAlt, true},
		{"__modifier_CTRL", action.ModCtrl, true},
		{"__modifier_foo", 0, false},
		{"__modifier", 0, false},
		{"shift", 0, false},
		{"cmd", 0, false},
		{"", 0, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.key, func(t *testing.T) {
			mod, ok := parseModifierToggleKey(testCase.key)
			if ok != testCase.wantOk {
				t.Errorf(
					"parseModifierToggleKey(%q) ok = %v, want %v",
					testCase.key,
					ok,
					testCase.wantOk,
				)

				return
			}

			if mod != testCase.wantMod {
				t.Errorf(
					"parseModifierToggleKey(%q) = %v, want %v",
					testCase.key,
					mod,
					testCase.wantMod,
				)
			}
		})
	}
}
