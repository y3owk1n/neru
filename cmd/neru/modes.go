package main

import (
	"fmt"
	"image"
	"runtime"
	"strings"

	"github.com/y3owk1n/neru/internal/accessibility"
	"github.com/y3owk1n/neru/internal/bridge"
	"github.com/y3owk1n/neru/internal/grid"
	"github.com/y3owk1n/neru/internal/hints"
	"github.com/y3owk1n/neru/internal/overlay"
	"github.com/y3owk1n/neru/internal/scroll"
	"go.uber.org/zap"
)

const unknownAction = "unknown"

type HintsContext struct {
	selectedHint *hints.Hint
	inActionMode bool
}

type GridContext struct {
	gridInstance **grid.Grid
	gridOverlay  **grid.Overlay
	inActionMode bool
}

// activateMode activates a mode with a given action (for hints mode).
func (a *App) activateMode(mode Mode) {
	if mode == ModeIdle {
		// Explicit idle transition
		a.exitMode()
		return
	}
	if mode == ModeHints {
		a.activateHintMode()
		return
	}
	if mode == ModeGrid {
		a.activateGridMode()
		return
	}
	// Unknown or unsupported mode
	a.logger.Warn("Unknown mode", zap.String("mode", getModeString(mode)))
}

// activateHintMode activates hint mode, with an option to preserve action mode state.
func (a *App) activateHintMode() {
	a.activateHintModeInternal(false)
}

// activateHintModeInternal activates hint mode with option to preserve action mode state.
func (a *App) activateHintModeInternal(preserveActionMode bool) {
	if !a.enabled {
		a.logger.Debug("Neru is disabled, ignoring hint mode activation")
		return
	}
	// Respect mode enable flag
	if !a.config.Hints.Enabled {
		a.logger.Debug("Hints mode disabled by config, ignoring activation")
		return
	}

	// Centralized exclusion guard
	if a.isFocusedAppExcluded() {
		return
	}

	action := ActionMoveMouse
	actionString := getActionString(action)
	a.logger.Info("Activating hint mode", zap.String("action", actionString))

	// Only exit mode if not preserving action mode state
	if !preserveActionMode {
		a.exitMode()
	}

	if actionString == unknownAction {
		a.logger.Warn("Unknown action string, ignoring")
		return
	}

	// Always resize overlay to the active screen (where mouse is) before collecting elements
	// This ensures proper positioning when switching between multiple displays
	if overlay.Get() != nil {
		overlay.Get().ResizeToActiveScreenSync()
		a.hintOverlayNeedsRefresh = false
	}

	// Update roles for the current focused app
	a.updateRolesForCurrentApp()

	// Collect elements based on mode
	elements := a.collectElements()
	if len(elements) == 0 {
		a.logger.Warn("No elements found for action", zap.String("action", actionString))
		return
	}

	// Generate and setup hints
	err := a.setupHints(elements)
	if err != nil {
		a.logger.Error("Failed to setup hints", zap.Error(err), zap.String("action", actionString))
		return
	}

	// Update mode and enable event tap
	a.currentMode = ModeHints
	a.hintsCtx.selectedHint = nil

	// Enable event tap to capture keys (must be last to ensure proper initialization)
	if a.eventTap != nil {
		a.eventTap.Enable()
	}
	if overlay.Get() != nil {
		overlay.Get().SwitchTo(overlay.ModeHints)
	}
}

