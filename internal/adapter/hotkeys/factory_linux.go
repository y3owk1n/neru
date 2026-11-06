//go:build linux

package hotkeys

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/hotkeys/linux"
	"github.com/y3owk1n/neru/internal/ports"
)

// NewManager returns the platform hotkey manager.
//
// Each backend implements ports.HotkeyPort directly, with no wrapper. The
// manager registers itself as the process-wide one here, as a consequence of
// building it. macOS's native callback needs that, and elsewhere it is a
// no-op.
func NewManager(logger *zap.Logger) ports.HotkeyPort {
	manager := linux.NewManager(logger)
	linux.SetGlobalManager(manager)

	return manager
}
