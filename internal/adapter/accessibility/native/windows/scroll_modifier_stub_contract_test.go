//go:build windows

package windows_test

import (
	"testing"

	nativewindows "github.com/y3owk1n/neru/internal/adapter/accessibility/native/windows"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// TestScrollAtCursor_RefusesModifiersOnHorizontalScroll pins the loudness half
// of the scroll modifier contract on Windows. Wheel injection here has no
// horizontal axis, so a horizontal delta is dropped; answering nil would drop
// the modifier with it and still report success, and a user who asked for
// ctrl + scroll_left would be told a zoom happened that never did.
//
// A vertical scroll is the honored path and must stay silent about it.
func TestScrollAtCursor_RefusesModifiersOnHorizontalScroll(t *testing.T) {
	err := nativewindows.ScrollAtCursor(-50, 0, action.ModCtrl)
	if err == nil {
		t.Fatal(
			"horizontal ScrollAtCursor with ctrl returned nil; a dropped modifier must be reported",
		)
	}

	if !derrors.IsNotSupported(err) {
		t.Fatalf(
			"horizontal ScrollAtCursor with ctrl returned %v, want a CodeNotSupported error",
			err,
		)
	}

	// Unmodified horizontal scroll keeps its long-documented silent no-op:
	// nothing about this contract gave it an axis to travel on.
	unmodified := nativewindows.ScrollAtCursor(-50, 0, 0)
	if unmodified != nil {
		t.Fatalf("unmodified horizontal ScrollAtCursor returned %v, want nil", unmodified)
	}
}
