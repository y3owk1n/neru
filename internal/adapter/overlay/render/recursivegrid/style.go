package recursivegrid

import (
	"image"
	"strings"

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
	secondaryLineColor              string
	secondaryLineWidth              int
	highlightColor                  string
	textColor                       string
	fontSize                        int
	fontFamily                      string
	minFontSize                     int
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
	secondaryLineColorARGB     uint32
	highlightColorARGB         uint32
	textColorARGB              uint32
	labelBackgroundColorARGB   uint32
	subKeyPreviewTextColorARGB uint32

	// What a label and a preview key take per unit of font size, measured once
	// when the style is built so that fitting one to a cell is arithmetic on
	// the keypress path.
	labelMetrics         badge.LabelMetrics
	subKeyPreviewMetrics badge.LabelMetrics
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
	SecondaryLineColor              string
	SecondaryLineWidth              int
	HighlightColor                  string
	TextColor                       string
	FontSize                        int
	FontFamily                      string
	MinFontSize                     int
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
		secondaryLineColor:              opts.SecondaryLineColor,
		secondaryLineWidth:              opts.SecondaryLineWidth,
		highlightColor:                  opts.HighlightColor,
		textColor:                       opts.TextColor,
		fontSize:                        opts.FontSize,
		fontFamily:                      opts.FontFamily,
		minFontSize:                     opts.MinFontSize,
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

// SecondaryLineColor returns the secondary cell border color as a hex string.
func (s Style) SecondaryLineColor() string {
	return s.secondaryLineColor
}

// SecondaryLineWidth returns the secondary cell border width.
// If <= 0 and secondaryLineColor is set, it defaults to LineWidth.
func (s Style) SecondaryLineWidth() int {
	if s.secondaryLineWidth > 0 {
		return s.secondaryLineWidth
	}

	return s.lineWidth
}

// SecondaryLineWidthF returns the secondary cell border width as a float, clamped so a hairline stays visible.
func (s Style) SecondaryLineWidthF() float64 {
	return float64(max(s.SecondaryLineWidth(), minLineWidth))
}

// SecondaryLineColorARGB returns the secondary cell border color as packed ARGB.
func (s Style) SecondaryLineColorARGB() uint32 { return s.secondaryLineColorARGB }

// HasSecondaryLine reports whether a secondary cell border line is configured and visible.
func (s Style) HasSecondaryLine() bool {
	return s.secondaryLineColor != "" && s.SecondaryLineWidthF() > 0
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

// LabelFontSizeIn returns the size to draw every cell label of one draw at,
// and whether to draw them at all. font_size is the ceiling. The label shrinks
// so that it fits the cell and the cell stays label_autohide_multiplier x the
// font size, and hides once that would take it under min_font_size
// (badge.FontFit is the rule). cells are in device pixels and scale is device
// pixels per font unit, 1 where the two are the same.
//
// It takes every rectangle the labels will be drawn in and answers with the
// size that fits the smallest. The cells of a draw differ by a pixel, and a
// transition passes its first frame's cells with its last, so a label neither
// overflows on the way nor changes font on every frame.
//
// All three backends call this one. The macOS overlay used to decide a second
// time in Objective-C, on every animation frame, because its frames never
// return to Go. It is now handed the two answers a transition needs before the
// transition starts, so that copy and the test pinning it were deleted
// (ADR 0007).
func (s Style) LabelFontSizeIn(scale float64, cells ...image.Rectangle) (float64, bool) {
	return s.labelFit().SizeAcross(scale, cells...)
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
		secondaryLineColor: cfg.UI.SecondaryLineColor.ForTheme(
			theme,
			"",
			"",
		),
		secondaryLineWidth: cfg.UI.SecondaryLineWidth,
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
		minFontSize:            cfg.UI.MinFontSize,
	}.packColors().measureLabels(cfg.AllKeysIncludingLayers())
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

// packColors fills the ARGB fields from the hex ones. Both constructors call it
// as their last step, so no caller can produce a Style whose packed values
// disagree with its hex ones.
func (s Style) packColors() Style {
	s.lineColorARGB = badge.ParseHexARGB(s.lineColor)
	if s.secondaryLineColor != "" {
		s.secondaryLineColorARGB = badge.ParseHexARGB(s.secondaryLineColor)
	}
	s.highlightColorARGB = badge.ParseHexARGB(s.highlightColor)
	s.textColorARGB = badge.ParseHexARGB(s.textColor)
	s.labelBackgroundColorARGB = badge.ParseHexARGB(s.labelBackgroundColor)
	s.subKeyPreviewTextColorARGB = badge.ParseHexARGB(s.subKeyPreviewTextColor)

	return s
}

// labelFit is the fit rule with this style's label settings.
func (s Style) labelFit() badge.FontFit {
	fit := badge.FontFit{
		Requested:  s.LabelFontSize(),
		Floor:      float64(s.minFontSize),
		Multiplier: s.labelAutohideMultiplier,
		Metrics:    s.labelMetrics,
	}

	if s.labelBackground {
		// The plate is what has to fit, and its padding is resolved against
		// the configured size. A smaller font only ever needs less.
		border := s.LabelBackgroundBorderWidthF()
		fit.PadX = float64(
			badge.AutoPadding(fit.Requested, s.labelBackgroundPaddingX, true),
		) + border
		fit.PadY = float64(
			badge.AutoPadding(fit.Requested, s.labelBackgroundPaddingY, false),
		) + border
	}

	return fit
}

// widestCommonLabel stands in for the keys a style cannot see. Bisect's render
// configuration carries none, and a region too small for the configured shape
// falls back to keys of its own. Nothing a keyboard types is wider.
const widestCommonLabel = "W"

// measureLabels fills the label metrics by asking the platform's text layer,
// which is why it runs where a style is built and never in a draw. Labels are
// drawn upper-cased, one key to a cell, unless label_char replaces them all.
func (s Style) measureLabels(keys string) Style {
	// One string per character: splitting on "" cuts at every UTF-8 sequence.
	alphabet := strings.Split(widestCommonLabel+strings.ToUpper(keys), "")

	labels := alphabet
	if s.labelChar != "" {
		labels = []string{s.labelChar}
	}

	previews := alphabet
	if s.subKeyPreviewLabelChar != "" {
		previews = []string{s.subKeyPreviewLabelChar}
	}

	s.labelMetrics = badge.MeasureLabels(labels, s.fontFamily, s.LabelFontSize(), false)
	s.subKeyPreviewMetrics = badge.MeasureLabels(
		previews, s.fontFamily, s.SubKeyPreviewFontSizeF(), false,
	)

	return s
}
