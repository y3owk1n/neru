//go:build darwin

package darwin

/*
#include "accessibility.h"
*/
import "C"

import (
	"image"

	"github.com/y3owk1n/neru/internal/adapter/platform/mousestate"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/geometry"
)

// heldButtons records which mouse buttons Neru is currently holding down.
var heldButtons mousestate.Tracker

// cgButtonEvents maps a domain mouse button onto the CGEvent types and button
// number that address it.
type cgButtonEvents struct {
	down    C.CGEventType
	up      C.CGEventType
	dragged C.CGEventType
	button  C.CGMouseButton
}

// eventsForButton returns the CGEvent types addressing the given button.
func eventsForButton(button action.MouseButton) cgButtonEvents {
	switch button {
	case action.ButtonRight:
		return cgButtonEvents{
			down:    C.kCGEventRightMouseDown,
			up:      C.kCGEventRightMouseUp,
			dragged: C.kCGEventRightMouseDragged,
			button:  C.kCGMouseButtonRight,
		}
	case action.ButtonMiddle:
		return cgButtonEvents{
			down:    C.kCGEventOtherMouseDown,
			up:      C.kCGEventOtherMouseUp,
			dragged: C.kCGEventOtherMouseDragged,
			button:  C.kCGMouseButtonCenter,
		}
	case action.ButtonLeft:
		fallthrough
	default:
		return cgButtonEvents{
			down:    C.kCGEventLeftMouseDown,
			up:      C.kCGEventLeftMouseUp,
			dragged: C.kCGEventLeftMouseDragged,
			button:  C.kCGMouseButtonLeft,
		}
	}
}

// IsMouseButtonDown returns whether the given mouse button is down.
func IsMouseButtonDown(button action.MouseButton) bool {
	return heldButtons.IsDown(button)
}

// EnsureMouseUp releases every mouse button Neru is currently holding down.
//
// MouseUp owns the bookkeeping: it clears a button only once the release has
// actually been posted. A button whose release fails therefore stays recorded as
// held, so the next release retries it, rather than being forgotten while the
// window server still holds it — which would leave a toggle pressing it again.
func EnsureMouseUp() {
	for _, button := range heldButtons.HeldButtons() {
		_ = MouseUp(button)
	}
}

// dragEventType returns the event type a cursor move should be posted with, and
// the button it belongs to. Moving while a button is held has to be posted as a
// drag of that button or the application never sees the drag. When several
// buttons are held the left-most held button wins; a single move event cannot
// describe more than one.
func dragEventType() (C.CGEventType, C.CGMouseButton) {
	held := heldButtons.HeldButtons()
	if len(held) == 0 {
		return C.kCGEventMouseMoved, C.kCGMouseButtonLeft
	}

	events := eventsForButton(held[0])

	return events.dragged, events.button
}

// MoveMouse moves the mouse cursor to the specified point.
// If bypassSmooth is true, smooth cursor configuration is bypassed.
func MoveMouse(point image.Point, bypassSmooth bool) {
	eventType, button := dragEventType()

	cfg := currentConfig()
	if cfg != nil && cfg.SmoothCursor.MoveMouseEnabled && !bypassSmooth {
		MoveMouseSmooth(point, cfg.SmoothCursor.Steps, uint32(eventType), uint32(button))
	} else {
		cursorAnimator.stop()
		pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
		C.NeruMoveMouseWithTypeAndButton(pos, eventType, button)
	}
}

// PostMouseMove posts one absolute cursor move (or drag, while a button is
// held) and returns at once. Unlike MoveMouse it neither animates nor spins
// the run loop, so a fixed-rate motion loop can call it every tick.
func PostMouseMove(point image.Point) {
	cursorAnimator.stop()

	eventType, button := dragEventType()
	postCursorMoveEvent(point, uint32(eventType), uint32(button))
}

// MoveMouseSmooth moves the mouse cursor smoothly to the specified point.
func MoveMouseSmooth(end image.Point, steps int, eventType, button uint32) {
	cursorAnimator.animateTo(end, steps, eventType, button)
}

// MoveMouseRelativeSmooth animates a relative cursor move with the fixed
// per-move duration smooth_cursor.relative_movement_duration. It reports handled == false
// when smooth cursor is disabled or no config is wired, in which case the
// caller falls back to its instant warp path.
//
// While an animation is in flight, the delta extends the pending endpoint
// instead of restarting from the mid-animation cursor position, so no part of
// a delta is lost under key repeat. The target is clamped to the active
// screen, matching the warp fallback's clamping — and keeping the pending
// endpoint from drifting off-screen when a key is held at a screen edge.
func MoveMouseRelativeSmooth(delta image.Point) bool {
	cfg := currentConfig()
	if cfg == nil || !cfg.SmoothCursor.MoveMouseEnabled {
		return false
	}

	bounds := ActiveScreenBounds()
	eventType, button := dragEventType()
	cursorAnimator.animateRelativeBy(
		delta,
		func(p image.Point) image.Point {
			return image.Point{
				X: geometry.ClampInt(p.X, bounds.Min.X, max(bounds.Max.X-1, bounds.Min.X)),
				Y: geometry.ClampInt(p.Y, bounds.Min.Y, max(bounds.Max.Y-1, bounds.Min.Y)),
			}
		},
		cfg.SmoothCursor.Steps,
		cfg.SmoothCursor.RelativeMovementDuration,
		uint32(eventType),
		uint32(button),
	)

	return true
}

