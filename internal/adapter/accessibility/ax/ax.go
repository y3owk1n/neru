package ax

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain/action"
)

// Element is a generic accessibility element.
type Element interface {
	Release()
}

// Window is a window element.
type Window interface {
	Element
	Role() string
}

// Client is the interface for accessibility operations.
type Client interface {
	// Window and App operations
	FrontmostWindow(ctx context.Context) (Window, error)
	AllWindows(ctx context.Context) ([]Window, error)
	FrontmostAndPopoverWindows(ctx context.Context) ([]Window, error)
	FocusedApplication(ctx context.Context) (App, error)
	ApplicationByBundleID(ctx context.Context, bundleID string) (App, error)
	ClickableNodes(
		ctx context.Context,
		root Element,
		roles []string,
		maxDepth int,
	) ([]Node, error)
	MenuBarClickableElements(ctx context.Context, maxDepth int) ([]Node, error)
	ClickableElementsFromBundleID(
		ctx context.Context,
		bundleID string,
		roles []string,
		maxDepth int,
	) ([]Node, error)
	ActiveScreenBounds() image.Rectangle

	// Actions
	PerformAction(
		actionType action.Type,
		p image.Point,
		restoreCursor bool,
		modifiers action.Modifiers,
	) error
	Scroll(deltaX, deltaY int, modifiers action.Modifiers) error
	MoveMouse(p image.Point, bypassSmooth bool)
	CursorPosition() image.Point

	// System
	CheckPermissions() bool
	SetClickableRoles(roles []string)
	ClickableRoles() []string
	IsMissionControlActive() bool
	// SupportsSupplementaryElements reports whether the platform exposes the
	// macOS-specific auxiliary surfaces hints can also scan (Dock, menu bar,
	// Notification Center, Stage Manager, Picture-in-Picture, screen-capture
	// UI). False on Linux/Windows so those collectors are skipped.
	SupportsSupplementaryElements() bool

	// Close releases any resources held by the client (e.g. D-Bus connections
	// or AT-SPI accessibility status).
	Close() error
}

// AppInfo contains information about an application.
type AppInfo struct {
	Role  string
	Title string
}

// App is an application element.
type App interface {
	Element
	BundleIdentifier() string
	Info() (*AppInfo, error)
}

// Node is a node in the accessibility tree.
type Node interface {
	ID() string
	Bounds() image.Rectangle
	Role() string
	Title() string
	Description() string
	Value() string
	IsClickable() bool
	Release()
}
