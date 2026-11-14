package hints

import (
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ports"
)

// StyleMode is the visual styling for hint overlays, one type for every
// platform, because nothing about a hint's appearance differs by operating system.
//
// Colors are the hex strings the configuration uses. A backend drawing with
// packed ARGB converts at draw time, that being a property of the drawing API
// rather than of the style.
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
	backgroundGradientColor  string
	shadowColor              string
	shadowOffsetX            int
	shadowOffsetY            int
	shadowBlur               int
	textShadowColor          string
	textShadowOffsetX        int
	textShadowOffsetY        int
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

// BackgroundGradientColor returns the color the background fades to at the
// bottom of the badge, or "" for a flat background.
func (s StyleMode) BackgroundGradientColor() string { return s.backgroundGradientColor }

// ShadowColor returns the color of the shadow under the badge, or "" for none.
func (s StyleMode) ShadowColor() string { return s.shadowColor }

// ShadowOffsetX returns how far right of the badge its shadow falls, in points.
func (s StyleMode) ShadowOffsetX() int { return s.shadowOffsetX }

// ShadowOffsetY returns how far below the badge its shadow falls, in points.
func (s StyleMode) ShadowOffsetY() int { return s.shadowOffsetY }

// ShadowBlur returns the shadow's CSS blur radius in points.
func (s StyleMode) ShadowBlur() int { return s.shadowBlur }

// TextShadowColor returns the color of the copy of the label drawn under it,
// or "" for none.
func (s StyleMode) TextShadowColor() string { return s.textShadowColor }

// TextShadowOffsetX returns how far right of the label its shadow falls, in points.
func (s StyleMode) TextShadowOffsetX() int { return s.textShadowOffsetX }

// TextShadowOffsetY returns how far below the label its shadow falls, in points.
func (s StyleMode) TextShadowOffsetY() int { return s.textShadowOffsetY }

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

// BuildStyle builds the hints style from the configuration and the active theme.
func BuildStyle(cfg config.HintsConfig, theme config.ThemeProvider) StyleMode {
	return StyleMode{
		fontSize:     cfg.UI.FontSize,
		fontFamily:   ports.ResolveFont(cfg.UI.FontFamily),
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
		// The gradient and the shadows have no theme default. Unset is off.
		backgroundGradientColor:  cfg.UI.BackgroundGradientColor.ForTheme(theme, "", ""),
		shadowColor:              cfg.UI.ShadowColor.ForTheme(theme, "", ""),
		shadowOffsetX:            cfg.UI.ShadowOffsetX,
		shadowOffsetY:            cfg.UI.ShadowOffsetY,
		shadowBlur:               cfg.UI.ShadowBlur,
		textShadowColor:          cfg.UI.TextShadowColor.ForTheme(theme, "", ""),
		textShadowOffsetX:        cfg.UI.TextShadowOffsetX,
		textShadowOffsetY:        cfg.UI.TextShadowOffsetY,
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
