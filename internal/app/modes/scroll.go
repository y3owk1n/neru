package modes

import (
	"context"

	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"go.uber.org/zap"
)

// StartInteractiveScroll activates the interactive scroll mode,
// showing the scroll overlay and enabling key handling for scrolling.
func (h *Handler) StartInteractiveScroll() {
	h.cursorState.SkipNextRestore()

	h.exitModeLocked()

	if h.enableEventTap != nil {
		h.enableEventTap()
	}

	h.scroll.Context.SetIsActive(true)

	h.overlayManager.ResizeToActiveScreen()

	h.SetModeScroll()

	h.logger.Info("Interactive scroll activated")
	h.logger.Info("Use configured keys for navigation")
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
		currentLastKey := h.scroll.Context.LastKey()
		if currentLastKey != "" {
			h.logger.Debug("key lookup failed but sequence in progress, not clearing lastKey",
				zap.String("key", key),
				zap.String("lastKey", currentLastKey))
		} else {
			h.scroll.Context.SetLastKey("")
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
