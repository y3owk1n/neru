package modes

import (
	"context"
	"image"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/components/hints"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/core/domain"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/ui/overlay"
)

// ModeActivationOptions configures a mode activation request.
type ModeActivationOptions struct {
	Action                *string
	Repeat                bool
	CursorFollowSelection *bool
	FilterRoles           []string
	FilterTextContains    []string
	Search                bool
}

const (
	// HintTimeout is the timeout for hint operations.
	HintTimeout = 5 * time.Second
)

// ActivateMode activates a mode with a given action (for hints mode).
func (h *Handler) ActivateMode(mode domain.Mode) {
	h.ActivateModeWithOptions(mode, ModeActivationOptions{})
}

// ActivateModeWithAction activates a mode with an optional action parameter.
func (h *Handler) ActivateModeWithAction(mode domain.Mode, action *string) {
	h.ActivateModeWithOptions(mode, ModeActivationOptions{Action: action})
}

// ActivateModeWithOptions activates a mode with an optional action and repeat flag.
// When repeat is true the mode re-activates after performing the pending action.
func (h *Handler) ActivateModeWithOptions(mode domain.Mode, opts ModeActivationOptions) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if mode == domain.ModeIdle {
		h.exitModeLocked()

		return
	}

	modeImpl, exists := h.modes[mode]
	if !exists {
		h.logger.Warn("Unknown mode", zap.String("mode", domain.ModeString(mode)))

		return
	}

	modeImpl.Activate(opts)
}

// filterHintsForScreen returns only the hints whose element center falls within
// screenBounds, and deduplicates by position so that downstream code (overlay
// incremental updates, Objective-C NeruDrawIncrementHints) can safely use
// position as a unique key without silently dropping entries.
func filterHintsForScreen(
	allHints []*domainHint.Interface,
	screenBounds image.Rectangle,
) []*domainHint.Interface {
	filtered := make([]*domainHint.Interface, 0, len(allHints))

	seenPositions := make(map[image.Point]struct{}, len(allHints))
	for _, hint := range allHints {
		hintBounds := hint.Element().Bounds()

		hintCenter := image.Point{
			X: hintBounds.Min.X + hintBounds.Dx()/2,
			Y: hintBounds.Min.Y + hintBounds.Dy()/2,
		}
		if !hintCenter.In(screenBounds) {
			continue
		}

		if _, exists := seenPositions[hintCenter]; exists {
			continue
		}

		seenPositions[hintCenter] = struct{}{}

		filtered = append(filtered, hint)
	}

	return filtered
}

// activateHintModeWithAction activates hint mode with optional action parameter.
func (h *Handler) activateHintModeWithAction(
	action *string,
	repeat bool,
	cursorFollowSelection *bool,
	filterRoles []string,
	filterTextContains []string,
	search bool,
) {
	h.activateHintModeInternal(
		action,
		cursorFollowSelection,
		filterRoles,
		filterTextContains,
		search,
	)

	// Store repeat flag after activation so the context is already initialized.
	if repeat && h.hints != nil && h.hints.Context != nil {
		h.hints.Context.SetRepeat(true)
	}
}

// activateHintModeInternal activates hint mode with optional action.
// It handles mode validation, overlay positioning, element collection, hint generation,
// and UI setup for hint-based navigation.

