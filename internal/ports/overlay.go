package ports

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/grid"
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

// GridFrame is the grid surface: the cells that should be on screen, and the
// input they are drawn narrowed to.
//
// Unlike a hints frame it carries no screen. A grid is built against the
// active display's own space and its cells already sit in it, so there is no
// origin left for the adapter to subtract.
type GridFrame struct {
	// Grid is the cells to draw, in the overlay's screen-local space.
	Grid *grid.Grid

	// Input is what the user has typed so far. It selects which cells the
	// draw marks as matched; narrowing an already-drawn grid is an update
	// rather than a frame, and goes through UpdateGridMatches (ADR 0003).
	Input string
}

// Mode names the mode a grid frame draws.
func (GridFrame) Mode() domain.Mode { return domain.ModeGrid }

func (GridFrame) frame() {}

// RecursiveGridFrame is the recursive-grid surface: the region the user has
// zoomed into, the keys that divide it, and a preview of what the next
// keystroke will produce.
//
// Every recursive-grid keystroke redraws the whole surface — the mode has no
// incremental path to keep off the frame — so this frame is what a keystroke
// hands over, and it stays a plain value for that reason.
type RecursiveGridFrame struct {
	// Bounds is the region currently zoomed into, in the overlay's
	// screen-local space.
	Bounds image.Rectangle

	// Depth is how many divisions deep the user has gone, counted from zero.
	Depth int

	// Layout is how Bounds is divided and what the divisions are labeled.
	Layout RecursiveGridLayout

	// NextLayout is how the next keystroke would divide the cell it picks,
	// which each cell shows as a preview. It is zero when the grid can no
	// longer be divided.
	NextLayout RecursiveGridLayout

	// Pointer is the stand-in for the cursor, drawn on this surface in the
	// same pass as the cells.
	Pointer GridPointer
}

// RecursiveGridLayout is one depth of a recursive grid: the labels dividing a
// region, and the shape they are laid out in. A frame carries two of them —
// the depth on screen and the one a keystroke would produce — because the
// cells preview what pressing their key leads to.
type RecursiveGridLayout struct {
	// Keys are the labels, one per cell, in reading order.
	Keys string

	// Dimensions is how many cells the region is divided into. It travels as
	// one value rather than a row count beside a column count so that no
	// adapter, backend or cgo helper on the way to the division has a pair to
	// put in the wrong order (#1313).
	Dimensions domain.GridDimensions
}

// Mode names the mode a recursive-grid frame draws.
func (RecursiveGridFrame) Mode() domain.Mode { return domain.ModeRecursiveGrid }

func (RecursiveGridFrame) frame() {}

// MonitorSelectFrame is the monitor picker surface: one labeled panel per
// display the user can send the cursor to.
//
// It is the one frame a backend may have no surface for. Drawing it is an
// optional capability the backend declares for itself, so a backend without it
// reports CodeNotSupported and the mode refuses to activate rather than
// engaging with nothing on screen.
type MonitorSelectFrame struct {
	// Targets are the displays to draw, already labeled and narrowed to what
	// the user has typed. An empty frame takes the panels off the screen.
	Targets []MonitorSelectTarget
}

// MonitorSelectTarget is one display the monitor picker offers, in global
// coordinates: the panel is centered on the display it names.
type MonitorSelectTarget struct {
	// Bounds is the display, in global coordinates.
	Bounds image.Rectangle

	// Label is the key sequence that picks this display.
	Label string

	// Name is the display's own name, shown under the label.
	Name string

	// Selected is whether this is the display the current input points at.
	Selected bool

	// MatchedPrefixLen is how many leading runes of Label the user has already
	// typed, so the drawn label can show how far along they are.
	MatchedPrefixLen int
}

// Mode names the mode a monitor-select frame draws.
func (MonitorSelectFrame) Mode() domain.Mode { return domain.ModeMonitorSelect }

