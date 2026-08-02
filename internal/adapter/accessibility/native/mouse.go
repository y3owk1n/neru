package native

// EnsureMouseUp releases any mouse button the daemon left held.
//
// The implementation is per-platform (see element_darwin.go, element_linux.go,
// element_windows.go); this is the one name the adapter calls.
func EnsureMouseUp() { ensureMouseUp() }
