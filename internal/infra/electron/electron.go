package electron

import (
	"strings"
	"sync"

	"github.com/y3owk1n/neru/internal/infra/accessibility"
	"github.com/y3owk1n/neru/internal/infra/bridge"
	"github.com/y3owk1n/neru/internal/infra/logger"
	"go.uber.org/zap"
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
func EnsureElectronAccessibility(bundleID string) bool {
	app := accessibility.ApplicationByBundleID(bundleID)

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

	successSetElectron := bridge.SetApplicationAttribute(pid, electronAttributeName, true)

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

// ensureAccessibility enables AXEnhancedUserInterface for the specified application.
// This is a generic function used by both Chromium and Firefox accessibility enablers.
func ensureAccessibility(
	bundleID string,
	enabledPIDs map[int]struct{},
	pidsMu *sync.Mutex,
) bool {
	app := accessibility.ApplicationByBundleID(bundleID)

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

	success := bridge.SetApplicationAttribute(pid, enhancedAttributeName, true)

	if !success {
		logger.Warn("Failed to enable AXEnhancedUserInterface", zap.String("bundle_id", bundleID))

		return false
	}

	pidsMu.Lock()

	enabledPIDs[pid] = struct{}{}

	pidsMu.Unlock()

	enabledPIDs[pid] = struct{}{}

	pidsMu.Unlock()

	return true
}

// EnsureChromiumAccessibility enables AXEnhancedUserInterface for Chromium-based applications.
// This improves accessibility support for Chromium browsers and applications.
func EnsureChromiumAccessibility(bundleID string) bool {
	return ensureAccessibility(bundleID, chromiumEnabledPIDs, &chromiumPIDsMu)
}

// EnsureFirefoxAccessibility enables AXEnhancedUserInterface for Firefox-based applications.
// This improves accessibility support for Firefox browsers and applications.
func EnsureFirefoxAccessibility(bundleID string) bool {
	return ensureAccessibility(bundleID, firefoxEnabledPIDs, &firefoxPIDsMu)
}

// KnownChromiumBundles contains known Chromium-based application bundle identifiers.
// These applications benefit from AXEnhancedUserInterface accessibility improvements.
var KnownChromiumBundles = []string{
	"net.imput.helium",
	"com.google.Chrome",
	"com.brave.Browser",
	"company.thebrowser.Browser",
}

// KnownFirefoxBundles contains known Firefox-based application bundle identifiers.
// These applications benefit from AXEnhancedUserInterface accessibility improvements.
var KnownFirefoxBundles = []string{
	"org.mozilla.firefox",
	"app.zen-browser.zen",
}

// KnownElectronBundles contains known Electron-based application bundle identifiers.
// These applications require manual accessibility attribute toggling to work properly.
var KnownElectronBundles = []string{
	// electrons
	"com.microsoft.VSCode",
	"com.exafunction.windsurf",
	"com.tinyspeck.slackmacgap",
	"com.spotify.client",
	"md.obsidian",
}

// ShouldEnableElectronSupport determines if the provided bundle identifier
// should have Electron accessibility manually toggled based on defaults and
// user-specified overrides.
func ShouldEnableElectronSupport(bundleID string, additionalBundles []string) bool {
	if bundleID == "" {
		return false
	}

	if matchesAdditionalBundle(bundleID, additionalBundles) {
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

	if matchesAdditionalBundle(bundleID, additionalBundles) {
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

	if matchesAdditionalBundle(bundleID, additionalBundles) {
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

	for _, exact := range KnownElectronBundles {
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

	for _, exact := range KnownChromiumBundles {
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

	for _, exact := range KnownFirefoxBundles {
		if strings.EqualFold(strings.TrimSpace(exact), lower) {
			return true
		}
	}

	return false
}

// matchesAdditionalBundle checks if a bundle ID matches any user-provided additional bundles.
// It supports both exact matches and wildcard patterns (ending with *).
func matchesAdditionalBundle(bundleID string, additionalBundles []string) bool {
	if len(additionalBundles) == 0 {
		return false
	}

	lower := strings.ToLower(strings.TrimSpace(bundleID))
	for _, candidate := range additionalBundles {
		trimmed := strings.ToLower(strings.TrimSpace(candidate))
		if trimmed == "" {
			continue
		}

		if prefix, found := strings.CutSuffix(trimmed, "*"); found {
			if strings.HasPrefix(lower, prefix) {
				return true
			}
		} else if lower == trimmed {
			return true
		}
	}

	return false
}
