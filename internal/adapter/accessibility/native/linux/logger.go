//go:build linux

package linux

import (
	"sync"

	"go.uber.org/zap"
)

// pkgLogger is the process-global logger for the Linux injection backends,
// a slot beside configProvider for the same reason: the scroll path picks its
// backend at call time, deep below any struct that carries a logger, and the
// one thing worth saying from there is which backend it ended up on.
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
