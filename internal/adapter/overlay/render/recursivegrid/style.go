package recursivegrid

import (
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

// The recursive-grid style is one type for every platform, shaped like the grid
// style in render/grid/style.go: the fields hold the values the configuration
// writes, and the packed-ARGB and float forms that Cairo and GDI want are
// accessors that convert at the point of use.

const (
	minLineWidth = 1

	invalidColor = 0xFFFFFFFF
	hexPairCount = 2
	colorLen3    = 3
	colorLen6    = 6
	colorLen8    = 8
)

// Style is the resolved visual styling for the recursive-grid overlay.
type Style struct {
	lineColor                       string
	lineWidth                       int
	highlightColor                  string
	textColor                       string
	fontSize                        int
	fontFamily                      string
	labelBackground                 bool
	labelBackgroundColor            string
	labelBackgroundPaddingX         int
	labelBackgroundPaddingY         int
	labelBackgroundBorderRadius     int
	labelBackgroundBorderWidth      int
	labelChar                       string
	labelAutohideMultiplier         float64
	subKeyPreview                   bool
	subKeyPreviewFontSize           int
	subKeyPreviewAutohideMultiplier float64
	subKeyPreviewTextColor          string
	subKeyPreviewLabelChar          string

	// Packed ARGB forms of the colors above, resolved once when the style is
	// built. The overlay backends read these inside per-cell draw loops, so
	// parsing the hex on every read would put the conversion on the keypress
	// path.
	lineColorARGB              uint32
	highlightColorARGB         uint32
	textColorARGB              uint32
	labelBackgroundColorARGB   uint32
	subKeyPreviewTextColorARGB uint32
}

// StyleOptions constructs a Style without a configuration.
//
// BuildStyle is how the daemon builds one, resolving every field from config
// and theme. This is for callers that need a style with two or three fields
// set and defaults elsewhere — overlay tests exercising the autohide
// thresholds, mostly.
type StyleOptions struct {
	LineColor                       string
	LineWidth                       int
	HighlightColor                  string
	TextColor                       string
	FontSize                        int
	FontFamily                      string
	LabelBackground                 bool
	LabelBackgroundColor            string
	LabelBackgroundPaddingX         int
	LabelBackgroundPaddingY         int
	LabelBackgroundBorderRadius     int
	LabelBackgroundBorderWidth      int
	LabelChar                       string
	LabelAutohideMultiplier         float64
	SubKeyPreview                   bool
	SubKeyPreviewFontSize           int
	SubKeyPreviewAutohideMultiplier float64
	SubKeyPreviewTextColor          string
	SubKeyPreviewLabelChar          string
}

// NewStyle builds a Style from explicit values.
func NewStyle(opts StyleOptions) Style {
	return Style{
		lineColor:                       opts.LineColor,
		lineWidth:                       opts.LineWidth,
		highlightColor:                  opts.HighlightColor,
		textColor:                       opts.TextColor,
		fontSize:                        opts.FontSize,
		fontFamily:                      opts.FontFamily,
		labelBackground:                 opts.LabelBackground,
		labelBackgroundColor:            opts.LabelBackgroundColor,
		labelBackgroundPaddingX:         opts.LabelBackgroundPaddingX,
		labelBackgroundPaddingY:         opts.LabelBackgroundPaddingY,
		labelBackgroundBorderRadius:     opts.LabelBackgroundBorderRadius,
		labelBackgroundBorderWidth:      opts.LabelBackgroundBorderWidth,
		labelChar:                       opts.LabelChar,
		labelAutohideMultiplier:         opts.LabelAutohideMultiplier,
		subKeyPreview:                   opts.SubKeyPreview,
		subKeyPreviewFontSize:           opts.SubKeyPreviewFontSize,
		subKeyPreviewAutohideMultiplier: opts.SubKeyPreviewAutohideMultiplier,
		subKeyPreviewTextColor:          opts.SubKeyPreviewTextColor,
		subKeyPreviewLabelChar:          opts.SubKeyPreviewLabelChar,
	}.packColors()
}

// LineColor returns the cell border color as a hex string.
func (s Style) LineColor() string {
	return s.lineColor
}

// LineWidth returns the configured cell border width.
func (s Style) LineWidth() int {
	return s.lineWidth
}

// HighlightColor returns the active-cell highlight as a hex string.
func (s Style) HighlightColor() string {
	return s.highlightColor
}

// TextColor returns the label color as a hex string.
func (s Style) TextColor() string {
	return s.textColor
}

// FontSize returns the label font size in points.
func (s Style) FontSize() int {
	return s.fontSize
}

// FontFamily returns the resolved label font family.
func (s Style) FontFamily() string {
	return s.fontFamily
}

// LabelBackground reports whether labels are drawn on a background plate.
func (s Style) LabelBackground() bool {
	return s.labelBackground
}

// LabelBackgroundColor returns the label plate color as a hex string.
func (s Style) LabelBackgroundColor() string {
	return s.labelBackgroundColor
}

// LabelBackgroundPaddingX returns the label plate's horizontal padding.
func (s Style) LabelBackgroundPaddingX() int {
	return s.labelBackgroundPaddingX
}

// LabelBackgroundPaddingY returns the label plate's vertical padding.
func (s Style) LabelBackgroundPaddingY() int {
	return s.labelBackgroundPaddingY
}

// LabelBackgroundBorderRadius returns the label plate's corner radius.
func (s Style) LabelBackgroundBorderRadius() int {
	return s.labelBackgroundBorderRadius
}

// LabelBackgroundBorderWidth returns the label plate's border width.
func (s Style) LabelBackgroundBorderWidth() int {
	return s.labelBackgroundBorderWidth
}

// LabelChar returns the character set labels are drawn from.
func (s Style) LabelChar() string {
	return s.labelChar
}

// SubKeyPreviewLabelChar returns the character set the preview uses.
func (s Style) SubKeyPreviewLabelChar() string {
	return s.subKeyPreviewLabelChar
}

// SubKeyPreview reports whether the next level's keys are previewed.
func (s Style) SubKeyPreview() bool {
	return s.subKeyPreview
}

// SubKeyPreviewFontSize returns the preview font size in points.
func (s Style) SubKeyPreviewFontSize() int {
	return s.subKeyPreviewFontSize
}

// SubKeyPreviewAutohideMultiplier returns the cell-size multiple below which
// the preview hides itself.
func (s Style) SubKeyPreviewAutohideMultiplier() float64 {
	return s.subKeyPreviewAutohideMultiplier
}

// LabelAutohideMultiplier returns the cell-size multiple below which the label
// hides itself.
func (s Style) LabelAutohideMultiplier() float64 {
	return s.labelAutohideMultiplier
}

// SubKeyPreviewTextColor returns the preview label color as a hex string.
func (s Style) SubKeyPreviewTextColor() string {
	return s.subKeyPreviewTextColor
}

// BuildStyle resolves the recursive-grid style from configuration and the
// active theme.
func BuildStyle(cfg config.RecursiveGridConfig, theme config.ThemeProvider) Style {
	return Style{
		lineColor: cfg.UI.LineColor.ForTheme(
			theme,
			config.RecursiveGridLineColorLight,
			config.RecursiveGridLineColorDark,
		),
		lineWidth: cfg.UI.LineWidth,
		highlightColor: cfg.UI.HighlightColor.ForTheme(
			theme,
			config.RecursiveGridHighlightColorLight,
			config.RecursiveGridHighlightColorDark,
		),
		textColor: cfg.UI.TextColor.ForTheme(
			theme,
			config.RecursiveGridTextColorLight,
			config.RecursiveGridTextColorDark,
		),
		fontSize:        cfg.UI.FontSize,
		fontFamily:      ports.ResolveFont(cfg.UI.FontFamily, true),
		labelBackground: cfg.UI.LabelBackground,
		labelBackgroundColor: cfg.UI.LabelBackgroundColor.ForTheme(
			theme,
			config.RecursiveGridLabelBackgroundColorLight,
			config.RecursiveGridLabelBackgroundColorDark,
		),
		labelBackgroundPaddingX:         cfg.UI.LabelBackgroundPaddingX,
		labelBackgroundPaddingY:         cfg.UI.LabelBackgroundPaddingY,
		labelBackgroundBorderRadius:     cfg.UI.LabelBackgroundBorderRadius,
		labelBackgroundBorderWidth:      cfg.UI.LabelBackgroundBorderWidth,
		labelChar:                       cfg.UI.LabelChar,
		labelAutohideMultiplier:         cfg.UI.LabelAutohideMultiplier,
		subKeyPreview:                   cfg.UI.SubKeyPreview,
		subKeyPreviewFontSize:           cfg.UI.SubKeyPreviewFontSize,
		subKeyPreviewAutohideMultiplier: cfg.UI.SubKeyPreviewAutohideMultiplier,
		subKeyPreviewTextColor: cfg.UI.SubKeyPreviewTextColor.ForTheme(
			theme,
			config.RecursiveGridSubKeyPreviewTextColorLight,
			config.RecursiveGridSubKeyPreviewTextColorDark,
		),
		subKeyPreviewLabelChar: cfg.UI.SubKeyPreviewLabelChar,
	}.packColors()
}

// The accessors below serve backends that draw with packed ARGB and float
// dimensions: Cairo on Linux, GDI on Windows.

// LineWidthF returns the cell border width as a float, clamped so a hairline
// stays visible.
func (s Style) LineWidthF() float64 { return float64(max(s.lineWidth, minLineWidth)) }

// LabelFontSize returns the label font size as a float.
func (s Style) LabelFontSize() float64 { return float64(s.fontSize) }

// SubKeyPreviewFontSizeF returns the preview font size as a float, clamped to
// stay renderable.
func (s Style) SubKeyPreviewFontSizeF() float64 {
	return float64(max(s.subKeyPreviewFontSize, minLineWidth))
}

// LabelBackgroundBorderWidthF returns the label background border width as a
// non-negative float.
func (s Style) LabelBackgroundBorderWidthF() float64 {
	return float64(max(s.labelBackgroundBorderWidth, 0))
}

// LineColorARGB returns the cell border color as packed ARGB.
func (s Style) LineColorARGB() uint32 { return s.lineColorARGB }

// HighlightColorARGB returns the highlight color as packed ARGB.
func (s Style) HighlightColorARGB() uint32 { return s.highlightColorARGB }

// TextColorARGB returns the label color as packed ARGB.
func (s Style) TextColorARGB() uint32 { return s.textColorARGB }

// LabelBackgroundColorARGB returns the label background as packed ARGB.
func (s Style) LabelBackgroundColorARGB() uint32 { return s.labelBackgroundColorARGB }

// SubKeyPreviewTextColorARGB returns the preview label color as packed ARGB.
func (s Style) SubKeyPreviewTextColorARGB() uint32 { return s.subKeyPreviewTextColorARGB }

// ShowLabels reports whether cell labels are drawn.
func (s Style) ShowLabels() bool { return true }

// packColors fills the ARGB fields from the hex ones. Both constructors call it
// as their last step, so no caller can produce a Style whose packed values
// disagree with its hex ones.
func (s Style) packColors() Style {
	s.lineColorARGB = parseHexARGB(s.lineColor)
	s.highlightColorARGB = parseHexARGB(s.highlightColor)
	s.textColorARGB = parseHexARGB(s.textColor)
	s.labelBackgroundColorARGB = parseHexARGB(s.labelBackgroundColor)
	s.subKeyPreviewTextColorARGB = parseHexARGB(s.subKeyPreviewTextColor)

	return s
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
