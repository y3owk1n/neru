package config

// ThemeProvider is an interface for querying the current macOS system theme.
// This allows the config package to resolve theme-aware defaults without
// depending directly on the bridge package (avoiding import cycles).
type ThemeProvider interface {
	// IsDarkMode returns true if macOS Dark Mode is currently active.
	IsDarkMode() bool
}
