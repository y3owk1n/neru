//go:build linux && cgo

package linux

import "github.com/y3owk1n/neru/internal/adapter/platform"

// SetKeyboardLayout forces the XKB layout keys are named in, by the name the
// keymap gives it, or returns to the automatic choice for "". Only a backend
// holding a keymap can tell whether the name exists, so it always reports true
// here and the backend warns when the keymap has no such layout.
func (et *EventTap) SetKeyboardLayout(layoutID string) bool {
	requestedKeyboardLayout.Store(&layoutID)

	return true
}

// ListKeyboardLayouts lists the keyboard's layouts and names the one keys
// resolve in, for neru doctor. On Wayland it reports what the capture last
// saw, which is nothing until the capture has read a keymap.
func ListKeyboardLayouts() KeyboardLayouts {
	if platform.DetectLinuxBackend().IsWayland() {
		if layouts := waylandKeyboardLayouts.Load(); layouts != nil {
			return *layouts
		}

		return KeyboardLayouts{}
	}

	return x11KeyboardLayouts()
}
