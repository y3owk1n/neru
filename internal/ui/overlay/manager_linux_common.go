//go:build linux

package overlay

import (
	"image"
	"math"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/modeindicator"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components/stickyindicator"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/platform"
	"github.com/y3owk1n/neru/internal/core/ports"
)

const (
	subgridBackground               uint32 = 0x40000000
	subgridCellBackground           uint32 = 0x10000000
	hexColorOpaque                  uint32 = 0xFFFFFFFF
	hexColorRepeatCount                    = 2
	hexColorLenShort                       = 3
	hexColorLenNoAlpha                     = 6
	hexColorLenFull                        = 8
	subgridCols                            = 3
	subgridRows                            = 3
	subgridHalfPixel                       = 0.5
	subgridFontScale                       = 0.7
	subgridLineWidth                       = 1
	keyboardChanBuffer                     = 64
	badgePaddingSides                      = 2
	autoPaddingHorizontalMultiplier        = 0.6
	autoPaddingVerticalMultiplier          = 0.35
	autoPaddingMinHorizontal               = 6
	autoPaddingMinVertical                 = 4
	textWidthMultiplier                    = 0.7
	textHeightMultiplier                   = 1.4
	centeredRectDivisor                    = 2
	centeredRectHalf                       = 0.5
	paddingMultiplier                      = 2
	subKeyPreviewPaddingBottom             = 4
)

type linuxOverlayBackend string

const (
	linuxOverlayBackendUnknown        linuxOverlayBackend = "unknown"
	linuxOverlayBackendX11            linuxOverlayBackend = "x11"
	linuxOverlayBackendWaylandWlroots linuxOverlayBackend = "wayland-wlroots"
	initialSubscriberCapacity                             = 4
)

// Manager manages overlay rendering on Linux.
type Manager struct {
	logger *zap.Logger

	mu     sync.RWMutex
	mode   Mode
	subs   map[uint64]func(StateChange)
	nextID uint64

	// renderMu serializes all rendering dispatch to the backend overlays.
	// On macOS the Objective-C bridge serializes via dispatch_async to the
	// main thread; on Linux we must do this ourselves because Cairo/X11/
	// Wayland calls are not thread-safe.
	renderMu sync.Mutex

	backend linuxOverlayBackend
	x11     *x11Overlay
	wlroots *wlrootsOverlay

	hintOverlay            *hints.Overlay
	gridOverlay            *grid.Overlay
	modeIndicatorOverlay   *modeindicator.Overlay
	recursiveGridOverlay   *recursivegrid.Overlay
	stickyModifiersOverlay *stickyindicator.Overlay
}

var (
	linuxManager      *Manager
	linuxManagerOnce  sync.Once
	wlrootsKeyboardCh chan string
)

// NewOverlayManager creates a new overlay Manager.
func NewOverlayManager(logger *zap.Logger) *Manager {
	manager := &Manager{
		logger:  logger,
		mode:    ModeIdle,
		subs:    make(map[uint64]func(StateChange), initialSubscriberCapacity),
		backend: detectLinuxOverlayBackend(),
	}

	switch manager.backend {
	case linuxOverlayBackendX11:
		manager.x11 = newX11Overlay(logger)
	case linuxOverlayBackendWaylandWlroots:
		manager.wlroots = newWlrootsOverlay(logger)
	case linuxOverlayBackendUnknown:
		return nil
	}

	return manager
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
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.Hide()
	} else if m.wlroots != nil {
		m.wlroots.Hide()
	}
}

// Clear clears the overlay content.
func (m *Manager) Clear() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.Clear()
	} else if m.wlroots != nil {
		m.wlroots.Clear()
	}
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

// SwitchTo switches to a new mode.
func (m *Manager) SwitchTo(next Mode) {
	m.mu.Lock()

	prev := m.mode
	if prev == next {
		m.mu.Unlock()

		return
	}

	m.mode = next

	m.mu.Unlock()

	m.publish(StateChange{prev: prev, next: next})
}

// Subscribe registers a callback for mode changes.
func (m *Manager) Subscribe(subFn func(StateChange)) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	id := m.nextID
	m.subs[id] = subFn

	return id
}

// Unsubscribe removes a callback registration.
func (m *Manager) Unsubscribe(id uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.subs, id)
}

// Destroy cleans up the overlay Manager.
func (m *Manager) Destroy() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.Destroy()
		m.x11 = nil
	}

	if m.wlroots != nil {
		m.wlroots.Destroy()
		m.wlroots = nil
	}
}

// Mode returns the current mode.
func (m *Manager) Mode() Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.mode
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

// UseHintOverlay sets the hints overlay.
func (m *Manager) UseHintOverlay(o *hints.Overlay) {
	m.hintOverlay = o
}

// UseGridOverlay sets the grid overlay.
func (m *Manager) UseGridOverlay(o *grid.Overlay) {
	m.gridOverlay = o
}

// UseModeIndicatorOverlay sets the mode indicator overlay.
func (m *Manager) UseModeIndicatorOverlay(o *modeindicator.Overlay) {
	m.modeIndicatorOverlay = o
}

