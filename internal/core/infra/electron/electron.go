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

// axCacheEntry records the outcome of an enable attempt for one pid.
type axCacheEntry struct {
	bundle string
	ok     bool
	at     time.Time
}

var (
	enabledPIDsMu sync.Mutex
	// enabledPIDs caches the enable outcome per pid. Keying on the bundle id too
	// means a recycled pid (same number, different app after a quit/relaunch) is
	// treated as new. A positive result is cached permanently; a negative result
	// is honored only for non-patient apps and only for negativeCacheTTL, so a
	// native app is not re-probed on every activation while a classified app can
	// still be retried as its tree boots.
	enabledPIDs = make(map[int]axCacheEntry)
)

const (
	accessibilityRetryCount   = 10
	quickAccessibilityRetries = 3
	accessibilityRetryDelay   = 100 * time.Millisecond
	// negativeCacheTTL bounds how often a non-patient (unclassified) app that did
	// not become usable is re-probed.
	negativeCacheTTL = 30 * time.Second
	// maxAccessibilityDepth and maxAccessibilityNodes bound the tree probe so it
	// stays cheap even though EnsureAppAccessibility now runs for every activated
	// app (not just a whitelist).
	maxAccessibilityDepth = 8
	maxAccessibilityNodes = 400
)

// EnsureAppAccessibility wakes an application's accessibility tree if it is not
// already usable, caching the result per pid.
//
// AXManualAccessibility is always attempted first: it wakes Electron and
// Chromium trees and is a harmless no-op on apps that do not implement it, with
// no window side effects. AXEnhancedUserInterface — which some apps react to by
// relaying out or moving their windows — is attempted only when allowEnhanced is
// true, so it is never sprayed on native apps. Callers derive allowEnhanced from
// AdditionalAXSupport.EscalateEnhanced (browsers only by default).
//
// patient controls how hard we wait and re-probe. Classified apps (browsers,
// listed Electron) are patient: they get the full retry window and ignore the
// negative cache, because their tree may still be booting. Unclassified apps are
// impatient: a short wait and a negative cache, so a native app is not re-probed
// (a bounded but non-trivial tree walk) on every activation.
func EnsureAppAccessibility(bundleID string, allowEnhanced, patient bool, logger *zap.Logger) bool {
	if logger == nil {
		logger = zap.NewNop()
	}

	logger = logger.Named("electron")

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

	enabledPIDsMu.Lock()
	entry, cached := enabledPIDs[pid]
	enabledPIDsMu.Unlock()

	if cached && entry.bundle == bundleID {
		if entry.ok {
			return true
		}
		// Recently probed and unusable. Impatient apps skip the re-probe; patient
		// apps fall through and retry as their tree may still be coming up.
		if !patient && time.Since(entry.at) < negativeCacheTTL {
			return false
		}
	}

	// Already usable (accessibility already on, or a native app with a scroll
	// area): nothing to do, and no attribute is touched.
	if hasUsableAccessibilityTree(app, logger) {
		markPIDResult(pid, bundleID, true)

		return true
	}

	// Safe listless step for every app.
	if !platformSetApplicationAttribute(pid, manualAttributeName, true) {
		logger.Debug("Failed to set AXManualAccessibility",
			zap.Int("pid", pid), zap.String("bundle_id", bundleID))
	}

	if waitForAccessibility(app, patient, logger) {
		markPIDResult(pid, bundleID, true)

		return true
	}

	// Gated escalation: only for apps the caller allows (browsers by default),
	// never native apps.
	if !allowEnhanced {
		logger.Debug("Accessibility tree not usable; enhanced escalation not allowed",
			zap.Int("pid", pid), zap.String("bundle_id", bundleID))
		markPIDResult(pid, bundleID, false)

		return false
	}

	if !platformSetApplicationAttribute(pid, enhancedAttributeName, true) {
		logger.Debug("Failed to enable AXEnhancedUserInterface",
			zap.Int("pid", pid), zap.String("bundle_id", bundleID))
		markPIDResult(pid, bundleID, false)

		return false
	}

	if waitForAccessibility(app, patient, logger) {
		markPIDResult(pid, bundleID, true)

		return true
	}

	logger.Warn("Accessibility could not be enabled",
		zap.Int("pid", pid), zap.String("bundle_id", bundleID))
	markPIDResult(pid, bundleID, false)

	return false
}

func waitForAccessibility(app *accessibility.Element, patient bool, logger *zap.Logger) bool {
	retries := quickAccessibilityRetries
	if patient {
		retries = accessibilityRetryCount
	}

	for range retries {
		if hasUsableAccessibilityTree(app, logger) {
			return true
		}

		time.Sleep(accessibilityRetryDelay)
	}

	return false
}

func hasUsableAccessibilityTree(root *accessibility.Element, logger *zap.Logger) bool {
	if root == nil {
		return false
	}

	type entry struct {
		el    *accessibility.Element
		depth int
	}

	queue := []entry{{root, 0}}
	visited := 0

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.el == nil {
			continue
		}

		visited++
		if visited > maxAccessibilityNodes {
			return false
		}

		info, err := cur.el.Info()
		if err != nil || info == nil {
			continue
		}

		role := info.Role()

		switch role {
		case "AXWebArea", "AXScrollArea":
			logger.Info("Found usable accessibility tree", zap.String("role", role))

			return true
		}

		if cur.depth >= maxAccessibilityDepth {
			continue
		}

		children, err := cur.el.Children(role)
		if err != nil {
			continue
		}

		for _, child := range children {
			queue = append(queue, entry{child, cur.depth + 1})
		}
	}

	return false
}

func markPIDResult(pid int, bundleID string, ok bool) {
	enabledPIDsMu.Lock()
	defer enabledPIDsMu.Unlock()

	enabledPIDs[pid] = axCacheEntry{bundle: bundleID, ok: ok, at: time.Now()}
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
