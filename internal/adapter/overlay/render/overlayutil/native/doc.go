// Package native provides platform-native memory helpers for overlay
// operations.
//
// On macOS the overlay pipeline passes callback context pointers through cgo to
// Objective-C that runs asynchronously. Go's GC may move or collect heap
// objects, so those pointers must live on the C heap; this package owns their
// malloc/free lifecycle. Everywhere else the functions are no-ops
// (native_other.go).
//
// This file carries no build tag, so the package comment is present on every
// target rather than only on macOS.
package native
