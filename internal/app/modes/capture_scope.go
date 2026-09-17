package modes

import (
	"context"
	"image"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// captureScopeBundleTimeout bounds the focused-application lookup a per-app
// scope needs, so a slow accessibility answer cannot hold an activation.
const captureScopeBundleTimeout = time.Second

// captureScoped is a configuration section with a capture scope of its own
// and per-app entries that may shadow it: bisect, grid and recursive grid.
type captureScoped interface {
	HasAppCaptureScopeOverrides() bool
	CaptureScopeForApp(bundleID string) string
}

// resolveCaptureScope is the region a session starts from: the section's,
// shadowed by the focused application's app_configs entry, and then by the
// activation's --capture-scope. The application is asked once, under a short
// bound, the way hints asks at its activation. A lookup that fails leaves the
// configured scope in force. mode names the caller in the log.
func (h *handlerState) resolveCaptureScope(
	mode string,
	activation modecmd.Activation,
	section captureScoped,
) string {
	if activation.CaptureScope != nil {
		return *activation.CaptureScope
	}

	if !section.HasAppCaptureScopeOverrides() {
		return section.CaptureScopeForApp("")
	}

	bundleCtx, bundleCancel := context.WithTimeout(h.ctx, captureScopeBundleTimeout)
	bundleID, bundleIDErr := h.actionService.FocusedAppBundleID(bundleCtx)

	bundleCancel()

	if bundleIDErr != nil {
		h.logger.Debug("Failed to get the focused app for the capture scope",
			zap.String("mode", mode), zap.Error(bundleIDErr))

		return section.CaptureScopeForApp("")
	}

	return section.CaptureScopeForApp(bundleID)
}

// captureStart is the region a session begins with, in global coordinates:
// the focused window when the scope asks for it and one is focused, and the
// screen otherwise. A window that runs off the screen is cut to it, since
// the frame is drawn in that screen's space. mode names the caller in the log.
func (h *handlerState) captureStart(
	mode string,
	screen image.Rectangle,
	scope string,
) image.Rectangle {
	if scope != domain.CaptureScopeWindow || h.system == nil {
		return screen
	}

	window, focused, err := h.system.FocusedWindowBounds(h.ctx)
	if err != nil {
		// Warned at every code, CodeNotSupported included: the user asked for
		// a window and is getting the screen.
		h.logger.Warn("Failed to get the focused window; using the screen",
			zap.String("mode", mode), zap.Error(err))

		return screen
	}

	if !focused || window.Empty() {
		h.logger.Debug("No focused window; using the screen", zap.String("mode", mode))

		return screen
	}

	clipped := window.Intersect(screen)
	if clipped.Empty() {
		return screen
	}

	return clipped
}
