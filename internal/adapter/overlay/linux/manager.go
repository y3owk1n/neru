//go:build linux

package linux

import (
	"image"
	"math"
	"strings"
	"sync"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/logonce"
	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/modeindicator"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/stickyindicator"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	platformlinux "github.com/y3owk1n/neru/internal/adapter/platform/linux"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

// subgridCellBackground is the translucent fill painted behind a subgrid cell.
const subgridCellBackground uint32 = 0x10000000

const (
	subgridFontScale   = 0.7
	keyboardChanBuffer = 64
	halfDivisor        = 2
	paddingMultiplier  = 2
	// hintAutoRadiusMax caps the auto (border_radius = -1) hint badge corner
	// radius, matching the macOS overlay's MIN(height/2, 6).
	hintAutoRadiusMax = 6
	// hintBoundaryAutoRadiusMax caps the auto boundary_highlight corner
	// radius, matching macOS's 4.0 fallback and Windows'
	// winAutoRadiusBoundaryCap. Without it a large element box rounds into an
	// oval.
	hintBoundaryAutoRadiusMax = 4
	// searchInputAutoRadiusMax caps the auto search_input_ui corner radius,
	// matching macOS's MIN(height/2, 8). It is looser than the hint badge's
	// cap because the box is taller.
	searchInputAutoRadiusMax = 8

	stickyBadgeClearPadding = 3
)

type linuxOverlayBackend string

const (
	linuxOverlayBackendUnknown        linuxOverlayBackend = "unknown"
	linuxOverlayBackendX11            linuxOverlayBackend = "x11"
	linuxOverlayBackendWaylandWlroots linuxOverlayBackend = "wayland-wlroots"
)

// Manager manages overlay rendering on Linux.
type Manager struct {
	manager.Base

	logger *zap.Logger

	// placementRefusal warns once per hint placement this overlay refuses,
	// rather than on every hints draw.
	placementRefusal logonce.Latch

	// renderMu serializes all rendering dispatch to the backends, because
	// Cairo, X11 and Wayland calls are not thread-safe.
	renderMu sync.Mutex

	backend linuxOverlayBackend
	x11     *x11Overlay
	wlroots *wlrootsOverlay

	// The mouse-action indicator has its own lazily created overlay window,
	// because it fires after a click, once the mode overlay is hidden.
	// indicatorMu serializes creation and dispatch. indicatorRenderMu is its
	// render lock, and it must not share renderMu since it owns an
	// independent X11/Wayland connection.
	indicatorMu       sync.Mutex
	indicatorRenderMu sync.Mutex
	x11Indicator      *x11Overlay
	wlrootsIndicator  *wlrootsOverlay

	keyboardCaptureEnabled bool

	stickyBadgeRect    image.Rectangle
	stickyBadgeVisible bool

	modeIndicatorBadgeRect    image.Rectangle
	modeIndicatorBadgeVisible bool

	// searchBadgeRect is where the hints search badge was last painted, empty
	// when none is on screen. Like the two badge rects above, it exists
	// because the badge shares the one surface, so taking it off means
	// knowing what it covered.
	searchBadgeRect image.Rectangle
}

var (
	linuxManager      *Manager
	linuxManagerOnce  sync.Once
	wlrootsKeyboardCh chan string
)

// NewOverlayManager creates a new overlay Manager.
func NewOverlayManager(logger *zap.Logger) *Manager {
	instance := &Manager{
		Base:                   manager.NewBase(logger),
		logger:                 logger,
		backend:                detectLinuxOverlayBackend(),
		keyboardCaptureEnabled: true,
	}

	switch instance.backend {
	case linuxOverlayBackendX11:
		instance.x11 = newX11Overlay(logger)
		if instance.x11 != nil {
			instance.x11.setRenderMu(&instance.renderMu)
			warnXwaylandScaleMismatch(logger, instance.x11.Scale())
		}
	case linuxOverlayBackendWaylandWlroots:
		instance.wlroots = newWlrootsOverlay(logger)

		// Share renderMu with the wlroots overlay so the keyboard poller
		// serializes wl_display access with rendering. setRenderMu must run
		// before startPoller, which reads the mutex on every iteration.
		if instance.wlroots != nil {
			instance.wlroots.setRenderMu(&instance.renderMu)
			instance.wlroots.startPoller()
		}
	case linuxOverlayBackendUnknown:
		return nil
	}

	if instance.x11 != nil || instance.wlroots != nil {
		platformlinux.SetOwnWindowsHider(instance.hideForCapture)
	}

	return instance
}

// Get returns the global overlay Manager.
func Get() *Manager {
	return linuxManager
}

// Init initializes the global overlay Manager.
func Init(logger *zap.Logger) *Manager {
	linuxManagerOnce.Do(func() {
		linuxManager = NewOverlayManager(logger)
	})

	return linuxManager
}

// WaylandKeyboardChannel returns the keyboard input channel.
func (m *Manager) WaylandKeyboardChannel() <-chan string {
	return wlrootsKeyboardCh
}

// Show displays the overlay.
func (m *Manager) Show() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.Show()
	} else if m.wlroots != nil {
		m.wlroots.Show()
	}
}

// Hide hides the overlay.
func (m *Manager) Hide() {
	m.cancelBackendAnimation()

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.Hide()
	} else if m.wlroots != nil {
		m.wlroots.Hide()
	}

	m.forgetBadgesLocked()
}

// SetKeyboardCaptureEnabled controls whether the Wayland overlay requests
// exclusive keyboard focus or remains keyboard-passive.
func (m *Manager) SetKeyboardCaptureEnabled(enabled bool) {
	if m == nil {
		return
	}

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	m.keyboardCaptureEnabled = enabled

	if m.wlroots != nil {
		m.wlroots.setKeyboardCaptureEnabled(enabled)
	}
}

