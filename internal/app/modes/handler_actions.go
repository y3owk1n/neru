package modes

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/geometry"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/ports"
)

// ResetCurrentMode resets current mode input state without exiting.
func (h *Handler) ResetCurrentMode() {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch h.appState.CurrentMode() {
	case domain.ModeGrid:
		if h.grid != nil && h.grid.Manager != nil {
			h.grid.Manager.Reset()

			// Clear stale selection — input was reset so no cell is selected.
			h.grid.Context.ClearSelectionPoint()

			gridInstancePtr := h.grid.Context.GridInstance()
			if gridInstancePtr != nil && *gridInstancePtr != nil {
				err := h.renderer.DrawGrid(
					*gridInstancePtr,
					h.grid.Manager.CurrentInput(),
				)
				if err != nil {
					h.logger.Error("Failed to redraw grid after reset", zap.Error(err))
				}

				h.refreshGridVirtualPointer()
			}
		}
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid != nil && h.recursiveGrid.Manager != nil {
			h.recursiveGrid.Manager.Reset()

			center := h.recursiveGrid.Manager.CurrentCenter()

			absoluteCenter := geometry.ConvertToAbsoluteCoordinates(center, h.screenBounds)
			if h.recursiveGrid.Context != nil {
				h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)
			}

			h.updateRecursiveGridOverlay()

			if h.recursiveGrid.Context != nil {
				if !h.recursiveGrid.Context.CursorFollowSelection() {
					h.refreshRecursiveGridVirtualPointer()

					return
				}
			}

			err := h.actionService.MoveCursorToPoint(
				h.ctx,
				absoluteCenter,
			)
			if err != nil {
				h.logger.Error("Failed to move cursor after recursive-grid reset", zap.Error(err))
			}
		}
	case domain.ModeMonitorSelect:
		if h.monitorSelect != nil {
			h.monitorSelect.input = ""
			h.monitorSelect.selectedIndex = 0
			h.redrawMonitorSelect()
		}
	case domain.ModeIdle, domain.ModeHints, domain.ModeScroll:
		// no-op
	}
}

// BackspaceCurrentMode performs mode-aware backspace behavior without exiting.
func (h *Handler) BackspaceCurrentMode() {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch h.appState.CurrentMode() {
	case domain.ModeHints:
		if h.hints != nil && h.hints.Context != nil && h.hints.Context.Manager() != nil {
			backspaceErr := h.hints.Context.Manager().HandleBackspace()
			if backspaceErr != nil {
				h.logger.Error("Hint backspace failed", zap.Error(backspaceErr))
			}
		}

		h.cycleHintIndex = -1
	case domain.ModeGrid:
		if h.grid != nil && h.grid.Manager != nil {
			h.grid.Manager.HandleBackspace()
		}
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid != nil && h.recursiveGrid.Manager != nil &&
			h.recursiveGrid.Manager.Backtrack() {
			center := h.recursiveGrid.Manager.CurrentCenter()

			absoluteCenter := geometry.ConvertToAbsoluteCoordinates(center, h.screenBounds)

			if h.recursiveGrid.Context != nil {
				h.recursiveGrid.Context.SetSelectionPoint(absoluteCenter)
			}

			h.updateRecursiveGridOverlay()

			if h.recursiveGrid.Context != nil {
				if !h.recursiveGrid.Context.CursorFollowSelection() {
					h.refreshRecursiveGridVirtualPointer()

					return
				}
			}

			err := h.actionService.MoveCursorToPoint(
				h.ctx,
				absoluteCenter,
			)
			if err != nil {
				h.logger.Error(
					"Failed to move cursor after recursive-grid backspace",
					zap.Error(err),
				)
			}
		}
	case domain.ModeMonitorSelect:
		if h.monitorSelect != nil {
			h.monitorSelect.Backspace()
			h.redrawMonitorSelect()
		}
	case domain.ModeIdle, domain.ModeScroll:
		// no-op
	}
}

