//go:build windows

package grid

import (
	"image"
	"strconv"
	"strings"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
)

const (
	minLineWidth = 1
	invalidColor = 0xFFFFFFFF
	hexPairCount = 2
	colorLen3    = 3
	colorLen6    = 6
	colorLen8    = 8
)

// Style holds the styling information for a grid.
type Style struct {
	LineWidth              float64
	LineColor              uint32
	BackgroundColor        uint32
	LabelFontColor         uint32
	LabelFontSize          float64
	LabelFontName          string
	MatchedTextColor       uint32
	MatchedBackgroundColor uint32
	MatchedBorderColor     uint32
	TextOutlineColor       uint32
	TextOutlineWidth       int
	ShowLabels             bool
}

// Overlay manages the rendering of grid overlays using native platform APIs (Windows stub).
type Overlay struct {
	window unsafe.Pointer
	config config.GridConfig
	logger *zap.Logger
}

// NewOverlay creates a new grid overlay instance (Windows stub).
func NewOverlay(cfg config.GridConfig, logger *zap.Logger) (*Overlay, error) {
	return NewOverlayWithWindow(cfg, logger, nil), nil
}

// NewOverlayWithWindow creates a grid overlay instance using a shared window (Windows stub).
func NewOverlayWithWindow(
	cfg config.GridConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) *Overlay {
	return &Overlay{
		config: cfg,
		logger: logger,
		window: windowPtr,
	}
}

// DrawGrid draws the grid for the specified grid instance (Windows stub).
func (o *Overlay) DrawGrid(grid *domainGrid.Grid) error {
	return nil
}

// Show shows the grid overlay (Windows stub).
func (o *Overlay) Show() {}

// Hide hides the grid overlay (Windows stub).
func (o *Overlay) Hide() {}

// Destroy destroys the grid overlay (Windows stub).
func (o *Overlay) Destroy() {}

// Clear clears the grid overlay (Windows stub).
func (o *Overlay) Clear() {}

// ShowVirtualPointer is a Windows stub.
func (o *Overlay) ShowVirtualPointer(_ image.Point, _ int, _ string) {}

// HideVirtualPointer is a Windows stub.
func (o *Overlay) HideVirtualPointer() {}

// SetConfig updates the grid configuration (Windows stub).
func (o *Overlay) SetConfig(cfg config.GridConfig) {
	o.config = cfg
}

// SetVirtualPointerConfig stores the virtual pointer UI config (Windows stub).
func (o *Overlay) SetVirtualPointerConfig(_ config.VirtualPointerUI, _ string) {}

// Config returns the grid configuration (Windows stub).
func (o *Overlay) Config() config.GridConfig {
	return o.config
}

// Window returns the overlay window (Windows stub).
func (o *Overlay) Window() unsafe.Pointer {
	return o.window
}

// BuildStyle builds the grid style from the configuration.
func BuildStyle(cfg config.GridConfig, theme config.ThemeProvider) Style {
	return Style{
		LineWidth: float64(max(cfg.UI.BorderWidth, minLineWidth)),
		LineColor: parseWindowsColor(
			cfg.UI.BorderColor.ForTheme(
				theme,
				config.GridBorderColorLight,
				config.GridBorderColorDark,
			),
		),
		BackgroundColor: parseWindowsColor(
			cfg.UI.BackgroundColor.ForTheme(
				theme,
				config.GridBackgroundColorLight,
				config.GridBackgroundColorDark,
			),
		),
		LabelFontColor: parseWindowsColor(
			cfg.UI.TextColor.ForTheme(theme, config.GridTextColorLight, config.GridTextColorDark),
		),
		LabelFontSize: float64(cfg.UI.FontSize),
		LabelFontName: cfg.UI.FontFamily,
		MatchedTextColor: parseWindowsColor(
			cfg.UI.MatchedTextColor.ForTheme(
				theme,
				config.GridMatchedTextColorLight,
				config.GridMatchedTextColorDark,
			),
		),
		MatchedBackgroundColor: parseWindowsColor(
			cfg.UI.MatchedBackgroundColor.ForTheme(
				theme,
				config.GridMatchedBackgroundColorLight,
				config.GridMatchedBackgroundColorDark,
			),
		),
		MatchedBorderColor: parseWindowsColor(
			cfg.UI.MatchedBorderColor.ForTheme(
				theme,
				config.GridMatchedBorderColorLight,
				config.GridMatchedBorderColorDark,
			),
		),
		TextOutlineColor: parseWindowsColor(cfg.UI.TextOutlineColor.ForTheme(
			theme,
			"",
			"",
		)),
		TextOutlineWidth: cfg.UI.TextOutlineWidth,
		ShowLabels:       true,
	}
}

func parseWindowsColor(value string) uint32 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	value = strings.TrimPrefix(value, "#")
	switch len(value) {
	case colorLen3:
		value = "FF" + strings.Repeat(string(value[0]), hexPairCount) +
			strings.Repeat(string(value[1]), hexPairCount) +
			strings.Repeat(string(value[2]), hexPairCount)
	case colorLen6:
		value = "FF" + value
	case colorLen8:
	default:
		return invalidColor
	}

	parsed, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return invalidColor
	}

	return uint32(parsed)
}
