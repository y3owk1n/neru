//go:build darwin

package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#include "../../platform/darwin/overlay.h"
#include <stdlib.h>
*/
import "C"

import (
	"image"
	"sync"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/virtualpointer"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	// NSWindowSharingNone represents NSWindowSharingNone (0) - hidden from screen sharing.
	NSWindowSharingNone = 0
	// NSWindowSharingReadOnly represents NSWindowSharingReadOnly (1) - visible in screen sharing.
	NSWindowSharingReadOnly = 1
)

func boolToInt(v bool) int {
	if v {
		return 1
	}

	return 0
}

// Manager manages multiple overlay windows.
type Manager struct {
	manager.Base

	window C.OverlayWindow
	logger *zap.Logger

	// shareMu guards hideInScreenShare. It is deliberately separate from the
	// Base mode/subscriber mutex so SetSharingType can never contend with
	// SwitchTo/Subscribe callers.
	shareMu           sync.RWMutex
	hideInScreenShare bool
}

var (
	instance *Manager
	once     sync.Once
)

// Init initializes the singleton overlay manager with a new overlay window.
func Init(logger *zap.Logger) *Manager {
	once.Do(func() {
		window := C.NeruCreateOverlayWindow()
		instance = &Manager{
			Base:   manager.NewBase(logger),
			window: window,
			logger: logger,
		}
	})

	return instance
}

// Get returns the singleton instance of the overlay instance.
func Get() *Manager {
	return instance
}

// WindowPtr returns the window pointer.
func (m *Manager) WindowPtr() unsafe.Pointer {
	return unsafe.Pointer(m.window)
}

// Ensure the manager keeps declaring the optional headless capability. Its own
// BuildComponents reads Headless directly, so drift there fails to compile;
// this pins the shared spelling every backend answers headlessness with.
var _ manager.HeadlessReporter = (*Manager)(nil)

// Headless reports whether the overlay window failed to be created, leaving
// nothing for the render overlays to draw on.
func (m *Manager) Headless() bool {
	return m == nil || m.window == nil
}

// BuildComponents constructs the render components this manager draws through,
// on the window it owns, then hands the virtual pointer the screen-share
// visibility the manager is already holding — it has its own window, so the
// state the other overlays were given does not otherwise reach it.
func (m *Manager) BuildComponents(
	cfg *config.Config,
	theme config.ThemeProvider,
) (manager.Components, error) {
	if m == nil {
		return manager.Components{}, nil
	}

	built, err := m.Base.BuildComponents(manager.ComponentSpec{
		Config:   cfg,
		Theme:    theme,
		Logger:   m.logger,
		Window:   m.WindowPtr(),
		Headless: m.Headless(),
	})
	if err != nil {
		return manager.Components{}, err
	}

	m.UseVirtualPointerOverlay(built.VirtualPointer)

	return built, nil
}

// WaylandKeyboardChannel returns nil for macOS (not applicable).
func (m *Manager) WaylandKeyboardChannel() <-chan string {
	return nil
}

// Logger returns the logger.
func (m *Manager) Logger() *zap.Logger {
	return m.logger
}

// OverlayCapabilities reports current darwin overlay support.
func (m *Manager) OverlayCapabilities() ports.FeatureCapability {
	return ports.FeatureCapability{
		Status: ports.FeatureStatusSupported,
		Detail: "native darwin overlays are available",
	}
}

// Show shows the overlay window.
func (m *Manager) Show() {
	C.NeruShowOverlayWindow(m.window)
}

// Hide hides the overlay window.
func (m *Manager) Hide() {
	C.NeruHideOverlayWindow(m.window)

	if m.ModeIndicatorOverlay() != nil {
		m.ModeIndicatorOverlay().Hide()
	}

	if m.StickyModifiersOverlay() != nil {
		m.StickyModifiersOverlay().Hide()
	}

	if m.RecursiveGridOverlay() != nil {
		m.RecursiveGridOverlay().Hide()
	}
}