// Clear clears the overlay content.
func (m *Manager) Clear() {
	m.cancelBackendAnimation()

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.Clear()
	} else if m.wlroots != nil {
		m.wlroots.Clear()
	}

	m.forgetBadgesLocked()
}

// ClearCache is a no-op on Linux. The overlay backend does not retain stale
// cross-mode cache state.
func (m *Manager) ClearCache() {}

// scaleBadgeFont returns a copy of style with its font size multiplied by the
// backend scale, so badgeBounds computes a rect matching the scaled draw.
func scaleBadgeFont(style overlayBadgeStyle, scale float64) overlayBadgeStyle {
	scaled := style
	scaled.fontSize = style.fontSize * scale

	return scaled
}

// ResizeToActiveScreen resizes the overlay to the active screen.
func (m *Manager) ResizeToActiveScreen() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.Resize()
	} else if m.wlroots != nil {
		m.wlroots.Resize()
	}
}

// SetActiveScreenOrigin records the active screen's top-left origin in global
// coordinates, so the backend can move screen-local grid, recursive-grid and
// hint content onto the right monitor. The Linux overlays span the whole
// desktop, so without it that content lands on the leftmost monitor.
func (m *Manager) SetActiveScreenOrigin(origin image.Point) {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.setOriginOffset(origin)
	}

	if m.wlroots != nil {
		m.wlroots.setOriginOffset(origin)
	}
}

// Destroy cleans up the overlay Manager.
func (m *Manager) Destroy() {
	platformlinux.SetOwnWindowsHider(nil)

	// Capture and nil the backend pointers under the lock, then release it
	// before calling Destroy. The wlroots Destroy waits for the
	// keyboardPoller goroutine, which takes renderMu every iteration, so
	// holding renderMu here would deadlock.
	m.renderMu.Lock()
	x11 := m.x11
	wlroots := m.wlroots
	m.x11 = nil
	m.wlroots = nil
	m.renderMu.Unlock()

	if x11 != nil {
		x11.Destroy()
	}

	if wlroots != nil {
		wlroots.Destroy()
	}

	// Same for the indicator overlay, whose poller takes indicatorRenderMu.
	m.indicatorMu.Lock()
	x11Indicator := m.x11Indicator
	wlrootsIndicator := m.wlrootsIndicator
	m.x11Indicator = nil
	m.wlrootsIndicator = nil
	m.indicatorMu.Unlock()

	if x11Indicator != nil {
		x11Indicator.Destroy()
	}

	if wlrootsIndicator != nil {
		wlrootsIndicator.Destroy()
	}
}

// WindowPtr returns the raw window pointer.
func (m *Manager) WindowPtr() unsafe.Pointer {
	if m.x11 != nil {
		return m.x11.WindowPtr()
	} else if m.wlroots != nil {
		return m.wlroots.WindowPtr()
	}

	return nil
}

// BuildComponents constructs the render components this manager draws
// through. Everything renders onto the one surface it owns, so the components
// hold its handle and nothing else.
func (m *Manager) BuildComponents(
	cfg *config.Config,
	theme config.ThemeProvider,
) (manager.Components, error) {
	// A backend that found no display server is handed out as a typed nil,
	// and reaching Base through it would panic.
	if m == nil {
		return manager.Components{}, nil
	}

	return m.Base.BuildComponents(manager.ComponentSpec{
		Config:   cfg,
		Theme:    theme,
		Logger:   m.logger,
		Window:   m.WindowPtr(),
		Headless: m.Headless(),
	})
}

// ConfigureComponents hands the configuration to the render components, as
// Base does, then measures the characters badges are sized around
// (manager.WarmBadgeTextWidths). Configuration arrives here at startup and on
// reload, so a draw, which has a key waiting on it, measures nothing.
func (m *Manager) ConfigureComponents(cfg *config.Config, pointer manager.PointerAppearance) {
	if m == nil {
		return
	}

	m.Base.ConfigureComponents(cfg, pointer)
	manager.WarmBadgeTextWidths(cfg)
}

// Every backend reports headlessness through manager.HeadlessReporter.
var _ manager.HeadlessReporter = (*Manager)(nil)

// Linux draws vim mode's surface on the shared overlay.
var _ manager.VimDrawer = (*Manager)(nil)

// Headless reports whether no backend surface was created: no display server,
// a failed backend init, or a build without cgo.
func (m *Manager) Headless() bool {
	return m == nil || (m.x11 == nil && m.wlroots == nil)
}

// OverlayCapabilities returns the feature capabilities.
func (m *Manager) OverlayCapabilities() ports.FeatureCapability {
	switch m.backend {
	case linuxOverlayBackendX11:
		if m.x11 != nil && m.x11.Healthy() {
			return ports.FeatureCapability{
				Status: ports.FeatureStatusSupported,
				Detail: "native Linux overlays available via X11 + Cairo",
			}
		}

		return ports.FeatureCapability{
			Status: ports.FeatureStatusStub,
			Detail: "X11 overlay backend failed to initialize",
		}
	case linuxOverlayBackendWaylandWlroots:
		if m.wlroots != nil && m.wlroots.Healthy() {
			return ports.FeatureCapability{
				Status: ports.FeatureStatusSupported,
				Detail: "native Linux overlays available via wlroots layer-shell + Cairo",
			}
		}

		return ports.FeatureCapability{
			Status: ports.FeatureStatusStub,
			Detail: "wlroots layer-shell overlay backend failed to initialize",
		}
	case linuxOverlayBackendUnknown:
		return ports.FeatureCapability{
			Status: ports.FeatureStatusStub,
			Detail: "native Linux overlays are not available (no display detected)",
		}
	default:
		return ports.FeatureCapability{
			Status: ports.FeatureStatusStub,
			Detail: "native Linux overlays are not implemented for this backend",
		}
	}
}

