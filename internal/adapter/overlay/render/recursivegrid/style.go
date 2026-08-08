package recursivegrid

import (
	"image"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	minLineWidth = 1
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

// ShowLabelIn reports whether a cell is large enough for its key label to be
// worth drawing: both cell dimensions must reach
// label_autohide_multiplier x the label font size. A non-positive multiplier
// disables autohide, so the label always shows.
//
// The Cairo and GDI backends both call this, and they have to answer the same
// way — a cell one labels and the other leaves blank is the same configuration
// producing two different screens. The macOS backend asks the same question in
// Objective-C (drawGridLabel: in
// internal/adapter/platform/darwin/overlay_darwin.m), so Go cannot be its one
// implementation; ADR 0007 asks for a test holding that copy to this one
// instead, and internal/architecture/label_autohide_rule_test.go is it — change
// the rule here and that test fails until the Objective-C copy follows.
func (s Style) ShowLabelIn(cell image.Rectangle) bool {
	if s.labelAutohideMultiplier <= 0 {
		return true
	}

	threshold := s.LabelFontSize() * s.labelAutohideMultiplier

	return float64(cell.Dx()) >= threshold && float64(cell.Dy()) >= threshold
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
		fontFamily:      ports.ResolveFont(cfg.UI.FontFamily),
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
	s.lineColorARGB = badge.ParseHexARGB(s.lineColor)
	s.highlightColorARGB = badge.ParseHexARGB(s.highlightColor)
	s.textColorARGB = badge.ParseHexARGB(s.textColor)
	s.labelBackgroundColorARGB = badge.ParseHexARGB(s.labelBackgroundColor)
	s.subKeyPreviewTextColorARGB = badge.ParseHexARGB(s.subKeyPreviewTextColor)

	return s
}
