//go:build windows

package overlay

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/windows"
)

// Init builds the process-wide overlay manager.
func Init(logger *zap.Logger) manager.Interface { return windows.Init(logger) }

// Get returns the process-wide overlay manager, or nil before Init.
//
// It is a package-level accessor because the overlay is one native window per
// process. Since #1213 its only caller is the Linux event tap, whose evdev and
// Wayland keyboard paths have no injected handle to use instead; cleanup and
// indicator polling reach the overlay through ports.OverlayPort.
func Get() manager.Interface {
	built := windows.Get()
	if built == nil {
		// A typed nil would give the caller a non-nil interface around a nil
		// pointer, and every "if m != nil" guard would pass.
		return nil
	}

	return built
}
