//go:build windows

package windows

import (
	"context"
	"image"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/grid"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/hints"
	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	winplatform "github.com/y3owk1n/neru/internal/adapter/platform/windows"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

// Manager manages overlay rendering on Windows, backed by a layered Win32
// HWND and GDI rendering of grid, hints, and recursive-grid overlays.
// Does not implement keyboard capture (handled by the low-level keyboard hook).
type Manager struct {
	manager.Base

	logger *zap.Logger

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

	// mouseWin is a small dedicated layered window for mouse action indicators.
	mouseWin *winplatform.OverlayWindow

	// monitorWins are the monitor_select panels, one layered window per
	// display, in target order (monitor_select.go).
	monitorWins []*winplatform.OverlayWindow
	// mouseActionCancel cancels any running mouse action animation.
	mouseActionCancel context.CancelFunc
}

var (
	windowsManager     *Manager
	windowsManagerOnce sync.Once
)

// NewOverlayManager creates a new overlay Manager.
func NewOverlayManager(logger *zap.Logger) *Manager {
	return &Manager{
		Base:   manager.NewBase(logger),
		logger: logger,
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

	if m.mouseActionCancel != nil {
		m.mouseActionCancel()
		m.mouseActionCancel = nil
	}

	if m.win != nil {
		m.win.Hide()
	}

	if m.indicatorWin != nil {
		m.indicatorWin.Hide()
	}

	if m.stickyWin != nil {
		m.stickyWin.Hide()
	}

	if m.mouseWin != nil {
		m.mouseWin.Hide()
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

// SetActiveScreenOrigin is a no-op on Windows. The overlay window is resized
// and repositioned to the active screen, so its content uses window-local
// coordinates and needs no global translation.
func (m *Manager) SetActiveScreenOrigin(_ image.Point) {}

// ActiveScreenBounds returns the overlay window bounds in screen coordinates.
func (m *Manager) ActiveScreenBounds() (image.Rectangle, bool) {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.win == nil {
		return image.Rectangle{}, false
	}

	return m.win.screenBounds()
}

// Destroy destroys overlay resources.
func (m *Manager) Destroy() {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.mouseActionCancel != nil {
		m.mouseActionCancel()
		m.mouseActionCancel = nil
	}

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

	if m.mouseWin != nil {
		m.mouseWin.Destroy()
		m.mouseWin = nil
	}

	m.destroyMonitorWindowsLocked()
}

// WindowPtr returns the native overlay window handle.
func (m *Manager) WindowPtr() unsafe.Pointer {
	if m.win == nil {
		return nil
	}

	return m.win.WindowPtr()
}

// BuildComponents constructs the render components this manager draws through.
//
// It never reports itself headless, and deliberately does not declare the
// HeadlessReporter capability at all: the surface is recreated on demand at
// draw time, so having no window while components are built is not a verdict
// on whether it can render. The components here only store the handle they are
// given, so building them against a missing surface costs nothing.
func (m *Manager) BuildComponents(
	cfg *config.Config,
	theme config.ThemeProvider,
) (manager.Components, error) {
	if m == nil {
		return manager.Components{}, nil
	}

	return m.Base.BuildComponents(manager.ComponentSpec{
		Config:   cfg,
		Theme:    theme,
		Logger:   m.logger,
		Window:   m.WindowPtr(),
		Headless: false,
	})
}

// WaylandKeyboardChannel returns nil on Windows.
func (m *Manager) WaylandKeyboardChannel() <-chan string {
	return nil
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
//
// The placement is resolved once, before anything is drawn: it is the same for
// every badge in the frame, and a placement this overlay cannot draw is
// reported rather than approximated. Drawing every hint centered instead would
// leave the user looking at labels in a placement they did not choose, with
// nothing anywhere saying so.
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

	offset, offsetErr := resolveHintBadgeOffset(style.Placement())
	if offsetErr != nil {
		// The error reaches the mode handler, which degrades quietly on
		// CodeNotSupported — so say it here too, or a placement this overlay
		// cannot draw costs the user every hint with only a debug line to
		// explain it. The placement is a fixed configuration keyword.
		if m.logger != nil {
			m.logger.Warn(
				"hint placement not drawn by the windows overlay",
				zap.String("placement", style.Placement()),
			)
		}

		return offsetErr
	}

	// Shared activation may draw before the resize; enforce monitor bounds here.
	m.win.Resize()
	m.win.DrawHints(hintsSlice, style, offset)

	return nil
}

// resolveHintBadgeOffset maps a configured placement onto the offset this
// overlay draws it with. It is the one place the placement vocabulary is read
// here, and every value config.HintPlacements() declares has a branch.
//
// Anything else is refused rather than drawn, the way the Linux and macOS
// overlays refuse it: a placement added to the vocabulary without a branch
// here would otherwise validate, reach this backend and draw somewhere nobody
// chose, with nothing failing anywhere.
//
// The empty string is not one of those: it is a style that reached the overlay
// before a configuration settled it, and it draws at the documented default —
// the same answer the other two backends give it, so a draw that beats the
// first Apply puts hints in the same place on all three.
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
			"hint placement is not drawn by the windows overlay",
		)
	}
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

	if m.win.lastHints != nil {
		m.win.DrawHints(m.win.lastHints, m.win.lastHintStyle, m.win.lastHintOffset)
	} else {
		m.win.Clear()
	}

	pos := frame.Position()
	width := frame.Width()

	fontSize := float64(max(style.FontSize(), 1))
	paddingX := badge.AutoPadding(fontSize, style.PaddingX(), true)
	paddingY := badge.AutoPadding(fontSize, style.PaddingY(), false)

	// / query  count /  format
	label := "/ " + query
	if resultCount >= 0 {
		label += "  " + strconv.Itoa(resultCount) + " /"
	} else {
		label += " /"
	}

	badgeWidth := badge.EstimateTextWidth(label, fontSize) + paddingX*winPaddingMultiplier
	badgeHeight := badge.EstimateTextHeight(fontSize) + paddingY*winPaddingMultiplier
	bounds := image.Rect(pos.X, pos.Y, pos.X+max(badgeWidth, width), pos.Y+badgeHeight)

	m.win.drawFilledRect(
		bounds,
		badge.ParseHexARGB(style.BackgroundColor()),
		badge.ParseHexARGB(style.BorderColor()),
		float64(max(style.BorderWidth(), 0)),
		badge.BorderRadius(style.BorderRadius(), bounds, winAutoRadiusBadgeCap),
	)
	m.win.drawTextCentered(
		label,
		bounds,
		style.FontFamily(),
		fontSize,
		badge.ParseHexARGB(style.TextColor()),
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
		m.win.DrawHints(m.win.lastHints, m.win.lastHintStyle, m.win.lastHintOffset)
	}
}

// DrawModeIndicator renders a mode indicator badge in its own dedicated
// layered window that repositions every tick to follow the cursor. This
// avoids the clear-then-flush blink that occurs when drawing transient
// badges into the shared full-screen overlay pixel buffer.
func (m *Manager) DrawModeIndicator(cursorX, cursorY int) {
	if m.ModeIndicatorOverlay() == nil {
		return
	}

	mode := m.Mode()
	if mode == manager.ModeIdle {
		return
	}

	cfg := m.ModeIndicatorOverlay().IndicatorConfig()

	label := m.ModeIndicatorOverlay().ResolveLabelText(string(mode))
	if label == "" {
		return
	}

	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	offsetX := cfg.UI.IndicatorXOffset
	offsetY := cfg.UI.IndicatorYOffset
	fontSize := float64(max(cfg.UI.FontSize, 1))

	paddingX := badge.AutoPadding(fontSize, cfg.UI.PaddingX, true)
	paddingY := badge.AutoPadding(fontSize, cfg.UI.PaddingY, false)
	badgeWidth := badge.EstimateTextWidth(label, fontSize) + paddingX*winPaddingMultiplier
	badgeHeight := badge.EstimateTextHeight(fontSize) + paddingY*winPaddingMultiplier
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

	// ResolveLabelText already returned non-empty, so the mode config exists.
	modeCfg, _ := m.ModeIndicatorOverlay().ResolveModeConfig(string(mode))
	bgColor := modeCfg.BackgroundColor.ForThemeWithOverride(
		cfg.UI.BackgroundColor,
		m.ModeIndicatorOverlay().ThemeProvider(),
		config.ModeIndicatorBackgroundColorLight,
		config.ModeIndicatorBackgroundColorDark,
	)
	textColor := modeCfg.TextColor.ForThemeWithOverride(
		cfg.UI.TextColor,
		m.ModeIndicatorOverlay().ThemeProvider(),
		config.ModeIndicatorTextColorLight,
		config.ModeIndicatorTextColorDark,
	)
	borderColor := modeCfg.BorderColor.ForThemeWithOverride(
		cfg.UI.BorderColor,
		m.ModeIndicatorOverlay().ThemeProvider(),
		config.ModeIndicatorBorderColorLight,
		config.ModeIndicatorBorderColorDark,
	)

	badgeBounds := image.Rect(
		borderWidth,
		borderWidth,
		badgeWidth+borderWidth,
		badgeHeight+borderWidth,
	)

	indicatorRadius := badge.BorderRadius(
		cfg.UI.BorderRadius, badgeBounds, winAutoRadiusBadgeCap,
	)
	m.indicatorWin.FillRoundedRect(badgeBounds, indicatorRadius, badge.ParseHexARGB(bgColor))

	if borderWidth > 0 {
		m.indicatorWin.StrokeRoundedRect(
			badgeBounds, indicatorRadius, badge.ParseHexARGB(borderColor), float64(borderWidth),
		)
	}

	m.indicatorWin.DrawTextCentered(
		label,
		badgeBounds,
		ports.ResolveFont(cfg.UI.FontFamily),
		fontSize,
		badge.ParseHexARGB(textColor),
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
	if m.StickyModifiersOverlay() == nil || symbols == "" {
		return
	}

	indicatorUI := m.StickyModifiersOverlay().UIConfig()
	fontSize := float64(max(indicatorUI.FontSize, 1))

	paddingX := badge.AutoPadding(fontSize, indicatorUI.PaddingX, true)
	paddingY := badge.AutoPadding(fontSize, indicatorUI.PaddingY, false)
	badgeWidth := badge.EstimateTextWidth(symbols, fontSize) + paddingX*winPaddingMultiplier
	badgeHeight := badge.EstimateTextHeight(fontSize) + paddingY*winPaddingMultiplier
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
		m.StickyModifiersOverlay().ThemeProvider(),
		config.StickyModifiersBackgroundColorLight,
		config.StickyModifiersBackgroundColorDark,
	)
	textColor := indicatorUI.TextColor.ForTheme(
		m.StickyModifiersOverlay().ThemeProvider(),
		config.StickyModifiersTextColorLight,
		config.StickyModifiersTextColorDark,
	)
	borderColor := indicatorUI.BorderColor.ForTheme(
		m.StickyModifiersOverlay().ThemeProvider(),
		config.StickyModifiersBorderColorLight,
		config.StickyModifiersBorderColorDark,
	)

	badgeBounds := image.Rect(
		borderWidth,
		borderWidth,
		badgeWidth+borderWidth,
		badgeHeight+borderWidth,
	)

	stickyRadius := badge.BorderRadius(
		indicatorUI.BorderRadius,
		badgeBounds,
		winAutoRadiusBadgeCap,
	)
	m.stickyWin.FillRoundedRect(badgeBounds, stickyRadius, badge.ParseHexARGB(bgColor))

	if borderWidth > 0 {
		m.stickyWin.StrokeRoundedRect(
			badgeBounds,
			stickyRadius,
			badge.ParseHexARGB(borderColor),
			float64(borderWidth),
		)
	}

	m.stickyWin.DrawTextCentered(
		symbols,
		badgeBounds,
		ports.ResolveFont(indicatorUI.FontFamily),
		fontSize,
		badge.ParseHexARGB(textColor),
	)

	err := m.stickyWin.Flush()
	if err != nil {
		if m.logger != nil {
			m.logger.Error("sticky flush failed", zap.Error(err))
		}
	}

	m.stickyWin.Show()
}

// DrawVirtualPointer renders the cursor-following virtual pointer overlay.
func (m *Manager) DrawVirtualPointer(xCoordinate, yCoordinate, size int, fillColor string) {
	if m.VirtualPointerOverlay() == nil {
		return
	}

	m.VirtualPointerOverlay().Draw(xCoordinate, yCoordinate, size, fillColor)
}

// DrawMouseActionIndicator renders a transient mouse action indicator on the Windows overlay.
func (m *Manager) DrawMouseActionIndicator(
	point image.Point,
	style ports.MouseActionIndicatorStyle,
) {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	// Cancel any running mouse action animation.
	if m.mouseActionCancel != nil {
		m.mouseActionCancel()
		m.mouseActionCancel = nil
	}

	maxScale := max(style.StartScale, style.EndScale)
	if maxScale <= 0 {
		maxScale = 1.0
	}

	baseSize := float64(max(style.Size, 1))
	maxIndicatorSize := baseSize * maxScale
	borderWidth := float64(max(style.BorderWidth, 0))

	// Create window bounds to fit the maximum indicator size plus border.
	const paddingFactor = 4

	winSize := int(maxIndicatorSize) + int(borderWidth)*2 + paddingFactor
	halfWinSize := winSize / 2 //nolint:mnd // divide by 2

	// Not badge.CenteredOn: this places a native window, which takes an origin
	// and a size rather than a rectangle, and the animation inside it measures
	// from the same half.
	posX := point.X - halfWinSize
	posY := point.Y - halfWinSize

	if m.mouseWin == nil || !m.mouseWin.Healthy() {
		if m.mouseWin != nil {
			m.mouseWin.Destroy()
		}

		win, err := winplatform.NewOverlayWindowAt(posX, posY, winSize, winSize)
		if err != nil {
			if m.logger != nil {
				m.logger.Error("failed to create mouse action overlay window", zap.Error(err))
			}

			return
		}

		m.mouseWin = win
	} else {
		_ = m.mouseWin.ResizeTo(posX, posY, winSize, winSize)
	}

	m.mouseWin.Clear()

	ctx, cancel := context.WithCancel(context.Background())
	m.mouseActionCancel = cancel

	go m.animateMouseAction(ctx, winSize, style)
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

	m.syncSublayerKeysLocked()

	m.win.DrawGrid(gridValue, input, style)

	return nil
}

// DrawRecursiveGrid draws the recursive-grid overlay using the Windows GDI backend.
//
// The next-depth keys and dimensions are handed on to the draw, which previews
// them as a mini-grid inside each cell. The depth is not: this backend has no
// transition animation, so nothing here compares the depth against the last one
// drawn (docs/CROSS_PLATFORM.md owns that status).
func (m *Manager) DrawRecursiveGrid(
	bounds image.Rectangle,
	_ int,
	keys string,
	dims domain.GridDimensions,
	nextKeys string,
	nextDims domain.GridDimensions,
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
	m.win.DrawRecursiveGrid(bounds, keys, dims, nextKeys, nextDims, style, virtualPointer)

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

// ShowSubgrid shows a subgrid inside the selected cell, with the pointer
// stand-in that belongs on the same surface once the cell has been picked.
//
// Both are painted into one pixel buffer here, so the pointer rides the open
// (#1492) rather than following it as a call of its own: applied afterwards
// through Base.ApplyGridPointer it would dispatch statically past the
// DrawGridPointer override below and repaint the surface a second time for
// one keystroke, arrow keys inside a subgrid included.
func (m *Manager) ShowSubgrid(
	cell *domainGrid.Cell,
	style grid.Style,
	virtualPointer recursivegrid.VirtualPointerState,
) {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.win != nil {
		m.syncSublayerKeysLocked()
		m.win.ShowSubgrid(cell, style, virtualPointer)
	}
}

// DrawGridPointer puts grid mode's pointer stand-in on the layered window.
//
// Overrides the Base method, which resolves a mode to a render component: off
// darwin those components own no surface, which is why the pointer was a no-op
// in grid mode here while recursive grid, whose pointer rides the frame every
// keystroke hands over, drew it. Recursive grid is still delegated so the Base
// keeps answering for a mode this backend does not paint on its own.
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

// HideGridPointer takes grid mode's pointer stand-in off the window, leaving
// the grid on it. Mode teardown calls it before the frame is cleared, and the
// mode's own selection going away is the other caller.
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

func (m *Manager) animateMouseAction(
	ctx context.Context,
	winSize int,
	style ports.MouseActionIndicatorStyle,
) {
	duration := time.Duration(style.DurationMS) * time.Millisecond
	if duration <= 0 {
		duration = 260 * time.Millisecond //nolint:mnd // default duration
	}

	startTime := time.Now()

	ticker := time.NewTicker(16 * time.Millisecond) //nolint:mnd // ~60 FPS
	defer ticker.Stop()

	halfWinSize := float64(winSize) / 2.0 //nolint:mnd // divide by 2
	borderWidth := float64(max(style.BorderWidth, 0))

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			elapsed := time.Since(startTime)

			progressFraction := float64(elapsed) / float64(duration)
			if progressFraction >= 1.0 {
				progressFraction = 1.0
			}

			progress := ease(progressFraction, style.Easing)
			scale := style.StartScale + progress*(style.EndScale-style.StartScale)
			opacity := style.StartOpacity + progress*(style.EndOpacity-style.StartOpacity)

			baseSize := float64(max(style.Size, 1))
			currentSize := baseSize * scale
			halfSize := currentSize / 2.0 //nolint:mnd // divide by 2

			bounds := image.Rect(
				int(halfWinSize-halfSize),
				int(halfWinSize-halfSize),
				int(halfWinSize+halfSize),
				int(halfWinSize+halfSize),
			)

			bgColor := scaleColorAlpha(style.BackgroundColor, opacity)
			borderColor := scaleColorAlpha(style.BorderColor, opacity)

			var radius float64
			if style.Shape == "circle" {
				radius = halfSize
			} else {
				radius = max(
					currentSize*winMouseActionSquareRadiusScale,
					winMouseActionMinSquareRadius,
				)
			}

			m.renderMu.Lock()
			select {
			case <-ctx.Done():
				m.renderMu.Unlock()

				return
			default:
			}

			if m.mouseWin != nil && m.mouseWin.Healthy() {
				m.mouseWin.Clear()
				m.mouseWin.FillRoundedRect(bounds, radius, bgColor)

				if borderWidth > 0 {
					m.mouseWin.StrokeRoundedRect(bounds, radius, borderColor, borderWidth)
				}

				_ = m.mouseWin.Flush()
				m.mouseWin.Show()
			}
			m.renderMu.Unlock()

			if progressFraction >= 1.0 {
				m.renderMu.Lock()
				if m.mouseWin != nil {
					m.mouseWin.Hide()
				}
				m.renderMu.Unlock()

				return
			}
		}
	}
}

