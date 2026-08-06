//go:build darwin

package modes

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#include "../../adapter/platform/darwin/overlay.h"
*/
import "C"

// The header above only declares these symbols; the Objective-C that defines
// them is compiled by the darwin platform package. This package used to reach
// it by accident, through the overlay adapter it imported for the Linux
// keyboard-capture extension — since #1213 that import is gone, so the link
// edge is stated here instead of inherited.
import _ "github.com/y3owk1n/neru/internal/adapter/platform/darwin"

// CursorVisibilitySupported returns true on macOS where system cursor visibility control is available.
func (h *Handler) CursorVisibilitySupported() bool { return true }

func (h *handlerState) hideSystemCursorNative() {
	C.NeruHideSystemCursor()
}

func (h *handlerState) showSystemCursorNative() {
	C.NeruShowSystemCursor()
}

// RehideSystemCursor performs a show+hide pair so the CGDisplayHideCursor ref
// count stays at 1. Use this to recover from Mission Control, Exposé, or
// workspace switches that reveal the cursor.
func (h *Handler) RehideSystemCursor() {
	C.NeruRehideSystemCursor()
}
