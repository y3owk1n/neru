//go:build linux

package linux

import "sync/atomic"

// requestedKeyboardLayout is the XKB layout name general.kb_layout_to_use
// forces, or "" for the automatic choice. The event tap and the X11 hotkey
// grab read it, and config reload writes it from another goroutine, so it is
// published whole rather than guarded.
var requestedKeyboardLayout atomic.Pointer[string]

// RequestedKeyboardLayout returns the forced XKB layout name, or "" for the
// automatic choice.
func RequestedKeyboardLayout() string {
	if requested := requestedKeyboardLayout.Load(); requested != nil {
		return *requested
	}

	return ""
}

// KeyboardLayouts lists the layouts of the keyboard in use, spelled as
// general.kb_layout_to_use takes them, and names the one keys resolve in.
type KeyboardLayouts struct {
	Names []string
	// Reference is the layout keys are named in, or "" when that is whichever
	// layout is active.
	Reference string
	// Unmatched is true when general.kb_layout_to_use names a layout the
	// keymap does not have.
	Unmatched bool
}
