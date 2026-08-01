package hotkeys

import "github.com/y3owk1n/neru/internal/core/ports"

// HotkeyID and Callback alias the port types so a hotkey registered through
// ports.HotkeyPort and one registered through this package are the same thing.
//
// These used to be declared separately in manager_darwin.go,
// manager_linux_common.go and manager_windows.go — three identical
// declarations that the app layer then had to import infra to name.
type (
	// HotkeyID identifies one registered hotkey.
	HotkeyID = ports.HotkeyID
	// Callback is invoked when a registered hotkey fires.
	Callback = ports.HotkeyCallback
)

// Ensure the platform Manager satisfies the port on every target. The manager
// is used directly — there is no wrapper adapter — so this assertion is what
// keeps the three platform implementations in agreement.
var _ ports.HotkeyPort = (*Manager)(nil)