// UseStickyModifiersOverlay sets the sticky modifiers overlay.
func (m *Manager) UseStickyModifiersOverlay(o *stickyindicator.Overlay) {
	m.stickyModifiersOverlay = o
}

// UseRecursiveGridOverlay sets the recursive grid overlay.
func (m *Manager) UseRecursiveGridOverlay(o *recursivegrid.Overlay) {
	m.recursiveGridOverlay = o
}

// HintOverlay returns the hints overlay.
func (m *Manager) HintOverlay() *hints.Overlay { return m.hintOverlay }

// GridOverlay returns the grid overlay.
func (m *Manager) GridOverlay() *grid.Overlay { return m.gridOverlay }

// ModeIndicatorOverlay returns the mode indicator overlay.
func (m *Manager) ModeIndicatorOverlay() *modeindicator.Overlay { return m.modeIndicatorOverlay }

// StickyModifiersOverlay returns the sticky modifiers overlay.
func (m *Manager) StickyModifiersOverlay() *stickyindicator.Overlay { return m.stickyModifiersOverlay }

// RecursiveGridOverlay returns the recursive grid overlay.
func (m *Manager) RecursiveGridOverlay() *recursivegrid.Overlay { return m.recursiveGridOverlay }

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
func (m *Manager) DrawHintsWithStyle(hintsSlice []*hints.Hint, style hints.StyleMode) error {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.DrawHints(hintsSlice, style)

		return nil
	}

	if m.wlroots != nil {
		m.wlroots.DrawHints(hintsSlice, style)

		return nil
	}

	return derrors.New(derrors.CodeNotSupported, "overlay hints not implemented on linux backend")
}

// DrawModeIndicator draws the mode indicator overlay.
func (m *Manager) DrawModeIndicator(posX, posY int) {
	if m.modeIndicatorOverlay == nil {
		return
	}

	mode := m.Mode()
	if mode == ModeIdle {
		return
	}

	label, colors, style, ok := resolveModeIndicatorAppearance(
		string(mode),
		m.modeIndicatorOverlay,
	)
	if !ok {
		return
	}

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.DrawBadge(posX, posY, label, colors, style)
	} else if m.wlroots != nil {
		m.wlroots.DrawBadge(posX, posY, label, colors, style)
	}
}

