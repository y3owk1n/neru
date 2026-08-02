package hotkeys

import "github.com/y3owk1n/neru/internal/ports"

// HotkeyID and Callback alias the port types so a hotkey registered through
// ports.HotkeyPort and one registered through this package are the same thing.
//
// These used to be declared separately in each platform manager — three
// identical declarations that the app layer then had to import an adapter to
// name.
type (
	// HotkeyID identifies one registered hotkey.
	HotkeyID = ports.HotkeyID
	// Callback is invoked when a registered hotkey fires.
	Callback = ports.HotkeyCallback
)
