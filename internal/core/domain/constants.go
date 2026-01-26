package domain

import "time"

// Mode-related constants.
const (
	UnknownAction = "unknown"
	UnknownMode   = "unknown"
)

// Bundle ID constants for macOS system applications.
const (
	BundleIDDock               = "com.apple.dock"
	BundleIDNotificationCenter = "com.apple.notificationcenterui"
)

// Timeout constants.
const (
	ShellCommandTimeout = 30 * time.Second
)

// Default values.
const (
	DefaultHintCharacters = "asdfghjkl"
	DefaultExitKey        = "escape"
)

// Grid subgrid dimensions.
const (
	SubgridRows = 3
	SubgridCols = 3
)
