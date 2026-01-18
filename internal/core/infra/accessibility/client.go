package accessibility

import (
	"image"

	"github.com/y3owk1n/neru/internal/core/domain/action"
)

// AXElement represents a generic accessibility element.
//
//nolint:iface // Intentionally small interface for future extension
type AXElement interface {
	Release()
}

// AXClient defines the interface for accessibility operations.
//
//nolint:interfacebloat // Facade interface for accessibility operations
type AXClient interface {
	// Window and App operations
	FrontmostWindow() (AXWindow, error)
	FocusedApplication() (AXApp, error)
	ApplicationByBundleID(bundleID string) (AXApp, error)
	ClickableNodes(root AXElement, includeOffscreen bool) ([]AXNode, error)
	MenuBarClickableElements() ([]AXNode, error)
	ClickableElementsFromBundleID(bundleID string) ([]AXNode, error)
	ActiveScreenBounds() image.Rectangle

	// Actions
	PerformAction(actionType action.Type, p image.Point, restoreCursor bool) error
	Scroll(deltaX, deltaY int) error
	MoveMouse(p image.Point, bypassSmooth bool)
	CursorPosition() image.Point

	// System
	CheckPermissions() bool
	SetClickableRoles(roles []string)
	ClickableRoles() []string
	IsMissionControlActive() bool
}

// AXWindow represents a window element.
//
//nolint:iface // Intentionally small interface for future extension
type AXWindow interface {
	AXElement
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
}
