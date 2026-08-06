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

// NoOpManager is a no-op implementation of Interface for headless environments.
type NoOpManager struct{}

// Ensure NoOpManager always implements Interface, and keeps declaring the
// optional headless capability the component factory reads.
var (
	_ Interface        = (*NoOpManager)(nil)
	_ HeadlessReporter = (*NoOpManager)(nil)
)

// WaylandKeyboardChannel returns nil channel.
func (n *NoOpManager) WaylandKeyboardChannel() <-chan string { return nil }

// Show is a no-op implementation.
func (n *NoOpManager) Show() {}

// Hide is a no-op implementation.
func (n *NoOpManager) Hide() {}

// Clear is a no-op implementation.
func (n *NoOpManager) Clear() {}

// ClearCache is a no-op implementation.
func (n *NoOpManager) ClearCache() {}

// ResizeToActiveScreen is a no-op implementation.
func (n *NoOpManager) ResizeToActiveScreen() {}

// SetActiveScreenOrigin is a no-op implementation.
func (n *NoOpManager) SetActiveScreenOrigin(origin image.Point) {}

// SwitchTo is a no-op implementation.
func (n *NoOpManager) SwitchTo(next Mode) {}

// Subscribe is a no-op implementation.
func (n *NoOpManager) Subscribe(fn func(StateChange)) uint64 { return 0 }

// Unsubscribe is a no-op implementation.
func (n *NoOpManager) Unsubscribe(id uint64) {}

// Destroy is a no-op implementation.
func (n *NoOpManager) Destroy() {}

// Mode returns ModeIdle.
func (n *NoOpManager) Mode() Mode { return ModeIdle }

// Headless reports that the no-op manager has no surface to render on.
func (n *NoOpManager) Headless() bool { return true }

// BuildComponents builds nothing. This manager declares itself headless, and
// that is the whole reason: there is no surface to build against.
func (n *NoOpManager) BuildComponents(
	cfg *config.Config,
	theme config.ThemeProvider,
) (Components, error) {
	return Components{}, nil
}

// ConfigureComponents is a no-op implementation.
func (n *NoOpManager) ConfigureComponents(cfg *config.Config, virtualPointerFill string) {}

// DrawHintsWithStyle is a no-op implementation.
func (n *NoOpManager) DrawHintsWithStyle(
	hs []*hints.Hint,
	style hints.StyleMode,
) error {
	return nil
}

// DrawHintSearchInput is a no-op implementation.
func (n *NoOpManager) DrawHintSearchInput(
	query string,
	resultCount int,
	frame hints.SearchInputFrame,
	style hints.SearchInputStyle,
) error {
	return nil
}

// HideHintSearchInput is a no-op implementation.
func (n *NoOpManager) HideHintSearchInput() {}

// DrawModeIndicator is a no-op implementation.
func (n *NoOpManager) DrawModeIndicator(x, y int) {}

// DrawStickyModifiersIndicator is a no-op implementation.
func (n *NoOpManager) DrawStickyModifiersIndicator(x, y int, symbols string) {}

// DrawVirtualPointer is a no-op implementation.
func (n *NoOpManager) DrawVirtualPointer(_, _ int, _ int, _ string) {}

// ShowIndicator is a no-op implementation.
func (n *NoOpManager) ShowIndicator(indicator ports.Indicator) {}

// HideIndicator is a no-op implementation.
func (n *NoOpManager) HideIndicator(indicator ports.Indicator) {}

// ResizeIndicatorToActiveScreen is a no-op implementation.
func (n *NoOpManager) ResizeIndicatorToActiveScreen(indicator ports.Indicator) {}

// DrawMouseActionIndicator is a no-op implementation.
func (n *NoOpManager) DrawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
}

// DrawGrid is a no-op implementation.
func (n *NoOpManager) DrawGrid(
	g *domainGrid.Grid,
	input string,
	style grid.Style,
) error {
	return nil
}

// DrawRecursiveGrid is a no-op implementation.
func (n *NoOpManager) DrawRecursiveGrid(
	bounds image.Rectangle,
	depth int,
	keys string,
	gridCols int,
	gridRows int,
	nextKeys string,
	nextGridCols int,
	nextGridRows int,
	style recursivegrid.Style,
	virtualPointer recursivegrid.VirtualPointerState,
) error {
	return nil
}

// UpdateGridMatches is a no-op implementation.
func (n *NoOpManager) UpdateGridMatches(prefix string) {}

// ShowSubgrid is a no-op implementation.
func (n *NoOpManager) ShowSubgrid(cell *domainGrid.Cell, style grid.Style) {}

// SetHideUnmatched is a no-op implementation.
func (n *NoOpManager) SetHideUnmatched(hide bool) {}

// DrawGridPointer is a no-op implementation.
func (n *NoOpManager) DrawGridPointer(_ Mode, _ image.Point, _ int, _ string) {}

// HideGridPointer is a no-op implementation.
func (n *NoOpManager) HideGridPointer(_ Mode) {}

// SetSharingType is a no-op implementation.
func (n *NoOpManager) SetSharingType(hide bool) {}

// Flush is a no-op implementation.
func (n *NoOpManager) Flush() {}

// OverlayCapabilities reports that NoOpManager does not render overlays.
func (n *NoOpManager) OverlayCapabilities() ports.FeatureCapability {
	return ports.FeatureCapability{
		Status: ports.FeatureStatusHeadless,
		Detail: "headless no-op overlay manager",
	}
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
	// virtual pointer fill color comes with it because appearance is resolved
	// above this and never here.
	ConfigureComponents(cfg *config.Config, virtualPointerFill string)

	DrawHintsWithStyle(hs []*hints.Hint, style hints.StyleMode) error
	DrawHintSearchInput(
		query string,
		resultCount int,
		frame hints.SearchInputFrame,
		style hints.SearchInputStyle,
	) error
	HideHintSearchInput()
	DrawModeIndicator(x, y int)
	DrawStickyModifiersIndicator(x, y int, symbols string)
	DrawVirtualPointer(x, y int, size int, fillColor string)

	ShowIndicator(indicator ports.Indicator)
	HideIndicator(indicator ports.Indicator)
	ResizeIndicatorToActiveScreen(indicator ports.Indicator)

	DrawMouseActionIndicator(point image.Point, style ports.MouseActionIndicatorStyle)
	DrawGrid(g *domainGrid.Grid, input string, style grid.Style) error
	DrawRecursiveGrid(
		bounds image.Rectangle,
		depth int,
		keys string,
		gridCols int,
		gridRows int,
		nextKeys string,
		nextGridCols int,
		nextGridRows int,
		style recursivegrid.Style,
		virtualPointer recursivegrid.VirtualPointerState,
	) error
	UpdateGridMatches(prefix string)
	ShowSubgrid(cell *domainGrid.Cell, style grid.Style)
	SetHideUnmatched(hide bool)

	// DrawGridPointer and HideGridPointer drive the pointer stand-in drawn on
	// a grid mode's own surface, which is not one of the cursor-following
	// Indicators: it belongs to the mode's drawing, and the mode names it by
	// mode rather than by render component.
	DrawGridPointer(mode Mode, point image.Point, size int, fillColor string)
	HideGridPointer(mode Mode)
	Flush()
	SetSharingType(hide bool)
}
