//go:build darwin

package eventtap

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/eventtap/darwin"
	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
)

// NewEventTap returns the Quartz event tap on macOS.
func NewEventTap(callback tap.Callback, logger *zap.Logger) tap.Tap {
	built := darwin.NewEventTap(callback, logger)
	if built == nil {
		// Returning the typed nil would hand back a non-nil interface holding a
		// nil pointer, and every caller's `tap != nil` check would pass.
		return nil
	}

	return built
}
