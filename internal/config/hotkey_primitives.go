package config

import "strings"

const (
	// HotkeyPrimitiveReset resets current mode state.
	HotkeyPrimitiveReset = "reset"
	// HotkeyPrimitiveBackspace performs a mode-aware backspace operation.
	HotkeyPrimitiveBackspace = "backspace"
	// HotkeyPrimitiveWaitForModeExit blocks execution until the app mode returns to idle.
	HotkeyPrimitiveWaitForModeExit = "wait_for_mode_exit"
	// HotkeyPrimitiveSaveCursorPos stores the current cursor position for later restore.
	HotkeyPrimitiveSaveCursorPos = "save_cursor_pos"
	// HotkeyPrimitiveRestoreCursor restores cursor position saved by save_cursor_pos.
	HotkeyPrimitiveRestoreCursor = "restore_cursor"
)

// IsHotkeyPrimitive reports whether actionStr is a supported hotkey primitive.
func IsHotkeyPrimitive(actionStr string) bool {
	switch strings.TrimSpace(actionStr) {
	case HotkeyPrimitiveReset,
		HotkeyPrimitiveBackspace,
		HotkeyPrimitiveWaitForModeExit,
		HotkeyPrimitiveSaveCursorPos,
		HotkeyPrimitiveRestoreCursor:
		return true
	default:
		return false
	}
}
