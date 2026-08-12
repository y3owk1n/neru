//go:build linux

package linux

import (
	"image"
	"math"
	"strings"
	"sync"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/modeindicator"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/stickyindicator"
	"github.com/y3owk1n/neru/internal/adapter/platform"
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
	// radius so labels get a subtle rounded corner rather than a full pill,
	// matching the macOS overlay's MIN(height/2, 6).
	hintAutoRadiusMax = 6
	// hintBoundaryAutoRadiusMax caps the auto
	// (boundary_highlight.border_radius = -1) corner radius of the element
	// boundary, matching the macOS overlay's 4.0 fallback and the Windows
	// winAutoRadiusBoundaryCap. An element box is much larger than a badge, so
	// without the cap the auto radius would round it into an oval.
	hintBoundaryAutoRadiusMax = 4
	// searchInputAutoRadiusMax caps the auto (search_input_ui.border_radius =
	// -1) corner radius of the hints search badge, matching the macOS overlay's
	// MIN(height/2, 8). It is looser than the hint badge's cap because the box
	// is taller: the same 6 would read as square corners on it.
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

	// renderMu serializes all rendering dispatch to the backend overlays.
	// On macOS the Objective-C bridge serializes via dispatch_async to the
	// main thread; on Linux we must do this ourselves because Cairo/X11/
	// Wayland calls are not thread-safe.
	renderMu sync.Mutex

	backend linuxOverlayBackend
	x11     *x11Overlay
	wlroots *wlrootsOverlay

	// The mouse-action indicator renders on its own dedicated overlay window,
	// separate from the mode overlay above, because it fires *after* a click
	// once the mode has exited and the mode overlay has been hidden. It is
	// created lazily on first use. indicatorMu serializes creation and
	// dispatch; indicatorRenderMu is the indicator overlay's own render lock
	// (it owns an independent X11/Wayland connection, so it must not share the
	// mode overlay's renderMu).
	indicatorMu       sync.Mutex
	indicatorRenderMu sync.Mutex
	x11Indicator      *x11Overlay
	wlrootsIndicator  *wlrootsOverlay

	keyboardCaptureEnabled bool

	stickyBadgeRect    image.Rectangle
	stickyBadgeVisible bool

	modeIndicatorBadgeRect    image.Rectangle
	modeIndicatorBadgeVisible bool

	// searchBadgeRect is where the hints search badge was last painted, and the
	// empty rectangle means there is none on screen. It is the third piece of
	// screen state this manager keeps, for the same reason as the two above:
	// the badge is painted onto the one shared surface rather than into a
	// window of its own, so taking it off means knowing what it covered.
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
		}
	case linuxOverlayBackendWaylandWlroots:
		instance.wlroots = newWlrootsOverlay(logger)

		// Share renderMu with the wlroots overlay so the keyboard poller
		// serializes wl_display access with the rendering path. The Wayland
		// client API is not thread-safe.
		// setRenderMu must be called before startPoller — the poller reads
		// the mutex on every iteration, so it must be visible before launch.
		if instance.wlroots != nil {
			instance.wlroots.setRenderMu(&instance.renderMu)
			instance.wlroots.startPoller()
		}
	case linuxOverlayBackendUnknown:
		return nil
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

// ClearCache is a no-op on Linux; the overlay backend does not retain stale
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

// SetActiveScreenOrigin records the active screen's top-left origin (in global
// coordinates) so the backend can translate screen-local overlay content — the
// grid, recursive-grid and hint coordinates, which are normalized to a
// screen-origin of (0,0) — onto the correct monitor. The Linux overlays span
// the whole desktop (one X11 window / one layer-shell surface per output), so
// without this offset that content is always composited at the global origin,
// i.e. the leftmost monitor.
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
	// Capture and nil-out backend pointers under the lock, then release
	// the lock *before* calling Destroy on each backend. The wlroots
	// Destroy waits for the keyboardPoller goroutine to exit, and that
	// goroutine acquires renderMu on every iteration.
	// Holding renderMu while waiting on the poller would deadlock.
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

	// Tear down the dedicated mouse-action indicator overlay, if it was ever
	// created. Same locking rationale as above: the wlroots Destroy waits on
	// its poller goroutine, which acquires indicatorRenderMu each iteration.
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