// setupHints generates hints and draws them with appropriate styling.
func (a *App) setupHints(elements []*accessibility.TreeNode) error {
	var msBefore runtime.MemStats
	runtime.ReadMemStats(&msBefore)
	// Get active screen bounds to calculate offset for normalization
	screenBounds := bridge.GetActiveScreenBounds()
	screenOffsetX := screenBounds.Min.X
	screenOffsetY := screenBounds.Min.Y

	// Generate hints
	hintList, err := a.hintGenerator.Generate(elements)
	if err != nil {
		return fmt.Errorf("failed to generate hints: %w", err)
	}

	// Normalize hint positions to window-local coordinates
	// The overlay window is positioned at the screen origin, but the view uses local coordinates
	for _, hint := range hintList {
		hint.Position.X -= screenOffsetX
		hint.Position.Y -= screenOffsetY
	}

	localBounds := image.Rect(0, 0, screenBounds.Dx(), screenBounds.Dy())
	filtered := make([]*hints.Hint, 0, len(hintList))
	for _, h := range hintList {
		if h.IsVisible(localBounds) {
			filtered = append(filtered, h)
		}
	}
	hintList = filtered

	// Set up hints in the hint manager
	hintCollection := hints.NewHintCollection(hintList)
	a.hintManager.SetHints(hintCollection)

	// Draw hints with mode-specific styling
	style := hints.BuildStyle(a.config.Hints)
	if overlay.Get() != nil {
		err := overlay.Get().DrawHintsWithStyle(hintList, style)
		if err != nil {
			return fmt.Errorf("failed to draw hints: %w", err)
		}
		overlay.Get().Show()
	} else {
		err := a.hintOverlay.DrawHintsWithStyle(hintList, style)
		if err != nil {
			return fmt.Errorf("failed to draw hints: %w", err)
		}
	}
	var msAfter runtime.MemStats
	runtime.ReadMemStats(&msAfter)
	a.logger.Info("Hints setup perf",
		zap.Int("hints", len(hintList)),
		zap.Uint64("alloc_bytes_delta", msAfter.Alloc-msBefore.Alloc),
		zap.Uint64("sys_bytes_delta", msAfter.Sys-msBefore.Sys))
	return nil
}

func (a *App) activateGridMode() {
	if !a.enabled {
		a.logger.Debug("Neru is disabled, ignoring grid mode activation")
		return
	}
	// Respect mode enable flag
	if !a.config.Grid.Enabled {
		a.logger.Debug("Grid mode disabled by config, ignoring activation")
		return
	}

	// Centralized exclusion guard
	if a.isFocusedAppExcluded() {
		return
	}

	action := ActionMoveMouse
	actionString := getActionString(action)
	a.logger.Info("Activating grid mode", zap.String("action", actionString))

	a.exitMode() // Exit current mode first

	// Always resize overlay to the active screen (where mouse is) before drawing grid
	// This ensures proper positioning when switching between multiple displays
	if overlay.Get() != nil {
		overlay.Get().ResizeToActiveScreenSync()
		a.gridOverlayNeedsRefresh = false
	}

	err := a.setupGrid()
	if err != nil {
		a.logger.Error("Failed to setup grid", zap.Error(err), zap.String("action", actionString))
		return
	}

	// Update mode and enable event tap
	a.currentMode = ModeGrid

	// Enable event tap to capture keys
	if a.eventTap != nil {
		a.eventTap.Enable()
	}
	if overlay.Get() != nil {
		overlay.Get().SwitchTo(overlay.ModeGrid)
	}

	a.logger.Info("Grid mode activated", zap.String("action", actionString))
	a.logger.Info("Type a grid label to select a location")
}