// MoveCellCurrentMode slides the active mode's selection count cells in dir
// without changing the active layer.
//
// Grid mode moves an open subgrid to the neighboring cell; recursive-grid
// mode slides the highlighted region at the current depth, crossing into a
// neighboring parent when it runs off the edge of its own. Modes with no cell
// selection ignore it, as does a move that would leave the screen.
func (h *Handler) MoveCellCurrentMode(dir domain.Direction, count int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch h.appState.CurrentMode() {
	case domain.ModeGrid:
		if h.grid == nil || h.grid.Manager == nil {
			return
		}

		// The subgrid callback already sets the selection point, redraws the
		// subgrid over the new cell, and moves the cursor when cursor-follow
		// is on, so there is nothing left to do here.
		h.grid.Manager.MoveDirection(dir, count)
	case domain.ModeRecursiveGrid:
		if h.recursiveGrid == nil || h.recursiveGrid.Manager == nil {
			return
		}

		center, moved := h.recursiveGrid.Manager.MoveDirection(dir, count)
		if !moved {
			return
		}

		// The update callback set the selection point and redrew the overlay;
		// only cursor tracking is left.
		absoluteCenter := geometry.ConvertToAbsoluteCoordinates(center, h.screenBounds)

		if h.recursiveGrid.Context != nil &&
			!h.recursiveGrid.Context.CursorFollowSelection() {
			h.refreshRecursiveGridVirtualPointer()

			return
		}

		err := h.actionService.MoveCursorToPoint(h.ctx, absoluteCenter)
		if err != nil {
			h.logger.Error(
				"Failed to move cursor after recursive-grid cell move",
				zap.Error(err),
			)
		}
	case domain.ModeIdle, domain.ModeHints, domain.ModeScroll, domain.ModeMonitorSelect:
		// no-op
	}
}

// StartHintSearch activates text filtering for hints mode.
func (h *Handler) StartHintSearch() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.startHintSearch()
}

// CycleHint cycles through visible hints in hints mode, selecting the next or previous one.
// When executeAction is true, any pending action is performed on the selected hint
// (used by search confirmation). When false, only the cursor moves (used by the
// cycle_hint IPC action so users can browse results without triggering clicks).
func (h *Handler) CycleHint(ctx context.Context, backward bool, executeAction bool) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.appState.CurrentMode() != domain.ModeHints {
		return derrors.New(derrors.CodeInvalidInput, "cycle_hint requires hints mode")
	}

	if h.hints == nil || h.hints.Context == nil {
		return derrors.New(derrors.CodeActionFailed, "hints component not available")
	}

	manager := h.hints.Context.Manager()
	if manager == nil {
		return derrors.New(derrors.CodeActionFailed, "hints manager not available")
	}

	filteredHints := manager.FilteredHints()
	if len(filteredHints) == 0 {
		filteredHints = h.hints.Context.Hints().All()
	}

	if len(filteredHints) == 0 {
		return derrors.New(derrors.CodeActionFailed, "no hints available")
	}

	if h.cycleHintIndex >= len(filteredHints) {
		h.cycleHintIndex = len(filteredHints) - 1
	}

	switch {
	case h.cycleHintIndex < 0:
		h.cycleHintIndex = 0
		if backward {
			h.cycleHintIndex = len(filteredHints) - 1
		}
	default:
		if backward {
			if h.cycleHintIndex > 0 {
				h.cycleHintIndex--
			} else {
				h.cycleHintIndex = len(filteredHints) - 1
			}
		} else {
			if h.cycleHintIndex < len(filteredHints)-1 {
				h.cycleHintIndex++
			} else {
				h.cycleHintIndex = 0
			}
		}
	}

	selectedHint := filteredHints[h.cycleHintIndex]

	center := selectedHint.Element().Center()

	moveErr := h.actionService.MoveCursorToPoint(ctx, center)
	if moveErr != nil {
		h.logger.Error("Failed to move cursor during cycle_hint", zap.Error(moveErr))

		return derrors.New(derrors.CodeActionFailed, "failed to move cursor: "+moveErr.Error())
	}

	pendingAction := h.hints.Context.PendingAction()

	pendingModifier := h.hints.Context.PendingModifier()
	if pendingAction != nil && executeAction {
		repeat := h.hints.Context.Repeat()
		cursorFollowSelection := h.hints.Context.CursorFollowSelection()
		filterRoles := h.hints.Context.FilterRoles()
		filterTextContains := h.hints.Context.FilterTextContains()
		startWithSearch := h.hints.Context.StartWithSearch()
		strategyOverride := h.hints.Context.StrategyOverride()
		labelDirectionOverride := h.hints.Context.LabelDirectionOverride()
		splitWord := h.hints.Context.SplitWord()

		h.executeActionAtPoint(pendingAction, pendingModifier, center, repeat, func() {
			h.activateHintModeInternal(modecmd.Activation{
				Mode:               domain.ModeHints,
				FilterRoles:        filterRoles,
				FilterTextContains: filterTextContains,
				Search:             &startWithSearch,
				Strategy:           &strategyOverride,
				LabelDirection:     &labelDirectionOverride,
				SplitWord:          &splitWord,
				// OnExit is left nil to preserve the stored steps across
				// re-activation.
			})

			// Restore state so subsequent cycles continue to execute the action
			// Guard: only restore if repeat was originally set (mode is still hints).
			if repeat && h.appState.CurrentMode() == domain.ModeHints &&
				h.hints != nil && h.hints.Context != nil {
				h.hints.Context.SetPendingAction(pendingAction)
				h.hints.Context.SetPendingModifier(pendingModifier)
				h.hints.Context.SetRepeat(true)
				h.hints.Context.SetCursorFollowSelection(cursorFollowSelection)
				h.hints.Context.SetFilterRoles(filterRoles)
				h.hints.Context.SetFilterTextContains(filterTextContains)
				h.hints.Context.SetStartWithSearch(startWithSearch)
				h.hints.Context.SetStrategyOverride(strategyOverride)
				h.hints.Context.SetLabelDirectionOverride(labelDirectionOverride)
				h.hints.Context.SetSplitWord(splitWord)
			}
		})
	}

	return nil
}

