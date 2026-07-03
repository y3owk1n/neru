//go:build windows

package app

// headless always returns false on Windows. Mode and sticky indicator overlays
// are config/theme data holders that do not create native windows, so the
// headless guard is unnecessary.
func (f *ComponentFactory) headless() bool {
	return false
}