// setupGrid generates grid and draws it.
func (a *App) setupGrid() error {
	// Create grid with active screen bounds (screen containing mouse cursor)
	// This ensures proper multi-monitor support
	screenBounds := bridge.GetActiveScreenBounds()

	// Normalize bounds to window-local coordinates (0,0 origin)
	// The overlay window is positioned at the screen origin, but the view uses local coordinates
	bounds := image.Rect(0, 0, screenBounds.Dx(), screenBounds.Dy())

	characters := a.config.Grid.Characters
	if strings.TrimSpace(characters) == "" {
		characters = a.config.Hints.HintCharacters
	}
	gridInstance := grid.NewGrid(characters, bounds, a.logger)
	*a.gridCtx.gridInstance = gridInstance

	// Grid overlay already created in NewApp - update its config and use it
	(*a.gridCtx.gridOverlay).UpdateConfig(a.config.Grid)

	// Ensure the overlay is properly sized for the active screen
	if overlay.Get() != nil {
		overlay.Get().ResizeToActiveScreenSync()
	}

	// Reset the grid manager state when setting up the grid
	if a.gridManager != nil {
		a.gridManager.Reset()
	}

	// Get style for current action
	gridStyle := grid.BuildStyle(a.config.Grid)

	// Subgrid configuration and keys (fallback to grid characters): always 3x3
	keys := strings.TrimSpace(a.config.Grid.SublayerKeys)
	if keys == "" {
		keys = a.config.Grid.Characters
	}
	const subRows = 3
	const subCols = 3

	// Initialize manager with the new grid
	a.gridManager = grid.NewManager(gridInstance, subRows, subCols, keys, func(forceRedraw bool) {
		// Update matches only (no full redraw)
		input := a.gridManager.GetInput()

		// special case to handle only when exiting subgrid
		if forceRedraw {
			if overlay.Get() != nil {
				overlay.Get().Clear()
				err := overlay.Get().DrawGrid(gridInstance, input, gridStyle)
				if err != nil {
					return
				}
				overlay.Get().Show()
			} else {
				(*a.gridCtx.gridOverlay).Clear()
				err := (*a.gridCtx.gridOverlay).Draw(gridInstance, input, gridStyle)
				if err != nil {
					return
				}
			}
		}

		// Set hideUnmatched based on whether we have input and the config setting
		hideUnmatched := a.config.Grid.HideUnmatched && len(input) > 0
		if overlay.Get() != nil {
			overlay.Get().SetHideUnmatched(hideUnmatched)
			overlay.Get().UpdateGridMatches(input)
		} else {
			(*a.gridCtx.gridOverlay).SetHideUnmatched(hideUnmatched)
			(*a.gridCtx.gridOverlay).UpdateMatches(input)
		}
	}, func(cell *grid.Cell) {
		// Draw 3x3 subgrid inside selected cell
		if overlay.Get() != nil {
			overlay.Get().ShowSubgrid(cell, gridStyle)
		} else {
			(*a.gridCtx.gridOverlay).ShowSubgrid(cell, gridStyle)
		}
	}, a.logger)
	a.gridRouter = grid.NewRouter(a.gridManager, a.logger)

	// Draw initial grid
	if overlay.Get() != nil {
		err := overlay.Get().DrawGrid(gridInstance, "", gridStyle)
		if err != nil {
			return fmt.Errorf("failed to draw grid: %w", err)
		}
		overlay.Get().Show()
	} else {
		err := (*a.gridCtx.gridOverlay).Draw(gridInstance, "", gridStyle)
		if err != nil {
			return fmt.Errorf("failed to draw grid: %w", err)
		}
	}

	return nil
}

// handleActiveKey dispatches key events by current mode.
func (a *App) handleKeyPress(key string) {
	// If in idle mode, check if we should handle scroll keys
	if a.currentMode == ModeIdle {
		// Handle escape to exit standalone scroll
		if key == "\x1b" || key == "escape" {
			a.logger.Info("Exiting standalone scroll mode")
			if overlay.Get() != nil {
				overlay.Get().Clear()
				overlay.Get().Hide()
			}
			if a.eventTap != nil {
				a.eventTap.Disable()
			}
			a.idleScrollLastKey = "" // Reset scroll state
			return
		}
		// Try to handle scroll keys with generic handler using persistent state
		// If it's not a scroll key, it will just be ignored
		a.handleGenericScrollKey(key, &a.idleScrollLastKey)
		return
	}

	// Handle Tab key to toggle between overlay mode and action mode
	if key == "\t" { // Tab key
		a.handleTabKey()
		return
	}

	// Handle Escape key to exit action mode or current mode
	if key == "\x1b" || key == "escape" {
		a.handleEscapeKey()
		return
	}

	// Explicitly dispatch by current mode
	a.handleModeSpecificKey(key)
}

