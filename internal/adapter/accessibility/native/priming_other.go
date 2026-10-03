//go:build !darwin

package native

import "go.uber.org/zap"

// PrimeApplication reports success immediately so callers do not retry. The
// asynchronous tree it would wait for is a Chromium/Gecko behavior behind
// macOS's AXManualAccessibility; AT-SPI and UI Automation expose their trees
// eagerly, so there is nothing to prime.
func PrimeApplication(_ string, _ *zap.Logger) bool {
	return true
}