// DrawStickyModifiersIndicator draws the sticky modifiers indicator overlay.
func (m *Manager) DrawStickyModifiersIndicator(posX, posY int, symbols string) {
	if m.stickyModifiersOverlay == nil || symbols == "" {
		return
	}

	colors, style, ok := resolveStickyIndicatorAppearance(m.stickyModifiersOverlay)
	if !ok {
		return
	}

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

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
		// Pass sublayer keys from grid overlay config so subgrid labels match config.
		if m.gridOverlay != nil {
			cfg := m.gridOverlay.Config()

			keys := strings.TrimSpace(cfg.SublayerKeys)
			if keys == "" {
				keys = cfg.Characters
			}

			m.x11.sublayerKeys = strings.ToUpper(keys)
		}

		m.x11.DrawGrid(grid, input, style)

		return nil
	} else if m.wlroots != nil {
		// Pass sublayer keys from grid overlay config so subgrid labels match config.
		if m.gridOverlay != nil {
			cfg := m.gridOverlay.Config()

			keys := strings.TrimSpace(cfg.SublayerKeys)
			if keys == "" {
				keys = cfg.Characters
			}

			m.wlroots.sublayerKeys = strings.ToUpper(keys)
		}

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
	gridCols int,
	gridRows int,
	nextKeys string,
	nextGridCols int,
	nextGridRows int,
	style recursivegrid.Style,
	virtualPointer recursivegrid.VirtualPointerState,
) error {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		m.x11.DrawRecursiveGrid(bounds, depth, keys, gridCols, gridRows, style, virtualPointer)

		return nil
	} else if m.wlroots != nil {
		m.wlroots.DrawRecursiveGrid(bounds, depth, keys, gridCols, gridRows, style, virtualPointer)

		return nil
	}

	_ = nextKeys
	_ = nextGridCols
	_ = nextGridRows

	return derrors.New(
		derrors.CodeNotSupported,
		"recursive grid overlay not implemented on linux backend",
	)
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
func (m *Manager) ShowSubgrid(cell *domainGrid.Cell, style grid.Style) {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.x11 != nil {
		// Ensure sublayer keys are set from grid overlay config.
		if m.gridOverlay != nil {
			cfg := m.gridOverlay.Config()

			keys := strings.TrimSpace(cfg.SublayerKeys)
			if keys == "" {
				keys = cfg.Characters
			}

			m.x11.sublayerKeys = strings.ToUpper(keys)
		}

		m.x11.ShowSubgrid(cell, style)
	} else if m.wlroots != nil {
		// Ensure sublayer keys are set from grid overlay config.
		if m.gridOverlay != nil {
			cfg := m.gridOverlay.Config()

			keys := strings.TrimSpace(cfg.SublayerKeys)
			if keys == "" {
				keys = cfg.Characters
			}

			m.wlroots.sublayerKeys = strings.ToUpper(keys)
		}

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

func (m *Manager) publish(change StateChange) {
	m.mu.RLock()

	subs := make([]func(StateChange), 0, len(m.subs))
	for _, fn := range m.subs {
		subs = append(subs, fn)
	}

	m.mu.RUnlock()

	for _, fn := range subs {
		fn(change)
	}
}

// detectLinuxOverlayBackend delegates to the canonical
// platform.DetectLinuxBackend so that compositor-family detection (GNOME, KDE,
// wlroots, etc.) is consistent across all layers.
func detectLinuxOverlayBackend() linuxOverlayBackend {
	switch platform.DetectLinuxBackend() {
	case platform.BackendX11:
		return linuxOverlayBackendX11
	case platform.BackendWaylandWlroots:
		return linuxOverlayBackendWaylandWlroots
	case platform.BackendUnknown, platform.BackendWaylandGNOME,
		platform.BackendWaylandKDE, platform.BackendWaylandOther:
		return linuxOverlayBackendUnknown
	}
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
		background: parseHexColor(
			modeCfg.BackgroundColor.ForThemeWithOverride(
				cfg.UI.BackgroundColor,
				theme,
				config.ModeIndicatorBackgroundColorLight,
				config.ModeIndicatorBackgroundColorDark,
			),
		),
		border: parseHexColor(
			modeCfg.BorderColor.ForThemeWithOverride(
				cfg.UI.BorderColor,
				theme,
				config.ModeIndicatorBorderColorLight,
				config.ModeIndicatorBorderColorDark,
			),
		),
		text: parseHexColor(
			modeCfg.TextColor.ForThemeWithOverride(
				cfg.UI.TextColor,
				theme,
				config.ModeIndicatorTextColorLight,
				config.ModeIndicatorTextColorDark,
			),
		),
	}

	style := overlayBadgeStyle{
		fontFamily:  cfg.UI.FontFamily,
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
		background: parseHexColor(
			cfg.BackgroundColor.ForTheme(
				theme,
				config.StickyModifiersBackgroundColorLight,
				config.StickyModifiersBackgroundColorDark,
			),
		),
		border: parseHexColor(
			cfg.BorderColor.ForTheme(
				theme,
				config.StickyModifiersBorderColorLight,
				config.StickyModifiersBorderColorDark,
			),
		),
		text: parseHexColor(
			cfg.TextColor.ForTheme(
				theme,
				config.StickyModifiersTextColorLight,
				config.StickyModifiersTextColorDark,
			),
		),
	}

	style := overlayBadgeStyle{
		fontFamily:  cfg.FontFamily,
		fontSize:    float64(max(cfg.FontSize, 1)),
		paddingX:    cfg.PaddingX,
		paddingY:    cfg.PaddingY,
		borderWidth: float64(max(cfg.BorderWidth, 0)),
		offsetX:     cfg.IndicatorXOffset,
		offsetY:     cfg.IndicatorYOffset,
	}

	return colors, style, true
}

func resolveAutoPadding(fontSize float64, padding int, horizontal bool) int {
	if padding >= 0 {
		return padding
	}

	if horizontal {
		return max(int(fontSize*autoPaddingHorizontalMultiplier), autoPaddingMinHorizontal)
	}

	return max(int(fontSize*autoPaddingVerticalMultiplier), autoPaddingMinVertical)
}

func estimateTextWidth(text string, fontSize float64) int {
	return int(math.Ceil(float64(len([]rune(text))) * fontSize * textWidthMultiplier))
}

func estimateTextHeight(fontSize float64) int {
	return int(math.Ceil(fontSize * textHeightMultiplier))
}

func centeredRect(cell image.Rectangle, width, height int) image.Rectangle {
	centerX := cell.Min.X + cell.Dx()/centeredRectDivisor
	centerY := cell.Min.Y + cell.Dy()/centeredRectDivisor

	return image.Rect(
		centerX-width/centeredRectDivisor,
		centerY-height/centeredRectDivisor,
		centerX-width/centeredRectDivisor+width,
		centerY-height/centeredRectDivisor+height,
	)
}

func shouldShowSubKeyPreview(
	cell image.Rectangle,
	style recursivegrid.Style,
) bool {
	if !style.SubKeyPreview {
		return false
	}

	if style.SubKeyPreviewAutohideMultiplier <= 0 {
		return true
	}

	threshold := style.SubKeyPreviewFontSize * style.SubKeyPreviewAutohideMultiplier

	return float64(cell.Dx()) >= threshold && float64(cell.Dy()) >= threshold
}

func parseHexColor(value string) uint32 {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	switch len(value) {
	case hexColorLenShort:
		value = "FF" + strings.Repeat(string(value[0]), hexColorRepeatCount) +
			strings.Repeat(string(value[1]), hexColorRepeatCount) +
			strings.Repeat(string(value[2]), hexColorRepeatCount)
	case hexColorLenNoAlpha:
		value = "FF" + value
	case hexColorLenFull:
	default:
		return hexColorOpaque
	}

	parsed, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return hexColorOpaque
	}

	return uint32(parsed)
}
