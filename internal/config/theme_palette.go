package config

import "strings"

const (
	defaultThemeLightSurface     = "#EEF2FF"
	defaultThemeLightAccent      = "#465FBC"
	defaultThemeLightAccentAlt   = "#0B2377"
	defaultThemeLightOnAccentAlt = "#F8FAFF"
	defaultThemeLightText        = "#17327A"

	defaultThemeDarkSurface     = "#0A1338"
	defaultThemeDarkAccent      = "#6E82D6"
	defaultThemeDarkAccentAlt   = "#8FA2F0"
	defaultThemeDarkOnAccentAlt = "#081022"
	defaultThemeDarkText        = "#E8EEFF"

	shortHexColorLength = 4
	rgbHexColorLength   = 7
	argbHexColorLength  = 9
)

var (
	// VirtualPointerColorLight is the fallback light color for the virtual pointer.
	VirtualPointerColorLight = solidRGBHex(defaultThemeLightAccentAlt)
	// VirtualPointerColorDark is the fallback dark color for the virtual pointer.
	VirtualPointerColorDark = solidRGBHex(defaultThemeDarkAccentAlt)

	// HintsBackgroundColorLight is the fallback light background color for hints.
	HintsBackgroundColorLight = applyAlpha(defaultThemeLightSurface, "F2")
	// HintsBackgroundColorDark is the fallback dark background color for hints.
	HintsBackgroundColorDark = applyAlpha(defaultThemeDarkSurface, "F2")
	// HintsTextColorLight is the fallback light text color for hints.
	HintsTextColorLight = solidRGBHex(defaultThemeLightText)
	// HintsTextColorDark is the fallback dark text color for hints.
	HintsTextColorDark = solidRGBHex(defaultThemeDarkText)
	// HintsMatchedTextColorLight is the fallback light matched text color for hints.
	HintsMatchedTextColorLight = solidRGBHex(defaultThemeLightAccentAlt)
	// HintsMatchedTextColorDark is the fallback dark matched text color for hints.
	HintsMatchedTextColorDark = solidRGBHex(defaultThemeDarkAccentAlt)
	// HintsBorderColorLight is the fallback light border color for hints.
	HintsBorderColorLight = solidRGBHex(defaultThemeLightAccent)
	// HintsBorderColorDark is the fallback dark border color for hints.
	HintsBorderColorDark = solidRGBHex(defaultThemeDarkAccent)
	// HintsBoundaryBackgroundColorLight is the fallback light fill color for hint target boundaries.
	HintsBoundaryBackgroundColorLight = applyAlpha(defaultThemeLightAccent, "14")
	// HintsBoundaryBackgroundColorDark is the fallback dark fill color for hint target boundaries.
	HintsBoundaryBackgroundColorDark = applyAlpha(defaultThemeDarkAccentAlt, "1A")
	// HintsBoundaryBorderColorLight is the fallback light stroke color for hint target boundaries.
	HintsBoundaryBorderColorLight = applyAlpha(defaultThemeLightAccent, "73")
	// HintsBoundaryBorderColorDark is the fallback dark stroke color for hint target boundaries.
	HintsBoundaryBorderColorDark = applyAlpha(defaultThemeDarkAccentAlt, "73")

	// GridBackgroundColorLight is the fallback light background color for grid cells.
	GridBackgroundColorLight = applyAlpha(defaultThemeLightSurface, "99")
	// GridBackgroundColorDark is the fallback dark background color for grid cells.
	GridBackgroundColorDark = applyAlpha(defaultThemeDarkSurface, "99")
	// GridTextColorLight is the fallback light text color for grid cells.
	GridTextColorLight = solidRGBHex(defaultThemeLightText)
	// GridTextColorDark is the fallback dark text color for grid cells.
	GridTextColorDark = solidRGBHex(defaultThemeDarkText)
	// GridMatchedTextColorLight is the fallback light matched text color for grid cells.
	GridMatchedTextColorLight = solidRGBHex(defaultThemeLightOnAccentAlt)
	// GridMatchedTextColorDark is the fallback dark matched text color for grid cells.
	GridMatchedTextColorDark = solidRGBHex(defaultThemeDarkOnAccentAlt)
	// GridMatchedBackgroundColorLight is the fallback light matched background color for grid cells.
	GridMatchedBackgroundColorLight = applyAlpha(defaultThemeLightAccent, "73")
	// GridMatchedBackgroundColorDark is the fallback dark matched background color for grid cells.
	GridMatchedBackgroundColorDark = applyAlpha(defaultThemeDarkAccentAlt, "B3")
	// GridMatchedBorderColorLight is the fallback light matched border color for grid cells.
	GridMatchedBorderColorLight = applyAlpha(defaultThemeLightAccent, "99")
	// GridMatchedBorderColorDark is the fallback dark matched border color for grid cells.
	GridMatchedBorderColorDark = applyAlpha(defaultThemeDarkAccentAlt, "B3")
	// GridBorderColorLight is the fallback light border color for grid cells.
	GridBorderColorLight = applyAlpha(defaultThemeLightAccent, "99")
	// GridBorderColorDark is the fallback dark border color for grid cells.
	GridBorderColorDark = applyAlpha(defaultThemeDarkAccent, "99")

	// RecursiveGridLineColorLight is the fallback light line color for recursive grid.
	RecursiveGridLineColorLight = solidRGBHex(defaultThemeLightAccent)
	// RecursiveGridLineColorDark is the fallback dark line color for recursive grid.
	RecursiveGridLineColorDark = solidRGBHex(defaultThemeDarkAccent)
	// RecursiveGridHighlightColorLight is the fallback light highlight color for recursive grid.
	RecursiveGridHighlightColorLight = applyAlpha(defaultThemeLightAccentAlt, "4D")
	// RecursiveGridHighlightColorDark is the fallback dark highlight color for recursive grid.
	RecursiveGridHighlightColorDark = applyAlpha(defaultThemeDarkAccentAlt, "4D")
	// RecursiveGridTextColorLight is the fallback light text color for recursive grid.
	RecursiveGridTextColorLight = solidRGBHex(defaultThemeLightAccent)
	// RecursiveGridTextColorDark is the fallback dark text color for recursive grid.
	RecursiveGridTextColorDark = solidRGBHex(defaultThemeDarkAccent)
	// RecursiveGridLabelBackgroundColorLight is the fallback light label background color for recursive grid.
	RecursiveGridLabelBackgroundColorLight = solidRGBHex(defaultThemeLightSurface)
	// RecursiveGridLabelBackgroundColorDark is the fallback dark label background color for recursive grid.
	RecursiveGridLabelBackgroundColorDark = solidRGBHex(defaultThemeDarkSurface)
	// RecursiveGridSubKeyPreviewTextColorLight is the fallback light sub-key preview text color.
	RecursiveGridSubKeyPreviewTextColorLight = solidRGBHex(defaultThemeLightAccent)
	// RecursiveGridSubKeyPreviewTextColorDark is the fallback dark sub-key preview text color.
	RecursiveGridSubKeyPreviewTextColorDark = solidRGBHex(defaultThemeDarkAccent)

	// ModeIndicatorBackgroundColorLight is the fallback light background color for the mode indicator.
	ModeIndicatorBackgroundColorLight = applyAlpha(defaultThemeLightSurface, "F2")
	// ModeIndicatorBackgroundColorDark is the fallback dark background color for the mode indicator.
	ModeIndicatorBackgroundColorDark = applyAlpha(defaultThemeDarkSurface, "F2")
	// ModeIndicatorTextColorLight is the fallback light text color for the mode indicator.
	ModeIndicatorTextColorLight = solidRGBHex(defaultThemeLightText)
	// ModeIndicatorTextColorDark is the fallback dark text color for the mode indicator.
	ModeIndicatorTextColorDark = solidRGBHex(defaultThemeDarkText)
	// ModeIndicatorBorderColorLight is the fallback light border color for the mode indicator.
	ModeIndicatorBorderColorLight = solidRGBHex(defaultThemeLightAccent)
	// ModeIndicatorBorderColorDark is the fallback dark border color for the mode indicator.
	ModeIndicatorBorderColorDark = solidRGBHex(defaultThemeDarkAccent)

	// MouseActionBackgroundColorLight is the fallback light fill color for mouse action indicators.
	MouseActionBackgroundColorLight = applyAlpha(defaultThemeLightAccentAlt, "30")
	// MouseActionBackgroundColorDark is the fallback dark fill color for mouse action indicators.
	MouseActionBackgroundColorDark = applyAlpha(defaultThemeDarkAccentAlt, "40")
	// MouseActionBorderColorLight is the fallback light border color for mouse action indicators.
	MouseActionBorderColorLight = solidRGBHex(defaultThemeLightAccentAlt)
	// MouseActionBorderColorDark is the fallback dark border color for mouse action indicators.
	MouseActionBorderColorDark = solidRGBHex(defaultThemeDarkAccentAlt)

	// StickyModifiersBackgroundColorLight is the fallback light background color for sticky modifiers.
	StickyModifiersBackgroundColorLight = applyAlpha(defaultThemeLightSurface, "F2")
	// StickyModifiersBackgroundColorDark is the fallback dark background color for sticky modifiers.
	StickyModifiersBackgroundColorDark = applyAlpha(defaultThemeDarkSurface, "F2")
	// StickyModifiersTextColorLight is the fallback light text color for sticky modifiers.
	StickyModifiersTextColorLight = solidRGBHex(defaultThemeLightText)
	// StickyModifiersTextColorDark is the fallback dark text color for sticky modifiers.
	StickyModifiersTextColorDark = solidRGBHex(defaultThemeDarkText)
	// StickyModifiersBorderColorLight is the fallback light border color for sticky modifiers.
	StickyModifiersBorderColorLight = solidRGBHex(defaultThemeLightAccent)
	// StickyModifiersBorderColorDark is the fallback dark border color for sticky modifiers.
	StickyModifiersBorderColorDark = solidRGBHex(defaultThemeDarkAccent)

	// MonitorSelectBackgroundColorLight is the fallback light background color for monitor select badges.
	MonitorSelectBackgroundColorLight = applyAlpha(defaultThemeLightSurface, "F2")
	// MonitorSelectBackgroundColorDark is the fallback dark background color for monitor select badges.
	MonitorSelectBackgroundColorDark = applyAlpha(defaultThemeDarkSurface, "F2")
	// MonitorSelectTextColorLight is the fallback light text color for monitor select labels.
	MonitorSelectTextColorLight = solidRGBHex(defaultThemeLightText)
	// MonitorSelectTextColorDark is the fallback dark text color for monitor select labels.
	MonitorSelectTextColorDark = solidRGBHex(defaultThemeDarkText)
	// MonitorSelectMatchedTextColorLight is the fallback light matched-text color for monitor select labels.
	MonitorSelectMatchedTextColorLight = solidRGBHex(defaultThemeLightAccentAlt)
	// MonitorSelectMatchedTextColorDark is the fallback dark matched-text color for monitor select labels.
	MonitorSelectMatchedTextColorDark = solidRGBHex(defaultThemeDarkAccentAlt)
	// MonitorSelectBorderColorLight is the fallback light border color for monitor select badges.
	MonitorSelectBorderColorLight = solidRGBHex(defaultThemeLightAccent)
	// MonitorSelectBorderColorDark is the fallback dark border color for monitor select badges.
	MonitorSelectBorderColorDark = solidRGBHex(defaultThemeDarkAccent)
	// MonitorSelectBackdropColorLight is the fallback light backdrop tint for monitor select panels.
	MonitorSelectBackdropColorLight = applyAlpha("#000000", "33")
	// MonitorSelectBackdropColorDark is the fallback dark backdrop tint for monitor select panels.
	MonitorSelectBackdropColorDark = applyAlpha("#000000", "66")
	// MonitorSelectCurrentBackgroundColorLight is the fallback light background for the current-monitor badge.
	MonitorSelectCurrentBackgroundColorLight = applyAlpha(defaultThemeLightAccent, "73")
	// MonitorSelectCurrentBackgroundColorDark is the fallback dark background for the current-monitor badge.
	MonitorSelectCurrentBackgroundColorDark = applyAlpha(defaultThemeDarkAccentAlt, "B3")
	// MonitorSelectCurrentTextColorLight is the fallback light text color for the current-monitor badge.
	MonitorSelectCurrentTextColorLight = solidRGBHex(defaultThemeLightOnAccentAlt)
	// MonitorSelectCurrentTextColorDark is the fallback dark text color for the current-monitor badge.
	MonitorSelectCurrentTextColorDark = solidRGBHex(defaultThemeDarkOnAccentAlt)
	// MonitorSelectCurrentBorderColorLight is the fallback light border color for the current-monitor badge.
	MonitorSelectCurrentBorderColorLight = solidRGBHex(defaultThemeLightAccent)
	// MonitorSelectCurrentBorderColorDark is the fallback dark border color for the current-monitor badge.
	MonitorSelectCurrentBorderColorDark = solidRGBHex(defaultThemeDarkAccent)
	// MonitorSelectSubtitleTextColorLight is the fallback light subtitle text color for monitor select badges.
	MonitorSelectSubtitleTextColorLight = applyAlpha(defaultThemeLightText, "B3")
	// MonitorSelectSubtitleTextColorDark is the fallback dark subtitle text color for monitor select badges.
	MonitorSelectSubtitleTextColorDark = applyAlpha(defaultThemeDarkText, "B3")
)