// Clear clears the overlay window.
func (m *Manager) Clear() {
	C.NeruClearOverlay(m.window)
	if m.GridOverlay() != nil {
		m.GridOverlay().Clear()
	}
	if m.HintOverlay() != nil {
		m.HintOverlay().Clear()
	}

	if m.ModeIndicatorOverlay() != nil {
		m.ModeIndicatorOverlay().Clear()
	}

	if m.StickyModifiersOverlay() != nil {
		m.StickyModifiersOverlay().Clear()
	}

	if m.RecursiveGridOverlay() != nil {
		m.RecursiveGridOverlay().Clear()
	}
}

// ClearCache is a no-op on Darwin; each overlay owns its own NSWindow and
// does not retain stale cross-mode cache state.
func (m *Manager) ClearCache() {}

// ResizeToActiveScreen resizes the overlay window to the active screen.
func (m *Manager) ResizeToActiveScreen() {
	C.NeruResizeOverlayToActiveScreen(m.window)

	if m.ModeIndicatorOverlay() != nil {
		m.ModeIndicatorOverlay().ResizeToActiveScreen()
	}

	if m.StickyModifiersOverlay() != nil {
		m.StickyModifiersOverlay().ResizeToActiveScreen()
	}
}

// SetKeyboardCaptureEnabled is a no-op on Darwin.
func (m *Manager) SetKeyboardCaptureEnabled(_ bool) {}

// SetActiveScreenOrigin is a no-op on Darwin. Each screen has its own overlay
// window positioned at the screen origin, so overlay content already uses
// window-local coordinates and needs no global translation.
func (m *Manager) SetActiveScreenOrigin(_ image.Point) {}

// Destroy destroys the overlay window and cleans up all overlay resources.
func (m *Manager) Destroy() {
	// Clean up Go-side resources (callbackManager, styleCache, labelCache) for
	// overlays that share the manager's window. We call Cleanup() instead of
	// Destroy() because the shared window is destroyed below — calling each
	// overlay's Destroy() would double-destroy the same native window.
	if m.HintOverlay() != nil {
		m.HintOverlay().Cleanup()
		m.UseHintOverlay(nil)
	}
	if m.GridOverlay() != nil {
		m.GridOverlay().Cleanup()
		m.UseGridOverlay(nil)
	}
	if m.RecursiveGridOverlay() != nil {
		m.RecursiveGridOverlay().Cleanup()
		m.UseRecursiveGridOverlay(nil)
	}

	// Mode indicator owns its own window, so use full Destroy().
	if m.ModeIndicatorOverlay() != nil {
		m.ModeIndicatorOverlay().Destroy()
		m.UseModeIndicatorOverlay(nil)
	}

	// Sticky modifiers indicator owns its own window, so use full Destroy().
	if m.StickyModifiersOverlay() != nil {
		m.StickyModifiersOverlay().Destroy()
		m.UseStickyModifiersOverlay(nil)
	}

	if m.window != nil {
		C.NeruDestroyOverlayWindow(m.window)
		m.window = nil
	}
}

// UseVirtualPointerOverlay sets the cursor-following virtual pointer overlay
// renderer, propagating the current screen-share visibility to it. Overrides
// the Base method; the other renderers are registered through Base directly.
func (m *Manager) UseVirtualPointerOverlay(overlay *virtualpointer.Overlay) {
	m.Base.UseVirtualPointerOverlay(overlay)

	m.shareMu.RLock()
	hideInScreenShare := m.hideInScreenShare
	m.shareMu.RUnlock()

	if overlay != nil && hideInScreenShare {
		overlay.SetSharingType(true)
	}
}