// DrawHintsWithStyle draws the hints overlay using the active Linux backend.
// The placement is resolved once, before anything is drawn, and one this
// overlay cannot draw is reported rather than drawn centered.
func (m *Manager) DrawHintsWithStyle(hintsSlice []*hints.Hint, style hints.StyleMode) error {
	// Cancel first so a refusal cannot leave an animation painting the
	// surface. The same call reads the backend pointers under renderMu.
	attached := m.cancelBackendAnimation()
	if !attached {
		return derrors.New(
			derrors.CodeNotSupported,
			"overlay hints not implemented on linux backend",
		)
	}

	offset, offsetErr := resolveHintBadgeOffset(style.Placement())
	if offsetErr != nil {
		// The mode handler degrades quietly on CodeNotSupported, so warn here too
		// or the user loses every hint with only a debug line. The placement is a
		// fixed configuration keyword.
		m.placementRefusal.Warn(m.logger, style.Placement(),
			"Hint placement not drawn by the linux overlay",
			zap.String("placement", style.Placement()),
		)

		return offsetErr
	}

	m.placementRefusal.Reset()

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	// A hints draw clears the whole surface, taking any search badge with it.
	// Forgetting it keeps the next hide from erasing a rectangle out of the
	// hints.
	m.searchBadgeRect = image.Rectangle{}

	if m.x11 != nil {
		m.x11.DrawHints(hintsSlice, style, offset)

		return nil
	}

	// Nil-checked again, because a Destroy may land while renderMu was
	// released for the cancel (../AGENTS.md).
	if m.wlroots != nil {
		m.wlroots.DrawHints(hintsSlice, style, offset)
	}

	return nil
}

// DrawHintSearchInput paints the hints search badge on the active backend.
// It only displays. The mode handler owns the query, and keys go to the event
// tap or, on layer-shell Wayland compositors, to the textinput adapter's
// hint-search field.
//
// Nothing is cleared first. The badge is painted over the hints the same draw
// cycle just put down, since erasing the labels being narrowed would defeat
// the search.
func (m *Manager) DrawHintSearchInput(
	query string,
	resultCount int,
	frame hints.SearchInputFrame,
	style hints.SearchInputStyle,
) error {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	label := badge.SearchLabel(query, resultCount)

	if m.x11 != nil {
		m.searchBadgeRect = m.x11.DrawHintSearchInput(label, frame, style)
	} else if m.wlroots != nil {
		m.searchBadgeRect = m.wlroots.DrawHintSearchInput(label, frame, style)
	}

	// An empty rectangle means nothing was painted (no backend, or a closed
	// handle). Reporting success then would claim a query was on screen.
	if m.searchBadgeRect.Empty() {
		return derrors.New(
			derrors.CodeNotSupported,
			"no linux overlay surface to draw the hint search input on",
		)
	}

	return nil
}

// HideHintSearchInput takes the hints search badge off the shared surface.
// It is errorless on purpose (#1328). A draw makes a claim about the screen
// that only the error channel can withdraw, while a hide claims nothing that
// could be untrue, and it runs from teardown where no caller could act on an
// error.
func (m *Manager) HideHintSearchInput() {
	// Cancel before the lock (see cancelBackendAnimation), so a running
	// animation does not paint over the repainted hints. For the same reason
	// the repaint uses repaintHints, not drawHints.
	if !m.cancelBackendAnimation() {
		return
	}

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	painted := m.searchBadgeRect
	if painted.Empty() {
		return
	}

	m.searchBadgeRect = image.Rectangle{}

	// Nil-checked again, because a Destroy may land while renderMu was
	// released for the cancel.
	if m.x11 != nil {
		m.x11.HideHintSearchInput(painted)
	} else if m.wlroots != nil {
		m.wlroots.HideHintSearchInput(painted)
	}
}

// DrawModeIndicator draws the mode indicator overlay.
func (m *Manager) DrawModeIndicator(posX, posY int) {
	if m.ModeIndicatorOverlay() == nil {
		return
	}

	mode := m.Mode()
	if mode == manager.ModeIdle {
		return
	}

	label, colors, style, ok := resolveModeIndicatorAppearance(
		string(mode),
		m.ModeIndicatorOverlay(),
	)
	if !ok {
		return
	}

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	m.clearModeIndicatorBadgeLocked()

	rect := badgeBounds(posX, posY, label, scaleBadgeFont(style, m.overlayScale()))
	m.modeIndicatorBadgeRect = expandRect(rect, stickyBadgeClearPadding)
	m.modeIndicatorBadgeVisible = true

	if m.x11 != nil {
		m.x11.DrawBadge(posX, posY, label, colors, style)
	} else if m.wlroots != nil {
		m.wlroots.DrawBadge(posX, posY, label, colors, style)
	}
}

// DrawStickyModifiersIndicator draws the sticky modifiers indicator overlay.
func (m *Manager) DrawStickyModifiersIndicator(posX, posY int, symbols string) {
	if m.StickyModifiersOverlay() == nil {
		return
	}

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if symbols == "" {
		m.clearStickyBadgeLocked()

		return
	}

	colors, style, ok := resolveStickyIndicatorAppearance(m.StickyModifiersOverlay())
	if !ok {
		return
	}

	m.clearStickyBadgeLocked()
	m.stickyBadgeRect = expandRect(
		badgeBounds(posX, posY, symbols, scaleBadgeFont(style, m.overlayScale())),
		stickyBadgeClearPadding,
	)
	m.stickyBadgeVisible = true

	if m.x11 != nil {
		m.x11.DrawBadge(posX, posY, symbols, colors, style)
	} else if m.wlroots != nil {
		m.wlroots.DrawBadge(posX, posY, symbols, colors, style)
	}
}