func defaultThemeConfig() ThemeConfig {
	return ThemeConfig{
		Light: ThemePalette{
			Surface:     defaultThemeLightSurface,
			Accent:      defaultThemeLightAccent,
			AccentAlt:   defaultThemeLightAccentAlt,
			OnAccentAlt: defaultThemeLightOnAccentAlt,
			Text:        defaultThemeLightText,
		},
		Dark: ThemePalette{
			Surface:     defaultThemeDarkSurface,
			Accent:      defaultThemeDarkAccent,
			AccentAlt:   defaultThemeDarkAccentAlt,
			OnAccentAlt: defaultThemeDarkOnAccentAlt,
			Text:        defaultThemeDarkText,
		},
	}
}

func expandHexRGB(color string) string {
	if len(color) != shortHexColorLength || color[0] != '#' {
		return ""
	}

	var builder strings.Builder
	builder.Grow(rgbHexColorLength)
	builder.WriteByte('#')

	for i := 1; i < len(color); i++ {
		builder.WriteByte(color[i])
		builder.WriteByte(color[i])
	}

	return builder.String()
}

func solidRGBHex(color string) string {
	switch len(color) {
	case shortHexColorLength:
		return expandHexRGB(color)
	case rgbHexColorLength:
		return color
	case argbHexColorLength:
		return "#" + color[3:]
	default:
		return ""
	}
}

