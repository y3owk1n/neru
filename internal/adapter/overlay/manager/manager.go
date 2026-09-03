package manager

import (
	"image"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

// Mode is the overlay mode.
type Mode string

const (
	// ModeIdle is the idle mode.
	ModeIdle Mode = Mode(domain.ModeNameIdle)
	// ModeHints is the hints mode.
	ModeHints Mode = Mode(domain.ModeNameHints)
	// ModeGrid is the grid mode.
	ModeGrid Mode = Mode(domain.ModeNameGrid)
	// ModeScroll is the scroll mode.
	ModeScroll Mode = Mode(domain.ModeNameScroll)
	// ModeRecursiveGrid is the recursive-grid mode.
	ModeRecursiveGrid Mode = Mode(domain.ModeNameRecursiveGrid)
	// ModeMonitorSelect is the monitor_select mode.
	ModeMonitorSelect Mode = Mode(domain.ModeNameMonitorSelect)
)

// StateChange is a change in overlay mode.
type StateChange struct {
	prev Mode
	next Mode
}

// NewStateChange records a transition from prev to next.
//
// The fields stay unexported so a subscriber cannot rewrite the transition it
// was handed; the backends construct one through here.
func NewStateChange(prev, next Mode) StateChange {
	return StateChange{prev: prev, next: next}
}

// Prev returns the previous mode.
func (sc StateChange) Prev() Mode {
	return sc.prev
}

// Next returns the next mode.
func (sc StateChange) Next() Mode {
	return sc.next
}

// CapabilityReporter is the contract a Manager implements to report its own
// runtime support state. It aliases ports.OverlayCapabilityReporter, which is
// where the contract is declared, so overlay capability reporting shares one
// vocabulary with the rest of the platform surface.
type CapabilityReporter = ports.OverlayCapabilityReporter

// HeadlessReporter is the optional extension a manager implements when it can
// state that it has no surface to draw on — a no-op manager, or a backend that
// found no display server to attach to.
//
// It exists because the render overlays are built against a native surface: on
// a headless manager there is nothing to build them on, and the caller has to
// know that before it tries. Reach it by type assertion and treat a manager
// that does not implement it as able to render, the same way every other
// optional overlay capability works.
type HeadlessReporter interface {
	// Headless reports whether this manager has no surface to render on.
	Headless() bool
}

// KeyboardCaptureController is the optional extension a backend implements
// when its overlay surface can hold or release the keyboard.
//
// Only the Linux backends can: an evdev grab held by the overlay deactivates
// the focused toplevel, so the indicator poller has to release it. Reach it by
// type assertion and skip the behavior when the assertion fails, the same way
// every other optional platform capability works.
type KeyboardCaptureController interface {
	SetKeyboardCaptureEnabled(enabled bool)
}

// MonitorSelectTarget is one selectable monitor rendered by the monitor_select
// overlay: a labeled panel centered on the monitor's bounds.
type MonitorSelectTarget struct {
	Bounds           image.Rectangle
	Label            string
	Subtitle         string
	Selected         bool
	MatchedPrefixLen int
}

// PointerAppearance is the part of the virtual pointer's look that a render
// component cannot take from the configuration it is handed as written: the
// fill color resolved against the active theme, the font family settled
// through the shared font resolver, and the char and font size settled against
// their documented defaults. All of them are resolved once per configuration
// or theme change and travel with that notification, so no component resolves
// per draw — and the pointer redraws on every cursor move.
type PointerAppearance struct {
	// FillColor is the themed fill, as a hex string.
	FillColor string
	// FontFamily is the family the platform will actually find, with generic
	// aliases already settled.
	FontFamily string
	// Char is the glyph to draw, with an empty configured value already
	// replaced by the documented default.
	Char string
	// FontSize is the glyph's point size, with a configured value below 1
	// already replaced by the documented default.
	FontSize int
}

// MonitorSelectStyle carries the resolved (theme-applied) appearance for the
// monitor_select overlay. Colors are hex strings, parsed by the backend, to
// mirror how the hints/grid styles are threaded.
type MonitorSelectStyle struct {
	FontSize           int
	SubtitleFontSize   int
	FontFamily         string
	SubtitleFontFamily string
	BorderRadius       int
	PaddingX           int
	PaddingY           int
	BorderWidth        int
	BackgroundColor    string
	TextColor          string
	MatchedTextColor   string
	BorderColor        string
	BackdropColor      string
	SubtitleTextColor  string
	HideInScreenShare  bool
}

// MonitorSelector is the optional extension a backend implements when it can
// draw the monitor-select overlay. The darwin and Linux backends do; elsewhere
// the mode reports CodeNotSupported.
type MonitorSelector interface {
	DrawMonitorSelect(targets []MonitorSelectTarget, style MonitorSelectStyle) error
	HideMonitorSelect()
}

// Interface is the overlay window management contract.
type Interface interface {
	WaylandKeyboardChannel() <-chan string
	Show()
	Hide()
	Clear()
	ClearCache()
	ResizeToActiveScreen()
	// SetActiveScreenOrigin informs the overlay of the active screen's
	// top-left origin in global coordinates. Backends whose overlay surfaces
	// span the whole desktop (Linux X11 / Wayland) use it to translate the
	// screen-local coordinates of grid, recursive-grid and hint content onto
	// the correct monitor. It is a no-op where each screen has its own overlay
	// window positioned at the screen origin (macOS).
	SetActiveScreenOrigin(origin image.Point)
	SwitchTo(next Mode)
	Subscribe(fn func(StateChange)) uint64
	Unsubscribe(id uint64)
	Destroy()
	Mode() Mode

	// BuildComponents constructs the render components this manager draws
	// through, from the configuration and theme it is handed and on the
	// surface it owns. The set is returned as well as kept, because a few
	// app-layer call sites still talk to a render component directly.
	BuildComponents(cfg *config.Config, theme config.ThemeProvider) (Components, error)
	// ConfigureComponents hands a new configuration to those components. It is
	// the notification a config reload or a theme change needs; the resolved
	// virtual pointer appearance comes with it because appearance is resolved
	// above this and never here.
	ConfigureComponents(cfg *config.Config, pointer PointerAppearance)

	DrawHintsWithStyle(hs []*hints.Hint, style hints.StyleMode) error
	// DrawHintSearchInput draws the query and result count over the hints
	// surface. A backend that draws no such badge reports CodeNotSupported —
	// nil means it is on screen, and a backend that answers nil without drawing
	// leaves the mode handler with no way to know (#1328).
	DrawHintSearchInput(
		query string,
		resultCount int,
		frame hints.SearchInputFrame,
		style hints.SearchInputStyle,
	) error
	// HideHintSearchInput takes that badge off the surface. It reports nothing
	// on purpose: a backend that never drew one has nothing left on screen, so
	// there is no failure for it to report, and no caller could act on one from
	// a call that runs at teardown (#1328).
	HideHintSearchInput()
	DrawModeIndicator(x, y int)
	DrawStickyModifiersIndicator(x, y int, symbols string)
	DrawVirtualPointer(x, y int, size int, fillColor string)

	ShowIndicator(indicator ports.Indicator)
	HideIndicator(indicator ports.Indicator)
	ResizeIndicatorToActiveScreen(indicator ports.Indicator)

	DrawMouseActionIndicator(point image.Point, style ports.MouseActionIndicatorStyle)
	DrawGrid(g *domainGrid.Grid, input string, style grid.Style) error
	// DrawRecursiveGrid draws one depth of the recursive grid, with the
	// preview of the depth a keystroke would produce.
	//
	// dims and nextDims carry each depth's cell counts whole rather than as a
	// column count beside a row count: this contract used to spell both pairs
	// as adjacent ints, and every backend re-paired them itself right before
	// dividing the region (#1313).
	DrawRecursiveGrid(
		bounds image.Rectangle,
		depth int,
		keys string,
		dims domain.GridDimensions,
		nextKeys string,
		nextDims domain.GridDimensions,
		style recursivegrid.Style,
		virtualPointer recursivegrid.VirtualPointerState,
	) error
	UpdateGridMatches(prefix string)
	// ShowSubgrid replaces the cells on the grid surface with the finer grid
	// inside one cell.
	//
	// It takes the pointer stand-in the way DrawRecursiveGrid does, and for the
	// same reason (#1492): a backend that paints both onto one surface repaints
	// that surface once per call, and the keystroke that opens a subgrid is the
	// keystroke that moves the pointer. A backend whose pointer is a layer of
	// its own applies it after the open and pays nothing for the pairing.
	ShowSubgrid(
		cell *domainGrid.Cell,
		style grid.Style,
		virtualPointer recursivegrid.VirtualPointerState,
	)
	SetHideUnmatched(hide bool)

	// DrawGridPointer and HideGridPointer drive the pointer stand-in drawn on
	// a grid mode's own surface, which is not one of the cursor-following
	// Indicators: it belongs to the mode's drawing, and the mode names it by
	// mode rather than by render component.
	//
	// The whole resolved appearance travels with the position, not the size
	// and fill alone: a backend that paints the glyph on its own surface —
	// every Linux one — holds no render component with the pointer's
	// configuration in it, so the char and the family have nowhere else to
	// come from. The darwin components read them from the configuration
	// ConfigureComponents hands them and ignore these two.
	DrawGridPointer(mode Mode, point image.Point, appearance PointerAppearance)
	HideGridPointer(mode Mode)
	Flush()
	SetSharingType(hide bool)
}