func (MonitorSelectFrame) frame() {}

// ScrollFrame is the scroll surface, which draws nothing: scroll mode is a
// mode the indicators report, not a surface with content of its own.
//
// It is still a Frame, because entering scroll mode is still a transition —
// whatever the previous mode drew has to come off the screen, and the overlay
// has to know which mode it is in so the indicators can name it. Saying that
// with a Frame is what keeps one path from a mode to the screen.
type ScrollFrame struct{}

// Mode names the mode a scroll frame draws.
func (ScrollFrame) Mode() domain.Mode { return domain.ModeScroll }

func (ScrollFrame) frame() {}

// GridPointer is the pointer stand-in a grid surface draws where the selection
// is, for a user who has told the cursor not to follow it. It carries position
// only: how big it is and what color it is are Style, resolved by the overlay.
type GridPointer struct {
	// Visible is whether the pointer should be on screen at all.
	Visible bool

	// Position is where it sits, in the overlay's screen-local space.
	Position image.Point
}

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
	//
	// It reports CodeNotSupported for the same reasons RedrawFrame does — the
	// frame's first draw goes out through here — so a caller that acts on the
	// verdict reads that code as degradation, not failure.
	ShowFrame(ctx context.Context, frame Frame) error

	// RedrawFrame draws a Frame whose overlay is already up, without the
	// window sequence. It is the incremental half of this port (ADR 0003):
	// hint labels narrow on every keystroke, and paying for a window show and
	// a mode switch per keystroke is the latency regression AGENTS.md forbids.
	//
	// A backend with no surface for the frame reports CodeNotSupported, which
	// is degradation rather than failure; callers branch on IsNotSupported.
	// A backend that has the surface but cannot place what the frame carries
	// reports the same code — a hint placement it has no branch for (#1331,
	// #1333) — because the caller reads it as "this content is not on screen",
	// and drawing it somewhere the user did not configure is the silent
	// fallback the error vocabulary exists to replace.
	RedrawFrame(ctx context.Context, frame Frame) error

	// ClearFrame takes whatever frame is on screen off it, content and all,
	// and returns the overlay to idle. It is the leaving half of the same
	// sequence, owned in the same place.
	//
	// "Content and all" includes what the incremental calls below left on the
	// grid surface — the hide-unmatched flag and the pointer stand-in (#1492).
	// A mode used to reset those itself on the way out, which on a backend
	// that repaints the whole surface per call meant repainting a grid twice
	// to throw it away; and a caller that has to take half a surface down
	// itself is a caller that can leave half of it behind.
	ClearFrame(ctx context.Context) error

	// SetActiveScreen names the display the overlay's screen-local content
	// belongs to. Grid, recursive-grid and hint content is drawn in a screen's
	// own space; a backend whose surface spans the whole desktop needs the
	// screen to place that content on the right monitor, and one that gives
	// each display its own window needs nothing and ignores it.
	SetActiveScreen(screen image.Rectangle)

	// DrawHintSearch draws the hint search input over the hints frame. Like
	// RedrawFrame it is an update rather than a transition: it fires on every
	// keystroke typed into the search, over an overlay already on screen.
	//
	// An overlay that draws no search input reports CodeNotSupported, which is
	// degradation rather than failure: search itself is unaffected, because the
	// query reaches the hints through the event tap's key stream and not
	// through this. Callers branch on IsNotSupported (#1328).
	//
	// An overlay that does draw one reports the same code when it cannot place
	// the anchor it was configured with (#1329) — the box does not appear
	// either way, and drawing it somewhere the user did not ask for is the
	// silent fallback the error vocabulary exists to replace. So a caller reads
	// the code as "no search input on screen", not as a verdict on the backend.
	DrawHintSearch(search HintSearch) error

	// HideHintSearch takes the search input off the screen, leaving the hint
	// labels behind it where they are.
	//
	// It reports nothing, and deliberately so (#1328): unlike the draw, it
	// makes no claim about the screen that could turn out to be untrue — an
	// overlay that never drew a search input has nothing left to take off — and
	// it runs from teardown, where no caller could act on an answer anyway.
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

	// UpdateGridMatches narrows the grid on screen to the cells whose label
	// starts with prefix. With SetGridHideUnmatched it is the incremental
	// half of grid mode: both fire on every keystroke, over a grid already
	// drawn, and routing them through a frame is the latency regression
	// AGENTS.md forbids (ADR 0003).
	UpdateGridMatches(prefix string)

	// SetGridHideUnmatched says whether cells that no longer match should
	// disappear rather than dim.
	SetGridHideUnmatched(hide bool)

	// ShowGridSubgrid opens the finer grid drawn inside one cell, over the
	// grid already on screen. It fires on the keystroke that picks the cell,
	// so it is an update rather than a frame for the same reason.
	//
	// It carries the pointer stand-in the way a recursive-grid frame does, and
	// for the same reason (#1492): the keystroke that picks a cell is also the
	// keystroke that moves the selection, and on a backend that paints both
	// onto one surface — every Linux one — saying it in two calls repaints
	// that surface twice. The pointer travels here so one call paints both.
	// It is the pointer as it should stand *after* the cell was picked, so an
	// invisible one takes it off the subgrid the same call opens.
	ShowGridSubgrid(cell *grid.Cell, pointer GridPointer)

	// UpdateGridPointer moves the pointer stand-in drawn on a grid surface,
	// or takes it off. The mode names which surface — grid or recursive grid
	// — the same way a frame names the mode it draws.
	//
	// A recursive-grid frame carries the same value, because that backend
	// paints the pointer in the same pass as the cells and a redraw without it
	// would wipe it. ShowGridSubgrid carries it for the same reason. This call
	// is for the keystrokes that move the pointer and nothing else — the
	// activation that places it, the toggle that hides it, the screen change
	// that invalidates it.
	UpdateGridPointer(mode domain.Mode, pointer GridPointer)

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

	// Flush commits everything drawn since the last flush, so a user never
	// sees one indicator moved and another still where it was. Backends that
	// present every draw as it is made have nothing to commit and ignore it.
	Flush()

	// IsVisible returns true if any overlay is currently visible.
	IsVisible() bool

	// Refresh updates the overlay display (e.g., after screen changes).
	Refresh(ctx context.Context) error

	// ApplyConfig hands the overlay a configuration that has just changed. The
	// overlay owns config + theme -> Style, so this is the whole of what a
	// config reload owes it: one notification, one re-resolution, every
	// overlay picking the new values up. A caller that fanned out instead was
	// a caller that could miss one and leave an overlay in the old colors.
	ApplyConfig(cfg *config.Config)

	// RefreshStyles re-resolves those Styles against the configuration the
	// overlay already holds. A light/dark change goes through here: nothing
	// about the configuration moved, only what it resolves to.
	RefreshStyles()

	// SetHiddenInScreenShare says whether the overlay should be excluded from
	// screen captures and shared windows. Backends that cannot exclude
	// themselves ignore it.
	SetHiddenInScreenShare(hidden bool)

	// SetKeyboardCaptureEnabled asks the overlay to hold or release the
	// keyboard. Only an overlay that grabs input has anything to release: on
	// Linux an evdev grab held by the overlay deactivates the focused toplevel,
	// so the indicator poller releases it before reading which window is
	// focused. Where the overlay never takes the keyboard from the focused
	// application, this is a no-op, the same way Flush is on a backend that
	// presents every draw.
	SetKeyboardCaptureEnabled(enabled bool)

	// Destroy releases everything the overlay owns — native windows, surfaces,
	// connections. It runs on the shutdown path, after the modes have already
	// cleared what they drew, and must be safe to call when nothing was ever
	// shown.
	Destroy()
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
