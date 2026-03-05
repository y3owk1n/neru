package config

// ThemeProvider is an interface for querying the current macOS system theme.
// This allows the config package to resolve theme-aware defaults without
// depending directly on the bridge package (avoiding import cycles).
type ThemeProvider interface {
	// IsDarkMode returns true if macOS Dark Mode is currently active.
	IsDarkMode() bool
}

// ResolveColor returns the effective color based on the current system theme.
// If lightColor is non-empty, it is used as the base. If darkColor is also non-empty,
// the appropriate color is selected based on the theme.
// If darkColor is empty, falls back to lightColor.
// If lightColor is empty, falls back to darkColor.
// If both are empty, returns the provided theme-aware defaults.
func ResolveColor(
	lightColor, darkColor string,
	theme ThemeProvider,
	defaultLight, defaultDark string,
) string {
	if lightColor != "" || darkColor != "" {
		if theme != nil && theme.IsDarkMode() {
			if darkColor != "" {
				return darkColor
			}
		}

		if lightColor != "" {
			return lightColor
		}

		return darkColor
	}

	// Both are empty, use theme-aware defaults
	if theme != nil && theme.IsDarkMode() {
		return defaultDark
	}

	return defaultLight
}