func applyAlpha(color, alpha string) string {
	rgb := solidRGBHex(color)
	if rgb == "" || len(alpha) != 2 {
		return ""
	}

	return "#" + alpha + rgb[1:]
}

func mergeColorWithDefault(target *Color, fallback Color) {
	if target.Light == "" {
		target.Light = fallback.Light
	}

	if target.Dark == "" {
		target.Dark = fallback.Dark
	}
}

func themedColor(lightBase, darkBase, alpha string) Color {
	return Color{
		Light: applyAlpha(lightBase, alpha),
		Dark:  applyAlpha(darkBase, alpha),
	}
}

func solidThemedColor(lightBase, darkBase string) Color {
	return Color{
		Light: solidRGBHex(lightBase),
		Dark:  solidRGBHex(darkBase),
	}
}

// ResolveThemeDefaults derives component color defaults from the configured
// light/dark theme palettes. Explicit component colors are preserved.
func (c *Config) ResolveThemeDefaults() {
	mergeColorWithDefault(&c.Hints.UI.BackgroundColor, themedColor(
		c.Theme.Light.Surface, c.Theme.Dark.Surface, "F2",
	))
	mergeColorWithDefault(&c.Hints.UI.TextColor, solidThemedColor(
		c.Theme.Light.Text, c.Theme.Dark.Text,
	))
	mergeColorWithDefault(&c.Hints.UI.MatchedTextColor, solidThemedColor(
		c.Theme.Light.AccentAlt, c.Theme.Dark.AccentAlt,
	))
	mergeColorWithDefault(&c.Hints.UI.BorderColor, solidThemedColor(
		c.Theme.Light.Accent, c.Theme.Dark.Accent,
	))
	mergeColorWithDefault(&c.Hints.SearchInputUI.BackgroundColor, themedColor(
		c.Theme.Light.Surface, c.Theme.Dark.Surface, "F2",
	))
	mergeColorWithDefault(&c.Hints.SearchInputUI.TextColor, solidThemedColor(
		c.Theme.Light.Text, c.Theme.Dark.Text,
	))
	mergeColorWithDefault(&c.Hints.SearchInputUI.BorderColor, solidThemedColor(
		c.Theme.Light.Accent, c.Theme.Dark.Accent,
	))
	mergeColorWithDefault(&c.Hints.BoundaryHighlight.BackgroundColor, themedColor(
		c.Theme.Light.Accent, c.Theme.Dark.AccentAlt, "1A",
	))
	mergeColorWithDefault(&c.Hints.BoundaryHighlight.BorderColor, themedColor(
		c.Theme.Light.Accent, c.Theme.Dark.AccentAlt, "73",
	))

	mergeColorWithDefault(&c.Grid.UI.BackgroundColor, themedColor(
		c.Theme.Light.Surface, c.Theme.Dark.Surface, "99",
	))
	mergeColorWithDefault(&c.Grid.UI.TextColor, solidThemedColor(
		c.Theme.Light.Text, c.Theme.Dark.Text,
	))
	mergeColorWithDefault(&c.Grid.UI.MatchedTextColor, Color{
		Light: solidRGBHex(c.Theme.Light.OnAccentAlt),
		Dark:  solidRGBHex(c.Theme.Dark.OnAccentAlt),
	})
	mergeColorWithDefault(&c.Grid.UI.MatchedBackgroundColor, Color{
		Light: applyAlpha(c.Theme.Light.Accent, "73"),
		Dark:  applyAlpha(c.Theme.Dark.AccentAlt, "B3"),
	})
	mergeColorWithDefault(&c.Grid.UI.MatchedBorderColor, Color{
		Light: applyAlpha(c.Theme.Light.Accent, "99"),
		Dark:  applyAlpha(c.Theme.Dark.AccentAlt, "B3"),
	})
	mergeColorWithDefault(&c.Grid.UI.BorderColor, themedColor(
		c.Theme.Light.Accent, c.Theme.Dark.Accent, "99",
	))

	mergeColorWithDefault(&c.RecursiveGrid.UI.LineColor, solidThemedColor(
		c.Theme.Light.Accent, c.Theme.Dark.Accent,
	))
	mergeColorWithDefault(&c.RecursiveGrid.UI.HighlightColor, themedColor(
		c.Theme.Light.AccentAlt, c.Theme.Dark.AccentAlt, "4D",
	))
	mergeColorWithDefault(&c.RecursiveGrid.UI.TextColor, solidThemedColor(
		c.Theme.Light.Accent, c.Theme.Dark.Accent,
	))
	mergeColorWithDefault(&c.RecursiveGrid.UI.LabelBackgroundColor, solidThemedColor(
		c.Theme.Light.Surface, c.Theme.Dark.Surface,
	))
	mergeColorWithDefault(&c.RecursiveGrid.UI.SubKeyPreviewTextColor, solidThemedColor(
		c.Theme.Light.Accent, c.Theme.Dark.Accent,
	))

	mergeColorWithDefault(&c.VirtualPointer.UI.Color, solidThemedColor(
		c.Theme.Light.AccentAlt, c.Theme.Dark.AccentAlt,
	))

	mergeColorWithDefault(&c.ModeIndicator.UI.BackgroundColor, themedColor(
		c.Theme.Light.Surface, c.Theme.Dark.Surface, "F2",
	))
	mergeColorWithDefault(&c.ModeIndicator.UI.TextColor, solidThemedColor(
		c.Theme.Light.Text, c.Theme.Dark.Text,
	))
	mergeColorWithDefault(&c.ModeIndicator.UI.BorderColor, solidThemedColor(
		c.Theme.Light.Accent, c.Theme.Dark.Accent,
	))

	mergeColorWithDefault(&c.MouseAction.UI.BackgroundColor, themedColor(
		c.Theme.Light.AccentAlt, c.Theme.Dark.AccentAlt, "30",
	))
	mergeColorWithDefault(&c.MouseAction.UI.BorderColor, solidThemedColor(
		c.Theme.Light.AccentAlt, c.Theme.Dark.AccentAlt,
	))

	mergeColorWithDefault(&c.StickyModifiers.UI.BackgroundColor, themedColor(
		c.Theme.Light.Surface, c.Theme.Dark.Surface, "F2",
	))
	mergeColorWithDefault(&c.StickyModifiers.UI.TextColor, solidThemedColor(
		c.Theme.Light.Text, c.Theme.Dark.Text,
	))
	mergeColorWithDefault(&c.StickyModifiers.UI.BorderColor, solidThemedColor(
		c.Theme.Light.Accent, c.Theme.Dark.Accent,
	))

	mergeColorWithDefault(&c.MonitorSelect.UI.BackgroundColor, themedColor(
		c.Theme.Light.Surface, c.Theme.Dark.Surface, "F2",
	))
	mergeColorWithDefault(&c.MonitorSelect.UI.TextColor, solidThemedColor(
		c.Theme.Light.Text, c.Theme.Dark.Text,
	))
	mergeColorWithDefault(&c.MonitorSelect.UI.MatchedTextColor, solidThemedColor(
		c.Theme.Light.AccentAlt, c.Theme.Dark.AccentAlt,
	))
	mergeColorWithDefault(&c.MonitorSelect.UI.BorderColor, solidThemedColor(
		c.Theme.Light.Accent, c.Theme.Dark.Accent,
	))
	mergeColorWithDefault(&c.MonitorSelect.UI.BackdropColor, Color{
		Light: applyAlpha("#000000", "33"),
		Dark:  applyAlpha("#000000", "66"),
	})
	mergeColorWithDefault(&c.MonitorSelect.UI.CurrentBackgroundColor, Color{
		Light: applyAlpha(c.Theme.Light.Accent, "73"),
		Dark:  applyAlpha(c.Theme.Dark.AccentAlt, "B3"),
	})
	mergeColorWithDefault(&c.MonitorSelect.UI.CurrentTextColor, Color{
		Light: solidRGBHex(c.Theme.Light.OnAccentAlt),
		Dark:  solidRGBHex(c.Theme.Dark.OnAccentAlt),
	})
	mergeColorWithDefault(&c.MonitorSelect.UI.CurrentBorderColor, solidThemedColor(
		c.Theme.Light.Accent, c.Theme.Dark.Accent,
	))
	mergeColorWithDefault(&c.MonitorSelect.UI.SubtitleTextColor, Color{
		Light: applyAlpha(c.Theme.Light.Text, "B3"),
		Dark:  applyAlpha(c.Theme.Dark.Text, "B3"),
	})
}
