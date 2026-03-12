//go:build darwin

// Package native provides platform-native memory helpers for overlay operations.
//
// On macOS, the overlay rendering pipeline passes callback context pointers
// through CGo to Objective-C code that runs asynchronously. Because Go's GC
// can move or collect heap objects, these pointers must live on the C heap.
// This package owns the malloc/free lifecycle for those C-heap allocations.
//
// On non-darwin platforms all functions are no-ops (see native_stub.go).
package native

import "github.com/y3owk1n/neru/internal/core/infra/platform/darwin"

import "unsafe"

// MallocCallbackContext allocates a CallbackContext struct on the C heap.
// The returned pointer is safe for C code to retain across async dispatch
// boundaries because it lives outside the Go GC's reach.
// Caller must call FreeCallbackContext exactly once when done.
func MallocCallbackContext(callbackID, generation uint64) unsafe.Pointer {
	return darwin.MallocCallbackContext(callbackID, generation)
}

// FreeCallbackContext releases a C-heap CallbackContext previously allocated
// by MallocCallbackContext. Safe to call with nil.
func FreeCallbackContext(ptr unsafe.Pointer) {
	darwin.FreeCallbackContext(ptr)
}

// FreeCString releases a C string previously allocated by C.CString.
// Safe to call with nil.
func FreeCString(ptr unsafe.Pointer) {
	darwin.FreeCString(ptr)
}
