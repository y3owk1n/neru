package electron

import (
	"strings"
	"sync"
	"time"

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

// axState records which accessibility attributes have been set on one running
// application. bundle is the application's bundle id, used to detect pid reuse
// (see enabledPIDs). The *Failed flags remember that a set already failed and
// was logged, so an application that does not support an attribute is retried on
// later focus without repeating the log line.
type axState struct {
	bundle         string
	manual         bool
	manualFailed   bool
	enhanced       bool
	enhancedFailed bool
}

// enabledPIDs caches the accessibility state of running applications, keyed by
// pid. An entry is dropped when its application terminates (see
// ForgetAppAccessibility), so a later process that reuses the retired pid starts
// from a clean slate. The axState.bundle field covers the window before that
// termination is processed: if the OS hands the pid to a different application
// first, the mismatched bundle resets the record so the new application has its
// attributes set too.
var (
	enabledPIDsMu sync.Mutex
	enabledPIDs   = make(map[int]axState)
)

// EnsureAppAccessibility sets the accessibility attributes that make an
// application's hint targets readable.
//
// `AXManualAccessibility` is set on every application. It wakes Electron and
// Chromium accessibility trees, and is a harmless no-op on applications that do
// not implement it.
//
// `AXEnhancedUserInterface` is set only when useEnhanced is true, which the
// caller restricts to Firefox browsers with web-content hints turned on. It
// exposes browser web-area content but can relayout or move windows under
// tiling window managers, so it stays off every other application.
//
// A successful set is cached per pid, so re-focusing an already-woken app does
// no further work. A failed set is not cached, so a later focus retries it.
//
// A freshly launched app may not have its accessibility tree ready the moment
// it first gains focus, so the attributes are set with exponential-backoff
// retries. The retry burst is confined to an app's first encounter: an app
// already handled (woken, or one that does not take the attribute) returns
// after a single attempt, so ordinary native apps are not put through the whole
// burst on every focus. The retries sleep between attempts, so callers run this
// on a goroutine.
func EnsureAppAccessibility(bundleID string, useEnhanced bool, logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}

	logger = logger.Named("electron")

	const (
		maxAttempts   = 5
		initialDelay  = 100 * time.Millisecond
		backoffFactor = 2
	)

	ready, retry := ensureAppAccessibilityOnce(bundleID, useEnhanced, logger)
	if ready || !retry {
		return
	}

	delay := initialDelay

	for attempt := 1; attempt < maxAttempts; attempt++ {
		time.Sleep(delay)
		delay *= backoffFactor

		if ready, _ := ensureAppAccessibilityOnce(bundleID, useEnhanced, logger); ready {
			return
		}
	}
}

// ensureAppAccessibilityOnce resolves the application for bundleID and applies
// its accessibility attributes once. It reports whether every wanted attribute
// is in place (ready) and whether a backoff retry may still help (retry): true
// while the app is not yet resolvable or is seen for the first time, and false
// once it has been handled.
func ensureAppAccessibilityOnce(bundleID string, useEnhanced bool, logger *zap.Logger) (ready, retry bool) {
	app := accessibility.ApplicationByBundleID(bundleID)
	if app == nil {
		logger.Debug("Application not found for bundle ID", zap.String("bundle_id", bundleID))

		return false, true
	}

	info, infoErr := app.Info()
	if infoErr != nil {
		return false, true
	}

	pid := info.PID()
	if pid <= 0 {
		return false, true
	}

	return ensurePIDAccessibility(pid, bundleID, useEnhanced, logger)
}

// ensurePIDAccessibility applies and caches the accessibility attributes for a
// resolved pid. It is separated from the application lookup so the caching and
// gating rules can be tested without a live accessibility tree. It reports
// whether every wanted attribute is in place (ready) and whether this is the
// first time the process behind the pid is seen (retry), which the caller uses
// to limit the backoff retry burst to freshly launched apps.
func ensurePIDAccessibility(pid int, bundleID string, useEnhanced bool, logger *zap.Logger) (ready, retry bool) {
	enabledPIDsMu.Lock()
	state := enabledPIDs[pid]
	firstEncounter := !strings.EqualFold(state.bundle, bundleID)
	if firstEncounter {
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

	if useEnhanced {
		if !state.enhanced {
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
	} else if state.enhanced {
		// AXEnhancedUserInterface can relayout or move windows under tiling
		// window managers, so when web-content hints are off it is cleared to
		// keep that side effect from outlasting the setting.
		if setAttribute(pid, enhancedAttributeName, false) {
			state.enhanced = false

			logger.Debug(
				"enhanced accessibility cleared",
				zap.String("bundle_id", bundleID),
				zap.Int("pid", pid),
			)
		}
	}

	enabledPIDsMu.Lock()
	enabledPIDs[pid] = state
	enabledPIDsMu.Unlock()

	return state.manual && (!useEnhanced || state.enhanced), firstEncounter
}

// ForgetAppAccessibility drops the cached accessibility state for every pid
// belonging to the given bundle id. It is called when an application terminates
// so a later process that reuses a retired pid, or a fresh instance of the same
// app, starts from a clean slate and has its attributes set again.
func ForgetAppAccessibility(bundleID string) {
	if bundleID == "" {
		return
	}

	enabledPIDsMu.Lock()
	defer enabledPIDsMu.Unlock()

	for pid, state := range enabledPIDs {
		if strings.EqualFold(state.bundle, bundleID) {
			delete(enabledPIDs, pid)
		}
	}
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