// CursorPosition returns the current cursor position.
func CursorPosition() image.Point {
	pos := C.NeruGetCurrentCursorPosition()

	return image.Point{X: int(pos.x), Y: int(pos.y)}
}

// LeftClickAtPoint performs a left mouse click at the specified point.
func LeftClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	cursorAnimator.stop()

	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	result := C.NeruPerformLeftClickAtPosition(
		pos,
		C.bool(restoreCursor),
		modifiersToCGEventFlags(modifiers),
	)
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform left-click at position (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

// RightClickAtPoint performs a right mouse click at the specified point.
func RightClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	cursorAnimator.stop()

	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	result := C.NeruPerformRightClickAtPosition(
		pos,
		C.bool(restoreCursor),
		modifiersToCGEventFlags(modifiers),
	)
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform right-click at position (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

// MiddleClickAtPoint performs a middle mouse click at the specified point.
func MiddleClickAtPoint(point image.Point, restoreCursor bool, modifiers action.Modifiers) error {
	cursorAnimator.stop()

	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}
	result := C.NeruPerformMiddleClickAtPosition(
		pos,
		C.bool(restoreCursor),
		modifiersToCGEventFlags(modifiers),
	)
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform middle-click at position (%d, %d)",
			point.X,
			point.Y,
		)
	}

	return nil
}

// MouseDownAtPoint presses and holds the given mouse button at the specified point.
func MouseDownAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	cursorAnimator.stop()

	// Recorded before posting so that a move racing the press is still posted
	// as a drag; the state is rolled back when the press fails.
	heldButtons.SetDown(button, point, modifiers)

	events := eventsForButton(button)
	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}

	result := C.NeruPerformMouseDownAtPosition(
		pos,
		events.down,
		events.button,
		modifiersToCGEventFlags(modifiers),
	)
	if result == 0 {
		heldButtons.Clear(button)

		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform %s-mouse-down at position (%d, %d)",
			button,
			point.X,
			point.Y,
		)
	}

	return nil
}

// MouseUpAtPoint releases the given mouse button at the specified point.
func MouseUpAtPoint(
	point image.Point,
	button action.MouseButton,
	modifiers action.Modifiers,
) error {
	cursorAnimator.stop()

	events := eventsForButton(button)
	pos := C.CGPoint{x: C.double(point.X), y: C.double(point.Y)}

	result := C.NeruPerformMouseUpAtPosition(
		pos,
		events.up,
		events.button,
		modifiersToCGEventFlags(modifiers),
	)
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform %s-mouse-up at position (%d, %d)",
			button,
			point.X,
			point.Y,
		)
	}

	heldButtons.Clear(button)

	return nil
}

// MouseUp releases the given mouse button at the current cursor position.
func MouseUp(button action.MouseButton) error {
	cursorAnimator.stop()

	events := eventsForButton(button)

	result := C.NeruPerformMouseUpAtCursor(events.up, events.button)
	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to perform %s-mouse-up at cursor",
			button,
		)
	}

	heldButtons.Clear(button)

	return nil
}

// ScrollAtCursor performs a scroll action at the current cursor position,
// stamping modifiers onto the event so a ctrl-modified scroll zooms rather than
// pans. Under smooth scroll the set travels on the animation request, because
// every chunk the animator posts has to carry it — a zoom that applied only to
// the first frame would read as a broken binding.
func ScrollAtCursor(deltaX, deltaY int, modifiers action.Modifiers) error {
	cfg := currentConfig()
	if cfg != nil && cfg.SmoothScroll.Enabled {
		scrollAnim.animate(
			deltaX,
			deltaY,
			modifiers,
			cfg.SmoothScroll.Steps,
			cfg.SmoothScroll.MaxDuration,
			cfg.SmoothScroll.DurationPerPixel,
		)

		return nil
	}

	scrollAnim.stop()

	pos := CursorPosition()
	cgPos := C.CGPoint{x: C.double(pos.X), y: C.double(pos.Y)}
	result := C.NeruScrollAtPoint(
		cgPos,
		C.int(deltaX),
		C.int(deltaY),
		modifiersToCGEventFlags(modifiers),
	)

	if result == 0 {
		return derrors.Newf(
			derrors.CodeActionFailed,
			"failed to scroll at cursor with delta (%d, %d)",
			deltaX,
			deltaY,
		)
	}

	return nil
}

// modifiersToCGEventFlags converts domain Modifiers to macOS CGEventFlags.
func modifiersToCGEventFlags(mods action.Modifiers) C.CGEventFlags {
	var flags C.CGEventFlags
	if mods.Has(action.ModCmd) {
		flags |= C.kCGEventFlagMaskCommand
	}
	if mods.Has(action.ModShift) {
		flags |= C.kCGEventFlagMaskShift
	}
	if mods.Has(action.ModAlt) {
		flags |= C.kCGEventFlagMaskAlternate
	}
	if mods.Has(action.ModCtrl) {
		flags |= C.kCGEventFlagMaskControl
	}

	return flags
}
