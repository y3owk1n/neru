package config

// ThemeProvider is an interface for querying the current macOS system theme.
// This allows the config package to resolve theme-aware defaults without
// depending directly on the bridge package (avoiding import cycles).
type ThemeProvider interface {
	// IsDarkMode returns true if macOS Dark Mode is currently active.
	IsDarkMode() bool
}

// ResolvedLabelColor returns the effective label color for the recursive grid.
// If the configured LabelColor is non-empty (user-specified), it is returned as-is.
// If it is empty (theme-aware default), the color is resolved based on the current
// system theme: white (#FFFFFFFF) in Dark Mode, black (#FF000000) in Light Mode.
func ResolvedLabelColor(labelColor string, theme ThemeProvider) string {
	if labelColor != "" {
		return labelColor
	}

	if theme != nil && theme.IsDarkMode() {
		return LabelColorDarkMode
	}

	return LabelColorLightMode
}