// DrawGrid draws the grid overlay.
func (m *Manager) DrawGrid(grid *domainGrid.Grid, input string, style grid.Style) error {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.syncSublayerKeysLocked(&m.x11.sublayerKeys)

		m.x11.DrawGrid(grid, input, style)

		return nil
	} else if m.wlroots != nil {
		m.syncSublayerKeysLocked(&m.wlroots.sublayerKeys)

		m.wlroots.DrawGrid(grid, input, style)

		return nil
	}

	return derrors.New(derrors.CodeNotSupported, "overlay grid not implemented on linux backend")
}

// DrawRecursiveGrid draws the recursive grid overlay.
func (m *Manager) DrawRecursiveGrid(
	bounds image.Rectangle,
	depth int,
	keys string,
	dims domain.GridDimensions,
	nextKeys string,
	nextDims domain.GridDimensions,
	style recursivegrid.Style,
	virtualPointer recursivegrid.VirtualPointerState,
) error {
	return m.drawRegionGrid(
		m.RecursiveGridOverlay(),
		"recursive grid",
		bounds, depth, keys, dims, nextKeys, nextDims, style, virtualPointer,
	)
}

// DrawBisect draws the bisect region divided in four, with the transition
// [bisect.animation] configures on the component built for it.
func (m *Manager) DrawBisect(
	bounds image.Rectangle,
	depth int,
	keys string,
	style recursivegrid.Style,
	virtualPointer recursivegrid.VirtualPointerState,
) error {
	return m.drawRegionGrid(
		m.BisectOverlay(),
		"bisect",
		bounds, depth, keys, bisectDimensions(), "", domain.GridDimensions{}, style, virtualPointer,
	)
}

// bisectDimensions is the one shape a bisect region is drawn in: four
// quadrants.
func bisectDimensions() domain.GridDimensions {
	return domain.GridDimensions{Rows: 2, Cols: 2} //nolint:mnd // four quadrants
}

// DrawGridPointer puts grid mode's pointer stand-in on the shared surface.
//
// Recursive grid does not come through here. Its pointer rides the frame each
// keystroke hands over, so it is already painted with the cells. Grid mode
// has no such frame, because narrowing and opening a subgrid are plain calls
// (ADR 0003), so its pointer arrives alone and is kept for the next repaint.
//
// Overrides the Base method, whose render components own no surface off
// darwin (#1463). Recursive grid is still delegated to Base.
func (m *Manager) DrawGridPointer(
	mode manager.Mode,
	point image.Point,
	appearance manager.PointerAppearance,
) {
	if m == nil {
		return
	}

	if mode != manager.ModeGrid {
		m.Base.DrawGridPointer(mode, point, appearance)

		return
	}

	m.dispatchGridPointer(recursivegrid.VirtualPointerState{
		Visible:   true,
		Position:  point,
		Size:      appearance.FontSize,
		FillColor: appearance.FillColor,
		Char:      appearance.Char,
		FontName:  appearance.FontFamily,
	})
}

// HideGridPointer takes grid mode's pointer stand-in off the shared surface,
// leaving the grid. Mode teardown calls it before the frame is cleared, and
// so does the mode's selection going away.
func (m *Manager) HideGridPointer(mode manager.Mode) {
	if m == nil {
		return
	}

	if mode != manager.ModeGrid {
		m.Base.HideGridPointer(mode)

		return
	}

	m.dispatchGridPointer(recursivegrid.VirtualPointerState{})
}

// UpdateGridMatches updates the grid overlay matches.
func (m *Manager) UpdateGridMatches(prefix string) {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.UpdateGridMatches(prefix)
	} else if m.wlroots != nil {
		m.wlroots.UpdateGridMatches(prefix)
	}
}

// ShowSubgrid shows the subgrid overlay with the pointer stand-in that belongs
// on it once the cell is picked (#1492). Both are painted into one Cairo
// target, so handing over the pointer before the paint keeps a keystroke to
// one repaint, which matters for arrow keys repeating inside a subgrid.
//
// It cancels before taking the lock (see cancelBackendAnimation), and the
// same call answers whether there is a backend at all.
func (m *Manager) ShowSubgrid(
	cell *domainGrid.Cell,
	style grid.Style,
	virtualPointer recursivegrid.VirtualPointerState,
) {
	if !m.cancelBackendAnimation() {
		return
	}

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	// Nil-checked again, because a Destroy may land while renderMu was
	// released for the cancel.
	if m.x11 != nil {
		m.syncSublayerKeysLocked(&m.x11.sublayerKeys)

		m.x11.ShowSubgrid(cell, style, virtualPointer)
	} else if m.wlroots != nil {
		m.syncSublayerKeysLocked(&m.wlroots.sublayerKeys)

		m.wlroots.ShowSubgrid(cell, style, virtualPointer)
	}
}

// SetHideUnmatched sets the hide unmatched overlay option.
func (m *Manager) SetHideUnmatched(hide bool) {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.SetHideUnmatched(hide)
	} else if m.wlroots != nil {
		m.wlroots.SetHideUnmatched(hide)
	}
}

// SetSharingType is a no-op on Linux.
func (m *Manager) SetSharingType(_ bool) {}

// Flush commits all pending drawing operations to the display.
func (m *Manager) Flush() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.Flush()
	} else if m.wlroots != nil {
		m.wlroots.Flush()
	}
}

// warnXwaylandScaleMismatch warns when the GNOME overlay would draw in a
// coordinate space other than the shared logical layout. Mutter keeps the X
// root equal to that layout by default, but with its experimental
// xwayland-native-scaling feature the root is in physical pixels and Xft.dpi
// announces the factor. The overlay and the X-root cursor position would then
// be wrong on any scaled monitor. Grid and scroll on a 1x layout still work,
// so the daemon starts, but the mismatch must not be silent.
func warnXwaylandScaleMismatch(logger *zap.Logger, scale float64) {
	if logger == nil || scale == 1 ||
		platform.DetectLinuxBackend() != platform.BackendWaylandGNOME {
		return
	}

	logger.Warn(
		"Xwayland reports a HiDPI scale on GNOME; overlay geometry and cursor sync will be misaligned on scaled monitors (disable Mutter's xwayland-native-scaling)",
		zap.Float64("xft_scale", scale),
	)
}

