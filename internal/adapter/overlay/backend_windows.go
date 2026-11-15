//go:build windows

package overlay

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/adapter/overlay/windows"
)

// Init builds the process-wide overlay manager.
func Init(logger *zap.Logger) manager.Interface { return windows.Init(logger) }
