package accessibility

import (
	"image"

	"github.com/y3owk1n/neru/internal/core/domain/action"
)

// AXElement represents a generic accessibility element.
type AXElement interface {
	Release()
}

// AXWindow represents a window element.
type AXWindow interface {
	AXElement
	Role() string
}

// AXClient defines the interface for accessibility operations.
type AXClient interface {
	// Window and App operations
	FrontmostWindow() (AXWindow, error)
	AllWindows() ([]AXWindow, error)
	FocusedApplication() (AXApp, error)
	ApplicationByBundleID(bundleID string) (AXApp, error)
	ClickableNodes(root AXElement, includeOffscreen bool, roles []string) ([]AXNode, error)
	MenuBarClickableElements(strictFiltering bool) ([]AXNode, error)
	ClickableElementsFromBundleID(
		bundleID string,
		roles []string,
		strictFiltering bool,
	) ([]AXNode, error)
	ActiveScreenBounds() image.Rectangle

	// Actions
	PerformAction(
		actionType action.Type,
		p image.Point,
		restoreCursor bool,
		modifiers action.Modifiers,
	) error
	Scroll(deltaX, deltaY int) error
	MoveMouse(p image.Point, bypassSmooth bool)
	CursorPosition() image.Point

	// System
	CheckPermissions() bool
	SetClickableRoles(roles []string)
	ClickableRoles() []string
	IsMissionControlActive() bool

	// Cache
	ClearCache()
}

// AXAppInfo contains information about an application.
type AXAppInfo struct {
	Role  string
	Title string
}

// AXApp represents an application element.
type AXApp interface {
	AXElement
	BundleIdentifier() string
	Info() (*AXAppInfo, error)
}

// AXNode represents a node in the accessibility tree.
type AXNode interface {
	ID() string
	Bounds() image.Rectangle
	Role() string
	Title() string
	Description() string
	IsClickable() bool
	Release()
}