// detectLinuxOverlayBackend delegates to platform.DetectLinuxBackend so
// compositor detection stays consistent across layers.
func detectLinuxOverlayBackend() linuxOverlayBackend {
	switch platform.DetectLinuxBackend() {
	case platform.BackendX11:
		return linuxOverlayBackendX11
	case platform.BackendWaylandGNOME:
		// Mutter has no layer shell, and a fullscreen xdg_toplevel is animated
		// on map and takes focus. An override-redirect Xwayland window stacks
		// above every toplevel, honors an empty input shape, and is neither
		// animated nor focused. The factory already refused GNOME without
		// Xwayland (platform.XwaylandAvailable).
		return linuxOverlayBackendX11
	case platform.BackendWaylandWlroots, platform.BackendWaylandKDE, platform.BackendWaylandCOSMIC:
		return linuxOverlayBackendWaylandWlroots
	case platform.BackendUnknown, platform.BackendWaylandOther:
		return linuxOverlayBackendUnknown
	}

	return linuxOverlayBackendUnknown
}

const (
	// These mirror monitor_select_overlay_darwin.m: auto padding from the
	// label font for -1, a label/subtitle gap, an 80% panel cap, and default
	// font sizes.
	monitorSelectLabelGap       = 4
	monitorSelectAutoPadXMin    = 24
	monitorSelectAutoPadYMin    = 12
	monitorSelectAutoPadXRatio  = 0.3
	monitorSelectAutoPadYRatio  = 0.15
	monitorSelectMaxFraction    = 0.8
	monitorSelectMaxRadius      = 16
	monitorSelectDefaultFont    = 96
	monitorSelectDefaultSubFont = 18
)

// monitorSelectFontOr returns the configured font size, or a default when unset
// (<= 0), matching the macOS overlay's fallbacks (96 label / 18 subtitle).
func monitorSelectFontOr(value, fallback int) float64 {
	if value <= 0 {
		return float64(fallback)
	}

	return float64(value)
}

// monitorSelectPanelLayout computes, in device pixels, the centered panel
// rect, the label and subtitle rects, and the corner radius, mirroring macOS
// sizing and its -1 "auto" sentinels. scale is the backend HiDPI factor (X11
// Xft.dpi, 1 on Wayland, which scales via the compositor buffer).
func monitorSelectPanelLayout(
	monitor image.Rectangle,
	label, subtitle string,
	style manager.MonitorSelectStyle,
	scale float64,
) (image.Rectangle, image.Rectangle, image.Rectangle, float64) {
	labelFont := monitorSelectFontOr(style.FontSize, monitorSelectDefaultFont) * scale
	subFont := monitorSelectFontOr(style.SubtitleFontSize, monitorSelectDefaultSubFont) * scale

	// Auto padding from the label font when the config uses -1 (matching darwin).
	padX := float64(style.PaddingX) * scale
	if style.PaddingX < 0 {
		padX = math.Max(
			monitorSelectAutoPadXMin*scale,
			math.Round(labelFont*monitorSelectAutoPadXRatio),
		)
	}

	padY := float64(style.PaddingY) * scale
	if style.PaddingY < 0 {
		padY = math.Max(
			monitorSelectAutoPadYMin*scale,
			math.Round(labelFont*monitorSelectAutoPadYRatio),
		)
	}

	labelW := badge.TextWidth(label, style.FontFamily, labelFont, true)
	labelH := badge.EstimateTextHeight(labelFont)

	subW, subH, gap := 0, 0, 0
	if subtitle != "" {
		subW = badge.TextWidth(subtitle, style.SubtitleFontFamily, subFont, false)
		subH = badge.EstimateTextHeight(subFont)
		gap = int(math.Round(float64(monitorSelectLabelGap) * scale))
	}

	panelW := max(labelW, subW) + int(padX)*paddingMultiplier

	panelH := labelH + int(padY)*paddingMultiplier
	if subtitle != "" {
		panelH += subH + gap
	}

	// Cap the panel to a fraction of the monitor, matching darwin.
	if maxW := int(float64(monitor.Dx()) * monitorSelectMaxFraction); panelW > maxW {
		panelW = maxW
	}

	if maxH := int(float64(monitor.Dy()) * monitorSelectMaxFraction); panelH > maxH {
		panelH = maxH
	}

	// The panel hangs on the monitor's center point.
	center := image.Pt(
		monitor.Min.X+monitor.Dx()/halfDivisor,
		monitor.Min.Y+monitor.Dy()/halfDivisor,
	)
	panel := badge.CenteredOn(center, panelW, panelH)

	// Corner radius: auto = min(panelH/2, 16), matching darwin.
	radius := float64(style.BorderRadius) * scale
	if style.BorderRadius < 0 {
		radius = math.Min(float64(panelH)/halfDivisor, monitorSelectMaxRadius*scale)
	}

	// Vertically center the label (+ subtitle) block within the panel.
	totalTextH := labelH
	if subtitle != "" {
		totalTextH += gap + subH
	}

	textTop := panel.Min.Y + (panelH-totalTextH)/halfDivisor

	labelRect := image.Rect(panel.Min.X, textTop, panel.Max.X, textTop+labelH)

	subtitleRect := image.Rectangle{}
	if subtitle != "" {
		subTop := labelRect.Max.Y + gap
		subtitleRect = image.Rect(panel.Min.X, subTop, panel.Max.X, subTop+subH)
	}

	return panel, labelRect, subtitleRect, radius
}

