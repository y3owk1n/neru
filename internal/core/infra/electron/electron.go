package electron

import (
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
)

const (
	manualAttributeName   = "AXManualAccessibility"
	enhancedAttributeName = "AXEnhancedUserInterface"
)

// setAttribute is the platform accessibility setter. It is a package variable
// so tests can substitute a fake for the cross-process AX call.
var setAttribute = platformSetApplicationAttribute

// axState records which accessibility attributes have been set on a running
// application, keyed by pid. bundle guards against pid reuse: when the OS hands
// a retired pid to a different application, the mismatched bundle resets the
// record so the new application has its attributes set too. The *Failed flags
// remember that a set already failed and was logged, so a permanently
// unsupported app is retried on later focus without repeating the log line.
type axState struct {
	bundle         string
	manual         bool
	manualFailed   bool
	enhanced       bool
	enhancedFailed bool
}

var (
	enabledPIDsMu sync.Mutex
	enabledPIDs   = make(map[int]axState)
)

// EnsureAppAccessibility sets the accessibility attributes that make an
// application's hint targets readable.
//
// AXManualAccessibility is set on every application. It wakes Electron and
// Chromium accessibility trees, and is a harmless no-op on applications that do
// not implement it.
//
// AXEnhancedUserInterface is set only when useEnhanced is true, which the
// caller restricts to Chromium/Firefox browsers with web-content hints turned
// on. It exposes browser web-area content but can relayout or move windows
// under tiling window managers, so it stays off every other application.
//
// A successful set is cached per pid, so re-focusing an already-woken app does
// no further work. A failed set is not cached, so a later focus retries it.
func EnsureAppAccessibility(bundleID string, useEnhanced bool, logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}

	logger = logger.Named("electron")

	app := accessibility.ApplicationByBundleID(bundleID)
	if app == nil {
		logger.Debug("Application not found for bundle ID", zap.String("bundle_id", bundleID))

		return
	}

	info, infoErr := app.Info()
	if infoErr != nil {
		return
	}

	pid := info.PID()
	if pid <= 0 {
		return
	}

	ensurePIDAccessibility(pid, bundleID, useEnhanced, logger)
}

// ensurePIDAccessibility applies and caches the accessibility attributes for a
// resolved pid. It is separated from the application lookup so the caching and
// gating rules can be tested without a live accessibility tree.
func ensurePIDAccessibility(pid int, bundleID string, useEnhanced bool, logger *zap.Logger) {
	enabledPIDsMu.Lock()
	state := enabledPIDs[pid]
	if !strings.EqualFold(state.bundle, bundleID) {
		state = axState{bundle: bundleID}
	}
	enabledPIDsMu.Unlock()

	if !state.manual {
		if setAttribute(pid, manualAttributeName, true) {
			state.manual = true
			state.manualFailed = false

			logger.Debug(
				"manual accessibility set",
				zap.String("bundle_id", bundleID),
				zap.Int("pid", pid),
			)
		} else if !state.manualFailed {
			state.manualFailed = true

			logger.Debug(
				"manual accessibility set failed",
				zap.String("bundle_id", bundleID),
				zap.Int("pid", pid),
			)
		}
	}

	if useEnhanced && !state.enhanced {
		if setAttribute(pid, enhancedAttributeName, true) {
			state.enhanced = true
			state.enhancedFailed = false

			logger.Debug(
				"enhanced accessibility set for web content",
				zap.String("bundle_id", bundleID),
				zap.Int("pid", pid),
			)
		} else if !state.enhancedFailed {
			state.enhancedFailed = true

			logger.Debug(
				"enhanced accessibility set failed",
				zap.String("bundle_id", bundleID),
				zap.Int("pid", pid),
			)
		}
	}

	enabledPIDsMu.Lock()
	enabledPIDs[pid] = state
	enabledPIDsMu.Unlock()
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
