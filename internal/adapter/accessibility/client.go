package accessibility

import (
	"image"

	"github.com/y3owk1n/neru/internal/domain/action"
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
	GetFrontmostWindow() (AXWindow, error)
	GetFocusedApplication() (AXApp, error)
	GetApplicationByBundleID(bundleID string) (AXApp, error)
	GetClickableNodes(root AXElement, includeOffscreen bool) ([]AXNode, error)
	GetMenuBarClickableElements() ([]AXNode, error)
	GetClickableElementsFromBundleID(bundleID string) ([]AXNode, error)
	GetActiveScreenBounds() image.Rectangle

	// Actions
	PerformAction(actionType action.Type, p image.Point, restoreCursor bool) error
	Scroll(deltaX, deltaY int) error
	MoveMouse(p image.Point)
	GetCursorPosition() image.Point

	// System
	CheckPermissions() bool
	SetClickableRoles(roles []string)
	GetClickableRoles() []string
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
	GetBundleIdentifier() string
	GetInfo() (*AXAppInfo, error)
}

// AXNode represents a node in the accessibility tree.
type AXNode interface {
	GetID() string
	GetBounds() image.Rectangle
	GetRole() string
	GetTitle() string
	GetDescription() string
	IsClickable() bool
}