// DrawHintsWithStyle draws hints with the specified style using the hint overlay renderer.
//
// A placement the renderer cannot draw is reported as CodeNotSupported (#1333),
// and the caller degrades on that code — so it travels unwrapped, the way the
// overlay adapter already passes one on. Wrapping it as an overlay failure
// would report a degradation as a fault.
func (m *Manager) DrawHintsWithStyle(hs []*hints.Hint, style hints.StyleMode) error {
	if m.HintOverlay() == nil {
		return nil
	}
	drawHintsErr := m.HintOverlay().DrawHintsWithStyle(hs, style)
	if drawHintsErr != nil {
		if derrors.IsNotSupported(drawHintsErr) {
			return drawHintsErr
		}

		return derrors.Wrap(
			drawHintsErr,
			derrors.CodeOverlayFailed,
			"failed to draw hints with style",
		)
	}

	return nil
}

// DrawHintSearchInput draws the hints search input using the hint overlay renderer.
func (m *Manager) DrawHintSearchInput(
	query string,
	resultCount int,
	frame hints.SearchInputFrame,
	style hints.SearchInputStyle,
) error {
	if m.HintOverlay() == nil {
		return nil
	}

	err := m.HintOverlay().DrawSearchInput(query, resultCount, frame, style)
	if err != nil {
		return derrors.Wrap(err, derrors.CodeOverlayFailed, "failed to draw hint search input")
	}

	return nil
}

// HideHintSearchInput hides the hints search input.
func (m *Manager) HideHintSearchInput() {
	if m.HintOverlay() == nil {
		return
	}

	m.HintOverlay().HideSearchInput()
}

// DrawModeIndicator renders a mode indicator using the shared overlay renderer.
func (m *Manager) DrawModeIndicator(xCoordinate, yCoordinate int) {
	if m.ModeIndicatorOverlay() == nil {
		return
	}

	mode := m.Mode()
	if mode == manager.ModeIdle {
		return
	}

	m.ModeIndicatorOverlay().DrawModeIndicator(string(mode), xCoordinate, yCoordinate)
}

// DrawStickyModifiersIndicator renders the sticky modifiers indicator using the sticky modifiers overlay renderer.
func (m *Manager) DrawStickyModifiersIndicator(xCoordinate, yCoordinate int, symbols string) {
	if m.StickyModifiersOverlay() == nil {
		return
	}

	mode := m.Mode()
	if mode == manager.ModeIdle {
		return
	}

	m.StickyModifiersOverlay().Draw(xCoordinate, yCoordinate, symbols)
}

// DrawVirtualPointer renders the cursor-following virtual pointer overlay.
func (m *Manager) DrawVirtualPointer(xCoordinate, yCoordinate, size int, fillColor string) {
	if m.VirtualPointerOverlay() == nil {
		return
	}

	m.VirtualPointerOverlay().Draw(xCoordinate, yCoordinate, size, fillColor)
}

// DrawMouseActionIndicator renders a transient mouse action indicator in its own native window.
func (m *Manager) DrawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
	m.shareMu.RLock()
	hideInScreenShare := m.hideInScreenShare
	m.shareMu.RUnlock()

	backgroundColor := C.CString(style.BackgroundColor)
	borderColor := C.CString(style.BorderColor)
	shape := C.CString(style.Shape)
	easing := C.CString(style.Easing)

	defer C.free(unsafe.Pointer(backgroundColor))
	defer C.free(unsafe.Pointer(borderColor))
	defer C.free(unsafe.Pointer(shape))
	defer C.free(unsafe.Pointer(easing))

	C.NeruShowMouseActionIndicator(
		C.CGPoint{x: C.double(point.X), y: C.double(point.Y)},
		C.MouseActionIndicatorStyle{
			size:              C.int(style.Size),
			borderWidth:       C.int(style.BorderWidth),
			backgroundColor:   backgroundColor,
			borderColor:       borderColor,
			shape:             shape,
			durationMS:        C.int(style.DurationMS),
			startScale:        C.double(style.StartScale),
			endScale:          C.double(style.EndScale),
			startOpacity:      C.double(style.StartOpacity),
			endOpacity:        C.double(style.EndOpacity),
			easing:            easing,
			hideInScreenShare: C.int(boolToInt(style.HideInScreenShare || hideInScreenShare)),
		},
	)
}

