package ports

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/hint"
)

// Frame is the complete description of what should be on screen for one mode.
//
// A mode hands over a Frame; realizing it — sizing the overlay to the active
// screen, showing it, switching it to the frame's mode, drawing the frame's
// content — belongs to the adapter. That sequence used to be open-coded at
// every call site with nothing checking the order, and getting it wrong showed
// an empty overlay or left the previous mode's on screen.
//
// A Frame carries domain values only: never a resolved Style, never a render
// model, never a platform handle. That constraint is what keeps the overlay's
// own vocabulary out of the app layer, so it is not negotiable.
//
// The interface is sealed to this package: an adapter realizes a frame by
// naming the surfaces it knows about, and a new surface cannot appear without
// the adapter being told.
type Frame interface {
	// Mode names the mode this frame draws. Naming it on the frame is what
	// stops a frame and the mode it is realized in from disagreeing: the
	// adapter translates one value rather than deciding twice.
	Mode() domain.Mode

	// frame seals the interface.
	frame()
}

// HintsFrame is the hints surface: the labels that should be on screen, and
// the display they are drawn on.
type HintsFrame struct {
	// Screen is the active display in global coordinates. Hint positions are
	// global too; translating them into the overlay's screen-local space is
	// the adapter's business.
	Screen image.Rectangle

	// Hints are the labels to draw, already narrowed to what the user should
	// see.
	Hints []*hint.Interface
}

// Mode names the mode a hints frame draws.
func (HintsFrame) Mode() domain.Mode { return domain.ModeHints }

func (HintsFrame) frame() {}

// HintSearch is the hint search input: what has been typed into it, and how
// many labels still match. Where it is drawn is resolved from configuration by
// the overlay, so a caller never carries geometry through the mode layer to
// get it on screen.
type HintSearch struct {
	// Screen is the active display in global coordinates, which is what the
	// input is positioned against.
	Screen image.Rectangle

	// Query is the text typed so far.
	Query string

	// ResultCount is how many hints still match the query.
	ResultCount int
}

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

	// ShowFrame puts a Frame on screen. The adapter owns the whole sequence a
	// transition needs — size the overlay to the active screen, show it,
	// switch it to the frame's mode, draw the frame's content — so a caller
	// says what should be on screen and never orders the steps that get it
	// there.
	//
	// A draw may block; the mode handler computes what to show under its lock
	// and shows after releasing it, with the hint update callback the one
	// documented exception (`internal/app/modes/AGENTS.md`).
	ShowFrame(ctx context.Context, frame Frame) error

	// RedrawFrame draws a Frame whose overlay is already up, without the
	// window sequence. It is the incremental half of this port (ADR 0003):
	// hint labels narrow on every keystroke, and paying for a window show and
	// a mode switch per keystroke is the latency regression AGENTS.md forbids.
	//
	// A backend with no surface for the frame reports CodeNotSupported, which
	// is degradation rather than failure; callers branch on IsNotSupported.
	RedrawFrame(ctx context.Context, frame Frame) error

	// ClearFrame takes whatever frame is on screen off it, content and all,
	// and returns the overlay to idle. It is the leaving half of the same
	// sequence, owned in the same place.
	ClearFrame(ctx context.Context) error

	// DrawHintSearch draws the hint search input over the hints frame. Like
	// RedrawFrame it is an update rather than a transition: it fires on every
	// keystroke typed into the search, over an overlay already on screen.
	DrawHintSearch(search HintSearch) error

	// HideHintSearch takes the search input off the screen, leaving the hint
	// labels behind it where they are.
	HideHintSearch()

	// HintSearchBounds reports where the search input sits on a screen. The
	// platform's IME field has to be placed over it, and the overlay is the
	// one place that decides where "over it" is — asking beats deriving the
	// same rectangle a second time.
	//
	// The rectangle is screen-local, not global: it names a place on the
	// overlay's own drawing surface, which is the one exception the
	// coordinate rule in AGENTS.md already carries for drawn overlay content.
	// ports.TextInputFrame has always been fed this space.
	HintSearchBounds(screen image.Rectangle) image.Rectangle

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
