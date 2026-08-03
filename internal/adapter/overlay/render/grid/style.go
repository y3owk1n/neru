package grid

import (
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

// The grid style is one type for every platform.
//
// Its fields hold the values the configuration writes: hex color strings and
// integer sizes. Backends that draw with other representations — Cairo on Linux
// and GDI on Windows both want packed ARGB and floats — get them from the
// accessors further down, which convert at the point of use.
//
// Keeping the representation out of the struct is what lets manager.Interface
// name this type in a signature every platform shares.

const (
	// minLineWidth keeps a hairline visible on backends that would otherwise
	// round a zero-width stroke away.
	minLineWidth = 1

	invalidColor = 0xFFFFFFFF
	hexPairCount = 2
	colorLen3    = 3
	colorLen6    = 6
	colorLen8    = 8
)

// Style is the resolved visual styling for the grid overlay.
type Style struct {
	fontSize               int
	fontFamily             string
	borderWidth            int
	backgroundColor        string
	textColor              string
	matchedTextColor       string
	matchedBackgroundColor string
	matchedBorderColor     string
	borderColor            string
	showLabels             bool

	// Packed ARGB forms of the colors above, resolved once by BuildStyle.
	// The overlay backends read these inside per-cell draw loops, so parsing
	// the hex on every read would put the conversion on the keypress path.
	backgroundColorARGB        uint32
	textColorARGB              uint32
	matchedTextColorARGB       uint32
	matchedBackgroundColorARGB uint32
	matchedBorderColorARGB     uint32
	borderColorARGB            uint32
}

// FontSize returns the label font size in points.
func (s Style) FontSize() int { return s.fontSize }

// FontFamily returns the resolved label font family.
func (s Style) FontFamily() string { return s.fontFamily }

// BorderWidth returns the configured cell border width.
func (s Style) BorderWidth() int { return s.borderWidth }

// BackgroundColor returns the cell background as a hex string.
func (s Style) BackgroundColor() string { return s.backgroundColor }

// TextColor returns the label color as a hex string.
func (s Style) TextColor() string { return s.textColor }

// MatchedTextColor returns the matched-label color as a hex string.
func (s Style) MatchedTextColor() string { return s.matchedTextColor }

// MatchedBackgroundColor returns the matched-cell background as a hex string.
func (s Style) MatchedBackgroundColor() string { return s.matchedBackgroundColor }

// MatchedBorderColor returns the matched-cell border as a hex string.
func (s Style) MatchedBorderColor() string { return s.matchedBorderColor }

// BorderColor returns the cell border color as a hex string.
func (s Style) BorderColor() string { return s.borderColor }

// ShowLabels reports whether cell labels are drawn.
func (s Style) ShowLabels() bool { return s.showLabels }

// The accessors below serve backends that draw with packed ARGB and float
// dimensions: Cairo on Linux, GDI on Windows.

// LineWidth returns the border width as a float, clamped so a hairline stays
// visible.
func (s Style) LineWidth() float64 { return float64(max(s.borderWidth, minLineWidth)) }

// LabelFontSize returns the label font size as a float.
func (s Style) LabelFontSize() float64 { return float64(s.fontSize) }

// LineColorARGB returns the cell border color as packed ARGB.
func (s Style) LineColorARGB() uint32 { return s.borderColorARGB }

// BackgroundColorARGB returns the cell background as packed ARGB.
func (s Style) BackgroundColorARGB() uint32 { return s.backgroundColorARGB }

// TextColorARGB returns the label color as packed ARGB.
func (s Style) TextColorARGB() uint32 { return s.textColorARGB }

// MatchedTextColorARGB returns the matched-label color as packed ARGB.
func (s Style) MatchedTextColorARGB() uint32 { return s.matchedTextColorARGB }

// MatchedBackgroundColorARGB returns the matched-cell background as packed ARGB.
func (s Style) MatchedBackgroundColorARGB() uint32 { return s.matchedBackgroundColorARGB }

// MatchedBorderColorARGB returns the matched-cell border as packed ARGB.
func (s Style) MatchedBorderColorARGB() uint32 { return s.matchedBorderColorARGB }

// BuildStyle resolves the grid style from configuration and the active theme.
func BuildStyle(cfg config.GridConfig, theme config.ThemeProvider) Style {
	style := Style{
		fontSize:    cfg.UI.FontSize,
		fontFamily:  ports.ResolveFont(cfg.UI.FontFamily, true),
		borderWidth: cfg.UI.BorderWidth,
		backgroundColor: cfg.UI.BackgroundColor.ForTheme(
			theme,
			config.GridBackgroundColorLight,
			config.GridBackgroundColorDark,
		),
		textColor: cfg.UI.TextColor.ForTheme(
			theme,
			config.GridTextColorLight,
			config.GridTextColorDark,
		),
		matchedTextColor: cfg.UI.MatchedTextColor.ForTheme(
			theme,
			config.GridMatchedTextColorLight,
			config.GridMatchedTextColorDark,
		),
		matchedBackgroundColor: cfg.UI.MatchedBackgroundColor.ForTheme(
			theme,
			config.GridMatchedBackgroundColorLight,
			config.GridMatchedBackgroundColorDark,
		),
		matchedBorderColor: cfg.UI.MatchedBorderColor.ForTheme(
			theme,
			config.GridMatchedBorderColorLight,
			config.GridMatchedBorderColorDark,
		),
		borderColor: cfg.UI.BorderColor.ForTheme(
			theme,
			config.GridBorderColorLight,
			config.GridBorderColorDark,
		),
		showLabels: true,
	}

	style.backgroundColorARGB = parseHexARGB(style.backgroundColor)
	style.textColorARGB = parseHexARGB(style.textColor)
	style.matchedTextColorARGB = parseHexARGB(style.matchedTextColor)
	style.matchedBackgroundColorARGB = parseHexARGB(style.matchedBackgroundColor)
	style.matchedBorderColorARGB = parseHexARGB(style.matchedBorderColor)
	style.borderColorARGB = parseHexARGB(style.borderColor)

	return style
}

// parseHexARGB converts a "#RGB", "#RRGGBB" or "#AARRGGBB" color to packed
// ARGB, returning opaque white for anything it cannot read.
func parseHexARGB(value string) uint32 {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")

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
