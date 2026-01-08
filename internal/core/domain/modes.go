package domain

// Mode is the current mode of the application.
type Mode int

const (
	// ModeIdle represents the idle mode.
	ModeIdle Mode = iota
	// ModeHints represents the hints mode.
	ModeHints
	// ModeGrid represents the grid mode.
	ModeGrid
	// ModeScroll represents the scroll mode.
	ModeScroll
)