// DrawGrid renders a grid with the specified style using the grid overlay renderer.
func (m *Manager) DrawGrid(g *domainGrid.Grid, input string, style grid.Style) error {
	if m.GridOverlay() == nil {
		return nil
	}
	drawGridErr := m.GridOverlay().DrawGrid(g, input, style)
	if drawGridErr != nil {
		return derrors.Wrap(drawGridErr, derrors.CodeOverlayFailed, "failed to draw grid")
	}

	return nil
}

// DrawRecursiveGrid renders a recursive-grid with the specified style using the recursive-grid overlay renderer.
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
	if m.RecursiveGridOverlay() == nil {
		return nil
	}
	drawRecursiveGridErr := m.RecursiveGridOverlay().DrawRecursiveGrid(
		bounds,
		depth,
		keys,
		dims,
		nextKeys,
		nextDims,
		style,
		virtualPointer,
	)
	if drawRecursiveGridErr != nil {
		return derrors.Wrap(
			drawRecursiveGridErr,
			derrors.CodeOverlayFailed,
			"failed to draw recursive-grid",
		)
	}

	return nil
}

// UpdateGridMatches updates the grid matches with the specified prefix.
func (m *Manager) UpdateGridMatches(prefix string) {
	if m.GridOverlay() == nil {
		return
	}
	m.GridOverlay().UpdateMatches(prefix)
}

// ShowSubgrid shows a subgrid for the specified cell.
func (m *Manager) ShowSubgrid(cell *domainGrid.Cell, style grid.Style) {
	if m.GridOverlay() == nil {
		return
	}
	m.GridOverlay().ShowSubgrid(cell, style)
}

// SetHideUnmatched sets whether to hide unmatched cells.
func (m *Manager) SetHideUnmatched(hide bool) {
	if m.GridOverlay() == nil {
		return
	}
	m.GridOverlay().SetHideUnmatched(hide)
}

// Flush is a no-op on macOS — indicator overlays use independent windows
// that are positioned dynamically per draw call.
func (m *Manager) Flush() {}

// SetSharingType sets the window sharing type for screen sharing visibility.
// When hide is true, sets NSWindowSharingNone (hidden from screen share).
// When hide is false, sets NSWindowSharingReadOnly (visible in screen share).
//
// shareMu is dedicated to this flag, so holding it across the CGo call cannot
// contend with SwitchTo/Subscribe callers (and the C function dispatches
// asynchronously anyway).
func (m *Manager) SetSharingType(hide bool) {
	m.shareMu.Lock()
	defer m.shareMu.Unlock()

	m.hideInScreenShare = hide

	sharingType := C.int(NSWindowSharingReadOnly)
	if hide {
		sharingType = C.int(NSWindowSharingNone)
	}

	C.NeruSetOverlaySharingType(m.window, sharingType)

	// Also update grid, recursive_grid, and mode indicator overlay windows if they exist
	if m.GridOverlay() != nil {
		m.GridOverlay().SetSharingType(hide)
	}
	if m.RecursiveGridOverlay() != nil {
		m.RecursiveGridOverlay().SetSharingType(hide)
	}
	if m.ModeIndicatorOverlay() != nil {
		m.ModeIndicatorOverlay().SetSharingType(hide)
	}

	if m.StickyModifiersOverlay() != nil {
		m.StickyModifiersOverlay().SetSharingType(hide)
	}

	if m.VirtualPointerOverlay() != nil {
		m.VirtualPointerOverlay().SetSharingType(hide)
	}

	if m.logger != nil {
		m.logger.Info("Overlay screen share visibility toggled",
			zap.Bool("hidden", hide))
	}
}
