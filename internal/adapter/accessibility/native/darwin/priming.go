//go:build darwin

package darwin

import (
	"time"

	"go.uber.org/zap"
)

// macOS slot for AccessibilityPort.PrimeApplication. Electron, Chromium and
// Gecko apps build their AX tree asynchronously after macOS asks them to expose
// one (AXManualAccessibility), so the first hints activation after focusing such
// an app would otherwise find nothing. This polls until a web-content role
// appears. It does not enable accessibility; it only waits for the tree.
const (
	primingRetryCount = 10
	primingRetryDelay = 100 * time.Millisecond
	maxPrimingDepth   = 10
)

// readyRoles are the AX roles whose presence means the Chromium/Gecko
// accessibility tree has been built. They are native macOS role names: this
// file is the only place that inspects the tree, and it only runs on darwin.
var readyRoles = map[string]struct{}{
	"AXWebArea":    {},
	"AXScrollArea": {},
}

// PrimeApplication warms an application's accessibility tree so the first hint
// scan does not pay for a cold one. It reports whether the tree became ready.
func PrimeApplication(bundleID string, logger *zap.Logger) bool {
	app := ApplicationByBundleID(bundleID)
	if app == nil {
		logger.Debug("Application not found for bundle ID", zap.String("bundle_id", bundleID))

		return false
	}
	defer app.Release()

	return waitForAccessibility(app, logger)
}

func waitForAccessibility(app *Element, logger *zap.Logger) bool {
	for range primingRetryCount {
		if hasUsableAccessibilityTree(app, logger) {
			return true
		}

		time.Sleep(primingRetryDelay)
	}

	return false
}

func hasUsableAccessibilityTree(root *Element, logger *zap.Logger) bool {
	if root == nil {
		return false
	}

	type entry struct {
		el    *Element
		depth int
	}

	queue := []entry{{root, 0}}

	// Children() hands over retained AXUIElementRefs; release everything this
	// walk enqueues, but not the caller-owned root.
	releaseVisited := func(el *Element) {
		if el != root {
			el.Release()
		}
	}

	drainQueue := func() {
		for _, pending := range queue {
			if pending.el != nil {
				releaseVisited(pending.el)
			}
		}
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.el == nil {
			continue
		}

		info, err := cur.el.Info()
		if err != nil || info == nil {
			releaseVisited(cur.el)

			continue
		}

		role := info.Role()

		if _, ready := readyRoles[role]; ready {
			logger.Info("Found usable accessibility tree", zap.String("role", role))
			releaseVisited(cur.el)
			drainQueue()

			return true
		}

		if cur.depth >= maxPrimingDepth {
			releaseVisited(cur.el)

			continue
		}

		children, childrenErr := cur.el.Children(role)
		releaseVisited(cur.el)

		if childrenErr != nil {
			continue
		}

		for _, child := range children {
			queue = append(queue, entry{child, cur.depth + 1})
		}
	}

	return false
}