// handleTabKey handles the tab key to toggle between overlay mode and action mode.
func (a *App) handleTabKey() {
	switch a.currentMode {
	case ModeHints:
		if a.hintsCtx.inActionMode {
			// Switch back to overlay mode
			a.hintsCtx.inActionMode = false
			if overlay.Get() != nil {
				overlay.Get().Clear()
				overlay.Get().Hide()
			}
			// Re-activate hint mode while preserving action mode state
			a.activateHintModeInternal(true)
			a.logger.Info("Switched back to hints overlay mode")
			if overlay.Get() != nil {
				overlay.Get().SwitchTo(overlay.ModeHints)
			}
		} else {
			// Switch to action mode
			a.hintsCtx.inActionMode = true
			if overlay.Get() != nil {
				overlay.Get().Clear()
				overlay.Get().Hide()
			}
			a.drawHintsActionHighlight()
			overlay.Get().Show()
			a.logger.Info("Switched to hints action mode")
			if overlay.Get() != nil {
				overlay.Get().SwitchTo(overlay.ModeAction)
			}
		}
	case ModeGrid:
		if a.gridCtx.inActionMode {
			// Switch back to overlay mode
			a.gridCtx.inActionMode = false
			if overlay.Get() != nil {
				overlay.Get().Clear()
				overlay.Get().Hide()
			}
			// Re-setup grid to show grid again with proper refresh
			err := a.setupGrid()
			if err != nil {
				a.logger.Error("Failed to re-setup grid", zap.Error(err))
			}
			a.logger.Info("Switched back to grid overlay mode")
			if overlay.Get() != nil {
				overlay.Get().SwitchTo(overlay.ModeGrid)
			}
		} else {
			// Switch to action mode
			a.gridCtx.inActionMode = true
			if a.gridCtx.gridOverlay != nil {
				if overlay.Get() != nil {
					overlay.Get().Clear()
					overlay.Get().Hide()
				}
			}
			a.drawGridActionHighlight()
			overlay.Get().Show()
			a.logger.Info("Switched to grid action mode")
			if overlay.Get() != nil {
				overlay.Get().SwitchTo(overlay.ModeAction)
			}
		}
	case ModeIdle:
		// Nothing to do in idle mode
		return
	}
}

// handleEscapeKey handles the escape key to exit action mode or current mode.
func (a *App) handleEscapeKey() {
	switch a.currentMode {
	case ModeHints:
		if a.hintsCtx.inActionMode {
			// Exit action mode completely, go back to idle mode
			a.hintsCtx.inActionMode = false
			if overlay.Get() != nil {
				overlay.Get().Clear()
				overlay.Get().Hide()
			}
			a.exitMode()
			a.logger.Info("Exited hints action mode completely")
			if overlay.Get() != nil {
				overlay.Get().SwitchTo(overlay.ModeIdle)
			}
			return
		}
		// Fall through to exit mode
	case ModeGrid:
		if a.gridCtx.inActionMode {
			// Exit action mode completely, go back to idle mode
			a.gridCtx.inActionMode = false
			if overlay.Get() != nil {
				overlay.Get().Clear()
				overlay.Get().Hide()
			}
			a.exitMode()
			a.logger.Info("Exited grid action mode completely")
			if overlay.Get() != nil {
				overlay.Get().SwitchTo(overlay.ModeIdle)
			}
			return
		}
		// Fall through to exit mode
	case ModeIdle:
		// Nothing to do in idle mode
		return
	}
	a.exitMode()
	if overlay.Get() != nil {
		overlay.Get().SwitchTo(overlay.ModeIdle)
	}
}

// handleModeSpecificKey handles mode-specific key processing.
func (a *App) handleModeSpecificKey(key string) {
	switch a.currentMode {
	case ModeHints:
		// If in action mode, handle action keys
		if a.hintsCtx.inActionMode {
			a.handleHintsActionKey(key)
			// After handling the action, we stay in action mode
			// The user can press Tab to go back to overlay mode or perform more actions
			return
		}

		// Route hint-specific keys via hints router
		res := a.hintsRouter.RouteKey(key, a.hintsCtx.selectedHint != nil)
		if res.Exit {
			a.exitMode()
			return
		}

		// Hint input processed by router; if exact match, perform action
		if res.ExactHint != nil {
			hint := res.ExactHint
			info, err := hint.Element.Element.GetInfo()
			if err != nil {
				a.logger.Error("Failed to get element info", zap.Error(err))
				a.exitMode()
				return
			}
			center := image.Point{
				X: info.Position.X + info.Size.X/2,
				Y: info.Position.Y + info.Size.Y/2,
			}

			a.logger.Info("Found element", zap.String("label", a.hintManager.GetInput()))
			accessibility.MoveMouseToPoint(center)

			if a.hintManager != nil {
				a.hintManager.Reset()
			}
			a.hintsCtx.selectedHint = nil

			a.activateHintMode()

			return
		}
	case ModeGrid:
		// If in action mode, handle action keys
		if a.gridCtx.inActionMode {
			a.handleGridActionKey(key)
			// After handling the action, we stay in action mode
			// The user can press Tab to go back to overlay mode or perform more actions
			return
		}

		res := a.gridRouter.RouteKey(key)
		if res.Exit {
			a.exitMode()
			return
		}

		// Complete coordinate entered - perform action
		if res.Complete {
			targetPoint := res.TargetPoint

			// Convert from window-local coordinates to absolute screen coordinates
			// The grid was generated with normalized bounds (0,0 origin) but clicks need absolute coords
			screenBounds := bridge.GetActiveScreenBounds()
			absolutePoint := image.Point{
				X: targetPoint.X + screenBounds.Min.X,
				Y: targetPoint.Y + screenBounds.Min.Y,
			}

			a.logger.Info(
				"Grid move mouse",
				zap.Int("x", absolutePoint.X),
				zap.Int("y", absolutePoint.Y),
			)
			accessibility.MoveMouseToPoint(absolutePoint)

			// No need to exit grid mode, just let it going
			// a.exitMode()

			return
		}
	case ModeIdle:
		// Nothing to do in idle mode
		return
	}
}

