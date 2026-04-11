//go:build linux

package overlay

import (
	"image"
	"os"
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
	"github.com/y3owk1n/neru/internal/core/ports"
)

const (
	gridFillColor         uint32 = 0x18000000
	gridMatchedFillColor  uint32 = 0x66465FBC
	gridMatchedTextColor  uint32 = 0xFFF8FAFF
	subgridBackground     uint32 = 0x40000000
	subgridCellBackground uint32 = 0x10000000
	badgePaddingX                = 10
	badgeFontSize                = 14.0
	badgeCharWidth               = 9
	badgeHeight                  = 24
	hexColorOpaque        uint32 = 0xFFFFFFFF
	hexColorRepeatCount          = 2
	subgridCols                  = 3
	subgridRows                  = 3
	subgridHalfPixel             = 0.5
	subgridFontScale             = 0.7
	subgridLineWidth             = 1
	keyboardChanBuffer           = 64
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
	if m.x11 != nil {
		m.x11.Show()
	} else if m.wlroots != nil {
		m.wlroots.Show()
	}
}

// Hide hides the overlay.
func (m *Manager) Hide() {
	if m.x11 != nil {
		m.x11.Hide()
	} else if m.wlroots != nil {
		m.wlroots.Hide()
	}
}

// Clear clears the overlay content.
func (m *Manager) Clear() {
	if m.x11 != nil {
		m.x11.Clear()
	} else if m.wlroots != nil {
		m.wlroots.Clear()
	}
}

// ResizeToActiveScreen resizes the overlay to the active screen.
func (m *Manager) ResizeToActiveScreen() {
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

// DrawHintsWithStyle is a no-op on Linux.
func (m *Manager) DrawHintsWithStyle(_ []*hints.Hint, _ hints.StyleMode) error {
	return derrors.New(derrors.CodeNotSupported, "overlay hints not implemented on linux")
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

	if m.x11 != nil {
		m.x11.DrawBadge(posX, posY, string(mode), resolveModeIndicatorColors())
	} else if m.wlroots != nil {
		m.wlroots.DrawBadge(posX, posY, string(mode), resolveModeIndicatorColors())
	}
}

// DrawStickyModifiersIndicator draws the sticky modifiers indicator overlay.
func (m *Manager) DrawStickyModifiersIndicator(posX, posY int, symbols string) {
	if m.stickyModifiersOverlay == nil || symbols == "" {
		return
	}

	if m.x11 != nil {
		m.x11.DrawBadge(posX, posY, symbols, resolveStickyIndicatorColors())
	} else if m.wlroots != nil {
		m.wlroots.DrawBadge(posX, posY, symbols, resolveStickyIndicatorColors())
	}
}

// DrawGrid draws the grid overlay.
func (m *Manager) DrawGrid(grid *domainGrid.Grid, input string, style grid.Style) error {
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
	if m.x11 != nil {
		m.x11.UpdateGridMatches(prefix)
	} else if m.wlroots != nil {
		m.wlroots.UpdateGridMatches(prefix)
	}
}

// ShowSubgrid shows the subgrid overlay.
func (m *Manager) ShowSubgrid(cell *domainGrid.Cell, style grid.Style) {
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

func detectLinuxOverlayBackend() linuxOverlayBackend {
	switch {
	case os.Getenv("WAYLAND_DISPLAY") != "":
		return linuxOverlayBackendWaylandWlroots
	case os.Getenv("DISPLAY") != "":
		return linuxOverlayBackendX11
	default:
		return linuxOverlayBackendUnknown
	}
}

type overlayColors struct {
	background uint32
	border     uint32
	text       uint32
}

func resolveModeIndicatorColors() overlayColors {
	return overlayColors{
		background: parseHexColor(config.ModeIndicatorBackgroundColorLight),
		border:     parseHexColor(config.ModeIndicatorBorderColorLight),
		text:       parseHexColor(config.ModeIndicatorTextColorLight),
	}
}

func resolveStickyIndicatorColors() overlayColors {
	return overlayColors{
		background: parseHexColor(config.StickyModifiersBackgroundColorLight),
		border:     parseHexColor(config.StickyModifiersBorderColorLight),
		text:       parseHexColor(config.StickyModifiersTextColorLight),
	}
}

func parseHexColor(value string) uint32 {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	switch len(value) {
	case 3:
		value = "FF" + strings.Repeat(string(value[0]), hexColorRepeatCount) +
			strings.Repeat(string(value[1]), hexColorRepeatCount) +
			strings.Repeat(string(value[2]), hexColorRepeatCount)
	case 6:
		value = "FF" + value
	case 8:
	default:
		return hexColorOpaque
	}

	parsed, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return hexColorOpaque
	}

	return uint32(parsed)
}
