//go:build darwin

package overlay

import (
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/overlay/darwin"
	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
)

// Init builds the process-wide overlay manager.
func Init(logger *zap.Logger) manager.Interface { return darwin.Init(logger) }
