package grid

import (
	"image"
	"strings"
	"unicode/utf8"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
	"github.com/y3owk1n/neru/internal/config"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

const (
	// minLineWidth keeps a hairline visible on backends that would otherwise
	// round a zero-width stroke away.
	minLineWidth = 1

	// minLabelFontSize is the smallest a grid label shrinks to. A grid label is
	// never hidden, because typing it is the mode. Below this it is drawn at
	// this size and left to overhang, as every size used to be.
	minLabelFontSize = 4

	// capitalHeightPerSize is how tall a grid label is per unit of font size.
	// Labels are drawn upper-cased, so what has to fit a cell is a capital, not
	// the font's whole line. A line is half as tall again, and fitting to it
	// would shrink the default subgrid label on macOS, which sits a 10 point
	// font in a 10 point sub-cell and reads fine. 0.8 is a capital of any
	// ordinary face with a little room over it.
	capitalHeightPerSize = 0.8

	// typicalCharacters stands in for the label characters when the style can
	// see none, since grid.characters may fall back to another section's.
	typicalCharacters = "ASDFGHJKL"
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

	// characterMetrics is what a label character takes per unit of font size,
	// averaged over the alphabet and measured once when the style is built, so
	// that fitting a label is arithmetic on the keypress path.
	characterMetrics badge.LabelMetrics
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

// LineWidth returns the border width as a float, clamped so a hairline stays
// visible.
func (s Style) LineWidth() float64 { return float64(max(s.borderWidth, minLineWidth)) }

// LabelFontSize returns the configured label font size as a float. A draw
// uses LabelFontSizeFor, which fits it to the cells.
func (s Style) LabelFontSize() float64 { return float64(s.fontSize) }

// LabelFontSizeFor returns the size every label of a grid is drawn at:
// font_size, shrunk until a label fits the smallest cell (badge.FontFit), which
// only a large font_size ever needs, since cells keep a 30 to 50 pixel floor.
// scale is device pixels per font unit, 1 where the two are the same.
//
// A label is fitted at the alphabet's average character width rather than its
// widest. Grid labels are several characters of a whole alphabet, so the
// widest case is a label of nothing but W, and fitting to it would shrink the
// default 10 point label in a 30 pixel cell that every ordinary label already
// fits. The few all-wide labels may touch their border, as they always could.
//
// It reads one cell. Every coordinate of a grid has one length, and the
// remainder pixels go to the leading rows and columns, so the last cell is as
// small as any. Asking it alone is what keeps a narrowing redraw, which draws
// a subset, at the size the full draw chose.
func (s Style) LabelFontSizeFor(scale float64, grid *domainGrid.Grid) float64 {
	cells := grid.AllCells()
	if len(cells) == 0 {
		return s.LabelFontSize()
	}

	smallest := cells[len(cells)-1]

	return s.fitLabel(
		scale, s.LabelFontSize(),
		utf8.RuneCountInString(smallest.Coordinate()),
		smallest.Bounds(),
	)
}

// SubgridLabelFontSizeIn returns the size a subgrid's one-character labels are
// drawn at. That is the grid's font size times the backend's subgrid factor,
// shrunk to fit the smallest of cells.
func (s Style) SubgridLabelFontSizeIn(
	scale, subgridFactor float64,
	cells []image.Rectangle,
) float64 {
	requested := s.LabelFontSize() * subgridFactor
	size := requested

	for _, cell := range cells {
		size = min(size, s.fitLabel(scale, requested, 1, cell))
	}

	return size
}

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
		fontFamily:  ports.ResolveFont(cfg.UI.FontFamily),
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

	style.backgroundColorARGB = badge.ParseHexARGB(style.backgroundColor)
	style.textColorARGB = badge.ParseHexARGB(style.textColor)
	style.matchedTextColorARGB = badge.ParseHexARGB(style.matchedTextColor)
	style.matchedBackgroundColorARGB = badge.ParseHexARGB(style.matchedBackgroundColor)
	style.matchedBorderColorARGB = badge.ParseHexARGB(style.matchedBorderColor)
	style.borderColorARGB = badge.ParseHexARGB(style.borderColor)

	style.characterMetrics = measureCharacters(
		cfg.Characters+cfg.RowLabels+cfg.ColLabels+cfg.SublayerKeys,
		style.fontFamily, style.LabelFontSize(),
	)

	return style
}

// fitLabel fits a label of runes characters to one cell, and never hides it.
func (s Style) fitLabel(scale, requested float64, runes int, cell image.Rectangle) float64 {
	fit := badge.FontFit{
		Requested: requested,
		Metrics:   s.characterMetrics.Repeated(runes),
	}

	size, fits := fit.SizeIn(cell, scale)
	if !fits || size < minLabelFontSize {
		return min(requested, minLabelFontSize)
	}

	return size
}

// measureCharacters answers what one label character takes per unit of font
// size. That is the average width over the characters labels are written with,
// and a capital's height. Labels are drawn upper-cased. It asks the platform's
// text layer, which is why it runs where a style is built and never in a draw.
//
// It decides nothing about which characters a grid uses. written is only what
// the style can see of them, and when that is nothing a typical alphabet is
// measured in its place, because a width is needed either way.
func measureCharacters(written, family string, size float64) badge.LabelMetrics {
	written = strings.ToUpper(written)
	if written == "" {
		written = typicalCharacters
	}

	var totalWidth float64

	for _, character := range written {
		totalWidth += badge.MeasureLabels(
			[]string{string(character)}, family, size, false,
		).WidthPerSize
	}

	return badge.LabelMetrics{
		WidthPerSize:  totalWidth / float64(utf8.RuneCountInString(written)),
		HeightPerSize: capitalHeightPerSize,
	}
}
