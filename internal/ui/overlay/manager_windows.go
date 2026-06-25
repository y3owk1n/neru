//go:build windows

// internal/ui/overlay/manager_windows.go
// Windows overlay manager backed by a layered Win32 HWND and GDI rendering of
// grid, hints, and recursive-grid overlays.
// Does not implement keyboard capture (handled by the low-level keyboard hook).

package overlay

import (
	"image"
	"strings"
	"sync"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components/grid"
	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/components/modeindicator"
	"github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	"github.com/y3owk1n/neru/internal/app/components/stickyindicator"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
)

const winInitialSubscriberCapacity = 4

// Manager manages overlay rendering on Windows.
type Manager struct {
	logger *zap.Logger

	mu     sync.RWMutex
	mode   Mode
	subs   map[uint64]func(StateChange)
	nextID uint64

	renderMu sync.Mutex
	win      *winOverlay

	hintOverlay            *hints.Overlay
	gridOverlay            *grid.Overlay
	modeIndicatorOverlay   *modeindicator.Overlay
	recursiveGridOverlay   *recursivegrid.Overlay
	stickyModifiersOverlay *stickyindicator.Overlay
}

var (
	windowsManager     *Manager
	windowsManagerOnce sync.Once
)

// NewOverlayManager creates a new overlay Manager.
func NewOverlayManager(logger *zap.Logger) *Manager {
	return &Manager{
		logger: logger,
		mode:   ModeIdle,
		subs:   make(map[uint64]func(StateChange), winInitialSubscriberCapacity),
		win:    newWinOverlay(logger),
	}
}

// Get returns the global overlay Manager.
func Get() *Manager {
	return windowsManager
}

// Init initializes the global overlay Manager.
func Init(logger *zap.Logger) *Manager {
	windowsManagerOnce.Do(func() {
		windowsManager = NewOverlayManager(logger)
	})

	return windowsManager
}

// Show displays the overlay.
func (m *Manager) Show() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	m.ensureWinOverlayLocked()

	if m.win == nil {
		if m.logger != nil {
			m.logger.Error("manager Show aborted, overlay backend is nil")
		}

		return
	}

	m.win.Show()
}

// Hide hides the overlay.
func (m *Manager) Hide() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.win != nil {
		m.win.Hide()
	}
}

// Clear clears the overlay surface.
func (m *Manager) Clear() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.win != nil {
		m.win.Clear()
	}
}

// ResizeToActiveScreen resizes the overlay to the active monitor.
func (m *Manager) ResizeToActiveScreen() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	m.ensureWinOverlayLocked()

	if m.win != nil {
		m.win.Resize()
	}
}

// ActiveScreenBounds returns the overlay window bounds in screen coordinates.
func (m *Manager) ActiveScreenBounds() (image.Rectangle, bool) {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.win == nil {
		return image.Rectangle{}, false
	}

	return m.win.screenBounds()
}

// SwitchTo switches overlay mode and notifies subscribers.
func (m *Manager) SwitchTo(next Mode) {
	m.mu.Lock()
	prev := m.mode
	m.mode = next
	m.mu.Unlock()

	if prev != next {
		m.publish(StateChange{prev: prev, next: next})
	}
}

// Subscribe registers a callback for overlay mode changes.
func (m *Manager) Subscribe(subFn func(StateChange)) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	id := m.nextID
	m.subs[id] = subFn

	return id
}

// Unsubscribe removes a callback.
func (m *Manager) Unsubscribe(id uint64) {
	m.mu.Lock()
	delete(m.subs, id)
	m.mu.Unlock()
}

// Destroy destroys overlay resources.
func (m *Manager) Destroy() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.win != nil {
		m.win.Destroy()
		m.win = nil
	}
}

// Mode returns the current overlay mode.
func (m *Manager) Mode() Mode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.mode
}