// monitorSelectDrawSpec holds the once-parsed colors shared by both backends'
// DrawMonitorSelect. Font sizes are not here, because each target fits its
// own (manager.MonitorSelectStyle.FittedTo). Like darwin, every label uses
// the single text color, so matched and selected states look the same.
type monitorSelectDrawSpec struct {
	backdrop     uint32
	background   uint32
	border       uint32
	text         uint32
	subtitleText uint32
	borderWidth  float64
	hasBackdrop  bool
}

func newMonitorSelectDrawSpec(style manager.MonitorSelectStyle) monitorSelectDrawSpec {
	return monitorSelectDrawSpec{
		backdrop:     badge.ParseHexARGB(style.BackdropColor),
		background:   badge.ParseHexARGB(style.BackgroundColor),
		border:       badge.ParseHexARGB(style.BorderColor),
		text:         badge.ParseHexARGB(style.TextColor),
		subtitleText: badge.ParseHexARGB(style.SubtitleTextColor),
		borderWidth:  float64(max(style.BorderWidth, 1)),
		hasBackdrop:  strings.TrimSpace(style.BackdropColor) != "",
	}
}

// DrawMonitorSelect renders one labeled panel per monitor for the monitor
// picker, then shows the overlay. Unlike macOS's NSPanel per display, it
// reuses the shared spanning surface.
func (m *Manager) DrawMonitorSelect(
	targets []manager.MonitorSelectTarget,
	style manager.MonitorSelectStyle,
) error {
	m.cancelBackendAnimation()

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.DrawMonitorSelect(targets, style)
		m.x11.Show()

		return nil
	}

	if m.wlroots != nil {
		m.wlroots.DrawMonitorSelect(targets, style)
		m.wlroots.Show()

		return nil
	}

	return derrors.New(
		derrors.CodeNotSupported,
		"monitor_select overlay not implemented on linux backend",
	)
}

// DrawVim paints vim mode's surface. The adapter has already brought the
// shared window up, so this only draws.
func (m *Manager) DrawVim(scene manager.VimScene, style manager.VimStyle) error {
	m.cancelBackendAnimation()

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.DrawVim(scene, style)

		return nil
	}

	if m.wlroots != nil {
		m.wlroots.DrawVim(scene, style)

		return nil
	}

	return derrors.New(derrors.CodeNotSupported, "vim overlay not implemented on linux backend")
}

// HideMonitorSelect hides the monitor_select overlay, clearing its panels.
func (m *Manager) HideMonitorSelect() {
	m.Hide()
}

type overlayColors struct {
	background uint32
	border     uint32
	text       uint32
}

type overlayBadgeStyle struct {
	fontFamily  string
	fontSize    float64
	paddingX    int
	paddingY    int
	borderWidth float64
	offsetX     int
	offsetY     int
}

// DrawVirtualPointer renders the cursor-following virtual pointer overlay.
func (m *Manager) DrawVirtualPointer(xCoordinate, yCoordinate, size int, fillColor string) {
	if m.VirtualPointerOverlay() == nil {
		return
	}

	m.VirtualPointerOverlay().Draw(xCoordinate, yCoordinate, size, fillColor)
}

// DrawMouseActionIndicator animates a transient indicator at the click point
// on a lazily created window of its own, so it survives the mode exit that
// follows a click.
func (m *Manager) DrawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
	if m == nil {
		return
	}

	m.indicatorMu.Lock()
	defer m.indicatorMu.Unlock()

	// Both draws are nil-checked because the lazy build below can fail and
	// return a raw nil. This is the one place a backend pointer is created
	// outside startup (../AGENTS.md).
	switch m.backend {
	case linuxOverlayBackendX11:
		if m.x11Indicator == nil {
			m.x11Indicator = newX11Overlay(m.logger)
			if m.x11Indicator != nil {
				m.x11Indicator.setRenderMu(&m.indicatorRenderMu)
			}
		}

		if m.x11Indicator != nil {
			m.x11Indicator.DrawMouseActionIndicator(point, style)
		}
	case linuxOverlayBackendWaylandWlroots:
		if m.wlrootsIndicator == nil {
			m.wlrootsIndicator = newWlrootsOverlay(m.logger)
			if m.wlrootsIndicator != nil {
				// The indicator surface uses the indicator render lock and its
				// own poller, which only pumps configure and frame events.
				m.wlrootsIndicator.setRenderMu(&m.indicatorRenderMu)
				m.wlrootsIndicator.setKeyboardCaptureEnabled(false)
				m.wlrootsIndicator.startPoller()
			}
		}

		if m.wlrootsIndicator != nil {
			m.wlrootsIndicator.DrawMouseActionIndicator(point, style)
		}
	case linuxOverlayBackendUnknown:
	}
}

// HideIndicator takes an indicator off the screen and erases the badge it
// left. On Linux an indicator is a badge on the shared surface, so the render
// component's Hide is a no-op and erasing its rectangle is the hide. Without
// this it would stay painted until the mode ends.
func (m *Manager) HideIndicator(indicator ports.Indicator) {
	m.Base.HideIndicator(indicator)

	// The virtual pointer draws into its own surface, so there is nothing to
	// erase. Return before the lock, because the cursor-visibility path calls
	// this with the mode handler's lock held and every draw contends renderMu.
	if indicator == ports.VirtualPointerIndicator {
		return
	}

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	switch indicator {
	case ports.ModeIndicator:
		m.clearModeIndicatorBadgeLocked()
	case ports.StickyModifiersIndicator:
		m.clearStickyBadgeLocked()
	case ports.VirtualPointerIndicator:
		// Returned above.
	}
}

