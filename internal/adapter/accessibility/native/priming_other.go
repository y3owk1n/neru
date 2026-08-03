//go:build !darwin

// Non-darwin slot for AccessibilityPort.PrimeApplication.
// Does not wait for anything: the asynchronous tree it would wait for is a
// Chromium/Gecko behavior behind macOS's AXManualAccessibility. AT-SPI and UI
// Automation expose their trees eagerly, so there is nothing to prime.

package native

import "go.uber.org/zap"

// PrimeApplication reports success immediately so callers do not retry: no
// platform other than macOS needs the accessibility tree warmed.
func PrimeApplication(bundleID string, logger *zap.Logger) bool {
	logger.Debug(
		"Accessibility tree priming not required on this platform",
		zap.String("bundle_id", bundleID),
	)

	return true
}
