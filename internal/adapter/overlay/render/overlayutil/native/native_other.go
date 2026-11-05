//go:build !darwin

package native

import "unsafe"

// Malloc returns nil, because no C heap exists off darwin. Every function here
// is a deliberate no-op so overlayutil compiles on every platform without
// importing platform/darwin.
func Malloc(_ uintptr) unsafe.Pointer { return nil }

// Free is a no-op on non-darwin platforms.
func Free(_ unsafe.Pointer) {}

// FreeCString is a no-op on non-darwin platforms.
func FreeCString(_ unsafe.Pointer) {}
