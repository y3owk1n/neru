package electron

import (
	"go.uber.org/zap"
)

// EnsureAccessibility reports whether the application identified by bundleID
// has an accessibility tree neru can hint against, waiting briefly for it to
// appear. Electron, Chromium and Firefox build their tree asynchronously after
// being asked to expose one, so the first hints activation after focusing such
// an app would otherwise find nothing.
//
// It returns true once a usable tree is found, and false if the wait times out.
func EnsureAccessibility(bundleID string, logger *zap.Logger) bool {
	if logger == nil {
		logger = zap.NewNop()
	}

	return ensureAccessibility(bundleID, logger.Named("electron"))
}
