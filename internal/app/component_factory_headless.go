//go:build darwin || linux

package app

// headless returns true when the overlay manager has no real native window
// handle. Creating an overlay with a nil window pointer would crash on any
// platform call (CGo, X11), so callers must bail out early.
func (f *ComponentFactory) headless() bool {
	return f.overlayManager.WindowPtr() == nil
}
