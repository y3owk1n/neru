//go:build windows

// internal/ui/overlay/manager_windows.go
// Windows overlay manager backed by a layered Win32 HWND and GDI rendering of
// grid, hints, and recursive-grid overlays.
// Does not implement keyboard capture (handled by the low-level keyboard hook).

package overlay

import (
	"image"
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
	winplatform "github.com/y3owk1n/neru/internal/core/infra/platform/windows"
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

	// indicatorWin is a small dedicated layered window for the mode
	// indicator badge. It is created lazily on first use and repositioned
	// every tick. Keeping it separate from the main overlay avoids the
	// clear-then-flush blink caused by drawing transient badges into the
	// shared full-screen pixel buffer.
	indicatorWin *winplatform.OverlayWindow

	// stickyWin is a small dedicated layered window for the sticky modifiers
	// indicator badge, same pattern as indicatorWin.
	stickyWin *winplatform.OverlayWindow

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

	if m.indicatorWin != nil {
		m.indicatorWin.Hide()
	}

	if m.stickyWin != nil {
		m.stickyWin.Hide()
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

// ClearCache invalidates cached grid and hints state on the Windows overlay
// so that a subsequent Show() does not redraw stale content from a previous
// mode. Called during mode cleanup to prevent ghost artifacts.
func (m *Manager) ClearCache() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.win != nil {
		m.win.ClearCache()
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

	if m.indicatorWin != nil {
		m.indicatorWin.Destroy()
		m.indicatorWin = nil
	}

	if m.stickyWin != nil {
		m.stickyWin.Destroy()
		m.stickyWin = nil
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

// DrawHintSearchInput renders the hints search input on the Windows overlay.
func (m *Manager) DrawHintSearchInput(
	query string,
	resultCount int,
	frame hints.SearchInputFrame,
	style hints.SearchInputStyle,
) error {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	m.ensureWinOverlayLocked()

	if m.win == nil {
		return nil
	}

	m.win.Resize()

	pos := frame.Position()
	width := frame.Width()

	fontSize := float64(max(style.FontSize(), 1))
	paddingX := resolveWinAutoPadding(fontSize, style.PaddingX(), true)
	paddingY := resolveWinAutoPadding(fontSize, style.PaddingY(), false)

	// / query  count /  format
	label := "/ " + query
	if resultCount >= 0 {
		label += "  " + strconv.Itoa(resultCount) + " /"
	} else {
		label += " /"
	}

	badgeWidth := estimateWinTextWidth(label, fontSize) + paddingX*winPaddingMultiplier
	badgeHeight := estimateWinTextHeight(fontSize) + paddingY*winPaddingMultiplier
	bounds := image.Rect(pos.X, pos.Y, pos.X+max(badgeWidth, width), pos.Y+badgeHeight)

	m.win.drawFilledRect(
		bounds,
		parseHexColorARGB(style.BackgroundColor()),
		parseHexColorARGB(style.BorderColor()),
		float64(max(style.BorderWidth(), 0)),
	)
	m.win.drawTextCentered(
		label,
		bounds,
		style.FontFamily(),
		fontSize,
		parseHexColorARGB(style.TextColor()),
	)

	m.win.flushOverlay("search-input")

	return nil
}

// HideHintSearchInput redraws the hints overlay to clear the search input.
func (m *Manager) HideHintSearchInput() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.win == nil {
		return
	}

	if m.win.lastHints != nil {
		m.win.Resize()
		// DrawHints clears + redraws; using it erases the search overlay.
		m.win.DrawHints(m.win.lastHints, m.win.lastHintStyle)
	}
}

// DrawModeIndicator renders a mode indicator badge in its own dedicated
// layered window that repositions every tick to follow the cursor. This
// avoids the clear-then-flush blink that occurs when drawing transient
// badges into the shared full-screen overlay pixel buffer.
func (m *Manager) DrawModeIndicator(cursorX, cursorY int) {
	if m.modeIndicatorOverlay == nil {
		return
	}

	mode := m.Mode()
	if mode == ModeIdle {
		return
	}

	cfg := m.modeIndicatorOverlay.IndicatorConfig()

	label := modeIndicatorLabel(cfg, string(mode))
	if label == "" {
		return
	}

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	offsetX := cfg.UI.IndicatorXOffset
	offsetY := cfg.UI.IndicatorYOffset
	fontSize := float64(max(cfg.UI.FontSize, 1))

	paddingX := resolveWinAutoPadding(fontSize, cfg.UI.PaddingX, true)
	paddingY := resolveWinAutoPadding(fontSize, cfg.UI.PaddingY, false)
	badgeWidth := estimateWinTextWidth(label, fontSize) + paddingX*winPaddingMultiplier
	badgeHeight := estimateWinTextHeight(fontSize) + paddingY*winPaddingMultiplier
	borderWidth := max(cfg.UI.BorderWidth, 0)

	posX := cursorX + offsetX - borderWidth
	posY := cursorY + offsetY - borderWidth
	sizeX := badgeWidth + borderWidth*2  //nolint:mnd // simple arithmetic
	sizeY := badgeHeight + borderWidth*2 //nolint:mnd // simple arithmetic

	// Lazily create the small indicator overlay window.
	if m.indicatorWin == nil || !m.indicatorWin.Healthy() {
		if m.indicatorWin != nil {
			m.indicatorWin.Destroy()
		}

		win, err := winplatform.NewOverlayWindowAt(posX, posY, sizeX, sizeY)
		if err != nil {
			if m.logger != nil {
				m.logger.Error("failed to create indicator overlay window", zap.Error(err))
			}

			return
		}

		m.indicatorWin = win
	} else {
		_ = m.indicatorWin.ResizeTo(posX, posY, sizeX, sizeY)
	}

	// Clear and draw the badge into the small window.
	m.indicatorWin.Clear()

	modeCfg := m.modeIndicatorOverlay.ModeConfig(string(mode))
	bgColor := modeCfg.BackgroundColor.ForThemeWithOverride(
		cfg.UI.BackgroundColor,
		m.modeIndicatorOverlay.Theme(),
		config.ModeIndicatorBackgroundColorLight,
		config.ModeIndicatorBackgroundColorDark,
	)
	textColor := modeCfg.TextColor.ForThemeWithOverride(
		cfg.UI.TextColor,
		m.modeIndicatorOverlay.Theme(),
		config.ModeIndicatorTextColorLight,
		config.ModeIndicatorTextColorDark,
	)
	borderColor := modeCfg.BorderColor.ForThemeWithOverride(
		cfg.UI.BorderColor,
		m.modeIndicatorOverlay.Theme(),
		config.ModeIndicatorBorderColorLight,
		config.ModeIndicatorBorderColorDark,
	)

	badgeBounds := image.Rect(
		borderWidth,
		borderWidth,
		badgeWidth+borderWidth,
		badgeHeight+borderWidth,
	)

	m.indicatorWin.FillRect(badgeBounds, parseHexColorARGB(bgColor))

	if borderWidth > 0 {
		m.indicatorWin.StrokeRect(badgeBounds, parseHexColorARGB(borderColor), float64(borderWidth))
	}

	m.indicatorWin.DrawTextCentered(
		label,
		badgeBounds,
		ports.ResolveFont(cfg.UI.FontFamily, true),
		fontSize,
		parseHexColorARGB(textColor),
	)

	// Flush composites fills/strokes/texts into the pixel buffer and sends
	// the frame to the HWND via UpdateLayeredWindow. Must be called before
	// Show() so the window appears with the badge already rendered.
	err := m.indicatorWin.Flush()
	if err != nil {
		if m.logger != nil {
			m.logger.Error("indicator flush failed", zap.Error(err))
		}
	}

	m.indicatorWin.Show()
}

// DrawStickyModifiersIndicator renders a sticky modifiers indicator badge in
// its own dedicated layered window, following the cursor without touching the
// shared overlay.
func (m *Manager) DrawStickyModifiersIndicator(cursorX, cursorY int, symbols string) {
	if m.stickyModifiersOverlay == nil || symbols == "" {
		return
	}

	indicatorUI := m.stickyModifiersOverlay.UI()
	fontSize := float64(max(indicatorUI.FontSize, 1))

	paddingX := resolveWinAutoPadding(fontSize, indicatorUI.PaddingX, true)
	paddingY := resolveWinAutoPadding(fontSize, indicatorUI.PaddingY, false)
	badgeWidth := estimateWinTextWidth(symbols, fontSize) + paddingX*winPaddingMultiplier
	badgeHeight := estimateWinTextHeight(fontSize) + paddingY*winPaddingMultiplier
	borderWidth := max(indicatorUI.BorderWidth, 0)

	offsetX := indicatorUI.IndicatorXOffset
	offsetY := indicatorUI.IndicatorYOffset

	posX := cursorX + offsetX - borderWidth
	posY := cursorY + offsetY - borderWidth
	sizeX := badgeWidth + borderWidth*2  //nolint:mnd // simple arithmetic
	sizeY := badgeHeight + borderWidth*2 //nolint:mnd // simple arithmetic

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	// Lazily create the small sticky overlay window.
	if m.stickyWin == nil || !m.stickyWin.Healthy() {
		if m.stickyWin != nil {
			m.stickyWin.Destroy()
		}

		win, err := winplatform.NewOverlayWindowAt(posX, posY, sizeX, sizeY)
		if err != nil {
			if m.logger != nil {
				m.logger.Error("failed to create sticky overlay window", zap.Error(err))
			}

			return
		}

		m.stickyWin = win
	} else {
		_ = m.stickyWin.ResizeTo(posX, posY, sizeX, sizeY)
	}

	m.stickyWin.Clear()

	bgColor := indicatorUI.BackgroundColor.ForTheme(
		m.stickyModifiersOverlay.Theme(),
		config.StickyModifiersBackgroundColorLight,
		config.StickyModifiersBackgroundColorDark,
	)
	textColor := indicatorUI.TextColor.ForTheme(
		m.stickyModifiersOverlay.Theme(),
		config.StickyModifiersTextColorLight,
		config.StickyModifiersTextColorDark,
	)
	borderColor := indicatorUI.BorderColor.ForTheme(
		m.stickyModifiersOverlay.Theme(),
		config.StickyModifiersBorderColorLight,
		config.StickyModifiersBorderColorDark,
	)

	badgeBounds := image.Rect(
		borderWidth,
		borderWidth,
		badgeWidth+borderWidth,
		badgeHeight+borderWidth,
	)

	m.stickyWin.FillRect(badgeBounds, parseHexColorARGB(bgColor))

	if borderWidth > 0 {
		m.stickyWin.StrokeRect(badgeBounds, parseHexColorARGB(borderColor), float64(borderWidth))
	}

	m.stickyWin.DrawTextCentered(
		symbols,
		badgeBounds,
		ports.ResolveFont(indicatorUI.FontFamily, false),
		fontSize,
		parseHexColorARGB(textColor),
	)

	err := m.stickyWin.Flush()
	if err != nil {
		if m.logger != nil {
			m.logger.Error("sticky flush failed", zap.Error(err))
		}
	}

	m.stickyWin.Show()
}

// DrawMouseActionIndicator renders a transient mouse action indicator on the Windows overlay.
func (m *Manager) DrawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	m.ensureWinOverlayLocked()

	if m.win == nil {
		return
	}

	size := max(style.Size, 1)
	half := size / 2 //nolint:mnd // simple arithmetic
	bounds := image.Rect(point.X-half, point.Y-half, point.X+half, point.Y+half)

	bgColor := parseHexColorARGB(style.BackgroundColor)
	borderColor := parseHexColorARGB(style.BorderColor)

	m.win.drawFilledRect(
		bounds,
		bgColor,
		borderColor,
		float64(max(style.BorderWidth, 0)),
	)

	m.win.flushOverlay("mouse-action")
	m.win.Show()
}

// modeIndicatorLabel returns the configured label for the given mode string.
func modeIndicatorLabel(cfg config.ModeIndicatorConfig, mode string) string {
	switch mode {
	case "hints":
		return cfg.Hints.Text
	case "grid":
		return cfg.Grid.Text
	case "scroll":
		return cfg.Scroll.Text
	case "recursive_grid":
		return cfg.RecursiveGrid.Text
	default:
		return ""
	}
}

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
