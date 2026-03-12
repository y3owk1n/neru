package ports

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
)

// ElementDiscovery defines the interface for discovering UI elements.
type ElementDiscovery interface {
	// ClickableElements retrieves all clickable UI elements matching the filter.
	ClickableElements(ctx context.Context, filter ElementFilter) ([]*element.Element, error)
}

// ActionExecution defines the interface for executing actions on UI elements.
type ActionExecution interface {
	// PerformAction executes an action on the specified element.
	PerformAction(ctx context.Context, elem *element.Element, actionType action.Type) error

	// PerformActionAtPoint executes an action at the specified point.
	PerformActionAtPoint(ctx context.Context, actionType action.Type, point image.Point) error

	// Scroll performs a scroll action at the current cursor position.
	Scroll(ctx context.Context, deltaX, deltaY int) error
}

// ApplicationInfo defines the interface for getting application information.
type ApplicationInfo interface {
	// FocusedAppBundleID returns the bundle ID of the currently focused application.
	FocusedAppBundleID(ctx context.Context) (string, error)

	// IsAppExcluded checks if the given bundle ID is in the exclusion list.
	IsAppExcluded(ctx context.Context, bundleID string) bool
}

// AccessibilityPort defines the interface for interacting with the platform accessibility API.
// Implementations should handle all CGo/Objective-C/System-specific bridge complexity.
//
// This interface embeds segregated interfaces to reduce duplication and ensure
// method signatures stay synchronized across different concerns.
type AccessibilityPort interface {
	HealthCheck
	ElementDiscovery
	ActionExecution
	ApplicationInfo
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

	// IncludeMenubar includes menubar elements.
	IncludeMenubar bool

	// AdditionalMenubarTargets specifies additional bundle IDs to scan for menubar elements.
	AdditionalMenubarTargets []string

	// IncludeDock includes dock elements.
	IncludeDock bool

	// IncludeNotificationCenter includes notification center elements.
	IncludeNotificationCenter bool

	// IncludeStageManager includes stage manager elements.
	IncludeStageManager bool
}

// DefaultElementFilter returns a filter with sensible defaults.
func DefaultElementFilter() ElementFilter {
	return ElementFilter{
		IncludeOffscreen:          false,
		MinSize:                   image.Point{X: 1, Y: 1},
		IncludeMenubar:            false,
		AdditionalMenubarTargets:  []string{},
		IncludeDock:               false,
		IncludeNotificationCenter: false,
		IncludeStageManager:       false,
	}
}
