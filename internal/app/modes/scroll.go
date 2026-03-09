package modes

import (
	"context"

	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/core/domain"
	"go.uber.org/zap"
)

// scrollPassthroughKeys are the keys passed through to the OS when scroll mode
// is active with stay_active_in_background = true.
var scrollPassthroughKeys = []string{"Cmd+Tab", "Cmd+Shift+Tab"}

// StartInteractiveScroll activates the interactive scroll mode,
// showing the scroll overlay and enabling key handling for scrolling.
// If scroll mode is currently suspended (user switched apps while it was
// active and stay_active_in_background = true), this resumes the suspended
// session instead of starting a fresh one.
func (h *Handler) StartInteractiveScroll() {
	// If we are resuming a suspended scroll session, branch early.
	if h.appState.IsScrollSuspended() {
		h.resumeSuspendedScroll()
		return
	}

	h.cursorState.SkipNextRestore()

	// Defensively reset scroll context before exiting the current mode.
	// exitModeLocked returns early when already idle without running cleanup,
	// so this ensures no stale lastKey/isActive state leaks into the new
	// scroll activation.
	h.scroll.Context.Reset()

	h.exitModeLocked()

	if h.enableEventTap != nil {
		h.enableEventTap()
	}

	h.scroll.Context.SetIsActive(true)

	h.overlayManager.ResizeToActiveScreen()

	h.SetModeScroll()

	// Register Cmd+Tab / Cmd+Shift+Tab as passthrough keys so the user can
	// still switch applications while scroll mode is active (only when the
	// feature is enabled in config).
	if h.config != nil && h.config.Scroll.StayActiveInBackground {
		if h.setPassthroughKeys != nil {
			h.setPassthroughKeys(scrollPassthroughKeys)
		}
	}

	h.logger.Info("Interactive scroll activated")
	h.logger.Info("Use configured keys for navigation")
}

// resumeSuspendedScroll re-activates a scroll session that was suspended when
// the user switched away from the application. It re-enables the event tap and
// restarts the mode-indicator polling without re-running the full mode setup.
func (h *Handler) resumeSuspendedScroll() {
	h.appState.SetScrollSuspended(false)

	if h.enableEventTap != nil {
		h.enableEventTap()
	}

	// The mode is still ModeScroll — just restart the overlay + indicator.
	h.overlayManager.ResizeToActiveScreen()
	h.startModeIndicatorPolling(domain.ModeScroll)

	h.logger.Info("Scroll mode resumed after app switch")
}

func (h *Handler) handleGenericScrollKey(key string) {
	lastKey, lastKeyTime := h.scroll.Context.LastKeyState()

	h.logger.Debug("handleGenericScrollKey",
		zap.String("key", key),
		zap.String("lastKey", lastKey))

	var (
		action string
		found  bool
	)

	if lastKey != "" {
		action, found = h.handleSequenceKey(key, lastKey, lastKeyTime)
	} else {
		action, found = h.handleSingleKey(key)
	}

	h.logger.Debug("key lookup result",
		zap.String("key", key),
		zap.String("action", action),
		zap.Bool("found", found))

	if !found {
		if currentLastKey := h.scroll.Context.LastKey(); currentLastKey != "" {
			h.logger.Debug("key lookup failed but sequence in progress, not clearing lastKey",
				zap.String("key", key),
				zap.String("lastKey", currentLastKey))
		}

		return
	}

	h.scroll.Context.SetLastKey("")

	scrollAction, actionFound := h.scroll.KeyMap.Action(action)
	h.logger.Debug("scroll action lookup",
		zap.String("action", action),
		zap.Bool("found", actionFound),
		zap.Int("direction", int(scrollAction.Direction)),
		zap.Int("amount", int(scrollAction.Amount)))

	if !actionFound {
		h.logger.Warn("Action not found in scroll action map", zap.String("action", action))

		return
	}

	ctx := context.Background()

	scrollErr := h.scrollService.Scroll(ctx, scrollAction.Direction, scrollAction.Amount)
	if scrollErr != nil {
		h.logger.Error("Scroll failed", zap.Error(scrollErr))
	}
}

func (h *Handler) handleSingleKey(key string) (string, bool) {
	h.logger.Debug("handleSingleKey", zap.String("key", key))

	action, found := h.scroll.KeyMap.Lookup(key)
	if found {
		return action, true
	}

	if h.scroll.KeyMap.IsSequenceStart(key) {
		h.logger.Debug("sequence start detected, storing key", zap.String("key", key))
		h.scroll.Context.SetLastKey(key)

		return "", false
	}

	return "", false
}

func (h *Handler) handleSequenceKey(key, firstKey string, firstKeyTime int64) (string, bool) {
	h.logger.Debug("handleSequenceKey",
		zap.String("key", key),
		zap.String("firstKey", firstKey))

	seqState := scroll.NewSequenceState(firstKey, firstKeyTime)
	if seqState.Expired() {
		h.logger.Debug("sequence expired")
		h.scroll.Context.SetLastKey("")

		return h.handleSingleKey(key)
	}

	h.logger.Debug("sequence not expired, checking completion")

	if h.scroll.KeyMap.CanCompleteSequence(firstKey, key) {
		h.logger.Debug(
			"sequence can complete",
			zap.String("firstKey", firstKey),
			zap.String("key", key),
		)
		seq := firstKey + key

		action, seqFound := h.scroll.KeyMap.LookupSequence(seq)
		if seqFound {
			h.logger.Debug("sequence completed!", zap.String("action", action))

			return action, true
		}
	}

	h.logger.Debug("sequence not completed, checking lookup")

	action, keyFound := h.scroll.KeyMap.Lookup(key)
	if keyFound {
		return action, true
	}

	if h.scroll.KeyMap.IsSequenceStart(key) {
		h.logger.Debug("new sequence start while in sequence", zap.String("key", key))
		h.scroll.Context.SetLastKey(key)

		return "", false
	}

	h.scroll.Context.SetLastKey("")

	return "", false
}