func (h *handlerState) startHintSearch() error {
	if h.appState.CurrentMode() != domain.ModeHints {
		return derrors.New(derrors.CodeInvalidInput, "search_hints requires hints mode")
	}

	if h.hints == nil || h.hints.Context == nil {
		return derrors.New(derrors.CodeActionFailed, "hints component not available")
	}

	if h.hints.Context.SourceHints() == nil {
		return derrors.New(derrors.CodeActionFailed, "hints not available")
	}

	h.stopHintSearchTextInput(true)
	h.hints.Context.SetSearchQuery("")
	h.hints.Context.SetSearchActive(true)

	if h.hints.Context.HideOnEmptySearch() {
		// When hide-on-empty-search is active, hide all hints initially.
		// Hints will appear as the user types a query.
		setHintsErr := h.hints.Context.ClearVisibleHints()
		if setHintsErr != nil {
			return setHintsErr
		}
	} else {
		setHintsErr := h.hints.Context.SetVisibleHints(
			h.hints.Context.SourceHints(),
		)
		if setHintsErr != nil {
			return setHintsErr
		}
	}

	h.cycleHintIndex = -1
	h.drawHintSearchInput()

	if h.textInput != nil {
		searchFrame := h.searchInputFrame()
		position := searchFrame.Position()
		height := estimatedSearchInputHeight(h.config.Hints.SearchInputUI)
		textInputFrame := ports.TextInputFrame{
			X:      position.X,
			Y:      position.Y,
			Width:  searchFrame.Width(),
			Height: height,
		}

		started, _ := h.textInput.StartHintSearchSession(
			h.ctx,
			ports.TextInputCallbacks{
				OnQueryChanged: func(query string) {
					h.outer.mu.Lock()
					defer h.outer.mu.Unlock()

					if h.appState.CurrentMode() != domain.ModeHints || h.hints == nil ||
						h.hints.Context == nil {
						return
					}

					if !h.hints.Context.SearchActive() {
						return
					}

					h.hints.Context.SetSearchQuery(query)
					h.applyHintSearchFilter()
				},
				OnConfirm: func() {
					h.outer.mu.Lock()
					defer h.outer.mu.Unlock()

					if h.appState.CurrentMode() != domain.ModeHints {
						return
					}

					h.confirmHintSearch()
				},
				OnCancel: func() {
					h.outer.mu.Lock()
					defer h.outer.mu.Unlock()

					if h.appState.CurrentMode() != domain.ModeHints {
						return
					}

					h.cancelHintSearch()
				},
			},
			textInputFrame,
		)

		if started {
			h.hintSearchTextInputActive = true
			if h.hasEventTap() {
				h.disableEventTap()
				h.hintSearchEventTapDisabled = true
			}
		}
	}

	return nil
}

func (h *handlerState) stopHintSearchTextInput(keepEventTapDisabled bool) {
	if h.hintSearchTextInputActive && h.textInput != nil {
		// Use Background context since this may be called during cleanup,
		// after h.ctx has already been canceled.
		_ = h.textInput.StopHintSearchSession(context.Background())
	}

	h.hintSearchTextInputActive = false

	if h.hintSearchEventTapDisabled && h.hasEventTap() &&
		h.appState.CurrentMode() == domain.ModeHints && !keepEventTapDisabled {
		h.enableEventTap()
		h.hintSearchEventTapDisabled = false
	}
}