// BuildComponents constructs the render components this manager draws through,
// on the surface it owns. The X11 and Wayland overlays render everything onto
// that one surface, so the components hold its handle and nothing else.
func (m *Manager) BuildComponents(
	cfg *config.Config,
	theme config.ThemeProvider,
) (manager.Components, error) {
	// Nil-guarded like every other exported method here: a backend that found
	// no display server is handed out as a typed nil, and reaching Base
	// through it would panic before any receiver guard ran.
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

// Ensure the manager keeps declaring the optional headless capability. Its own
// BuildComponents reads Headless directly, so drift there fails to compile;
// this pins the shared spelling every backend answers headlessness with.
var _ manager.HeadlessReporter = (*Manager)(nil)

// Headless reports whether no backend surface was created — no display server
// was detected, the backend failed to initialize, or this is a build without
// cgo — leaving nothing for the render overlays to draw on.
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
//
// The placement is resolved once, before anything is drawn: it is the same for
// every badge in the frame, and a placement this overlay cannot draw is
// reported rather than approximated. Drawing every hint centered instead would
// leave the user looking at labels in a placement they did not choose, with
// nothing anywhere saying so.
func (m *Manager) DrawHintsWithStyle(hintsSlice []*hints.Hint, style hints.StyleMode) error {
	// Canceling first keeps the refusal below from leaving an animation
	// painting the surface this draw was about to replace, and the same call
	// answers whether there is a backend to draw on — so this refusal reads
	// the backend pointers under renderMu instead of beside it.
	attached := m.cancelBackendAnimation()
	if !attached {
		return derrors.New(
			derrors.CodeNotSupported,
			"overlay hints not implemented on linux backend",
		)
	}

	offset, offsetErr := resolveHintBadgeOffset(style.Placement())
	if offsetErr != nil {
		// The error reaches the mode handler, which degrades quietly on
		// CodeNotSupported — so say it here too, or a placement this overlay
		// cannot draw costs the user every hint with only a debug line to
		// explain it. The placement is a fixed configuration keyword.
		if m.logger != nil {
			m.logger.Warn(
				"hint placement not drawn by the linux overlay",
				zap.String("placement", style.Placement()),
			)
		}

		return offsetErr
	}

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	// A hints draw clears the whole surface, so any search badge on it went
	// with the labels. Forgetting it here is what keeps the hide that follows a
	// canceled search from erasing a rectangle out of the hints it just put
	// back — there is nothing left of the badge to take down.
	m.searchBadgeRect = image.Rectangle{}

	if m.x11 != nil {
		m.x11.DrawHints(hintsSlice, style, offset)

		return nil
	}

	// Nil-checked like every other dispatch here rather than trusting the
	// answer above: that one was taken before renderMu was released for the
	// cancel, so a Destroy landing in between leaves this pointer nil. This is
	// the site that used to dispatch on it unchecked (../AGENTS.md).
	if m.wlroots != nil {
		m.wlroots.DrawHints(hintsSlice, style, offset)
	}

	return nil
}

// DrawHintSearchInput paints the hints search badge on the active backend.
//
// It is a display and nothing more: the query arrives as an argument, having
// been read from the event tap's key stream and held by the mode handler, and
// no key reaches this surface. That is the whole difference from the macOS
// implementation, where the field is a real NSTextField that owns keyboard
// focus — here there is one owner of the query string and it is not the
// overlay.
//
// Nothing is cleared first. The badge is painted over the hints the same draw
// cycle just put on the surface, the way an indicator badge is, because a
// search that erased the labels it is narrowing would be showing the user the
// one thing they are not looking for.
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

	// The backend answers with the rectangle it painted, and the empty one means
	// it painted nothing — no backend at all, or one whose native handle is
	// closed. Reporting success then would tell the mode handler a query and a
	// match count were on screen when the screen is blank, which is the silent
	// no-op this method used to be for a different reason.
	if m.searchBadgeRect.Empty() {
		return derrors.New(
			derrors.CodeNotSupported,
			"no linux overlay surface to draw the hint search input on",
		)
	}

	return nil
}

