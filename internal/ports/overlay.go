package ports

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain/hint"
)

// MouseActionIndicatorStyle configures a transient mouse action indicator.
type MouseActionIndicatorStyle struct {
	Size              int
	BorderWidth       int
	BackgroundColor   string
	BorderColor       string
	Shape             string
	DurationMS        int
	StartScale        float64
	EndScale          float64
	StartOpacity      float64
	EndOpacity        float64
	Easing            string
	HideInScreenShare bool
}

// Indicator names one of the small cursor-following overlays that report
// state independently of the active mode's own drawing.
//
// The port speaks in indicators rather than in render objects: an indicator's
// whole life — whether it is on screen and where it is drawn — is owned by one
// service, and whether the overlay behind it was ever constructed is the
// adapter's business, not the caller's.
type Indicator int

const (
	// ModeIndicator is the label naming the mode currently active. Numbering
	// starts at one so a zero Indicator names nothing rather than silently
	// naming this one.
	ModeIndicator Indicator = iota + 1
	// StickyModifiersIndicator is the readout of the modifiers held sticky.
	StickyModifiersIndicator
	// VirtualPointerIndicator is the cursor stand-in drawn while the system
	// cursor is hidden.
	VirtualPointerIndicator
)

// String names the indicator for logs.
func (i Indicator) String() string {
	switch i {
	case ModeIndicator:
		return "mode"
	case StickyModifiersIndicator:
		return "sticky_modifiers"
	case VirtualPointerIndicator:
		return "virtual_pointer"
	default:
		return "unknown"
	}
}

// OverlayPort is the interface for managing UI overlays.
// Implementations handle the platform-specific rendering of hints and indicators.
type OverlayPort interface {
	// Health returns nil if the component is healthy, or an error if it is not.
	Health(ctx context.Context) error

	// ShowHints displays hint labels on the screen.
	ShowHints(ctx context.Context, hints []*hint.Interface) error

	// DrawModeIndicator draws a mode indicator at the specified position.
	DrawModeIndicator(x, y int)

	// DrawStickyModifiersIndicator draws the sticky modifiers indicator at the specified position.
	DrawStickyModifiersIndicator(x, y int, symbols string)

	// DrawVirtualPointer draws the cursor-following virtual pointer at the
	// given global position.
	DrawVirtualPointer(x, y int)

	// DrawMouseActionIndicator draws a transient indicator for a mouse action.
	DrawMouseActionIndicator(point image.Point, style MouseActionIndicatorStyle)

	// ShowIndicator makes an indicator visible. It draws nothing: position and
	// content arrive through the matching Draw call.
	ShowIndicator(indicator Indicator)

	// HideIndicator takes an indicator off the screen, content and all. What
	// that costs — clearing a window, erasing a rectangle from a shared
	// surface — is the backend's business; a caller that had to sequence it
	// was a caller that could leave the indicator behind.
	HideIndicator(indicator Indicator)

	// ResizeIndicatorToActiveScreen sizes an indicator's surface to the
	// display the cursor is on. Indicators that manage their own window and
	// backends that draw them onto the shared overlay both answer this; where
	// it is meaningless it is a no-op.
	//
	// Unlike the rest of this port, it is also called from the mode handler's
	// locked context — entering a mode sizes the indicators before the polling
	// goroutine starts — so an implementation must not wait on anything that
	// could itself be waiting on the app.
	ResizeIndicatorToActiveScreen(indicator Indicator)

	// Hide hides the overlay.s from the screen.
	Hide(ctx context.Context) error

	// IsVisible returns true if any overlay is currently visible.
	IsVisible() bool

	// Refresh updates the overlay display (e.g., after screen changes).
	Refresh(ctx context.Context) error
}

// OverlayCapabilityReporter is an optional OverlayPort extension for managers
// that can report their own support state — the one subsystem whose
// availability is decided at runtime (Wayland without layer-shell, X11 without
// a display). OverlayPort.Health asks through this and reports
// CodeNotSupported with the manager's detail. Managers that do not implement
// it are treated as healthy.
type OverlayCapabilityReporter interface {
	// OverlayCapabilities reports whether this manager can currently render.
	OverlayCapabilities() FeatureCapability
}
