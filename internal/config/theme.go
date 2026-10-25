package config

// ThemeProvider reports the current macOS system theme, so the config package
// can resolve theme-aware defaults without importing the bridge package and
// creating an import cycle.
type ThemeProvider interface {
	// IsDarkMode returns true if macOS Dark Mode is currently active.
	IsDarkMode() bool
}
