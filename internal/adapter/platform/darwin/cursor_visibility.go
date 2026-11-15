//go:build darwin

package darwin

/*
#include "overlay.h"
*/
import "C"

// HideSystemCursor hides the system cursor.
func HideSystemCursor() {
	C.NeruHideSystemCursor()
}

// ShowSystemCursor shows the system cursor.
func ShowSystemCursor() {
	C.NeruShowSystemCursor()
}

// RehideSystemCursor performs a show and hide pair, so the CGDisplayHideCursor
// count stays at 1. It recovers from Mission Control, Expose, or a workspace
// switch revealing the cursor.
func RehideSystemCursor() {
	C.NeruRehideSystemCursor()
}
