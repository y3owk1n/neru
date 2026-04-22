//go:build linux

//nolint:testpackage // These tests validate unexported Linux eventtap helpers directly.
package eventtap

import "testing"

func TestLinuxModifierToggleEventCanonicalizes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		modifier string
		isDown   bool
		want     string
	}{
		{name: "shift down", modifier: "shift", isDown: true, want: "__modifier_shift_down"},
		{name: "control alias up", modifier: "control", isDown: false, want: "__modifier_ctrl_up"},
		{name: "super alias down", modifier: "super", isDown: true, want: "__modifier_cmd_down"},
		{name: "option alias up", modifier: "option", isDown: false, want: "__modifier_alt_up"},
		{name: "unknown", modifier: "fn", isDown: true, want: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := linuxModifierToggleEvent(testCase.modifier, testCase.isDown)
			if got != testCase.want {
				t.Fatalf("linuxModifierToggleEvent(%q, %t) = %q, want %q",
					testCase.modifier,
					testCase.isDown,
					got,
					testCase.want)
			}
		})
	}
}

func TestSyntheticModifierSuppressionConsumesOnce(t *testing.T) {
	t.Parallel()

	eventTap := NewEventTap(nil, nil)
	eventTap.rememberSyntheticModifierEvent("shift", true)

	if !eventTap.consumeSyntheticModifierEvent("shift", true) {
		t.Fatal("expected first matching synthetic event to be consumed")
	}

	if eventTap.consumeSyntheticModifierEvent("shift", true) {
		t.Fatal("expected synthetic event to be consumed only once")
	}
}