// HideHintSearchInput takes the hints search badge off the shared surface.
//
// It keeps its errorless signature deliberately (#1328). The two directions are
// not symmetric: a draw makes a claim about what is on screen and the error
// channel is the only place to withdraw it, while hiding claims nothing that
// could be untrue — whatever was painted is gone, and a call with nothing to
// take down has already succeeded. Widening it would hand every platform a
// return value no caller could act on, on a call that runs from teardown.
func (m *Manager) HideHintSearchInput() {
	// Canceling before the lock is what DrawHintsWithStyle does and for the
	// same reason: putting the badge away repaints the hints, and an animation
	// still running would paint over that. It happens out here because
	// cancelAnimation waits for a goroutine that takes renderMu on every frame
	// — which is also why the repaint below goes through repaintHints rather
	// than drawHints, so no second cancel is attempted under the lock. The same
	// call answers whether there is a backend at all.
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

	// Nil-checked like every other dispatch here rather than trusting the
	// answer above: that one was taken before renderMu was released for the
	// cancel, so a Destroy landing in between leaves this pointer nil.
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
	m.cancelBackendAnimation()

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	animEnabled := false

	animDurationMS := 50

	if m.RecursiveGridOverlay() != nil {
		animCfg := m.RecursiveGridOverlay().Config().Animation
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
		"recursive grid overlay not implemented on linux backend",
	)
}

// DrawGridPointer puts grid mode's pointer stand-in on the shared surface.
//
// Recursive grid does not come through here: the pointer it draws rides the
// frame its every keystroke hands over (DrawRecursiveGrid), so it is already
// painted in the same pass as the cells and a second path would repaint a grid
// that mode never drew. Grid mode has no such frame — narrowing a prefix and
// opening a subgrid are plain calls (ADR 0003) — so its pointer arrives on its
// own and is kept until the next repaint reads it.
//
// Overrides the Base method, which resolves a mode to a render component: off
// darwin those components own no surface, which is why the pointer was a no-op
// in grid mode here (#1463). Recursive grid is still delegated so the Base
// keeps answering for a mode this backend does not paint itself.
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
// leaving the grid on it. It is what mode teardown calls before the frame is
// cleared, so the repaint it triggers is the first one without the pointer —
// and the mode's own selection going away is the other caller, where that
// repaint is the whole of what the user sees happen.
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

