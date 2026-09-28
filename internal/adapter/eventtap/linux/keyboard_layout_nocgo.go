//go:build linux && !cgo

package linux

// SetKeyboardLayout reports that forcing a layout needs the native backends,
// which a build without cgo does not have. "" is the automatic choice and
// needs nothing.
func (et *EventTap) SetKeyboardLayout(layoutID string) bool {
	return layoutID == ""
}

// ListKeyboardLayouts reports no layouts, because a build without cgo reads no
// keymap.
func ListKeyboardLayouts() KeyboardLayouts {
	return KeyboardLayouts{}
}
