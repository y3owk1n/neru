//go:build darwin

// Package space provides functions for focusing Mission Control spaces
// on macOS using the synthetic high-velocity horizontal dock swipe gesture
// technique.
//
// macOS does not expose a public API to directly activate a Mission Control
// space, so the implementation synthesizes a series of fast horizontal
// swipes (Began -> Ended, repeated) that the Dock treats as a real
// multi-finger gesture and uses to fast-forward to the destination space
// without the standard animation.
package space

/*
#cgo CFLAGS: -x objective-c
#include "../platform/darwin/accessibility.h"
*/
import "C"

import (
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	_ "github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

// FocusByIndex focuses the Mission Control space at the given 1-based
// index. Index 1 is the first space (typically the leftmost on the primary
// display), index 2 the second, and so on. Spaces are enumerated in
// Mission Control display order across all connected displays.
//
// The function refuses to switch spaces while Mission Control is active
// (so we don't fight the user's swipe). When the destination is on a
// different display, the cursor is warped to its center first so the
// synthetic gesture is attributed to the correct screen.
func FocusByIndex(index int) error {
	count := int(C.NeruCountMissionControlSpaces())
	if count == 0 {
		return derrors.New(derrors.CodeActionFailed, "failed to enumerate Mission Control spaces")
	}

	if index < 1 || index > count {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"space number %d is out of range; valid range is 1..%d",
			index,
			count,
		)
	}

	sid := uint64(C.NeruMissionControlSpaceID(C.int(index)))
	if sid == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to resolve Mission Control space at index %d",
			index,
		)
	}

	did := uint32(C.NeruSpaceDisplayID(C.uint64_t(sid)))
	if did == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to resolve display for Mission Control space at index %d",
			index,
		)
	}

	if C.NeruFocusSpaceUsingGesture(C.uint32_t(did), C.uint64_t(sid)) == 0 {
		return derrors.New(derrors.CodeActionFailed, "failed to focus Mission Control space")
	}

	return nil
}

// Count returns the total number of Mission Control spaces available
// across all connected displays, in Mission Control ordering. This is
// the inclusive upper bound of valid 1-based space numbers for
// FocusByIndex. Returns 0 if the spaces cannot be enumerated.
func Count() int {
	return int(C.NeruCountMissionControlSpaces())
}

// MoveWindowToSpaceByIndex moves the current focused window to the Mission Control space
// at the given 1-based index.
func MoveWindowToSpaceByIndex(index int) error {
	count := int(C.NeruCountMissionControlSpaces())
	if count == 0 {
		return derrors.New(derrors.CodeActionFailed, "failed to enumerate Mission Control spaces")
	}

	if index < 1 || index > count {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"space number %d is out of range; valid range is 1..%d",
			index,
			count,
		)
	}

	sid := uint64(C.NeruMissionControlSpaceID(C.int(index)))
	if sid == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to resolve Mission Control space at index %d",
			index,
		)
	}

	frontmost := C.NeruGetFrontmostWindow()
	if frontmost == nil {
		return derrors.New(
			derrors.CodeActionFailed,
			"no active window found to move",
		)
	}

	defer C.NeruReleaseElement(frontmost) //nolint:nlreturn

	if C.NeruMoveWindowToSpace(frontmost, C.uint64_t(sid)) == 0 { //nolint:nlreturn
		return derrors.New(derrors.CodeActionFailed, "failed to move window to space")
	}

	return nil
}
