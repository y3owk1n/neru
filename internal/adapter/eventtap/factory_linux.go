//go:build linux

package eventtap

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
)

// NewEventTap returns the Linux event tap. It captures through X11 or through
// evdev on Wayland; the package picks between them at runtime from the detected
// backend, so there is nothing to choose here.
func NewEventTap(callback tap.Callback, logger *zap.Logger) tap.Tap {
	built := linux.NewEventTap(callback, logger)
	if built == nil {
		// Returning the typed nil would hand back a non-nil interface holding a
		// nil pointer, and every caller's `tap != nil` check would pass.
		return nil
	}

	return built
}
