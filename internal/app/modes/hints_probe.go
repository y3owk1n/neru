package modes

import (
	"context"

	"go.uber.org/zap"

	domainHint "github.com/y3owk1n/neru/internal/domain/hint"
)

// ProbeHints runs the hint-generation pipeline once against the focused window
// and returns the hints that hints mode would draw on the active screen, with
// their labels, without drawing the overlay or entering hints mode. It backs
// `neru query hints`.
//
// The probe enumerates whatever window is focused when it runs, so when
// invoked directly from a terminal it reports that terminal's elements.
func (h *Handler) ProbeHints(
	ctx context.Context,
	filterRoles []string,
	filterTextContains []string,
	strategy string,
	captureScope string,
	labelDirection string,
	splitWord bool,
) ([]*domainHint.Interface, error) {
	bundleID, bundleErr := h.actionService.FocusedAppBundleID(ctx)
	if bundleErr != nil {
		h.logger.Debug("Failed to get the focused app for the hints probe", zap.Error(bundleErr))
	}

	screenBounds, boundsErr := h.actionService.ScreenBounds(ctx)
	if boundsErr != nil {
		return nil, boundsErr
	}

	generated, genErr := h.hintService.ProbeHints(
		ctx,
		filterRoles,
		filterTextContains,
		bundleID,
		strategy,
		captureScope,
		labelDirection,
		splitWord,
	)
	if genErr != nil {
		return nil, genErr
	}

	return filterHintsForScreen(generated, screenBounds), nil
}