// drawRegionGrid paints one region divided into cells on the shared surface,
// reading the transition settings from the render component that owns the
// mode being drawn. what names the mode for the refusal.
func (m *Manager) drawRegionGrid(
	component *recursivegrid.Overlay,
	what string,
	bounds image.Rectangle,
	depth int,
	keys string,
	dims domain.GridDimensions,
	nextKeys string,
	nextDims domain.GridDimensions,
	style recursivegrid.Style,
	virtualPointer recursivegrid.VirtualPointerState,
) error {
	m.cancelBackendAnimation()

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	animEnabled := false

	animDurationMS := 50

	if component != nil {
		animCfg := component.Config().Animation
		animEnabled = animCfg.Enabled
		animDurationMS = animCfg.DurationMS
	}

	if m.x11 != nil {
		m.x11.DrawRecursiveGridWithSubKeyPreview(
			bounds,
			depth,
			keys,
			dims,
			nextKeys,
			nextDims,
			style,
			virtualPointer,
			animEnabled,
			animDurationMS,
		)

		return nil
	} else if m.wlroots != nil {
		m.wlroots.DrawRecursiveGridWithSubKeyPreview(
			bounds,
			depth,
			keys,
			dims,
			nextKeys,
			nextDims,
			style,
			virtualPointer,
			animEnabled,
			animDurationMS,
		)

		return nil
	}

	return derrors.New(
		derrors.CodeNotSupported,
		what+" overlay not implemented on linux backend",
	)
}

// cancelBackendAnimation stops any animation the attached backend is running,
// and reports whether a backend was attached.
//
// Destroy nils both backend pointers under renderMu, so this reads them under
// it too, then releases the lock before canceling, because the cancel waits
// for a goroutine that takes renderMu every frame. Each pointer is captured
// once. Reading the field again for the call could hand a nil backend to the
// promoted cancelAnimation, which dereferences the receiver before any guard.
func (m *Manager) cancelBackendAnimation() bool {
	m.renderMu.Lock()
	x11 := m.x11
	wlroots := m.wlroots
	m.renderMu.Unlock()

	switch {
	case wlroots != nil:
		wlroots.cancelAnimation()
	case x11 != nil:
		x11.cancelAnimation()
	default:
		return false
	}

	return true
}

// overlayScale returns the active backend's HiDPI scale. X11 scales fonts and
// geometry by Xft.dpi. Wayland renders in logical units and returns 1. It
// sizes badge clear rects to match what is drawn.
func (m *Manager) overlayScale() float64 {
	if m.x11 != nil {
		return m.x11.Scale()
	}

	return 1
}

// dispatchGridPointer hands the pointer state to the attached backend, which
// keeps it and repaints the grid surface.
//
// Unlike DrawHintsWithStyle it cancels no animation. The only animation on
// this surface is the recursive-grid transition, which the frame coming up
// already canceled. The mouse-action indicator runs on its own backend
// behind indicatorRenderMu, so it shares neither surface nor lock.
func (m *Manager) dispatchGridPointer(pointer recursivegrid.VirtualPointerState) {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.SetGridPointer(pointer)
	} else if m.wlroots != nil {
		m.wlroots.SetGridPointer(pointer)
	}
}

// forgetBadgesLocked drops every record of what is painted on the shared
// surface. It follows a Hide or Clear, after which the records would describe
// a screen that is gone.
func (m *Manager) forgetBadgesLocked() {
	m.stickyBadgeVisible = false
	m.stickyBadgeRect = image.Rectangle{}
	m.modeIndicatorBadgeVisible = false
	m.modeIndicatorBadgeRect = image.Rectangle{}
	m.searchBadgeRect = image.Rectangle{}

	// The grid pointer record lives on the backend, whose pass paints it, but
	// it describes the same surface and goes too.
	if m.x11 != nil {
		m.x11.forgetGridPointer()
	} else if m.wlroots != nil {
		m.wlroots.forgetGridPointer()
	}
}

func (m *Manager) clearStickyBadgeLocked() {
	if !m.stickyBadgeVisible {
		return
	}

	if m.x11 != nil {
		m.x11.ClearRect(m.stickyBadgeRect)
	} else if m.wlroots != nil {
		m.wlroots.ClearRect(m.stickyBadgeRect)
	}

	m.stickyBadgeVisible = false
	m.stickyBadgeRect = image.Rectangle{}
}

func (m *Manager) clearModeIndicatorBadgeLocked() {
	if !m.modeIndicatorBadgeVisible {
		return
	}

	if m.x11 != nil {
		m.x11.ClearRect(m.modeIndicatorBadgeRect)
	} else if m.wlroots != nil {
		m.wlroots.ClearRect(m.modeIndicatorBadgeRect)
	}

	m.modeIndicatorBadgeVisible = false
	m.modeIndicatorBadgeRect = image.Rectangle{}
}

func resolveModeIndicatorAppearance(
	mode string,
	overlay *modeindicator.Overlay,
) (string, overlayColors, overlayBadgeStyle, bool) {
	if overlay == nil {
		return "", overlayColors{}, overlayBadgeStyle{}, false
	}

	label := overlay.ResolveLabelText(mode)
	if label == "" {
		return "", overlayColors{}, overlayBadgeStyle{}, false
	}

	modeCfg, ok := overlay.ResolveModeConfig(mode)
	if !ok || !modeCfg.Enabled {
		return "", overlayColors{}, overlayBadgeStyle{}, false
	}

	cfg := overlay.IndicatorConfig()
	theme := overlay.ThemeProvider()

	colors := overlayColors{
		background: badge.ParseHexARGB(
			modeCfg.BackgroundColor.ForThemeWithOverride(
				cfg.UI.BackgroundColor,
				theme,
				config.ModeIndicatorBackgroundColorLight,
				config.ModeIndicatorBackgroundColorDark,
			),
		),
		border: badge.ParseHexARGB(
			modeCfg.BorderColor.ForThemeWithOverride(
				cfg.UI.BorderColor,
				theme,
				config.ModeIndicatorBorderColorLight,
				config.ModeIndicatorBorderColorDark,
			),
		),
		text: badge.ParseHexARGB(
			modeCfg.TextColor.ForThemeWithOverride(
				cfg.UI.TextColor,
				theme,
				config.ModeIndicatorTextColorLight,
				config.ModeIndicatorTextColorDark,
			),
		),
	}

	style := overlayBadgeStyle{
		fontFamily:  ports.ResolveFont(cfg.UI.FontFamily),
		fontSize:    float64(max(cfg.UI.FontSize, 1)),
		paddingX:    cfg.UI.PaddingX,
		paddingY:    cfg.UI.PaddingY,
		borderWidth: float64(max(cfg.UI.BorderWidth, 0)),
		offsetX:     cfg.UI.IndicatorXOffset,
		offsetY:     cfg.UI.IndicatorYOffset,
	}

	return label, colors, style, true
}