func (h *Handler) activateHintModeInternal(
	actionStr *string,
	cursorFollowSelection *bool,
	filterRoles []string,
	filterTextContains []string,
	search bool,
) {
	// Detect refresh before validation so we can clean up on failure
	isRefresh := h.appState.CurrentMode() == domain.ModeHints

	// Reset cycle index on refresh since the hint list is regenerated
	if isRefresh {
		h.cycleHintIndex = -1
	}

	// Defer bundle ID fetch until after validation (secure input check) to avoid
	// unnecessary AX calls when a password field is focused.
	actionEnum, activated := h.activateModeBase(
		domain.ModeNameHints,
		h.config.Hints.Enabled,
		action.TypeMoveMouse,
		"",
	)
	if !activated {
		// If validation fails during a refresh (e.g., secure input activated,
		// focused app became excluded), exit cleanly instead of leaving stale
		// hints on the overlay.
		if isRefresh {
			h.exitModeLocked()
		}

		return
	}

	actionString := domain.ActionString(actionEnum)

	if isRefresh {
		// During refresh, only clear overlay and stop polling but do NOT change mode
		// or disable event tap. Mode and event tap are already in the correct state,
		// so SetModeHints() can be skipped on the success path.
		// This prevents leaving the app in idle mode with event tap disabled if hint
		// generation fails.
		h.overlayManager.Clear()
		h.stopIndicatorPolling()
	} else {
		h.exitModeLocked()
	}

	if actionString == domain.UnknownAction {
		h.logger.Warn("Unknown action string, ignoring")

		if isRefresh {
			h.exitModeLocked()
		}

		return
	}

	// Always resize overlay to the active screen (where mouse is) before collecting elements.
	// This ensures proper positioning when switching between multiple displays.
	var activeScreenBounds image.Rectangle

	if h.system != nil {
		b, err := h.system.ScreenBounds(context.Background())
		if err == nil {
			activeScreenBounds = b
		} else if !derrors.IsNotSupported(err) {
			h.logger.Warn("Failed to get screen bounds for hints", zap.Error(err))
		}
	}

	h.screenBounds = activeScreenBounds
	// Clear any previous overlay content (e.g., scroll highlights) before drawing hints.
	// This prevents scroll highlights from persisting when switching from scroll mode to hints mode.
	h.overlayManager.Clear()
	h.appState.SetHintOverlayNeedsRefresh(false)

	if h.hints != nil && h.hints.Context != nil {
		h.hints.Context.SetPendingAction(actionStr)
		h.hints.Context.SetRepeat(false)
		h.hints.Context.SetCursorFollowSelection(resolveCursorFollowSelection(
			domain.ModeHints,
			cursorFollowSelection,
		))
		h.hints.Context.SetFilterRoles(filterRoles)
		h.hints.Context.SetFilterTextContains(filterTextContains)
		h.hints.Context.SetStartWithSearch(search)
	}

	// Fetch bundle ID for hint generation. Validation already passed (secure input check,
	// exclusion check), so this is the only call. Use a dedicated short timeout so slow
	// AX doesn't erode the hint generation budget.
	bundleCtx, bundleCancel := context.WithTimeout(context.Background(), 1*time.Second)
	bundleID, bundleIDErr := h.actionService.FocusedAppBundleID(bundleCtx)

	bundleCancel()

	if bundleIDErr != nil {
		h.logger.Debug("Failed to get focused app bundle ID for hint generation",
			zap.Error(bundleIDErr))
	}

	// Initialize hint manager with update callback (created once, reused across activations)
	if h.hints.Context.Manager() == nil {
		h.initHintManager()
	}

	// Set mode and show overlay before element collection for immediate visual feedback
	if !isRefresh {
		h.setModeLocked(domain.ModeHints, overlay.ModeHints)
	} else {
		h.syncModifierPassthrough(domain.ModeHints)
	}

	h.hints.Context.SetRouter(domainHint.NewRouter(h.hints.Context.Manager(), h.logger))
	h.overlayManager.ResizeToActiveScreen()
	h.overlayManager.Show()

	// Cancel any previous hint stream before starting a new one
	h.cancelHintStream()

	// Try streaming hint generation first; fall back to batch on error
	streamed := h.tryActivateHintsStreaming(
		filterRoles,
		filterTextContains,
		bundleID,
		activeScreenBounds,
		search,
	)

	if !streamed {
		// Fall back to synchronous batch generation
		h.activateHintsBatch(
			context.Background(),
			filterRoles,
			filterTextContains,
			bundleID,
			activeScreenBounds,
			actionString,
			isRefresh,
			search,
		)
	}

	if actionStr != nil {
		h.logger.Info("Hints mode activated with pending action", zap.String("action", *actionStr))
	}

	h.logger.Info("Hints mode activated")
	h.startIndicatorPolling(domain.ModeHints)
}

// initHintManager creates the hint manager and its overlay update callback.
// The manager is created once and reused across activations.
func (h *Handler) initHintManager() {
	manager := domainHint.NewManager(h.logger, &h.mu)
	manager.SetUpdateCallback(func(filteredHints []*domainHint.Interface) {
		if h.hints.Overlay == nil {
			return
		}

		screenBounds := h.screenBounds

		overlayHints := make([]*hints.Hint, len(filteredHints))
		for index, hint := range filteredHints {
			localPos := image.Point{
				X: hint.Position().X - screenBounds.Min.X,
				Y: hint.Position().Y - screenBounds.Min.Y,
			}
			overlayHints[index] = hints.NewHint(
				hint.Label(),
				localPos,
				hint.Element().Bounds().Size(),
				hint.MatchedPrefix(),
			)
		}

		drawHintsErr := h.overlayManager.DrawHintsWithStyle(overlayHints, h.hints.Style)
		if drawHintsErr != nil {
			h.logger.Error("Failed to update hints overlay", zap.Error(drawHintsErr))
		}
	})
	h.hints.Context.SetManager(manager)
}