// handleGenericScrollKey handles scroll keys in a generic way.
func (a *App) handleGenericScrollKey(key string, lastScrollKey *string) {
	// Local storage for scroll state if not provided
	var localLastKey string
	if lastScrollKey == nil {
		lastScrollKey = &localLastKey
	}

	// Log every byte for debugging
	bytes := []byte(key)
	a.logger.Info("Scroll key pressed",
		zap.String("key", key),
		zap.Int("len", len(key)),
		zap.String("hex", fmt.Sprintf("%#v", key)),
		zap.Any("bytes", bytes))

	var err error

	// Check for control characters
	if len(key) == 1 {
		byteVal := key[0]
		a.logger.Info("Checking control char", zap.Uint8("byte", byteVal))
		// Only handle Ctrl+D / Ctrl+U here; let Tab (9) and other keys fall through to switch
		if byteVal == 4 || byteVal == 21 {
			op, _, ok := scroll.ParseKey(key, *lastScrollKey, a.logger)
			if ok {
				*lastScrollKey = ""
				switch op {
				case "half_down":
					a.logger.Info("Ctrl+D detected - half page down")
					err = a.scrollController.ScrollDownHalfPage()
					goto done
				case "half_up":
					a.logger.Info("Ctrl+U detected - half page up")
					err = a.scrollController.ScrollUpHalfPage()
					goto done
				}
			}
		}
	}

	// Regular keys
	a.logger.Debug(
		"Entering switch statement",
		zap.String("key", key),
		zap.String("keyHex", fmt.Sprintf("%#v", key)),
	)
	switch key {
	case "j":
		op, _, ok := scroll.ParseKey(key, *lastScrollKey, a.logger)
		if !ok {
			return
		}
		if op == "down" {
			err = a.scrollController.ScrollDown()
		}
	case "k":
		op, _, ok := scroll.ParseKey(key, *lastScrollKey, a.logger)
		if !ok {
			return
		}
		if op == "up" {
			err = a.scrollController.ScrollUp()
		}
	case "h":
		op, _, ok := scroll.ParseKey(key, *lastScrollKey, a.logger)
		if !ok {
			return
		}
		if op == "left" {
			err = a.scrollController.ScrollLeft()
		}
	case "l":
		op, _, ok := scroll.ParseKey(key, *lastScrollKey, a.logger)
		if !ok {
			return
		}
		if op == "right" {
			err = a.scrollController.ScrollRight()
		}
	case "g": // gg for top (need to press twice)
		operation, newLast, ok := scroll.ParseKey(key, *lastScrollKey, a.logger)
		if !ok {
			a.logger.Info("First g pressed, press again for top")
			*lastScrollKey = newLast
			return
		}
		if operation == "top" {
			a.logger.Info("gg detected - scroll to top")
			err = a.scrollController.ScrollToTop()
			*lastScrollKey = ""
			goto done
		}
	case "G": // Shift+G for bottom
		operation, _, ok := scroll.ParseKey(key, *lastScrollKey, a.logger)
		if ok && operation == "bottom" {
			a.logger.Info("G key detected - scroll to bottom")
			err = a.scrollController.ScrollToBottom()
			*lastScrollKey = ""
		}
	default:
		a.logger.Debug("Ignoring non-scroll key", zap.String("key", key))
		*lastScrollKey = ""
		return
	}

	// Reset last key for most commands
	*lastScrollKey = ""

done:
	if err != nil {
		a.logger.Error("Scroll failed", zap.Error(err))
	}
}