func resolveStickyIndicatorAppearance(
	overlay *stickyindicator.Overlay,
) (overlayColors, overlayBadgeStyle, bool) {
	if overlay == nil {
		return overlayColors{}, overlayBadgeStyle{}, false
	}

	cfg := overlay.UIConfig()
	theme := overlay.ThemeProvider()

	colors := overlayColors{
		background: badge.ParseHexARGB(
			cfg.BackgroundColor.ForTheme(
				theme,
				config.StickyModifiersBackgroundColorLight,
				config.StickyModifiersBackgroundColorDark,
			),
		),
		border: badge.ParseHexARGB(
			cfg.BorderColor.ForTheme(
				theme,
				config.StickyModifiersBorderColorLight,
				config.StickyModifiersBorderColorDark,
			),
		),
		text: badge.ParseHexARGB(
			cfg.TextColor.ForTheme(
				theme,
				config.StickyModifiersTextColorLight,
				config.StickyModifiersTextColorDark,
			),
		),
	}

	style := overlayBadgeStyle{
		fontFamily:  ports.ResolveFont(cfg.FontFamily),
		fontSize:    float64(max(cfg.FontSize, 1)),
		paddingX:    cfg.PaddingX,
		paddingY:    cfg.PaddingY,
		borderWidth: float64(max(cfg.BorderWidth, 0)),
		offsetX:     cfg.IndicatorXOffset,
		offsetY:     cfg.IndicatorYOffset,
	}

	return colors, style, true
}

func badgeBounds(posX, posY int, text string, style overlayBadgeStyle) image.Rectangle {
	return badge.Bounds(
		posX, posY,
		style.offsetX, style.offsetY,
		text,
		// An indicator badge is drawn bold (drawBadge).
		badge.TextFont{Family: style.fontFamily, Size: style.fontSize, Bold: true},
		style.paddingX, style.paddingY,
	)
}

// Hint connector tail edge, matching the C hint-badge renderer: which badge
// edge the triangular tail is merged into (0 = none).
const (
	hintTailNone   = 0
	hintTailTop    = 1 // tail on the badge's top edge, apex above (placement "bottom")
	hintTailBottom = 2 // tail on the badge's bottom edge, apex below (placement "top")
)

// resolveHintBadgeOffset maps a configured placement onto the offset this
// overlay draws it with. It is the one place the placement vocabulary is read
// here, and every value config.HintPlacements() declares has a branch.
// Anything else is refused, so a placement added without a Linux branch fails
// loudly instead of drawing centered.
//
// The empty string is a style that reached the overlay before a configuration
// settled it. It draws at the documented default, as macOS does
// (`hintPlacementValue`).
func resolveHintBadgeOffset(placement string) (badge.HintOffset, error) {
	if placement == "" {
		placement = config.HintPlacementDefault
	}

	switch placement {
	case config.HintPlacementTop:
		return badge.HintAbove, nil
	case config.HintPlacementCenter:
		return badge.HintOnTarget, nil
	case config.HintPlacementBottom:
		return badge.HintBelow, nil
	default:
		return badge.HintOnTarget, derrors.New(
			derrors.CodeNotSupported,
			"hint placement is not drawn by the linux overlay",
		)
	}
}

// hintTailEdge reports which badge edge the connector tail merges into, so
// the renderer can draw badge and tail as one outline. It returns
// hintTailNone when there is no arrow. It lives here, not beside
// badge.PlaceHint, because its values are the C renderer's.
func hintTailEdge(badgeRect image.Rectangle, arrow badge.HintArrow, hasArrow bool) int {
	if !hasArrow {
		return hintTailNone
	}

	if arrow.Tip.Y < badgeRect.Min.Y {
		return hintTailTop
	}

	return hintTailBottom
}

func expandRect(rect image.Rectangle, amount int) image.Rectangle {
	return image.Rect(
		rect.Min.X-amount,
		rect.Min.Y-amount,
		rect.Max.X+amount,
		rect.Max.Y+amount,
	)
}

// syncSublayerKeysLocked hands the surface the subgrid keys. Every draw that
// can show a subgrid calls it, because a surface with no keys draws a subgrid
// the user cannot see but can still act on (config.ResolveSublayerKeys).
func (m *Manager) syncSublayerKeysLocked(target *string) {
	if m.GridOverlay() == nil {
		return
	}

	*target = m.GridOverlay().Config().SublayerKeys
}

// hideForCapture runs capture with the overlay cleared, then restores exactly
// what it showed. It holds renderMu throughout, so no draw lands between the
// clear and the capture. On Wayland the keyboard poller reads under the same
// lock, so keys typed meanwhile are read once it ends. The click indicator
// has its own surface and stays in the capture.
func (m *Manager) hideForCapture(capture func() error) error {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		return m.x11.withBlanked(capture)
	}

	if m.wlroots != nil {
		return m.wlroots.withBlanked(capture)
	}

	return capture()
}
