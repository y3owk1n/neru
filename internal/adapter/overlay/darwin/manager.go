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
	_ "github.com/y3owk1n/neru/internal/adapter/platform/darwin" // links the Objective-C behind overlay.h (ADR 0009)
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	// NSWindowSharingNone represents NSWindowSharingNone (0), which hides the window from screen sharing.
	NSWindowSharingNone = 0
	// NSWindowSharingReadOnly represents NSWindowSharingReadOnly (1), which shows the window in screen sharing.
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

	// shareMu guards hideInScreenShare and everything SetSharingType reaches
	// through: the shared window handle and the render overlays that own their
	// own. Destroy takes it too, because the screen-share callback is the one
	// port call that arrives on a goroutine of its own (state.AppState notifies
	// on `go callback(...)`) and can therefore overlap a teardown. Reading a
	// window handle the release is about to hand to CFBridgingRelease is a
	// use-after-free, not a stale write. It is deliberately separate from the
	// Base mode/subscriber mutex so SetSharingType can never contend with
	// SwitchTo/Subscribe callers.
	//
	// What makes a teardown safe to run under it is a property of the callers,
	// not of the lock. Every native call SetSharingType makes is a
	// dispatch_async, so a holder always finishes without needing the main
	// thread, and a Destroy waiting here can never be waiting on something
	// waiting on it. Destroy's own calls are not async, which is why Destroy
	// is main-thread-only.
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
// BuildComponents reads Headless directly, so drift there fails to compile,
// and this pins the shared spelling every backend answers headlessness with.
var _ manager.HeadlessReporter = (*Manager)(nil)

// Headless reports whether the overlay window failed to be created, leaving
// nothing for the render overlays to draw on.
func (m *Manager) Headless() bool {
	return m == nil || m.window == nil
}

// BuildComponents constructs the render components this manager draws through,
// on the window it owns, then hands the virtual pointer the screen-share
// visibility the manager is already holding. It has its own window, so the
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

	if m.BisectOverlay() != nil {
		m.BisectOverlay().Hide()
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

	if m.BisectOverlay() != nil {
		m.BisectOverlay().Clear()
	}

	C.NeruClearVim(m.window)
}

