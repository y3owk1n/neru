//go:build linux && !cgo

package linux

// SetKeyboardLayout reports that explicit layout selection needs the native
// backend. Empty selects the automatic fallback and needs no native support.
func (et *EventTap) SetKeyboardLayout(layoutID string) bool { return layoutID == "" }