// drawHintsActionHighlight draws a highlight border around the active screen for hints action mode.
func (a *App) drawHintsActionHighlight() {
	// Resize overlay to active screen (where mouse cursor is) for multi-monitor support
	if overlay.Get() != nil {
		overlay.Get().ResizeToActiveScreenSync()
	}

	// Get active screen bounds
	screenBounds := bridge.GetActiveScreenBounds()
	localBounds := image.Rect(0, 0, screenBounds.Dx(), screenBounds.Dy())

	// Draw action highlight using action overlay
	a.actionOverlay.DrawActionHighlight(
		localBounds.Min.X,
		localBounds.Min.Y,
		localBounds.Dx(),
		localBounds.Dy(),
	)

	a.logger.Debug("Drawing hints action highlight",
		zap.Int("x", localBounds.Min.X),
		zap.Int("y", localBounds.Min.Y),
		zap.Int("width", localBounds.Dx()),
		zap.Int("height", localBounds.Dy()))
}

// drawGridActionHighlight draws a highlight border around the active screen for grid action mode.
func (a *App) drawGridActionHighlight() {
	// Resize overlay to active screen (where mouse cursor is) for multi-monitor support
	if overlay.Get() != nil {
		overlay.Get().ResizeToActiveScreenSync()
	}

	// Get active screen bounds
	screenBounds := bridge.GetActiveScreenBounds()
	localBounds := image.Rect(0, 0, screenBounds.Dx(), screenBounds.Dy())

	// Draw action highlight using action overlay
	a.actionOverlay.DrawActionHighlight(
		localBounds.Min.X,
		localBounds.Min.Y,
		localBounds.Dx(),
		localBounds.Dy(),
	)

	a.logger.Debug("Drawing grid action highlight",
		zap.Int("x", localBounds.Min.X),
		zap.Int("y", localBounds.Min.Y),
		zap.Int("width", localBounds.Dx()),
		zap.Int("height", localBounds.Dy()))
}

func (a *App) drawScrollHighlightBorder() {
	// Resize overlay to active screen (where mouse cursor is) for multi-monitor support
	if overlay.Get() != nil {
		overlay.Get().ResizeToActiveScreenSync()
	}

	// Get active screen bounds
	screenBounds := bridge.GetActiveScreenBounds()
	localBounds := image.Rect(0, 0, screenBounds.Dx(), screenBounds.Dy())

	// Draw scroll highlight using scroll overlay
	a.scrollOverlay.DrawScrollHighlight(
		localBounds.Min.X,
		localBounds.Min.Y,
		localBounds.Dx(),
		localBounds.Dy(),
	)
	if overlay.Get() != nil {
		overlay.Get().SwitchTo(overlay.ModeScroll)
	}
}

