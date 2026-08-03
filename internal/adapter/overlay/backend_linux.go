//go:build linux

package overlay

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/linux"
	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
)

// Init builds the process-wide overlay manager.
func Init(logger *zap.Logger) manager.Interface { return linux.Init(logger) }

// Get returns the process-wide overlay manager, or nil before Init.
//
// It is a package-level accessor because the overlay is one native window per
// process; the callers that reach for it — cleanup, indicator polling, the
// Wayland keyboard path — have no injected handle to use instead.
func Get() manager.Interface {
	built := linux.Get()
	if built == nil {
		// A typed nil would give the caller a non-nil interface around a nil
		// pointer, and every "if m != nil" guard would pass.
		return nil
	}

	return built
}
