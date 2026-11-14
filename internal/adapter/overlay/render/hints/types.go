package hints

import (
	"image"
	"unicode"
	"unicode/utf8"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

// Hint is a hint to be displayed on the overlay.
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

// TypedPrefix is the part of the label already typed: as many of the label's
// leading characters as the matched prefix runs to, spelled as the label spells
// them. A label draws these in the matched text color and the rest in the text
// color. The macOS overlay colors the same characters, counting them itself.
func (h *Hint) TypedPrefix() string {
	typed := []rune(h.matchedPrefix)
	end := 0

	for index, char := range h.label {
		if len(typed) == 0 || unicode.ToUpper(char) != unicode.ToUpper(typed[0]) {
			break
		}

		typed = typed[1:]
		end = index + utf8.RuneLen(char)
	}

	return h.label[:end]
}

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

// SearchInputStyle is the visual styling configuration for hints search.
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

// FontSize returns the font size.
func (s SearchInputStyle) FontSize() int { return s.fontSize }

// FontFamily returns the font family.
func (s SearchInputStyle) FontFamily() string { return s.fontFamily }

// BorderRadius returns the border radius.
func (s SearchInputStyle) BorderRadius() int { return s.borderRadius }

// PaddingX returns the horizontal padding.
func (s SearchInputStyle) PaddingX() int { return s.paddingX }

// PaddingY returns the vertical padding.
func (s SearchInputStyle) PaddingY() int { return s.paddingY }

// BorderWidth returns the border width.
func (s SearchInputStyle) BorderWidth() int { return s.borderWidth }

// BackgroundColor returns the background color.
func (s SearchInputStyle) BackgroundColor() string { return s.backgroundColor }

// TextColor returns the text color.
func (s SearchInputStyle) TextColor() string { return s.textColor }

// BorderColor returns the border color.
func (s SearchInputStyle) BorderColor() string { return s.borderColor }

// BuildSearchInputStyle returns SearchInputStyle using the provided config.
func BuildSearchInputStyle(cfg config.HintsConfig, theme config.ThemeProvider) SearchInputStyle {
	return SearchInputStyle{
		fontSize:     cfg.SearchInputUI.FontSize,
		fontFamily:   ports.ResolveFont(cfg.SearchInputUI.FontFamily),
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