// exitMode exits the current mode.
func (a *App) exitMode() {
	if a.currentMode == ModeIdle {
		return
	}

	a.logger.Info("Exiting current mode", zap.String("mode", a.getCurrModeString()))

	// Mode-specific cleanup
	switch a.currentMode {
	case ModeHints:
		// Reset action mode state
		a.hintsCtx.inActionMode = false

		if a.hintManager != nil {
			a.hintManager.Reset()
		}
		a.hintsCtx.selectedHint = nil

		// Clear and hide overlay for hints
		if overlay.Get() != nil {
			overlay.Get().Clear()
			overlay.Get().Hide()
		}

		// Also clear and hide action overlay
		if overlay.Get() != nil {
			overlay.Get().Clear()
			overlay.Get().Hide()
		}

		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		a.logger.Info("Hints cleanup mem",
			zap.Uint64("alloc_bytes", ms.Alloc),
			zap.Uint64("sys_bytes", ms.Sys))
	case ModeGrid:
		// Reset action mode state
		a.gridCtx.inActionMode = false

		if a.gridManager != nil {
			a.gridManager.Reset()
		}
		// Hide overlays
		if a.gridCtx != nil && a.gridCtx.gridOverlay != nil && *a.gridCtx.gridOverlay != nil {
			a.logger.Info("Hiding grid overlay")
			if overlay.Get() != nil {
				overlay.Get().Hide()
			}
		}

		// Also clear and hide action overlay
		if overlay.Get() != nil {
			overlay.Get().Clear()
			overlay.Get().Hide()
		}
	case ModeIdle:
		// Already in idle mode, nothing to do
		return
	default:
		// No domain-specific cleanup for other modes yet
		// But still clear and hide action overlay
		if overlay.Get() != nil {
			overlay.Get().Clear()
			overlay.Get().Hide()
		}
	}

	// Clear scroll overlay
	if overlay.Get() != nil {
		overlay.Get().Clear()
	}

	// Disable event tap when leaving active modes
	if a.eventTap != nil {
		a.eventTap.Disable()
	}

	// Update mode after all cleanup is done
	a.currentMode = ModeIdle
	a.logger.Debug("Mode transition complete",
		zap.String("to", "idle"))
	if overlay.Get() != nil {
		overlay.Get().SwitchTo(overlay.ModeIdle)
	}

	// If a hotkey refresh was deferred while in an active mode, perform it now
	if a.hotkeyRefreshPending {
		a.hotkeyRefreshPending = false
		go a.refreshHotkeysForAppOrCurrent("")
	}
}

func getModeString(mode Mode) string {
	switch mode {
	case ModeIdle:
		return "idle"
	case ModeHints:
		return "hints"
	case ModeGrid:
		return "grid"
	default:
		return "unknown"
	}
}

func getActionString(action Action) string {
	switch action {
	case ActionLeftClick:
		return "left_click"
	case ActionRightClick:
		return "right_click"
	case ActionMouseUp:
		return "mouse_up"
	case ActionMouseDown:
		return "mouse_down"
	case ActionMiddleClick:
		return "middle_click"
	case ActionMoveMouse:
		return "move_mouse"
	case ActionScroll:
		return "scroll"
	default:
		return "unknown"
	}
}

// getCurrModeString returns the current mode as a string.
func (a *App) getCurrModeString() string {
	return getModeString(a.currentMode)
}

// handleActionKey handles action keys for both hints and grid modes.
func (a *App) handleActionKey(key string, mode string) {
	// Get the current cursor position
	cursorPos := accessibility.GetCurrentCursorPosition()

	// Map action keys to actions using configurable keys
	switch key {
	case a.config.Action.LeftClickKey: // Left click
		a.logger.Info(mode + " action: Left click")
		err := accessibility.LeftClickAtPoint(cursorPos, false)
		if err != nil {
			a.logger.Error("Failed to perform left click", zap.Error(err))
		}
	case a.config.Action.RightClickKey: // Right click
		a.logger.Info(mode + " action: Right click")
		err := accessibility.RightClickAtPoint(cursorPos, false)
		if err != nil {
			a.logger.Error("Failed to perform right click", zap.Error(err))
		}
	case a.config.Action.MiddleClickKey: // Middle click
		a.logger.Info(mode + " action: Middle click")
		err := accessibility.MiddleClickAtPoint(cursorPos, false)
		if err != nil {
			a.logger.Error("Failed to perform middle click", zap.Error(err))
		}
	case a.config.Action.MouseDownKey: // Mouse down
		a.logger.Info(mode + " action: Mouse down")
		err := accessibility.LeftMouseDownAtPoint(cursorPos)
		if err != nil {
			a.logger.Error("Failed to perform mouse down", zap.Error(err))
		}
	case a.config.Action.MouseUpKey: // Mouse up
		a.logger.Info(mode + " action: Mouse up")
		err := accessibility.LeftMouseUpAtPoint(cursorPos)
		if err != nil {
			a.logger.Error("Failed to perform mouse up", zap.Error(err))
		}
	default:
		a.logger.Debug("Unknown "+mode+" action key", zap.String("key", key))
	}
}

// handleHintsActionKey handles action keys when in hints action mode.
func (a *App) handleHintsActionKey(key string) {
	a.handleActionKey(key, "Hints")
}

// handleGridActionKey handles action keys when in grid action mode.
func (a *App) handleGridActionKey(key string) {
	a.handleActionKey(key, "Grid")
}