// ClearCache is a no-op on Darwin, because each overlay owns its own NSWindow and
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
//
// It runs under shareMu so an in-flight SetSharingType cannot overlap it. The
// adapter's destroyed flag stops calls that arrive after the release begins,
// but a screen-share callback already past that check is inside this manager,
// and the two touch the same window handles.
//
// It must be called on the main thread, which both callers are. They are
// App.Cleanup (cmd/neru/daemon_host_darwin.go runs it once the Cocoa loop has
// stopped) and the startup unwind, which reaches the manager before any
// screen-share subscription exists. Off it, the native release below is a
// dispatch_sync, and the indicator teardown takes a draw mutex whose holder
// dispatch_syncs as well. shareMu would then be held across a wait on the main
// queue, and every later screen-share callback would park behind it.
func (m *Manager) Destroy() {
	m.shareMu.Lock()
	defer m.shareMu.Unlock()

	// Clean up Go-side resources (callbackManager, styleCache, labelCache) for
	// overlays that share the manager's window. We call Cleanup() instead of
	// Destroy() because the shared window is destroyed below, and calling each
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
	if m.BisectOverlay() != nil {
		m.BisectOverlay().Cleanup()
		m.UseBisectOverlay(nil)
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

	// Forget the handle before releasing it, so a second Destroy cannot hand
	// the same pointer to CFBridgingRelease twice.
	window := m.window
	m.window = nil

	if window != nil {
		C.NeruDestroyOverlayWindow(window)
	}
}

// UseVirtualPointerOverlay sets the cursor-following virtual pointer overlay
// renderer, propagating the current screen-share visibility to it. It overrides
// the Base method. The other renderers are registered through Base directly.
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
// and the caller degrades on that code, so it travels unwrapped, the way the
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

// DrawBisect renders the bisect region on its own region-grid component, so
// its transition runs from the bounds that component last drew and reads the
// [bisect.animation] it was configured with.
func (m *Manager) DrawBisect(
	bounds image.Rectangle,
	depth int,
	keys string,
	style recursivegrid.Style,
	virtualPointer recursivegrid.VirtualPointerState,
) error {
	if m.BisectOverlay() == nil {
		return nil
	}

	drawErr := m.BisectOverlay().DrawRecursiveGrid(
		bounds,
		depth,
		keys,
		bisectDimensions(),
		"",
		domain.GridDimensions{},
		style,
		virtualPointer,
	)
	if drawErr != nil {
		return derrors.Wrap(drawErr, derrors.CodeOverlayFailed, "failed to draw bisect")
	}

	return nil
}

// bisectDimensions is the one shape a bisect region is drawn in: four
// quadrants.
func bisectDimensions() domain.GridDimensions {
	return domain.GridDimensions{Rows: 2, Cols: 2} //nolint:mnd // four quadrants
}

// UpdateGridMatches updates the grid matches with the specified prefix.
func (m *Manager) UpdateGridMatches(prefix string) {
	if m.GridOverlay() == nil {
		return
	}
	m.GridOverlay().UpdateMatches(prefix)
}

// ShowSubgrid shows a subgrid for the specified cell, with the pointer
// stand-in the selection that opened it asks for.
//
// The pointer is a layer of the overlay window here rather than something
// painted into the cells, so applying it after the subgrid costs a layer
// update and no redraw. That is why this platform never paid the double
// repaint #1492 is about, and why taking the pointer as an argument changes
// nothing it does.
func (m *Manager) ShowSubgrid(
	cell *domainGrid.Cell,
	style grid.Style,
	virtualPointer recursivegrid.VirtualPointerState,
) {
	if m.GridOverlay() == nil {
		return
	}
	m.GridOverlay().ShowSubgrid(cell, style)
	m.ApplyGridPointer(manager.ModeGrid, virtualPointer)
}

// SetHideUnmatched sets whether to hide unmatched cells.
func (m *Manager) SetHideUnmatched(hide bool) {
	if m.GridOverlay() == nil {
		return
	}
	m.GridOverlay().SetHideUnmatched(hide)
}

// Flush is a no-op on macOS, because indicator overlays use independent windows
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

	// Also update the render overlays that own windows of their own.
	if m.GridOverlay() != nil {
		m.GridOverlay().SetSharingType(hide)
	}
	if m.RecursiveGridOverlay() != nil {
		m.RecursiveGridOverlay().SetSharingType(hide)
	}
	if m.BisectOverlay() != nil {
		m.BisectOverlay().SetSharingType(hide)
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
		m.logger.Debug("Overlay screen share visibility toggled",
			zap.Bool("hidden", hide))
	}
}

// Ensure the manager keeps declaring that it draws the vim surface.
var _ manager.VimDrawer = (*Manager)(nil)

