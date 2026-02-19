package domain

import (
	"time"

	"go.uber.org/zap"
)

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
	// ModeQuadGrid represents the quad-grid navigation mode.
	ModeQuadGrid
)

// IPC Commands.
const (
	CommandPing              = "ping"
	CommandStart             = "start"
	CommandStop              = "stop"
	CommandAction            = "action"
	CommandStatus            = "status"
	CommandConfig            = "config"
	CommandReloadConfig      = "reload"
	CommandHealth            = "health"
	CommandMetrics           = "metrics"
	CommandToggleScreenShare = "toggle-screen-share"
)

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

// BaseManager provides common functionality for domain managers.
// It contains shared fields and methods used across different domain managers.
type BaseManager struct {
	currentInput string
	Logger       *zap.Logger
}

// SetCurrentInput sets the current input string.
func (m *BaseManager) SetCurrentInput(input string) {
	m.currentInput = input
}

// CurrentInput returns the current input string.
func (m *BaseManager) CurrentInput() string {
	return m.currentInput
}

// Reset resets the base manager to its initial state.
func (m *BaseManager) Reset() {
	m.currentInput = ""
}
