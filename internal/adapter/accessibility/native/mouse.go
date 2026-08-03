package native

// EnsureMouseUp releases any mouse button the daemon left held.
//
// The implementation is per-platform (see darwin/element.go, linux/element.go,
// element_windows.go); this is the one name the adapter calls.
func EnsureMouseUp() { ensureMouseUp() }
