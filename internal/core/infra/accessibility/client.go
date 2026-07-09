package accessibility

import (
	"context"
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
	FrontmostWindow(ctx context.Context) (AXWindow, error)
	AllWindows(ctx context.Context) ([]AXWindow, error)
	FrontmostAndPopoverWindows(ctx context.Context) ([]AXWindow, error)
	FocusedApplication(ctx context.Context) (AXApp, error)
	ApplicationByBundleID(ctx context.Context, bundleID string) (AXApp, error)
	ClickableNodes(
		ctx context.Context,
		root AXElement,
		roles []string,
		maxDepth int,
	) ([]AXNode, error)
	MenuBarClickableElements(ctx context.Context, maxDepth int) ([]AXNode, error)
	ClickableElementsFromBundleID(
		ctx context.Context,
		bundleID string,
		roles []string,
		maxDepth int,
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
}

// AXAppInfo contains information about an application.
type AXAppInfo struct {
	Role  string
	Title string
	PID   int
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
	Value() string
	IsClickable() bool
	Release()
}
