package hints

import (
	"image"

	"github.com/y3owk1n/neru/internal/config"
)

// Hint represents a hint to be displayed on the overlay.
type Hint struct {
	label         string
	position      image.Point
	size          image.Point
	matchedPrefix string
}

// NewHint creates a new Hint with the specified values.
func NewHint(label string, position, size image.Point, matchedPrefix string) *Hint {
	return &Hint{
		label:         label,
		position:      position,
		size:          size,
		matchedPrefix: matchedPrefix,
	}
}

// Label returns the hint label.
func (h *Hint) Label() string {
	return h.label
}

// Position returns the hint position.
func (h *Hint) Position() image.Point {
	return h.position
}

// Size returns the hint size.
func (h *Hint) Size() image.Point {
	return h.size
}

// MatchedPrefix returns the matched prefix.
func (h *Hint) MatchedPrefix() string {
	return h.matchedPrefix
}

// SearchInputPosition identifies where the hints search UI is anchored.
type SearchInputPosition string

const (
	SearchInputTopLeft      SearchInputPosition = "top_left"
	SearchInputTopCenter    SearchInputPosition = "top_center"
	SearchInputTopRight     SearchInputPosition = "top_right"
	SearchInputCenter       SearchInputPosition = "center"
	SearchInputBottomLeft   SearchInputPosition = "bottom_left"
	SearchInputBottomCenter SearchInputPosition = "bottom_center"
	SearchInputBottomRight  SearchInputPosition = "bottom_right"
)

// SearchInputFrame describes the search UI geometry in overlay-local coordinates.
type SearchInputFrame struct {
	position image.Point
	width    int
}

// NewSearchInputFrame creates a search input frame.
func NewSearchInputFrame(position image.Point, width int) SearchInputFrame {
	return SearchInputFrame{
		position: position,
		width:    width,
	}
}

// Position returns the search input top-left position in overlay-local coordinates.
func (f SearchInputFrame) Position() image.Point {
	return f.position
}

// Width returns the search input width.
func (f SearchInputFrame) Width() int {
	return f.width
}

// SearchInputStyle represents the visual styling configuration for hints search.
type SearchInputStyle struct {
	fontSize        int
	fontFamily      string
	borderRadius    int
	paddingX        int
	paddingY        int
	borderWidth     int
	backgroundColor string
	textColor       string
	borderColor     string
}

func (s SearchInputStyle) FontSize() int           { return s.fontSize }
func (s SearchInputStyle) FontFamily() string      { return s.fontFamily }
func (s SearchInputStyle) BorderRadius() int       { return s.borderRadius }
func (s SearchInputStyle) PaddingX() int           { return s.paddingX }
func (s SearchInputStyle) PaddingY() int           { return s.paddingY }
func (s SearchInputStyle) BorderWidth() int        { return s.borderWidth }
func (s SearchInputStyle) BackgroundColor() string { return s.backgroundColor }
func (s SearchInputStyle) TextColor() string       { return s.textColor }
func (s SearchInputStyle) BorderColor() string     { return s.borderColor }

// BuildSearchInputStyle returns SearchInputStyle using the provided config.
func BuildSearchInputStyle(cfg config.HintsConfig, theme config.ThemeProvider) SearchInputStyle {
	return SearchInputStyle{
		fontSize:     cfg.SearchInputUI.FontSize,
		fontFamily:   cfg.SearchInputUI.FontFamily,
		borderRadius: cfg.SearchInputUI.BorderRadius,
		paddingX:     cfg.SearchInputUI.PaddingX,
		paddingY:     cfg.SearchInputUI.PaddingY,
		borderWidth:  cfg.SearchInputUI.BorderWidth,
		backgroundColor: cfg.SearchInputUI.BackgroundColor.ForTheme(
			theme,
			config.HintsBackgroundColorLight,
			config.HintsBackgroundColorDark,
		),
		textColor: cfg.SearchInputUI.TextColor.ForTheme(
			theme,
			config.HintsTextColorLight,
			config.HintsTextColorDark,
		),
		borderColor: cfg.SearchInputUI.BorderColor.ForTheme(
			theme,
			config.HintsBorderColorLight,
			config.HintsBorderColorDark,
		),
	}
}
