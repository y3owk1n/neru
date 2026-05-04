package electron

import (
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
)

const (
	electronAttributeName = "AXManualAccessibility"
	enhancedAttributeName = "AXEnhancedUserInterface"
)

var (
	electronPIDsMu      sync.Mutex
	electronEnabledPIDs = make(map[int]struct{})
	chromiumPIDsMu      sync.Mutex
	chromiumEnabledPIDs = make(map[int]struct{})
	firefoxPIDsMu       sync.Mutex
	firefoxEnabledPIDs  = make(map[int]struct{})
)

// EnsureElectronAccessibility enables AXManualAccessibility for Electron-based applications.
// This allows Neru to properly interact with Electron applications that don't expose their
// UI elements correctly to the macOS accessibility API.
func EnsureElectronAccessibility(bundleID string, logger *zap.Logger) bool {
	if logger == nil {
		logger = zap.NewNop()
	}

	app := accessibility.ApplicationByBundleID(bundleID)
	if app == nil {
		logger.Debug("Application not found for bundle ID", zap.String("bundle_id", bundleID))

		return false
	}

	info, infoErr := app.Info()
	if infoErr != nil {
		return false
	}

	pid := info.PID()

	if pid <= 0 {
		return false
	}

	electronPIDsMu.Lock()

	_, already := electronEnabledPIDs[pid]

	electronPIDsMu.Unlock()

	if already {
		return true
	}

	successSetElectron := platformSetApplicationAttribute(pid, electronAttributeName, true)

	if !successSetElectron {
		logger.Warn(
			"Failed to enable AXManualAccessibility",
			zap.Int("pid", pid),
			zap.String("bundle_id", bundleID),
		)

		return false
	}

	electronPIDsMu.Lock()

	electronEnabledPIDs[pid] = struct{}{}

	electronPIDsMu.Unlock()

	return true
}

// ensureAccessibility enables accessibility attributes for the specified application.
// This is a generic function used by both Chromium and Firefox accessibility enablers.
// setManualAccessibility controls whether to also set AXManualAccessibility (Electron/Chromium only).
func ensureAccessibility(
	bundleID string,
	enabledPIDs map[int]struct{},
	pidsMu *sync.Mutex,
	logger *zap.Logger,
	setManualAccessibility bool,
) bool {
	if logger == nil {
		logger = zap.NewNop()
	}

	app := accessibility.ApplicationByBundleID(bundleID)
	if app == nil {
		logger.Debug("Application not found for bundle ID", zap.String("bundle_id", bundleID))

		return false
	}

	info, infoErr := app.Info()
	if infoErr != nil {
		logger.Debug("Failed to inspect app window", zap.Error(infoErr))

		return false
	}

	pid := info.PID()

	if pid <= 0 {
		return false
	}

	pidsMu.Lock()

	_, already := enabledPIDs[pid]

	pidsMu.Unlock()

	if already {
		return true
	}

	// Set AXManualAccessibility for Electron/Chromium apps (not for Firefox).
	// This is the preferred method for newer Electron/Chromium versions as it doesn't
	// have the "screen reader active" side effects.
	// Ref: https://github.com/electron/electron/issues/7206
	var amaSuccess bool
	if setManualAccessibility {
		amaSuccess = platformSetApplicationAttribute(pid, electronAttributeName, true)
	}

	// Also set AXEnhancedUserInterface - this is the classic way to enable accessibility
	// and is needed for older Chromium versions / Firefox.
	euiSuccess := platformSetApplicationAttribute(pid, enhancedAttributeName, true)

	if !amaSuccess && !euiSuccess {
		logger.Warn("Failed to enable accessibility attributes", zap.String("bundle_id", bundleID))

		return false
	}

	// Mark as enabled only if at least one succeeded, to avoid retrying failed attempts
	pidsMu.Lock()

	enabledPIDs[pid] = struct{}{}

	pidsMu.Unlock()

	return true
}

// EnsureChromiumAccessibility enables AXEnhancedUserInterface for Chromium-based applications.
// This improves accessibility support for Chromium browsers and applications.
func EnsureChromiumAccessibility(bundleID string, logger *zap.Logger) bool {
	return ensureAccessibility(bundleID, chromiumEnabledPIDs, &chromiumPIDsMu, logger, true)
}

// EnsureFirefoxAccessibility enables AXEnhancedUserInterface for Firefox-based applications.
// This improves accessibility support for Firefox browsers and applications.
func EnsureFirefoxAccessibility(bundleID string, logger *zap.Logger) bool {
	return ensureAccessibility(bundleID, firefoxEnabledPIDs, &firefoxPIDsMu, logger, false)
}

// ShouldEnableElectronSupport determines if the provided bundle identifier
// should have Electron accessibility manually toggled based on defaults and
// user-specified overrides.
func ShouldEnableElectronSupport(bundleID string, additionalBundles []string) bool {
	if bundleID == "" {
		return false
	}

	if config.MatchesAdditionalBundle(bundleID, additionalBundles) {
		return true
	}

	result := IsLikelyElectronBundle(bundleID)

	return result
}

// ShouldEnableChromiumSupport determines if Chromium accessibility should be enabled for the provided bundle.
// This function checks both known Chromium bundles and user-provided overrides.
func ShouldEnableChromiumSupport(bundleID string, additionalBundles []string) bool {
	if bundleID == "" {
		return false
	}

	if config.MatchesAdditionalBundle(bundleID, additionalBundles) {
		return true
	}

	result := IsLikelyChromiumBundle(bundleID)

	return result
}

// ShouldEnableFirefoxSupport determines if Firefox accessibility should be enabled for the provided bundle.
// This function checks both known Firefox bundles and user-provided overrides.
func ShouldEnableFirefoxSupport(bundleID string, additionalBundles []string) bool {
	if bundleID == "" {
		return false
	}

	if config.MatchesAdditionalBundle(bundleID, additionalBundles) {
		return true
	}

	result := IsLikelyFirefoxBundle(bundleID)

	return result
}

// IsLikelyElectronBundle returns true if the provided bundle identifier
// matches a known Electron signature.
func IsLikelyElectronBundle(bundleID string) bool {
	lower := strings.ToLower(strings.TrimSpace(bundleID))
	if lower == "" {
		return false
	}

	for _, exact := range config.KnownElectronBundles {
		if strings.EqualFold(strings.TrimSpace(exact), lower) {
			return true
		}
	}

	return false
}

// IsLikelyChromiumBundle returns true if the provided bundle identifier matches a known Chromium signature.
// This helps identify applications that would benefit from AXEnhancedUserInterface accessibility improvements.
func IsLikelyChromiumBundle(bundleID string) bool {
	lower := strings.ToLower(strings.TrimSpace(bundleID))
	if lower == "" {
		return false
	}

	for _, exact := range config.KnownChromiumBundles {
		if strings.EqualFold(strings.TrimSpace(exact), lower) {
			return true
		}
	}

	return false
}

// IsLikelyFirefoxBundle returns true if the provided bundle identifier matches a known Firefox signature.
// This helps identify applications that would benefit from AXEnhancedUserInterface accessibility improvements.
func IsLikelyFirefoxBundle(bundleID string) bool {
	lower := strings.ToLower(strings.TrimSpace(bundleID))
	if lower == "" {
		return false
	}

	for _, exact := range config.KnownFirefoxBundles {
		if strings.EqualFold(strings.TrimSpace(exact), lower) {
			return true
		}
	}

	return false
}
