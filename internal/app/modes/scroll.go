package modes

import (
	"context"
	"fmt"

	"github.com/y3owk1n/neru/internal/application/services"
	"github.com/y3owk1n/neru/internal/features/scroll"
	"go.uber.org/zap"
)

// StartInteractiveScroll initiates interactive scrolling mode with visual feedback.
func (h *Handler) StartInteractiveScroll() {
	h.Cursor.SkipNextRestore()
	// Reset scroll context before exiting current mode to ensure clean state transition
	h.Scroll.Context.SetIsActive(false)
	h.Scroll.Context.SetLastKey("")
	h.ExitMode()

	// Use ScrollService to show overlay
	// Note: DrawScrollHighlightBorder logic is now in ScrollService.ShowScrollOverlay
	// But ScrollService.ShowScrollOverlay needs context.
	// We'll use a background context for now or create one.
	ctx := context.Background()
	err := h.ScrollService.ShowScrollOverlay(ctx)
	if err != nil {
		h.Logger.Error("Failed to show scroll overlay", zap.Error(err))
	}

	if h.EnableEventTap != nil {
		h.EnableEventTap()
	}

	h.Scroll.Context.SetIsActive(true)

	h.Logger.Info("Interactive scroll activated")
	h.Logger.Info("Use j/k to scroll, Ctrl+D/U for half-page, g/G for top/bottom, Esc to exit")
}

// handleGenericScrollKey handles scroll keys in a generic way.
func (h *Handler) handleGenericScrollKey(key string, lastScrollKey *string) {
	var localLastKey string
	if lastScrollKey == nil {
		lastScrollKey = &localLastKey
	}

	bytes := []byte(key)
	h.Logger.Info("Scroll key pressed",
		zap.String("key", key),
		zap.Int("len", len(key)),
		zap.String("hex", fmt.Sprintf("%#v", key)),
		zap.Any("bytes", bytes))

	var err error
	ctx := context.Background()

	if len(key) == 1 {
		if h.handleControlScrollKey(key, *lastScrollKey, lastScrollKey) {
			return
		}
	}

	h.Logger.Debug(
		"Entering switch statement",
		zap.String("key", key),
		zap.String("keyHex", fmt.Sprintf("%#v", key)),
	)
	switch key {
	case "j", "k", "h", "l":
		err = h.handleDirectionalScrollKey(key, *lastScrollKey)
	case "g":
		operation, newLast, ok := scroll.ParseKey(key, *lastScrollKey, h.Logger)
		if !ok {
			h.Logger.Info("First g pressed, press again for top")
			*lastScrollKey = newLast
			return
		}
		if operation == "start_g" {
			// First 'g' pressed, wait for second 'g'
			h.Logger.Info("First g pressed, press again for top")
			*lastScrollKey = newLast
			return
		}
		if operation == "top" {
			h.Logger.Info("gg detected - scroll to top")
			err = h.ScrollService.Scroll(ctx, services.ScrollDirectionUp, services.ScrollAmountEnd)
			*lastScrollKey = ""
			goto done
		}
	case "G":
		operation, _, ok := scroll.ParseKey(key, *lastScrollKey, h.Logger)
		if ok && operation == "bottom" {
			h.Logger.Info("G key detected - scroll to bottom")
			err = h.ScrollService.Scroll(
				ctx,
				services.ScrollDirectionDown,
				services.ScrollAmountEnd,
			)
			*lastScrollKey = ""
		}
	default:
		h.Logger.Debug("Ignoring non-scroll key", zap.String("key", key))
		*lastScrollKey = ""
		return
	}

	// Only reset lastScrollKey if we didn't just set it for a multi-key sequence
	// This is handled in each case above

done:
	if err != nil {
		h.Logger.Error("Scroll failed", zap.Error(err))
	}
}

// handleControlScrollKey handles control character scroll keys.
func (h *Handler) handleControlScrollKey(key string, lastKey string, lastScrollKey *string) bool {
	byteVal := key[0]
	h.Logger.Info("Checking control char", zap.Uint8("byte", byteVal))
	// Only handle Ctrl+D / Ctrl+U here; let Tab (9) and other keys fall through to switch
	if byteVal == 4 || byteVal == 21 {
		op, _, ok := scroll.ParseKey(key, lastKey, h.Logger)
		if ok {
			*lastScrollKey = ""
			ctx := context.Background()
			var err error
			switch op {
			case "half_down":
				h.Logger.Info("Ctrl+D detected - half page down")
				err = h.ScrollService.Scroll(
					ctx,
					services.ScrollDirectionDown,
					services.ScrollAmountHalfPage,
				)
			case "half_up":
				h.Logger.Info("Ctrl+U detected - half page up")
				err = h.ScrollService.Scroll(
					ctx,
					services.ScrollDirectionUp,
					services.ScrollAmountHalfPage,
				)
			}
			return err == nil
		}
	}
	return false
}

// handleDirectionalScrollKey handles directional scroll keys (j, k, h, l).
func (h *Handler) handleDirectionalScrollKey(key string, lastKey string) error {
	op, _, ok := scroll.ParseKey(key, lastKey, h.Logger)
	if !ok {
		return nil
	}

	ctx := context.Background()

	switch key {
	case "j":
		if op == "down" {
			return h.ScrollService.Scroll(
				ctx,
				services.ScrollDirectionDown,
				services.ScrollAmountChar,
			)
		}
	case "k":
		if op == "up" {
			return h.ScrollService.Scroll(
				ctx,
				services.ScrollDirectionUp,
				services.ScrollAmountChar,
			)
		}
	case "h":
		if op == "left" {
			return h.ScrollService.Scroll(
				ctx,
				services.ScrollDirectionLeft,
				services.ScrollAmountChar,
			)
		}
	case "l":
		if op == "right" {
			return h.ScrollService.Scroll(
				ctx,
				services.ScrollDirectionRight,
				services.ScrollAmountChar,
			)
		}
	}
	return nil
}
