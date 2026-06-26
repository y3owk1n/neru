// internal/app/modes/hints_debug.go
// Read-only diagnostic that runs the hint-generation pipeline once and reports
// what would be hinted for the focused window, backing `neru hints --debug`.
// It does not draw any overlay, activate a mode, or mutate Handler state.

package modes

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// maxDebugProbeSamples caps how many elements are listed in the probe summary.
const maxDebugProbeSamples = 10

// DebugProbeHints runs the hint-generation pipeline once against the currently
// focused window and returns a human-readable summary (element count plus a
// short sample) without drawing the overlay or entering hints mode. It backs
// `neru hints --debug` so the platform accessibility pipeline (AX on macOS,
// AT-SPI on Linux, UI Automation on Windows) can be verified from the CLI.
//
// The probe enumerates whatever window is focused when the command runs, so
// when invoked directly from a terminal it reports that terminal's elements.
func (h *Handler) DebugProbeHints(
	ctx context.Context,
	filterRoles []string,
	filterTextContains []string,
	strategy string,
) (string, error) {
	bundleID, bundleErr := h.actionService.FocusedAppBundleID(ctx)
	if bundleErr != nil {
		h.logger.Debug("hints debug probe: failed to get focused app id", zap.Error(bundleErr))
	}

	screenBounds, boundsErr := h.actionService.ScreenBounds(ctx)
	if boundsErr != nil {
		return "", boundsErr
	}

	generated, genErr := h.hintService.GenerateHints(
		ctx,
		filterRoles,
		filterTextContains,
		bundleID,
		strategy,
		"", // labelDirectionOverride: probe uses the configured default
	)
	if genErr != nil {
		return "", genErr
	}

	onScreen := filterHintsForScreen(generated, screenBounds)

	var builder strings.Builder

	focused := bundleID
	if focused == "" {
		focused = "(unknown)"
	}

	fmt.Fprintf(&builder, "hints debug probe\n")
	fmt.Fprintf(&builder, "  focused app:   %s\n", focused)
	fmt.Fprintf(&builder, "  screen bounds: %s\n", screenBounds.String())
	fmt.Fprintf(
		&builder,
		"  clickable elements: %d detected, %d on active screen\n",
		len(generated),
		len(onScreen),
	)

	sample := onScreen
	if len(sample) == 0 {
		sample = generated
	}

	if len(sample) > maxDebugProbeSamples {
		sample = sample[:maxDebugProbeSamples]
	}

	for i, hintItem := range sample {
		elem := hintItem.Element()
		fmt.Fprintf(
			&builder,
			"  [%2d] role=%s title=%q bounds=%s\n",
			i+1,
			elem.Role(),
			elem.Title(),
			elem.Bounds().String(),
		)
	}

	return strings.TrimRight(builder.String(), "\n"), nil
}
