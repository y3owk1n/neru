package grid

import (
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

// The grid style is declared once, for every platform.
//
// It used to be three build-tagged declarations. macOS held the values as the
// config writes them — hex color strings, integer sizes — while Linux and
// Windows held them pre-converted to packed ARGB and floats, because Cairo and
// GDI want them that way. That is a property of the drawing API, not of the
// style, and it was enough to make ManagerInterface name a type that differed
// per platform, which in turn pinned the render models under the adapter layer
// instead of in the domain.
//
// So the fields hold the semantic values and the conversions are methods. A
// backend calls the accessor that matches its drawing API and pays the same
// conversion it paid before, at the same point.

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
// dimensions (Cairo on Linux, GDI on Windows). They are the same conversions
// those backends used to bake into the style at build time.

// LineWidth returns the border width as a float, clamped so a hairline stays
// visible.
func (s Style) LineWidth() float64 { return float64(max(s.borderWidth, minLineWidth)) }

// LabelFontSize returns the label font size as a float.
func (s Style) LabelFontSize() float64 { return float64(s.fontSize) }

// LineColorARGB returns the cell border color as packed ARGB.
func (s Style) LineColorARGB() uint32 { return parseHexARGB(s.borderColor) }

// BackgroundColorARGB returns the cell background as packed ARGB.
func (s Style) BackgroundColorARGB() uint32 { return parseHexARGB(s.backgroundColor) }

// TextColorARGB returns the label color as packed ARGB.
func (s Style) TextColorARGB() uint32 { return parseHexARGB(s.textColor) }

// MatchedTextColorARGB returns the matched-label color as packed ARGB.
func (s Style) MatchedTextColorARGB() uint32 { return parseHexARGB(s.matchedTextColor) }

// MatchedBackgroundColorARGB returns the matched-cell background as packed ARGB.
func (s Style) MatchedBackgroundColorARGB() uint32 {
	return parseHexARGB(s.matchedBackgroundColor)
}

// MatchedBorderColorARGB returns the matched-cell border as packed ARGB.
func (s Style) MatchedBorderColorARGB() uint32 { return parseHexARGB(s.matchedBorderColor) }

// BuildStyle resolves the grid style from configuration and the active theme.
func BuildStyle(cfg config.GridConfig, theme config.ThemeProvider) Style {
	return Style{
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
