//go:build darwin

package electron

import (
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
)

const (
	accessibilityRetryCount = 10
	accessibilityRetryDelay = 100 * time.Millisecond
	maxAccessibilityDepth   = 10
)

// readyRoles are the AX roles whose presence means the Chromium/Gecko
// accessibility tree has been built. They are native macOS role names: this
// file is the only place that inspects the tree, and it only runs on darwin.
var readyRoles = map[string]struct{}{
	"AXWebArea":    {},
	"AXScrollArea": {},
}

func ensureAccessibility(bundleID string, logger *zap.Logger) bool {
	app := accessibility.ApplicationByBundleID(bundleID)
	if app == nil {
		logger.Debug("Application not found for bundle ID", zap.String("bundle_id", bundleID))

		return false
	}

	return waitForAccessibility(app, logger)
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

		if _, ready := readyRoles[role]; ready {
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
