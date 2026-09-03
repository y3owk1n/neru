package domain

import (
	"time"

	"go.uber.org/zap"
)

// Mode is the current mode of the application.
type Mode int

const (
	// ModeIdle is the idle mode.
	ModeIdle Mode = iota
	// ModeHints is the hints mode.
	ModeHints
	// ModeGrid is the grid mode.
	ModeGrid
	// ModeScroll is the scroll mode.
	ModeScroll
	// ModeRecursiveGrid is the recursive-grid navigation mode.
	ModeRecursiveGrid
	// ModeMonitorSelect is the interactive monitor selection mode.
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

	// CommandHintsProbe reports what hints mode would target for the focused
	// window. It is a read-only query rather than a mode command: it draws
	// nothing, activates nothing, and answers with a summary. `neru hints
	// --debug` is its CLI spelling.
	CommandHintsProbe = "hints-probe"
)

// Mode-related constants.
const (
	UnknownAction = "unknown"
	UnknownMode   = "unknown"
)

// The values a mode command's options accept.
//
// They live here rather than beside the configuration fields or the mode that
// reads them because a mode command reaches the daemon from three places — the
// CLI, a hotkey binding, and a direct IPC caller — and all three have to agree
// on the vocabulary. Config validates bindings, so the module that parses a
// mode command cannot import config; putting the values beneath both is what
// lets every reader name the same constant.

// Element detection strategies, the values hints.strategy accepts.
const (
	// StrategyAXTree walks the accessibility tree. This is the default.
	StrategyAXTree = "axtree"

	// StrategyVision detects elements from the screen image instead.
	StrategyVision = "vision"

	// CaptureScopeWindow scans the focused window; the whole screen when
	// nothing is focused.
	CaptureScopeWindow = "window"

	// CaptureScopeScreen scans the whole active screen, so notifications,
	// panels and adjacent tiled windows get hints too.
	CaptureScopeScreen = "screen"

	// StrategyContour detects interactive targets from the screen image by
	// edge and contour analysis, an algorithm ported from wl-kbptr.
	StrategyContour = "contour"
)

// Hint label enumeration orders, the values hints.label_direction accepts.
const (
	// LabelDirectionReverse spreads labels across the alphabet by varying the
	// first character so same-prefix labels never cluster together.
	LabelDirectionReverse = "reverse"

	// LabelDirectionNormal uses the original prefix-avoidance algorithm that
	// prefers shorter labels. This is the default.
	LabelDirectionNormal = "normal"
)

// How the real cursor behaves while a selection is being made, the values
// --cursor-selection-mode accepts.
const (
	// CursorSelectionModeFollow keeps the real cursor synced with the current
	// selection.
	CursorSelectionModeFollow = "follow"

	// CursorSelectionModeHold keeps the real cursor stationary until an
	// explicit commit or move.
	CursorSelectionModeHold = "hold"
)

// Timeout constants.
const (
	ShellCommandTimeout = 30 * time.Second
)

// Default values.
const (
	DefaultExitKey = "Escape"
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
