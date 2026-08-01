//go:build !darwin

// Package native provides platform-native memory helpers for overlay operations.
//
// On non-darwin platforms the overlay rendering pipeline is not active, so
// all functions here are intentional no-ops. They exist so that overlayutil
// compiles on every platform without importing platform/darwin.
package native

import "unsafe"

// MallocCallbackContext is a no-op on non-darwin platforms.
// Returns nil because no C heap is available/needed.
func MallocCallbackContext(_, _ uint64) unsafe.Pointer { return nil }

// FreeCallbackContext is a no-op on non-darwin platforms.
func FreeCallbackContext(_ unsafe.Pointer) {}

// FreeCString is a no-op on non-darwin platforms.
func FreeCString(_ unsafe.Pointer) {}
