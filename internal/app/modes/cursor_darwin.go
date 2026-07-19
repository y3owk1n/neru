//go:build darwin

package modes

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#include "../../core/infra/platform/darwin/overlay.h"
*/
import "C"

// CursorVisibilitySupported returns true on macOS where system cursor visibility control is available.
func (h *Handler) CursorVisibilitySupported() bool { return true }

func (h *Handler) hideSystemCursorNative() {
	C.NeruHideSystemCursor()
}

func (h *Handler) showSystemCursorNative() {
	C.NeruShowSystemCursor()
}

// RehideSystemCursor performs a show+hide pair so the CGDisplayHideCursor ref
// count stays at 1. Use this to recover from Mission Control, Exposé, or
// workspace switches that reveal the cursor.
func (h *Handler) RehideSystemCursor() {
	C.NeruRehideSystemCursor()
}
