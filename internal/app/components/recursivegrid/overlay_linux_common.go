//go:build linux

package recursivegrid

import (
	"image"
	"strconv"
	"strings"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
)

const (
	minFontSize   = 14
	hexDigitCount = 2
	invalidColor  = 0xFFFFFFFF
	colorLen3     = 3
	colorLen6     = 6
	colorLen8     = 8
)

// Style holds the styling information for a recursive grid.
type Style struct {
	LineWidth                       float64
	LineColor                       uint32
	HighlightColor                  uint32
	LabelFontColor                  uint32
	LabelFontSize                   float64
	LabelFontName                   string
	LabelBackground                 bool
	LabelBackgroundColor            uint32
	LabelBackgroundPaddingX         int
	LabelBackgroundPaddingY         int
	LabelBackgroundBorderRadius     int
	LabelBackgroundBorderWidth      float64
	SubKeyPreview                   bool
	SubKeyPreviewFontSize           float64
	SubKeyPreviewAutohideMultiplier float64
	SubKeyPreviewTextColor          uint32
	ShowLabels                      bool
}

// Overlay manages the rendering of recursive_grid overlays using native platform APIs (Linux stub).
type Overlay struct {
	window unsafe.Pointer
	config config.RecursiveGridConfig
	logger *zap.Logger
}

// NewOverlay creates a new recursive_grid overlay instance (Linux stub).
func NewOverlay(cfg config.RecursiveGridConfig, logger *zap.Logger) (*Overlay, error) {
	return &Overlay{
		config: cfg,
		logger: logger,
	}, nil
}

// NewOverlayWithWindow creates a recursive_grid overlay instance using a shared window (Linux stub).
func NewOverlayWithWindow(
	cfg config.RecursiveGridConfig,
	logger *zap.Logger,
	windowPtr unsafe.Pointer,
) *Overlay {
	return &Overlay{
		config: cfg,
		logger: logger,
		window: windowPtr,
	}
}

// Window returns the overlay window (Linux stub).
func (o *Overlay) Window() unsafe.Pointer {
	return o.window
}

// Config returns the recursive_grid config (Linux stub).
func (o *Overlay) Config() config.RecursiveGridConfig {
	return o.config
}

// SetConfig updates the recursive_grid configuration (Linux stub).
func (o *Overlay) SetConfig(cfg config.RecursiveGridConfig) {
	o.config = cfg
}

// SetRecursiveGridConfig updates the recursive_grid configuration (Linux stub).
func (o *Overlay) SetRecursiveGridConfig(cfg config.RecursiveGridConfig) {
	o.SetConfig(cfg)
}

// Show shows the recursive_grid overlay (Linux stub).
func (o *Overlay) Show() {}

// Hide hides the recursive_grid overlay (Linux stub).
func (o *Overlay) Hide() {}

// Destroy destroys the recursive_grid overlay (Linux stub).
func (o *Overlay) Destroy() {}

// Clear clears the recursive_grid overlay (Linux stub).
func (o *Overlay) Clear() {}

// ShowVirtualPointer is a Linux stub.
func (o *Overlay) ShowVirtualPointer(_ image.Point, _ int, _ string) {}

// HideVirtualPointer is a Linux stub.
func (o *Overlay) HideVirtualPointer() {}

// BuildStyle builds the recursive grid style from the configuration (Linux stub).
func BuildStyle(cfg config.RecursiveGridConfig, theme config.ThemeProvider) Style {
	return Style{
		LineWidth: float64(max(cfg.UI.LineWidth, 1)),
		LineColor: parseLinuxColor(
			cfg.UI.LineColor.ForTheme(
				theme,
				config.RecursiveGridLineColorLight,
				config.RecursiveGridLineColorDark,
			),
		),
		HighlightColor: parseLinuxColor(
			cfg.UI.HighlightColor.ForTheme(
				theme,
				config.RecursiveGridHighlightColorLight,
				config.RecursiveGridHighlightColorDark,
			),
		),
		LabelFontColor: parseLinuxColor(
			cfg.UI.TextColor.ForTheme(
				theme,
				config.RecursiveGridTextColorLight,
				config.RecursiveGridTextColorDark,
			),
		),
		LabelFontSize:   float64(max(cfg.UI.FontSize, minFontSize)),
		LabelFontName:   cfg.UI.FontFamily,
		LabelBackground: cfg.UI.LabelBackground,
		LabelBackgroundColor: parseLinuxColor(
			cfg.UI.LabelBackgroundColor.ForTheme(
				theme,
				config.RecursiveGridLabelBackgroundColorLight,
				config.RecursiveGridLabelBackgroundColorDark,
			),
		),
		LabelBackgroundPaddingX:         cfg.UI.LabelBackgroundPaddingX,
		LabelBackgroundPaddingY:         cfg.UI.LabelBackgroundPaddingY,
		LabelBackgroundBorderRadius:     cfg.UI.LabelBackgroundBorderRadius,
		LabelBackgroundBorderWidth:      float64(max(cfg.UI.LabelBackgroundBorderWidth, 0)),
		SubKeyPreview:                   cfg.UI.SubKeyPreview,
		SubKeyPreviewFontSize:           float64(max(cfg.UI.SubKeyPreviewFontSize, 1)),
		SubKeyPreviewAutohideMultiplier: cfg.UI.SubKeyPreviewAutohideMultiplier,
		SubKeyPreviewTextColor: parseLinuxColor(
			cfg.UI.SubKeyPreviewTextColor.ForTheme(
				theme,
				config.RecursiveGridSubKeyPreviewTextColorLight,
				config.RecursiveGridSubKeyPreviewTextColorDark,
			),
		),
		ShowLabels: true,
	}
}

func parseLinuxColor(value string) uint32 {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	switch len(value) {
	case colorLen3:
		value = "FF" + strings.Repeat(string(value[0]), hexDigitCount) +
			strings.Repeat(string(value[1]), hexDigitCount) +
			strings.Repeat(string(value[2]), hexDigitCount)
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
