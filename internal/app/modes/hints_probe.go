package modes

import (
	"context"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// ProbeHints runs the hint-generation pipeline once against the focused window
// and returns the elements hints mode would label on the active screen,
// without drawing the overlay or entering hints mode. It backs `neru query
// hints`.
//
// The probe enumerates whatever window is focused when it runs, so when
// invoked directly from a terminal it reports that terminal's elements.
func (h *Handler) ProbeHints(
	ctx context.Context,
	filterRoles []string,
	filterTextContains []string,
	strategy string,
	captureScope string,
	splitWord bool,
) ([]*element.Element, error) {
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
		splitWord,
	)
	if genErr != nil {
		return nil, genErr
	}

	onScreen := filterHintsForScreen(generated, screenBounds)

	elements := make([]*element.Element, len(onScreen))
	for idx, hintItem := range onScreen {
		elements[idx] = hintItem.Element()
	}

	return elements, nil
}
