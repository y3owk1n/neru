//go:build linux

package overlay

import (
	"image"
	"os"
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

type linuxOverlayBackend string

const (
	linuxOverlayBackendUnknown        linuxOverlayBackend = "unknown"
	linuxOverlayBackendX11            linuxOverlayBackend = "x11"
	linuxOverlayBackendWaylandWlroots linuxOverlayBackend = "wayland-wlroots"
)

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
	linuxManager     *Manager
	linuxManagerOnce sync.Once
)

func NewOverlayManager(logger *zap.Logger) *Manager {
	manager := &Manager{
		logger:  logger,
		mode:    ModeIdle,
		subs:    make(map[uint64]func(StateChange), 4),
		backend: detectLinuxOverlayBackend(),
	}

	if manager.backend == linuxOverlayBackendX11 {
		manager.x11 = newX11Overlay(logger)
	} else if manager.backend == linuxOverlayBackendWaylandWlroots {
		manager.wlroots = newWlrootsOverlay(logger)
	}

	return manager
}

func Get() *Manager {
	return linuxManager
}

func Init(logger *zap.Logger) *Manager {
	linuxManagerOnce.Do(func() {
		linuxManager = NewOverlayManager(logger)
	})

	return linuxManager
}

func (m *Manager) Show() {
	if m.x11 != nil {
		m.x11.Show()
	} else if m.wlroots != nil {
		m.wlroots.Show()
	}
}

func (m *Manager) Hide() {
	if m.x11 != nil {
		m.x11.Hide()
	} else if m.wlroots != nil {
		m.wlroots.Hide()
	}
}

func (m *Manager) Clear() {
	if m.x11 != nil {
		m.x11.Clear()
	} else if m.wlroots != nil {
		m.wlroots.Clear()
	}
}

func (m *Manager) ResizeToActiveScreen() {
	if m.x11 != nil {
		m.x11.Resize()
	} else if m.wlroots != nil {
		m.wlroots.Resize()
	}
}

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

func (m *Manager) Subscribe(fn func(StateChange)) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	id := m.nextID
	m.subs[id] = fn
	return id
}

func (m *Manager) Unsubscribe(id uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subs, id)
}

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

func (m *Manager) Mode() Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mode
}

func (m *Manager) WindowPtr() unsafe.Pointer {
	if m.x11 != nil {
		return m.x11.WindowPtr()
	} else if m.wlroots != nil {
		return m.wlroots.WindowPtr()
	}

	return nil
}

func (m *Manager) UseHintOverlay(o *hints.Overlay) {
	m.hintOverlay = o
}

func (m *Manager) UseGridOverlay(o *grid.Overlay) {
	m.gridOverlay = o
}

func (m *Manager) UseModeIndicatorOverlay(o *modeindicator.Overlay) {
	m.modeIndicatorOverlay = o
}

func (m *Manager) UseStickyModifiersOverlay(o *stickyindicator.Overlay) {
	m.stickyModifiersOverlay = o
}

func (m *Manager) UseRecursiveGridOverlay(o *recursivegrid.Overlay) {
	m.recursiveGridOverlay = o
}

func (m *Manager) HintOverlay() *hints.Overlay                      { return m.hintOverlay }
func (m *Manager) GridOverlay() *grid.Overlay                       { return m.gridOverlay }
func (m *Manager) ModeIndicatorOverlay() *modeindicator.Overlay     { return m.modeIndicatorOverlay }
func (m *Manager) StickyModifiersOverlay() *stickyindicator.Overlay { return m.stickyModifiersOverlay }
func (m *Manager) RecursiveGridOverlay() *recursivegrid.Overlay     { return m.recursiveGridOverlay }

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
	default:
		return ports.FeatureCapability{
			Status: ports.FeatureStatusStub,
			Detail: "native Linux overlays are not implemented for this backend",
		}
	}
}

func (m *Manager) DrawHintsWithStyle(_ []*hints.Hint, _ hints.StyleMode) error {
	return derrors.New(derrors.CodeNotSupported, "overlay hints not implemented on linux")
}

func (m *Manager) DrawModeIndicator(x, y int) {
	if m.modeIndicatorOverlay == nil {
		return
	}
	mode := m.Mode()
	if mode == ModeIdle {
		return
	}

	if m.x11 != nil {
		m.x11.DrawBadge(x, y, string(mode), resolveModeIndicatorColors())
	} else if m.wlroots != nil {
		m.wlroots.DrawBadge(x, y, string(mode), resolveModeIndicatorColors())
	}
}

func (m *Manager) DrawStickyModifiersIndicator(x, y int, symbols string) {
	if m.stickyModifiersOverlay == nil || symbols == "" {
		return
	}

	if m.x11 != nil {
		m.x11.DrawBadge(x, y, symbols, resolveStickyIndicatorColors())
	} else if m.wlroots != nil {
		m.wlroots.DrawBadge(x, y, symbols, resolveStickyIndicatorColors())
	}
}

func (m *Manager) DrawGrid(g *domainGrid.Grid, input string, style grid.Style) error {
	if m.x11 != nil {
		m.x11.DrawGrid(g, input, style)
		return nil
	} else if m.wlroots != nil {
		m.wlroots.DrawGrid(g, input, style)
		return nil
	}

	return derrors.New(derrors.CodeNotSupported, "overlay grid not implemented on linux backend")
}

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

	return derrors.New(derrors.CodeNotSupported, "recursive grid overlay not implemented on linux backend")
}

func (m *Manager) UpdateGridMatches(prefix string) {
	if m.x11 != nil {
		m.x11.UpdateGridMatches(prefix)
	} else if m.wlroots != nil {
		m.wlroots.UpdateGridMatches(prefix)
	}
}

func (m *Manager) ShowSubgrid(cell *domainGrid.Cell, style grid.Style) {
	if m.x11 != nil {
		m.x11.ShowSubgrid(cell, style)
	} else if m.wlroots != nil {
		m.wlroots.ShowSubgrid(cell, style)
	}
}

func (m *Manager) SetHideUnmatched(hide bool) {
	if m.x11 != nil {
		m.x11.SetHideUnmatched(hide)
	} else if m.wlroots != nil {
		m.wlroots.SetHideUnmatched(hide)
	}
}

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
	case os.Getenv("DISPLAY") != "":
		return linuxOverlayBackendX11
	case os.Getenv("WAYLAND_DISPLAY") != "":
		return linuxOverlayBackendWaylandWlroots
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
