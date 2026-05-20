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

const (
	accessibilityRetryCount = 10
	accessibilityRetryDelay = 100 * time.Millisecond
	maxAccessibilityDepth   = 10
)

// EnsureElectronAccessibility ensures Electron accessibility is enabled for the provided bundle ID.
func EnsureElectronAccessibility(bundleID string, logger *zap.Logger) bool {
	return ensureAccessibility(
		bundleID,
		&electronPIDsMu,
		electronEnabledPIDs,
		logger,
		true,
	)
}

// EnsureChromiumAccessibility ensures Chromium accessibility is enabled for the provided bundle ID.
func EnsureChromiumAccessibility(bundleID string, logger *zap.Logger) bool {
	return ensureAccessibility(
		bundleID,
		&chromiumPIDsMu,
		chromiumEnabledPIDs,
		logger,
		false,
	)
}

// EnsureFirefoxAccessibility ensures Firefox accessibility is enabled for the provided bundle ID.
func EnsureFirefoxAccessibility(bundleID string, logger *zap.Logger) bool {
	return ensureAccessibility(
		bundleID,
		&firefoxPIDsMu,
		firefoxEnabledPIDs,
		logger,
		false,
	)
}

func ensureAccessibility(
	bundleID string,
	pidsMu *sync.Mutex,
	enabledPIDs map[int]struct{},
	logger *zap.Logger,
	isElectron bool,
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

	if hasUsableAccessibilityTree(app, logger) {
		markPIDEnabled(pidsMu, enabledPIDs, pid)

		return true
	}

	if isElectron {
		success := platformSetApplicationAttribute(pid, electronAttributeName, true)

		if !success {
			logger.Debug(
				"Failed to set AXManualAccessibility",
				zap.Int("pid", pid),
				zap.String("bundle_id", bundleID),
			)
		}
	}

	if waitForAccessibility(app, logger) {
		markPIDEnabled(pidsMu, enabledPIDs, pid)

		return true
	}

	success := platformSetApplicationAttribute(pid, enhancedAttributeName, true)

	if !success {
		logger.Debug(
			"Failed to enable AXEnhancedUserInterface",
			zap.Int("pid", pid),
			zap.String("bundle_id", bundleID),
		)

		return false
	}

	if waitForAccessibility(app, logger) {
		markPIDEnabled(pidsMu, enabledPIDs, pid)

		return true
	}

	logger.Warn(
		"Accessibility could not be enabled",
		zap.Int("pid", pid),
		zap.String("bundle_id", bundleID),
	)

	return false
}

func waitForAccessibility(app *accessibility.Element, logger *zap.Logger) bool {
	for range accessibilityRetryCount {
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

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.el == nil {
			continue
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

func markPIDEnabled(
	pidsMu *sync.Mutex,
	enabledPIDs map[int]struct{},
	pid int,
) {
	pidsMu.Lock()
	defer pidsMu.Unlock()

	enabledPIDs[pid] = struct{}{}
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
