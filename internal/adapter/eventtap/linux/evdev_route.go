//go:build linux

package linux

// The evdev key-code space has two button blocks inside it: BTN_MISC up to
// KEY_OK, and BTN_TRIGGER_HAPPY to the end. They are what the proxy keyboard
// leaves out (neru_uinput_proxy_is_keyboard_code), since a keyboard advertising
// them is classed as a pointer or a joystick, and what the pointer proxy carries.
const (
	evdevBtnMisc         uint16 = 0x100
	evdevBtnLeft         uint16 = 0x110
	evdevBtnTask         uint16 = 0x117
	evdevKeyOk           uint16 = 0x160
	evdevBtnTriggerHappy uint16 = 0x2c0
)

// isPointerButton reports whether a key code is a button rather than a key: a
// mouse button a remapper's output device presses, or a button on a receiver
// that exposes its mouse and keyboard on one node. Buttons are not keys to the
// forward rule, the hotkey matcher or a mode; they go to the pointer proxy as
// motion does.
func isPointerButton(code uint16) bool {
	return (code >= evdevBtnMisc && code < evdevKeyOk) || code >= evdevBtnTriggerHappy
}

// isMouseButton reports whether a button is one the pointer proxy advertises,
// BTN_LEFT through BTN_TASK (neru_uinput_create_proxy_pointer). Any other
// button on a grabbed keyboard is dropped rather than sent to a device that
// would drop it: the joystick and gamepad blocks would have udev class the
// proxy as a joystick, and a device carrying them has absolute axes and is
// never grabbed.
func isMouseButton(code uint16) bool {
	return code >= evdevBtnLeft && code <= evdevBtnTask
}