func ease(progressFraction float64, easing string) float64 {
	switch easing {
	case "ease_in":
		res := progressFraction * progressFraction * progressFraction

		return res

	case "ease_out":
		invT := 1.0 - progressFraction
		res := 1.0 - invT*invT*invT

		return res

	case "ease_in_out":
		if progressFraction < 0.5 { //nolint:mnd
			res := 4.0 * progressFraction * progressFraction * progressFraction

			return res
		}

		invT := 1.0 - progressFraction
		res := 1.0 - 4.0*invT*invT*invT

		return res

	case "linear":
		fallthrough
	default:
		return progressFraction
	}
}

func scaleColorAlpha(hexColor string, opacity float64) uint32 {
	colorVal := badge.ParseHexARGB(hexColor)
	alphaVal := float64((colorVal >> 24) & 0xFF) //nolint:mnd
	redVal := (colorVal >> 16) & 0xFF            //nolint:mnd
	greenVal := (colorVal >> 8) & 0xFF           //nolint:mnd
	blueVal := colorVal & 0xFF                   //nolint:mnd

	const maxAlpha = 255

	newA := uint32(max(0, min(maxAlpha, alphaVal*opacity)))
	res := (newA << 24) | (redVal << 16) | (greenVal << 8) | blueVal //nolint:mnd

	return res
}

// dispatchGridPointer hands the pointer state to the backend, which keeps it
// and repaints the grid surface with it. It takes renderMu the way
// UpdateGridMatches does for the same repaint, and no caller holds the lock
// already: the adapter reaches it directly from UpdateGridPointer and
// ClearFrame.
func (m *Manager) dispatchGridPointer(pointer recursivegrid.VirtualPointerState) {
	m.renderMu.Lock()
	defer m.renderMu.Unlock()

	if m.win != nil {
		m.win.SetGridPointer(pointer)
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

// syncSublayerKeysLocked hands the surface the keys the subgrid is drawn with.
// Every draw that can put a subgrid on screen calls it, because the surface is
// rebuilt behind this manager (ensureWinOverlayLocked) and a rebuilt one starts
// with no keys — and a surface with no keys draws a subgrid the user cannot see
// but can still act on.
func (m *Manager) syncSublayerKeysLocked() {
	if m.win == nil || m.GridOverlay() == nil {
		return
	}

	m.win.sublayerKeys = m.GridOverlay().Config().SublayerKeys
}
