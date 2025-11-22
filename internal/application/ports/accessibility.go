package ports

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// AccessibilityPort defines the interface for interacting with the macOS accessibility API.
// Implementations should handle all CGo/Objective-C bridge complexity.
type AccessibilityPort interface {
	// GetClickableElements retrieves all clickable UI elements matching the filter.
	GetClickableElements(ctx context.Context, filter ElementFilter) ([]*element.Element, error)

	// GetScrollableElements retrieves all scrollable UI elements.
	GetScrollableElements(ctx context.Context) ([]*element.Element, error)

	// PerformAction executes an action on the specified element.
	PerformAction(ctx context.Context, elem *element.Element, actionType action.Type) error

	// PerformActionAtPoint executes an action at the specified point.
	PerformActionAtPoint(ctx context.Context, actionType action.Type, point image.Point) error

	// Scroll performs a scroll action at the current cursor position.
	Scroll(ctx context.Context, deltaX, deltaY int) error

	// GetFocusedAppBundleID returns the bundle ID of the currently focused application.
	GetFocusedAppBundleID(ctx context.Context) (string, error)

	// IsAppExcluded checks if the given bundle ID is in the exclusion list.
	IsAppExcluded(ctx context.Context, bundleID string) bool

	// GetScreenBounds returns the bounds of the active screen.
	GetScreenBounds(ctx context.Context) (image.Rectangle, error)

	// MoveCursorToPoint moves the mouse cursor to the specified point.
	MoveCursorToPoint(ctx context.Context, point image.Point) error

	// GetCursorPosition returns the current cursor position.
	GetCursorPosition(ctx context.Context) (image.Point, error)

	// CheckPermissions verifies that accessibility permissions are granted.
	CheckPermissions(ctx context.Context) error
}

// ElementFilter defines criteria for filtering UI elements.
type ElementFilter struct {
	// Roles specifies which accessibility roles to include.
	Roles []element.Role

	// IncludeOffscreen includes elements outside the visible screen area.
	IncludeOffscreen bool

	// MinSize specifies the minimum element size to include.
	MinSize image.Point

	// ExcludeRoles specifies roles to exclude.
	ExcludeRoles []element.Role
}

// DefaultElementFilter returns a filter with sensible defaults.
func DefaultElementFilter() ElementFilter {
	return ElementFilter{
		IncludeOffscreen: false,
		MinSize:          image.Point{X: 1, Y: 1},
	}
}
