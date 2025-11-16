package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/y3owk1n/neru/internal/accessibility"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/ipc"
	"github.com/y3owk1n/neru/internal/overlay"
	"go.uber.org/zap"
)

// handleIPCCommand handles IPC commands from the CLI.
func (a *App) handleIPCCommand(cmd ipc.Command) ipc.Response {
	a.logger.Info(
		"Handling IPC command",
		zap.String("action", cmd.Action),
		zap.String("params", strings.Join(cmd.Args, ", ")),
	)

	switch cmd.Action {
	case "ping":
		return a.handlePing(cmd)
	case "start":
		return a.handleStart(cmd)
	case "stop":
		return a.handleStop(cmd)
	case modeHints:
		return a.handleHints(cmd)
	case modeGrid:
		return a.handleGrid(cmd)
	case "action":
		return a.handleAction(cmd)
	case "idle":
		return a.handleIdle(cmd)
	case "status":
		return a.handleStatus(cmd)
	default:
		return ipc.Response{Success: false, Message: "unknown command: " + cmd.Action}
	}
}

func (a *App) handlePing(_ ipc.Command) ipc.Response {
	return ipc.Response{Success: true, Message: "pong"}
}

func (a *App) handleStart(_ ipc.Command) ipc.Response {
	if a.enabled {
		return ipc.Response{Success: false, Message: "neru is already running"}
	}
	a.enabled = true
	return ipc.Response{Success: true, Message: "neru started"}
}

func (a *App) handleStop(_ ipc.Command) ipc.Response {
	if !a.enabled {
		return ipc.Response{Success: false, Message: "neru is already stopped"}
	}
	a.enabled = false
	a.exitMode()
	return ipc.Response{Success: true, Message: "neru stopped"}
}

func (a *App) handleHints(_ ipc.Command) ipc.Response {
	if !a.enabled {
		return ipc.Response{Success: false, Message: "neru is not running"}
	}
	if !a.config.Hints.Enabled {
		return ipc.Response{Success: false, Message: "hints mode is disabled by config"}
	}

	a.activateMode(ModeHints)

	return ipc.Response{Success: true, Message: "hint mode activated"}
}

func (a *App) handleGrid(_ ipc.Command) ipc.Response {
	if !a.enabled {
		return ipc.Response{Success: false, Message: "neru is not running"}
	}
	if !a.config.Grid.Enabled {
		return ipc.Response{Success: false, Message: "grid mode is disabled by config"}
	}

	a.activateMode(ModeGrid)

	return ipc.Response{Success: true, Message: "grid mode activated"}
}

func (a *App) handleAction(cmd ipc.Command) ipc.Response {
	if !a.enabled {
		return ipc.Response{Success: false, Message: "neru is not running"}
	}

	// Parse params
	params := cmd.Args
	if len(params) == 0 {
		return ipc.Response{Success: false, Message: "no action specified"}
	}

	// Get the current cursor position
	cursorPos := accessibility.GetCurrentCursorPosition()

	for _, param := range params {
		var err error
		switch param {
		case "left_click":
			err = accessibility.LeftClickAtPoint(cursorPos, false)
		case "right_click":
			err = accessibility.RightClickAtPoint(cursorPos, false)
		case "mouse_up":
			err = accessibility.LeftMouseUpAtPoint(cursorPos)
		case "mouse_down":
			err = accessibility.LeftMouseDownAtPoint(cursorPos)
		case "middle_click":
			err = accessibility.MiddleClickAtPoint(cursorPos, false)
		case "scroll":
			a.exitMode()

			// Enable event tap and let user scroll interactively at current position
			// Resize overlay to active screen for multi-monitor support
			if overlay.Get() != nil {
				overlay.Get().ResizeToActiveScreenSync()
			}

			// Draw highlight border if enabled
			if a.config.Scroll.HighlightScrollArea {
				a.drawScrollHighlightBorder()
				if overlay.Get() != nil {
					overlay.Get().Show()
				}
			}

			// Enable event tap for scroll key handling
			if a.eventTap != nil {
				a.eventTap.Enable()
			}

			a.logger.Info("Interactive scroll activated")
			a.logger.Info(
				"Use j/k to scroll, Ctrl+D/U for half-page, g/G for top/bottom, Esc to exit",
			)
			return ipc.Response{Success: true, Message: "scroll mode activated"}
		default:
			return ipc.Response{Success: false, Message: "unknown action: " + param}
		}

		if err != nil {
			return ipc.Response{Success: false, Message: "action failed: " + err.Error()}
		}
	}

	return ipc.Response{Success: true, Message: "action performed at cursor"}
}

func (a *App) handleIdle(_ ipc.Command) ipc.Response {
	if !a.enabled {
		return ipc.Response{Success: false, Message: "neru is not running"}
	}
	a.exitMode()
	return ipc.Response{Success: true, Message: "mode set to idle"}
}

func (a *App) handleStatus(_ ipc.Command) ipc.Response {
	cfgPath := a.resolveConfigPath()
	statusData := map[string]any{
		"enabled": a.enabled,
		"mode":    a.getCurrModeString(),
		"config":  cfgPath,
	}
	return ipc.Response{Success: true, Data: statusData}
}

// resolveConfigPath resolves the config path for status display.
func (a *App) resolveConfigPath() string {
	cfgPath := a.ConfigPath

	if cfgPath == "" {
		// Fallback to the standard config path if daemon wasn't started
		// with an explicit --config
		cfgPath = config.FindConfigFile()
	}

	// If config file doesn't exist, return default config
	var err error
	_, err = os.Stat(cfgPath)
	if os.IsNotExist(err) {
		return "No config file found, using default config without config file"
	}

	// Expand ~ to home dir and resolve relative paths to absolute
	if strings.HasPrefix(cfgPath, "~") {
		var home string
		var err error
		home, err = os.UserHomeDir()
		if err == nil {
			cfgPath = filepath.Join(home, cfgPath[1:])
		}
	}
	var abs string
	var err2 error
	abs, err2 = filepath.Abs(cfgPath)
	if err2 == nil {
		cfgPath = abs
	}

	return cfgPath
}
