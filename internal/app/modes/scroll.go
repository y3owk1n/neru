package modes

import (
	"context"
	"fmt"
	"time"

	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/app/services"
	"go.uber.org/zap"
)

// StartInteractiveScroll initiates interactive scrolling mode with visual feedback.
func (h *Handler) StartInteractiveScroll() {
	h.cursorState.SkipNextRestore()
	// Reset scroll context before exiting current mode to ensure clean state transition
	h.scroll.Context.SetIsActive(false)
	h.scroll.Context.SetLastKey("")
	h.ExitMode()

	// Enable event tap early to capture key presses immediately
	if h.enableEventTap != nil {
		h.enableEventTap()
	}

	// Set scroll context active before showing overlay
	h.scroll.Context.SetIsActive(true)

	// Position overlay on active screen before showing
	h.overlayManager.ResizeToActiveScreenSync()
	// Give the UI thread a moment to complete the resize
	time.Sleep(150 * time.Millisecond)

	ctx := context.Background()

	showScrollErr := h.scrollService.Show(ctx)
	if showScrollErr != nil {
		h.logger.Error("Failed to show scroll overlay", zap.Error(showScrollErr))
	}

	h.SetModeScroll()

	h.logger.Info("Interactive scroll activated")
	h.logger.Info("Use j/k to scroll, g/G for top/bottom, Esc to exit")
}

// handleGenericScrollKey handles scroll keys in a generic way.
// It processes various scroll commands including directional scrolling (j/k/h/l),
// page scrolling (Ctrl+D/Ctrl+U), and top/bottom navigation (gg/G).
// Maintains state for multi-key sequences like 'gg' for top.
func (h *Handler) handleGenericScrollKey(key string) {
	// Handle control scroll keys first; they manage lastKey directly via the context.
	if len(key) == 1 && h.handleControlScrollKey(key) {
		return
	}

	lastScrollKey := h.scroll.Context.LastKey()
	defer func() {
		h.scroll.Context.SetLastKey(lastScrollKey)
	}()

	bytes := []byte(key)
	h.logger.Info("Scroll key pressed",
		zap.String("key", key),
		zap.Int("len", len(key)),
		zap.String("hex", fmt.Sprintf("%#v", key)),
		zap.Any("bytes", bytes))

	var handleScrollErr error

	ctx := context.Background()

	h.logger.Debug(
		"Entering switch statement",
		zap.String("key", key),
		zap.String("keyHex", fmt.Sprintf("%#v", key)),
	)

	switch key {
	case "j", "k", "h", "l":
		// Handle directional scrolling (vim-style navigation)
		handleScrollErr = h.handleDirectionalScrollKey(key, lastScrollKey)
	case "g":
		// Handle 'g' key sequences: 'g' starts sequence, 'gg' scrolls to top
		operation, newLast, ok := scroll.ParseKey(key, lastScrollKey, h.logger)
		if !ok {
			h.logger.Info("First g pressed, press again for top")

			lastScrollKey = newLast

			return
		}

		if operation == "start_g" {
			// First 'g' pressed, wait for second 'g'
			h.logger.Info("First g pressed, press again for top")

			lastScrollKey = newLast

			return
		}

		if operation == "top" {
			h.logger.Info("gg detected - scroll to top")
			handleScrollErr = h.scrollService.Scroll(
				ctx,
				services.ScrollDirectionUp,
				services.ScrollAmountEnd,
			)
			lastScrollKey = ""

			goto done
		}
	case "G":
		// Handle 'G' key: scroll to bottom
		operation, _, ok := scroll.ParseKey(key, lastScrollKey, h.logger)
		if ok && operation == "bottom" {
			h.logger.Info("G key detected - scroll to bottom")
			handleScrollErr = h.scrollService.Scroll(
				ctx,
				services.ScrollDirectionDown,
				services.ScrollAmountEnd,
			)
			lastScrollKey = ""
		}
	default:
		// Reset state for unrecognized keys
		h.logger.Debug("Ignoring non-scroll key", zap.String("key", key))

		lastScrollKey = ""

		return
	}

done:
	if handleScrollErr != nil {
		h.logger.Error("Scroll failed", zap.Error(handleScrollErr))
	}
}

// handleControlScrollKey handles control character scroll keys.
func (h *Handler) handleControlScrollKey(key string) bool {
	byteVal := key[0]
	h.logger.Info("Checking control char", zap.Uint8("byte", byteVal))
	// Only handle Ctrl+D / Ctrl+U here; let Tab (9) and other keys fall through to switch
	if byteVal == 4 || byteVal == 21 {
		lastKey := h.scroll.Context.LastKey()

		operation, _, ok := scroll.ParseKey(key, lastKey, h.logger)
		if ok {
			h.scroll.Context.SetLastKey("")

			ctx := context.Background()

			var handleControlScrollKeyErr error

			switch operation {
			case "half_down":
				h.logger.Info("Ctrl+D detected - half page down")
				handleControlScrollKeyErr = h.scrollService.Scroll(
					ctx,
					services.ScrollDirectionDown,
					services.ScrollAmountHalfPage,
				)
			case "half_up":
				h.logger.Info("Ctrl+U detected - half page up")
				handleControlScrollKeyErr = h.scrollService.Scroll(
					ctx,
					services.ScrollDirectionUp,
					services.ScrollAmountHalfPage,
				)
			}

			if handleControlScrollKeyErr != nil {
				h.logger.Error("Control scroll failed", zap.Error(handleControlScrollKeyErr))
			}

			return handleControlScrollKeyErr == nil
		}
	}

	return false
}

// handleDirectionalScrollKey handles directional scroll keys (j, k, h, l).
func (h *Handler) handleDirectionalScrollKey(key string, lastKey string) error {
	operation, _, ok := scroll.ParseKey(key, lastKey, h.logger)
	if !ok {
		return nil
	}

	ctx := context.Background()

	switch key {
	case "j":
		if operation == "down" {
			return h.scrollService.Scroll(
				ctx,
				services.ScrollDirectionDown,
				services.ScrollAmountChar,
			)
		}
	case "k":
		if operation == "up" {
			return h.scrollService.Scroll(
				ctx,
				services.ScrollDirectionUp,
				services.ScrollAmountChar,
			)
		}
	case "h":
		if operation == "left" {
			return h.scrollService.Scroll(
				ctx,
				services.ScrollDirectionLeft,
				services.ScrollAmountChar,
			)
		}
	case "l":
		if operation == "right" {
			return h.scrollService.Scroll(
				ctx,
				services.ScrollDirectionRight,
				services.ScrollAmountChar,
			)
		}
	}

	return nil
}