// tryActivateHintsStreaming attempts to start streaming hint generation.
// Returns true if streaming was started successfully, false to signal
// the caller should fall back to the synchronous batch path.
func (h *Handler) tryActivateHintsStreaming(
	filterRoles []string,
	filterTextContains []string,
	bundleID string,
	screenBounds image.Rectangle,
	search bool,
) bool {
	ctx, cancel := context.WithTimeout(context.Background(), HintTimeout)
	h.streamCancel = cancel

	streamCh, doneCh, err := h.hintService.StreamHints(
		ctx,
		filterRoles,
		filterTextContains,
		bundleID,
		screenBounds,
		nil,
	)
	if err != nil {
		h.streamCancel = nil
		h.streamDone = nil

		cancel()

		h.logger.Warn("Streaming hints not available, falling back to batch",
			zap.Error(err))

		return false
	}

	h.streamDone = doneCh

	go h.processHintStream(streamCh, search)

	return true
}

// cancelHintStream cancels any in-flight hint streaming context and waits
// for all background streaming goroutines to fully exit before returning.
// Safe to call multiple times.
func (h *Handler) cancelHintStream() {
	if h.streamCancel != nil {
		h.streamCancel()
		h.streamCancel = nil
	}

	if h.streamDone != nil {
		done := h.streamDone
		h.streamDone = nil

		const cgoExitTimeout = 500 * time.Millisecond

		// Wait with a safety timeout to prevent hanging the UI indefinitely if a native AX call hangs
		select {
		case <-done:
			h.logger.Debug("Hint stream goroutines exited successfully")
		case <-time.After(cgoExitTimeout):
			h.logger.Warn("Timeout waiting for hint stream goroutines to exit; proceeding anyway")
		}
	}
}

// activateHintsBatch is the synchronous fallback path — collects all elements
// then renders hints in one shot.
func (h *Handler) activateHintsBatch(
	ctx context.Context,
	filterRoles []string,
	filterTextContains []string,
	bundleID string,
	screenBounds image.Rectangle,
	actionString string,
	isRefresh bool,
	search bool,
) {
	batchCtx, batchCancel := context.WithTimeout(ctx, HintTimeout)
	defer batchCancel()

	domainHints, err := h.hintService.GenerateHints(
		batchCtx,
		filterRoles,
		filterTextContains,
		bundleID,
	)
	if err != nil {
		h.logger.Error("Failed to show hints",
			zap.Error(err),
			zap.String("action", actionString))

		if isRefresh {
			h.exitModeLocked()
		}

		return
	}

	filteredHints := filterHintsForScreen(domainHints, screenBounds)

	h.logger.Debug("Filtered hints by screen (batch path)",
		zap.Int("total_hints", len(domainHints)),
		zap.Int("filtered_hints", len(filteredHints)),
		zap.String("screen_bounds", screenBounds.String()))

	if len(filteredHints) == 0 {
		h.logger.Warn("No hints generated for action", zap.String("action", actionString))

		if isRefresh {
			h.exitModeLocked()
		}

		return
	}

	hintCollection := domainHint.NewCollection(filteredHints)
	h.hints.Context.SetHints(hintCollection)

	if search {
		searchErr := h.startHintSearchLocked()
		if searchErr != nil {
			h.logger.Error("Failed to start hint search on activation", zap.Error(searchErr))
		}
	}
}

// processHintStream reads hint batches from the streaming channel and updates
// the overlay incrementally. It holds h.mu during each update. On the final
// batch (Done=true) it optionally activates hint search.
func (h *Handler) processHintStream(
	streamCh <-chan services.HintStreamBatch,
	search bool,
) {
	for batch := range streamCh {
		h.mu.Lock()

		if h.appState.CurrentMode() != domain.ModeHints {
			h.mu.Unlock()

			return
		}

		if batch.Err != nil {
			h.logger.Error("Hint stream error", zap.Error(batch.Err))
			h.mu.Unlock()

			continue
		}

		if len(batch.Hints) > 0 {
			filteredHints := filterHintsForScreen(batch.Hints, h.screenBounds)
			if len(filteredHints) > 0 {
				collection := domainHint.NewCollection(filteredHints)
				h.hints.Context.SetHints(collection)
			}
		}

		if batch.Done {
			h.logger.Debug("Hint stream completed",
				zap.Int("total_hints", len(batch.Hints)))

			if search && h.hints.Context.Hints() != nil &&
				!h.hints.Context.Hints().Empty() {
				searchErr := h.startHintSearchLocked()
				if searchErr != nil {
					h.logger.Error("Failed to start hint search",
						zap.Error(searchErr))
				}
			}

			h.mu.Unlock()

			return
		}

		h.mu.Unlock()
	}
}
