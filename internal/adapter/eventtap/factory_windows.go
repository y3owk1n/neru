//go:build windows

package eventtap

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/eventtap/tap"
	"github.com/y3owk1n/neru/internal/adapter/eventtap/windows"
)

// NewEventTap returns the low-level keyboard hook on Windows.
func NewEventTap(callback tap.Callback, logger *zap.Logger) tap.Tap {
	built := windows.NewEventTap(callback, logger)
	if built == nil {
		// Returning the typed nil would hand back a non-nil interface holding a
		// nil pointer, and every caller's `tap != nil` check would pass.
		return nil
	}

	return built
}
