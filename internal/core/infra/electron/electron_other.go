//go:build !darwin

package electron

import "go.uber.org/zap"

// ensureAccessibility is a no-op outside macOS. The asynchronous tree it waits
// for is a Chromium/Gecko behavior behind macOS's AXManualAccessibility; the
// AT-SPI and UI Automation backends expose their trees eagerly, so there is
// nothing to wait for. It reports success so callers do not retry.
func ensureAccessibility(bundleID string, logger *zap.Logger) bool {
	logger.Debug(
		"Accessibility tree priming not required on this platform",
		zap.String("bundle_id", bundleID),
	)

	return true
}