// ShowSubgrid shows the subgrid overlay.
//
// Canceling before the lock is what HideHintSearchInput does and for the same
// reason: the subgrid replaces the whole surface, and an animation still running
// would paint over it. It happens out here because cancelAnimation waits for a
// goroutine that takes renderMu on every frame — canceling from under the lock
// is a deadlock, not a slow path. The same call answers whether there is a
// backend at all.
func (m *Manager) ShowSubgrid(cell *domainGrid.Cell, style grid.Style) {
	if !m.cancelBackendAnimation() {
		return
	}

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	// Nil-checked like every other dispatch here rather than trusting the
	// answer above: that one was taken before renderMu was released for the
	// cancel, so a Destroy landing in between leaves this pointer nil.
	if m.x11 != nil {
		m.syncSublayerKeysLocked(&m.x11.sublayerKeys)

		m.x11.ShowSubgrid(cell, style)
	} else if m.wlroots != nil {
		m.syncSublayerKeysLocked(&m.wlroots.sublayerKeys)

		m.wlroots.ShowSubgrid(cell, style)
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

// detectLinuxOverlayBackend delegates to the canonical
// platform.DetectLinuxBackend so that compositor-family detection (GNOME, KDE,
// wlroots, etc.) is consistent across all layers.
func detectLinuxOverlayBackend() linuxOverlayBackend {
	switch platform.DetectLinuxBackend() {
	case platform.BackendX11:
		return linuxOverlayBackendX11
	case platform.BackendWaylandWlroots, platform.BackendWaylandKDE:
		return linuxOverlayBackendWaylandWlroots
	case platform.BackendUnknown, platform.BackendWaylandGNOME,
		platform.BackendWaylandOther:
		return linuxOverlayBackendUnknown
	}

	return linuxOverlayBackendUnknown
}

const (
	// These mirror the macOS overlay (monitor_select_overlay_darwin.m) so the
	// Linux monitor_select UI matches darwin: auto padding derived from the label
	// font when the config uses the -1 sentinel, a small label/subtitle gap, an
	// 80% cap on panel size relative to the monitor, and default font sizes.
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

// monitorSelectPanelLayout computes, in device pixels, the centered panel rect,
// the label/subtitle text rects, and the corner radius — mirroring the macOS
// overlay's sizing so the Linux monitor_select UI matches darwin. Padding and
// radius honor the same "auto" (-1) config sentinels. scale is the backend HiDPI
// factor (X11 Xft.dpi; 1 on Wayland, which scales via the compositor buffer).
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

	labelW := badge.EstimateTextWidth(label, labelFont)
	labelH := badge.EstimateTextHeight(labelFont)

	subW, subH, gap := 0, 0, 0
	if subtitle != "" {
		subW = badge.EstimateTextWidth(subtitle, subFont)
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

// monitorSelectDrawSpec holds the once-parsed colors and base (unscaled) font
// sizes shared by both backends' DrawMonitorSelect. Base font sizes are passed
// to drawTextCentered, which applies the backend scale (X11) or none (Wayland).
// Like darwin, every panel's label uses the single text color (matched/selected
// state is not visually distinguished).
type monitorSelectDrawSpec struct {
	backdrop     uint32
	background   uint32
	border       uint32
	text         uint32
	subtitleText uint32
	borderWidth  float64
	labelFont    float64
	subtitleFont float64
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
		labelFont:    monitorSelectFontOr(style.FontSize, monitorSelectDefaultFont),
		subtitleFont: monitorSelectFontOr(style.SubtitleFontSize, monitorSelectDefaultSubFont),
		hasBackdrop:  strings.TrimSpace(style.BackdropColor) != "",
	}
}

// DrawMonitorSelect renders one labeled panel per monitor for the interactive
// monitor picker, then shows the overlay. Unlike macOS (one NSPanel per display)
// this reuses the shared spanning X11 window / per-output layer-shell surfaces.
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

// DrawMouseActionIndicator animates a transient indicator at the click point on
// a dedicated overlay window, independent of the mode overlay so it survives
// the mode exit that follows a click. The indicator overlay is created lazily
// on first use.
func (m *Manager) DrawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
	if m == nil {
		return
	}

	m.indicatorMu.Lock()
	defer m.indicatorMu.Unlock()

	// Both draws are nil-checked because the lazy build below can fail: a
	// constructor that cannot reach the display server returns a raw nil, and
	// this is the one place a backend pointer is created outside startup
	// (../AGENTS.md).
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
				// The indicator surface shares the dedicated render lock and
				// runs its own event poller (keyboard capture stays disabled,
				// so it only pumps the surface's configure/frame events).
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

// HideIndicator takes an indicator off the screen and erases the badge it left
// behind.
//
// On Linux an indicator is a badge painted onto the one shared overlay surface
// rather than a window of its own, so the render component's own Hide is a
// no-op and clearing the rectangle it occupied is the whole of hiding it.
// Without this an indicator would stay painted until the entire overlay is
// hidden — which is the mode ending, not the indicator being turned off.
func (m *Manager) HideIndicator(indicator ports.Indicator) {
	m.Base.HideIndicator(indicator)

	// The virtual pointer draws into its own surface, so there is no rectangle
	// of it on the shared overlay to erase. Returning before the lock matters:
	// this one is hidden from the cursor-visibility path, which runs with the
	// mode handler's lock held, and renderMu is contended by every draw.
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

// cancelBackendAnimation stops any animation the attached backend is running,
// and reports whether there was a backend attached to cancel on.
//
// Destroy nils both backend pointers under renderMu, so this call reads them
// under it too — and then releases the lock before canceling, because the
// cancel waits for the animation goroutine to exit and that goroutine takes
// renderMu on every frame.
//
// Capturing each pointer once is what makes that read synchronized without
// widening the hold. Reading the field twice — once to nil-test it, once to
// make the call — would leave the nil test meaningless: a concurrent Destroy
// landing between the two hands a nil backend to cancelAnimation, which is
// promoted from the embedded sharedOverlay and so dereferences the receiver
// before any guard inside it can run.
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

// overlayScale returns the active backend's HiDPI UI scale. The X11 overlay
// enlarges fonts/geometry by this factor (Xft.dpi based); Wayland renders in
// logical units and scales via the compositor buffer, so it returns 1. The
// manager uses it to size badge clear rects consistently with what is drawn.
func (m *Manager) overlayScale() float64 {
	if m.x11 != nil {
		return m.x11.Scale()
	}

	return 1
}

// dispatchGridPointer hands the pointer state to the attached backend, which
// keeps it and repaints the grid surface with it.
//
// No animation is canceled first, the way DrawHintsWithStyle and
// HideHintSearchInput do: this repaints the same surface UpdateGridMatches
// already repaints on every keystroke and takes renderMu the same way, and the
// only animation painting *that* surface is the recursive-grid transition,
// which the frame coming up cleared and canceled before grid mode drew a cell.
// The mouse-action indicator animates too and is not a counter-example: it runs
// on its own backend instance behind indicatorRenderMu (DrawMouseActionIndicator
// below), so it shares neither this surface nor this lock.
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
// surface. It follows a Hide or a Clear, which take the whole surface away
// rather than one badge at a time, so there is nothing left to erase and the
// records would only describe a screen that is gone.
func (m *Manager) forgetBadgesLocked() {
	m.stickyBadgeVisible = false
	m.stickyBadgeRect = image.Rectangle{}
	m.modeIndicatorBadgeVisible = false
	m.modeIndicatorBadgeRect = image.Rectangle{}
	m.searchBadgeRect = image.Rectangle{}

	// The grid pointer is the one record kept on the backend rather than here,
	// because the pass that paints it is the backend's, but it describes the
	// same surface these do and goes with them.
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
		style.fontSize,
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
//
// Anything else is refused rather than drawn. The switch used to treat an
// unrecognized value as its default case, which drew a centered badge with no
// arrow — right for `center` by coincidence, and silent for a placement added
// to the vocabulary without a Linux branch: it would validate, be forced to
// exist on macOS, and then draw centered here with nothing failing anywhere.
//
// The empty string is not one of those: it is a style that reached the overlay
// before a configuration settled it, and it draws at the documented default —
// the same answer the macOS renderer gives it (`hintPlacementValue`), so a
// draw that beats the first Apply puts hints in the same place on both.
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

// hintTailEdge reports which badge edge the connector tail merges into, so the
// renderer can build the badge and tail as one outline. It returns hintTailNone
// when there is no arrow.
//
// It stays here rather than beside badge.PlaceHint because the values it
// answers are the C hint-badge renderer's, not geometry: the Windows backend
// draws the same arrow without a merged outline to describe.
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

// syncSublayerKeysLocked hands the surface the keys the subgrid is drawn with.
// Every draw that can put a subgrid on screen calls it, because a surface with
// no keys draws a subgrid the user cannot see but can still act on — the keys
// themselves are resolved once, by the config (config.ResolveSublayerKeys).
func (m *Manager) syncSublayerKeysLocked(target *string) {
	if m.GridOverlay() == nil {
		return
	}

	*target = m.GridOverlay().Config().SublayerKeys
}
