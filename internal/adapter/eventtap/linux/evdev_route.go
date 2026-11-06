//go:build linux

package linux

// The two button blocks in the evdev key-code space: BTN_MISC to KEY_OK, and
// BTN_TRIGGER_HAPPY to the end. The proxy keyboard leaves them out
// (neru_uinput_proxy_is_keyboard_code), since a keyboard advertising them is
// classed as a pointer or joystick, and the pointer proxy carries them.
const (
	evdevBtnMisc         uint16 = 0x100
	evdevBtnLeft         uint16 = 0x110
	evdevBtnTask         uint16 = 0x117
	evdevKeyOk           uint16 = 0x160
	evdevBtnTriggerHappy uint16 = 0x2c0
)

// isPointerButton reports whether a key code is a button, such as a mouse
// button from a remapper's output or a combo receiver. Buttons bypass the
// forward rule, the hotkey matcher and modes, and go to the pointer proxy.
func isPointerButton(code uint16) bool {
	return (code >= evdevBtnMisc && code < evdevKeyOk) || code >= evdevBtnTriggerHappy
}

// isMouseButton reports whether a button is one the pointer proxy advertises,
// BTN_LEFT through BTN_TASK (neru_uinput_create_proxy_pointer). Other buttons
// on a grabbed keyboard are dropped. Joystick and gamepad buttons would make
// udev class the proxy as a joystick, and devices carrying them have absolute
// axes and are never grabbed.
func isMouseButton(code uint16) bool {
	return code >= evdevBtnLeft && code <= evdevBtnTask
}
