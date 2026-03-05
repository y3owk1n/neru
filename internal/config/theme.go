package config

// ThemeProvider is an interface for querying the current macOS system theme.
// This allows the config package to resolve theme-aware defaults without
// depending directly on the bridge package (avoiding import cycles).
type ThemeProvider interface {
	// IsDarkMode returns true if macOS Dark Mode is currently active.
	IsDarkMode() bool
}

// ResolveColor returns the effective color based on the current system theme.
// If both lightColor and darkColor are non-empty, the appropriate one is
// selected based on the theme.
// If only one variant is set, it is used for its matching theme; the other
// theme falls back to the corresponding default (defaultLight or defaultDark).
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

			return defaultDark
		}

		if lightColor != "" {
			return lightColor
		}

		return defaultLight
	}

	// Both are empty, use theme-aware defaults
	if theme != nil && theme.IsDarkMode() {
		return defaultDark
	}

	return defaultLight
}
