//go:build !darwin

// internal/core/infra/overlay/render/overlayutil/native/native_other.go
// Non-darwin slot: the overlay rendering pipeline does not pass C-heap
// callback contexts here, so every function is an intentional no-op. They
// exist so overlayutil compiles on every platform without importing
// platform/darwin. The package comment lives in doc.go.

package native

import "unsafe"

// MallocCallbackContext is a no-op on non-darwin platforms.
// Returns nil because no C heap is available/needed.
func MallocCallbackContext(_, _ uint64) unsafe.Pointer { return nil }

// FreeCallbackContext is a no-op on non-darwin platforms.
func FreeCallbackContext(_ unsafe.Pointer) {}

// FreeCString is a no-op on non-darwin platforms.
func FreeCString(_ unsafe.Pointer) {}
