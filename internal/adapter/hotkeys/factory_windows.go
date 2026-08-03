//go:build windows

package hotkeys

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/hotkeys/windows"
	"github.com/y3owk1n/neru/internal/ports"
)

// NewManager returns the platform hotkey manager.
//
// There is no wrapper adapter: each backend implements ports.HotkeyPort
// directly, so this is only the choice of which one exists.
//
// Registering the manager as the process-wide one happens here rather than at
// the call site, because it is a consequence of building the manager and not a
// decision the caller makes. On macOS the native callback needs it; elsewhere
// it is a no-op.
func NewManager(logger *zap.Logger) ports.HotkeyPort {
	manager := windows.NewManager(logger)
	windows.SetGlobalManager(manager)

	return manager
}
