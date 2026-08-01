package domain

import (
	"strings"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// Direction identifies one of the four cardinal directions used to slide the
// current selection to a neighboring cell without changing the active layer.
// It is shared by grid and recursive-grid navigation.
type Direction uint8

const (
	// DirectionLeft moves the selection towards smaller X.
	DirectionLeft Direction = iota
	// DirectionRight moves the selection towards larger X.
	DirectionRight
	// DirectionUp moves the selection towards smaller Y.
	DirectionUp
	// DirectionDown moves the selection towards larger Y.
	DirectionDown
)

// String returns the canonical lowercase name of the direction.
func (d Direction) String() string {
	switch d {
	case DirectionLeft:
		return "left"
	case DirectionRight:
		return "right"
	case DirectionUp:
		return "up"
	case DirectionDown:
		return "down"
	default:
		return UnknownDirection
	}
}

// UnknownDirection is the string form of a Direction outside the known set.
const UnknownDirection = "unknown"

// Delta returns the unit step for the direction in screen coordinates
// (global top-left origin, Y growing downward).
func (d Direction) Delta() (int, int) {
	switch d {
	case DirectionLeft:
		return -1, 0
	case DirectionRight:
		return 1, 0
	case DirectionUp:
		return 0, -1
	case DirectionDown:
		return 0, 1
	default:
		return 0, 0
	}
}

// ParseDirection parses a user-supplied direction name. Parsing is
// case-insensitive and tolerant of surrounding whitespace.
func ParseDirection(name string) (Direction, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "left":
		return DirectionLeft, nil
	case "right":
		return DirectionRight, nil
	case "up":
		return DirectionUp, nil
	case "down":
		return DirectionDown, nil
	default:
		return 0, derrors.Newf(
			derrors.CodeInvalidInput,
			"invalid direction %q (expected left, right, up, or down)",
			name,
		)
	}
}
