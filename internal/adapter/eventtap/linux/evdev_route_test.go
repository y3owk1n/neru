//go:build linux

package linux

import "testing"

// The split has to agree with the proxy keyboard's advertised codes: a button
// sent there is dropped by the kernel, and a key sent to the pointer proxy is
// dropped the same way.
func TestIsPointerButton_MirrorsTheProxyKeyboardRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		code   uint16
		button bool
	}{
		{name: "letter", code: evdevKeySemicolon, button: false},
		{name: "modifier", code: evdevKeyLeftMeta, button: false},
		{name: "last key before the button block", code: evdevBtnMisc - 1, button: false},
		{name: "BTN_MISC", code: evdevBtnMisc, button: true},
		{name: "BTN_LEFT", code: evdevBtnLeft, button: true},
		{name: "BTN_TASK", code: evdevBtnTask, button: true},
		{name: "last button before KEY_OK", code: evdevKeyOk - 1, button: true},
		{name: "KEY_OK", code: evdevKeyOk, button: false},
		{name: "last key before BTN_TRIGGER_HAPPY", code: evdevBtnTriggerHappy - 1, button: false},
		{name: "BTN_TRIGGER_HAPPY", code: evdevBtnTriggerHappy, button: true},
		{name: "end of the key space", code: evdevKeyCodeCount - 1, button: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isPointerButton(tt.code); got != tt.button {
				t.Errorf("isPointerButton(%#x) = %v, want %v", tt.code, got, tt.button)
			}
		})
	}
}

// The pointer proxy advertises the mouse buttons and nothing else in the button
// blocks, so only those may be sent to it.
func TestIsMouseButton_IsTheRangeThePointerProxyAdvertises(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		code  uint16
		mouse bool
	}{
		{name: "BTN_0", code: evdevBtnMisc, mouse: false},
		{name: "last before BTN_LEFT", code: evdevBtnLeft - 1, mouse: false},
		{name: "BTN_LEFT", code: evdevBtnLeft, mouse: true},
		{name: "BTN_TASK", code: evdevBtnTask, mouse: true},
		{name: "BTN_JOYSTICK", code: evdevBtnTask + 1 + 8, mouse: false},
		{name: "BTN_TRIGGER_HAPPY", code: evdevBtnTriggerHappy, mouse: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isMouseButton(tt.code); got != tt.mouse {
				t.Errorf("isMouseButton(%#x) = %v, want %v", tt.code, got, tt.mouse)
			}
		})
	}
}
