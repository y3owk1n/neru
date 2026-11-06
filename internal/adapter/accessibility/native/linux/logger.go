//go:build linux

package linux

import (
	"sync"

	"go.uber.org/zap"
)

// pkgLogger is the process-global logger for the Linux injection backends, a
// slot beside configProvider because the scroll path picks its backend deep
// below any struct that carries a logger.
var (
	pkgLoggerMu sync.RWMutex
	pkgLogger   = zap.NewNop()
)

// SetLogger installs the logger the Linux injection backends report through.
// It is set once at daemon startup (see internal/app/runtime_config_linux.go).
func SetLogger(logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}

	pkgLoggerMu.Lock()
	pkgLogger = logger.Named("accessibility.native")
	pkgLoggerMu.Unlock()
}

func currentLogger() *zap.Logger {
	pkgLoggerMu.RLock()
	defer pkgLoggerMu.RUnlock()

	return pkgLogger
}
