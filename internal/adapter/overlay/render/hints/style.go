package hints

import (
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

// The hint style is one type for every platform. Nothing about a hint's
// appearance differs by operating system: the same font size, padding, radius
// and colors describe it everywhere.
//
// Colors are the hex strings the configuration uses. A backend that draws with
// packed ARGB converts at draw time, since that is a property of the drawing
// API rather than of the style.

// StyleMode is the visual styling configuration for hint overlays.
type StyleMode struct {
	fontSize                 int
	fontFamily               string
	borderRadius             int
	paddingX                 int
	paddingY                 int
	borderWidth              int
	placement                string
	backgroundColor          string
	textColor                string
	matchedTextColor         string
	borderColor              string
	boundaryHighlightEnabled bool
	boundaryBorderWidth      int
	boundaryBorderRadius     int
	boundaryBackgroundColor  string
	boundaryBorderColor      string
}

// FontSize returns the font size.
func (s StyleMode) FontSize() int { return s.fontSize }

// FontFamily returns the font family.
func (s StyleMode) FontFamily() string { return s.fontFamily }

// BorderRadius returns the border radius.
func (s StyleMode) BorderRadius() int { return s.borderRadius }

// PaddingX returns the padding X.
func (s StyleMode) PaddingX() int { return s.paddingX }

// PaddingY returns the padding Y.
func (s StyleMode) PaddingY() int { return s.paddingY }

// BorderWidth returns the border width.
func (s StyleMode) BorderWidth() int { return s.borderWidth }

// Placement returns the hint label placement relative to the target.
func (s StyleMode) Placement() string { return s.placement }

// BackgroundColor returns the background color.
func (s StyleMode) BackgroundColor() string { return s.backgroundColor }

// TextColor returns the text color.
func (s StyleMode) TextColor() string { return s.textColor }

// MatchedTextColor returns the matched text color.
func (s StyleMode) MatchedTextColor() string { return s.matchedTextColor }

// BorderColor returns the border color.
func (s StyleMode) BorderColor() string { return s.borderColor }

// BoundaryHighlightEnabled returns whether target boundaries are drawn behind hints.
func (s StyleMode) BoundaryHighlightEnabled() bool { return s.boundaryHighlightEnabled }

// BoundaryBorderWidth returns the target boundary stroke width.
func (s StyleMode) BoundaryBorderWidth() int { return s.boundaryBorderWidth }

// BoundaryBorderRadius returns the target boundary corner radius.
func (s StyleMode) BoundaryBorderRadius() int { return s.boundaryBorderRadius }

// BoundaryBackgroundColor returns the target boundary fill color.
func (s StyleMode) BoundaryBackgroundColor() string { return s.boundaryBackgroundColor }

// BoundaryBorderColor returns the target boundary stroke color.
func (s StyleMode) BoundaryBorderColor() string { return s.boundaryBorderColor }

// Overlay manages the rendering of hint overlays using native platform APIs (Linux stub).

// BuildStyle builds the hints style from the configuration .
func BuildStyle(cfg config.HintsConfig, theme config.ThemeProvider) StyleMode {
	return StyleMode{
		fontSize:     cfg.UI.FontSize,
		fontFamily:   ports.ResolveFont(cfg.UI.FontFamily, true),
		borderRadius: cfg.UI.BorderRadius,
		paddingX:     cfg.UI.PaddingX,
		paddingY:     cfg.UI.PaddingY,
		borderWidth:  cfg.UI.BorderWidth,
		placement:    cfg.UI.Placement,
		backgroundColor: cfg.UI.BackgroundColor.ForTheme(
			theme,
			config.HintsBackgroundColorLight,
			config.HintsBackgroundColorDark,
		),
		textColor: cfg.UI.TextColor.ForTheme(
			theme,
			config.HintsTextColorLight,
			config.HintsTextColorDark,
		),
		matchedTextColor: cfg.UI.MatchedTextColor.ForTheme(
			theme,
			config.HintsMatchedTextColorLight,
			config.HintsMatchedTextColorDark,
		),
		borderColor: cfg.UI.BorderColor.ForTheme(
			theme,
			config.HintsBorderColorLight,
			config.HintsBorderColorDark,
		),
		boundaryHighlightEnabled: cfg.BoundaryHighlight.Enabled,
		boundaryBorderWidth:      cfg.BoundaryHighlight.BorderWidth,
		boundaryBorderRadius:     cfg.BoundaryHighlight.BorderRadius,
		boundaryBackgroundColor: cfg.BoundaryHighlight.BackgroundColor.ForTheme(
			theme,
			config.HintsBoundaryBackgroundColorLight,
			config.HintsBoundaryBackgroundColorDark,
		),
		boundaryBorderColor: cfg.BoundaryHighlight.BorderColor.ForTheme(
			theme,
			config.HintsBoundaryBorderColorLight,
			config.HintsBoundaryBorderColorDark,
		),
	}
}
