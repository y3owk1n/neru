//go:build !darwin

package native

import "unsafe"

// MallocCallbackContext returns nil: no C heap exists off darwin. Every
// function here is a deliberate no-op so overlayutil compiles on every
// platform without importing platform/darwin.
func MallocCallbackContext(_, _ uint64) unsafe.Pointer { return nil }

// FreeCallbackContext is a no-op on non-darwin platforms.
func FreeCallbackContext(_ unsafe.Pointer) {}

// FreeCString is a no-op on non-darwin platforms.
func FreeCString(_ unsafe.Pointer) {}