// DrawVim draws the vim surface on the shared window. The C strings and
// rectangle arrays only live for the call, since the bridge copies them before
// it hops to the main thread.
func (m *Manager) DrawVim(scene manager.VimScene, style manager.VimStyle) error {
	if m.window == nil {
		return derrors.New(derrors.CodeNotSupported, "vim overlay has no window")
	}

	var strs []*C.char

	cstr := func(s string) *C.char {
		str := C.CString(s)
		strs = append(strs, str)

		return str
	}

	defer func() {
		for _, s := range strs {
			C.free(unsafe.Pointer(s))
		}
	}()

	selection, freeSelection := cRects(scene.Selection)
	defer freeSelection()

	matches, freeMatches := cRects(scene.Matches)
	defer freeMatches()

	labelBounds := make([]image.Rectangle, len(scene.Labels))
	labelTexts := make([]*C.char, len(scene.Labels))

	for index, label := range scene.Labels {
		labelBounds[index] = label.Bounds
		labelTexts[index] = C.CString(label.Text)
	}

	defer func() {
		for _, s := range labelTexts {
			C.free(unsafe.Pointer(s))
		}
	}()

	bounds, freeBounds := cRects(labelBounds)
	defer freeBounds()

	var labels **C.char
	if len(labelTexts) > 0 {
		labels = (**C.char)(
			C.malloc(C.size_t(len(labelTexts)) * C.size_t(unsafe.Sizeof(labelTexts[0]))),
		)
		defer C.free(unsafe.Pointer(labels))
		copy(unsafe.Slice(labels, len(labelTexts)), labelTexts)
	}

	C.NeruDrawVim(m.window, C.VimScene{
		region:         cRect(scene.Region),
		cursor:         cRect(scene.Cursor),
		pending:        C.int(boolToInt(scene.Pending)),
		selection:      selection,
		selectionCount: C.int(len(scene.Selection)),
		matches:        matches,
		matchCount:     C.int(len(scene.Matches)),
		currentMatch:   C.int(scene.Current),
		container:      cRect(scene.Container),
		block:          cRect(scene.Block),
		mode:           cstr(scene.Mode),
		modeKind:       C.int(scene.ModeKind),
		status:         cstr(scene.Status),
		statusRight:    cstr(scene.StatusRight),
		statusTyped:    C.int(boolToInt(scene.StatusTyped)),
		lensBefore:     cstr(scene.Lens[0]),
		lensAt:         cstr(scene.Lens[1]),
		lensAfter:      cstr(scene.Lens[2]),
		labels:         labels,
		labelBounds:    bounds,
		labelCount:     C.int(len(scene.Labels)),
		notice:         cstr(scene.Notice),
	}, C.VimStyle{
		fontSize:              C.int(style.FontSize),
		fontFamily:            cstr(style.FontFamily),
		cursorColor:           cstr(style.CursorColor),
		selectionColor:        cstr(style.SelectionColor),
		matchColor:            cstr(style.MatchColor),
		containerColor:        cstr(style.ContainerColor),
		statusBackgroundColor: cstr(style.StatusBackgroundColor),
		statusTextColor:       cstr(style.StatusTextColor),
		badgeFontSize:         C.int(style.Badge.FontSize),
		badgeFontFamily:       cstr(style.Badge.FontFamily),
		badgeBackground:       cstr(style.Badge.Background),
		badgeText:             cstr(style.Badge.Text),
		badgeBorder:           cstr(style.Badge.Border),
		badgeBorderWidth:      C.int(style.Badge.BorderWidth),
		badgePaddingX:         C.int(style.Badge.PaddingX),
		badgePaddingY:         C.int(style.Badge.PaddingY),
		badgeBorderRadius:     C.int(style.Badge.BorderRadius),
		modeColor:             cstr(style.ModeColor),
		visualModeColor:       cstr(style.VisualModeColor),
		onModeColor:           cstr(style.OnModeColor),
	})

	return nil
}

func cRect(r image.Rectangle) C.CGRect {
	return C.CGRect{
		origin: C.CGPoint{x: C.double(r.Min.X), y: C.double(r.Min.Y)},
		size:   C.CGSize{width: C.double(r.Dx()), height: C.double(r.Dy())},
	}
}

// cRects copies rects into C memory the caller frees with the returned func.
func cRects(rects []image.Rectangle) (*C.CGRect, func()) {
	if len(rects) == 0 {
		return nil, func() {}
	}

	ptr := (*C.CGRect)(C.malloc(C.size_t(len(rects)) * C.size_t(unsafe.Sizeof(C.CGRect{}))))
	out := unsafe.Slice(ptr, len(rects))

	for index, rect := range rects {
		out[index] = cRect(rect)
	}

	return ptr, func() { C.free(unsafe.Pointer(ptr)) }
}
