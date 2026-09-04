// Package keyfeed posts keyboard input directly to the host operating system.
//
// It backs the `neru key` action, which types into the focused application
// rather than driving Neru's own modes. Key strings arrive in the canonical
// form produced by config.CanonicalHotkeyForPlatform ("a", "Return", "Ctrl+c")
// and are normalized once in shared code before reaching a platform slot.
//
// Platform slots: keyfeed_darwin.go (CGEventPost), keyfeed_linux.go (a uinput
// virtual keyboard where /dev/uinput is writable, else zwp_virtual_keyboard_v1
// on wlroots), keyfeed_windows.go (SendInput), and keyfeed_other.go, which
// returns CodeNotSupported.
package keyfeed