// WindowPtr returns the native overlay window handle.
func (m *Manager) WindowPtr() unsafe.Pointer {
	if m.win == nil {
		return nil
	}

	return m.win.WindowPtr()
}

// WaylandKeyboardChannel returns nil on Windows.
func (m *Manager) WaylandKeyboardChannel() <-chan string {
	return nil
}

// UseHintOverlay sets the hints overlay component.
func (m *Manager) UseHintOverlay(o *hints.Overlay) { m.hintOverlay = o }

// UseGridOverlay sets the grid overlay component.
func (m *Manager) UseGridOverlay(o *grid.Overlay) { m.gridOverlay = o }

// UseModeIndicatorOverlay sets the mode-indicator overlay component.
func (m *Manager) UseModeIndicatorOverlay(o *modeindicator.Overlay) {
	m.modeIndicatorOverlay = o
}

// UseStickyModifiersOverlay sets the sticky-modifiers overlay component.
func (m *Manager) UseStickyModifiersOverlay(o *stickyindicator.Overlay) {
	m.stickyModifiersOverlay = o
}

// UseRecursiveGridOverlay sets the recursive-grid overlay component.
func (m *Manager) UseRecursiveGridOverlay(o *recursivegrid.Overlay) {
	m.recursiveGridOverlay = o
}

// HintOverlay returns the hints overlay component.
func (m *Manager) HintOverlay() *hints.Overlay { return m.hintOverlay }

// GridOverlay returns the grid overlay component.
func (m *Manager) GridOverlay() *grid.Overlay { return m.gridOverlay }

// ModeIndicatorOverlay returns the mode-indicator overlay component.
func (m *Manager) ModeIndicatorOverlay() *modeindicator.Overlay {
	return m.modeIndicatorOverlay
}

// StickyModifiersOverlay returns the sticky-modifiers overlay component.
func (m *Manager) StickyModifiersOverlay() *stickyindicator.Overlay {
	return m.stickyModifiersOverlay
}

// RecursiveGridOverlay returns the recursive-grid overlay component.
func (m *Manager) RecursiveGridOverlay() *recursivegrid.Overlay {
	return m.recursiveGridOverlay
}

// OverlayCapabilities reports Windows overlay support.
func (m *Manager) OverlayCapabilities() ports.FeatureCapability {
	if m.win != nil && m.win.Healthy() {
		return ports.FeatureCapability{
			Status: ports.FeatureStatusSupported,
			Detail: "native Windows overlays available via layered Win32 window + GDI",
		}
	}

	return ports.FeatureCapability{
		Status: ports.FeatureStatusStub,
		Detail: "Windows overlay backend failed to initialize (interactive desktop required)",
	}
}

// DrawHintsWithStyle draws the hints overlay using the Windows GDI backend.
func (m *Manager) DrawHintsWithStyle(hintsSlice []*hints.Hint, style hints.StyleMode) error {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	m.ensureWinOverlayLocked()

	if m.win == nil {
		return derrors.New(
			derrors.CodeNotSupported,
			"overlay hints not implemented on windows backend",
		)
	}

	// Shared activation may draw before the resize; enforce monitor bounds here.
	m.win.Resize()
	m.win.DrawHints(hintsSlice, style)

	return nil
}

// DrawHintSearchInput is a no-op on Windows; the hint search box is not rendered.
func (m *Manager) DrawHintSearchInput(
	_ string,
	_ int,
	_ hints.SearchInputFrame,
	_ hints.SearchInputStyle,
) error {
	return nil
}

// HideHintSearchInput is a no-op on Windows.
func (m *Manager) HideHintSearchInput() {}

// DrawModeIndicator is a no-op on Windows.
func (m *Manager) DrawModeIndicator(_, _ int) {}

// DrawStickyModifiersIndicator is a no-op on Windows.
func (m *Manager) DrawStickyModifiersIndicator(_, _ int, _ string) {}

