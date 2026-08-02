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
	// ModeRecursiveGrid represents the recursive-grid navigation mode.
	ModeRecursiveGrid
	// ModeMonitorSelect represents the interactive monitor selection mode.
	ModeMonitorSelect
)

// IPC Commands.
//
// CommandMacro deliberately spells the same word as config.MacroCommand, which
// names the step keyword inside a binding. A macro is invoked the same way
// wherever it is written, so the two must not drift apart; they are separate
// constants only because the domain package cannot import config.
const (
	CommandPing                        = "ping"
	CommandStart                       = "start"
	CommandStop                        = "stop"
	CommandAction                      = "action"
	CommandRun                         = "run"
	CommandMacro                       = "macro"
	CommandStatus                      = "status"
	CommandConfig                      = "config"
	CommandReloadConfig                = "reload"
	CommandHealth                      = "health"
	CommandToggleScreenShare           = "toggle-screen-share"
	CommandToggleCursorFollowSelection = "toggle-cursor-follow-selection"
	CommandToggleScrollInvert          = "toggle-scroll-invert"
	CommandConfigSet                   = "config-set"
)

// Mode-related constants.
const (
	UnknownAction = "unknown"
	UnknownMode   = "unknown"
)

// Timeout constants.
const (
	ShellCommandTimeout = 30 * time.Second
)

// Default values.
const (
	DefaultHintCharacters = "asdfghjkl"
	DefaultExitKey        = "Escape"
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