// DrawMouseActionIndicator is a no-op on Windows.
func (m *Manager) DrawMouseActionIndicator(_ image.Point, _ ports.MouseActionIndicatorStyle) {}

// DrawGrid draws the grid overlay.
func (m *Manager) DrawGrid(gridValue *domainGrid.Grid, input string, style grid.Style) error {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	m.ensureWinOverlayLocked()

	if m.win == nil {
		if m.logger != nil {
			m.logger.Error("manager DrawGrid aborted, overlay backend is nil")
		}

		return derrors.New(
			derrors.CodeNotSupported,
			"overlay grid not implemented on windows backend",
		)
	}

	// Shared activation may call draw before resize; enforce monitor bounds here.
	m.win.Resize()

	if m.logger != nil {
		cellCount := 0
		if gridValue != nil {
			cellCount = len(gridValue.AllCells())
		}

		m.logger.Debug("manager DrawGrid", zap.Int("cells", cellCount))
	}

	if m.gridOverlay != nil {
		cfg := m.gridOverlay.Config()

		keys := strings.TrimSpace(cfg.SublayerKeys)
		if keys == "" {
			keys = cfg.Characters
		}

		m.win.sublayerKeys = strings.ToUpper(keys)
	}

	m.win.DrawGrid(gridValue, input, style)

	return nil
}

// DrawRecursiveGrid draws the recursive-grid overlay using the Windows GDI backend.
// The next-depth preview parameters are folded into the style by the renderer,
// so they are unused here (matching the cross-platform software renderer).
func (m *Manager) DrawRecursiveGrid(
	bounds image.Rectangle,
	_ int,
	keys string,
	gridCols int,
	gridRows int,
	_ string,
	_ int,
	_ int,
	style recursivegrid.Style,
	virtualPointer recursivegrid.VirtualPointerState,
) error {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	m.ensureWinOverlayLocked()

	if m.win == nil {
		return derrors.New(
			derrors.CodeNotSupported,
			"recursive grid overlay not implemented on windows backend",
		)
	}

	// Shared activation may draw before the resize; enforce monitor bounds here.
	m.win.Resize()
	m.win.DrawRecursiveGrid(bounds, keys, gridCols, gridRows, style, virtualPointer)

	return nil
}

// UpdateGridMatches updates prefix highlighting for the grid overlay.
func (m *Manager) UpdateGridMatches(prefix string) {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.win != nil {
		m.win.UpdateGridMatches(prefix)
	}
}

// ShowSubgrid shows a subgrid inside the selected cell.
func (m *Manager) ShowSubgrid(cell *domainGrid.Cell, style grid.Style) {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.win != nil {
		m.win.ShowSubgrid(cell, style)
	}
}

// SetHideUnmatched toggles hiding unmatched grid cells.
func (m *Manager) SetHideUnmatched(hide bool) {
	if m.win != nil {
		m.win.SetHideUnmatched(hide)
	}
}

// SetSharingType is a no-op on Windows.
func (m *Manager) SetSharingType(_ bool) {}

// Flush pushes any batched overlay draws to the layered window.
func (m *Manager) Flush() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.win != nil {
		m.win.flushOverlay("manager-flush")
	}
}

// SetKeyboardCaptureEnabled is a no-op on Windows; the low-level keyboard hook
// manages capture directly and has no scroll-passthrough toggle.
func (m *Manager) SetKeyboardCaptureEnabled(_ bool) {}

func (m *Manager) publish(change StateChange) {
	for _, sub := range m.subs {
		sub(change)
	}
}

func (m *Manager) ensureWinOverlayLocked() {
	if m.win != nil && m.win.Healthy() {
		return
	}

	if m.win != nil {
		m.win.Destroy()
		m.win = nil
	}

	m.win = newWinOverlay(m.logger)
	if m.win == nil && m.logger != nil {
		m.logger.Error("Windows overlay window is unavailable; grid overlay cannot render")
	}
}
