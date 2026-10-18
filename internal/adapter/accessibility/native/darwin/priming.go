//go:build darwin

package darwin

import (
	"context"
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
	// maxPrimingNodes caps how many elements one walk takes from the
	// application. A tree with this many elements has been built, whatever
	// roles it holds, so reaching the cap counts as ready.
	maxPrimingNodes = 2000
)

// readyRoles are the AX roles whose presence means the Chromium/Gecko
// accessibility tree has been built. They are native macOS role names: this
// file is the only place that inspects the tree, and it only runs on darwin.
var readyRoles = map[string]struct{}{
	"AXWebArea":    {},
	"AXScrollArea": {},
}

// menuRoles are the AX roles this walk must not descend into. Reading
// kAXChildrenAttribute on a menu bar item is what makes AppKit build — and
// display — the menu behind it, so walking them does not observe the
// application, it operates it: for a status-bar app such as a clipboard
// manager, priming pops the menu open on the user's screen.
//
// The walk roots at the application element, which is what puts AXMenuBar in
// reach in the first place; AXMenu and AXMenuItem are here because an already
// open menu can also hang off a window. Nothing is lost by stopping: a
// readyRole is web content, and web content never lives inside a menu.
var menuRoles = map[string]struct{}{
	"AXMenuBar":     {},
	"AXMenuBarItem": {},
	"AXMenu":        {},
	"AXMenuItem":    {},
}

// PrimeApplication warms an application's accessibility tree so the first hint
// scan does not pay for a cold one. It reports whether the tree became ready,
// and gives up as soon as ctx is done.
func PrimeApplication(ctx context.Context, bundleID string, logger *zap.Logger) bool {
	app := ApplicationByBundleID(bundleID)
	if app == nil {
		logger.Debug("Application not found for bundle ID", zap.String("bundle_id", bundleID))

		return false
	}
	defer app.Release()

	return waitForAccessibility(ctx, app, logger)
}

func waitForAccessibility(ctx context.Context, app *Element, logger *zap.Logger) bool {
	for range primingRetryCount {
		if hasUsableAccessibilityTree(ctx, app, maxPrimingNodes, logger) {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(primingRetryDelay):
		}
	}

	return false
}

func hasUsableAccessibilityTree(
	ctx context.Context,
	root *Element,
	budget int,
	logger *zap.Logger,
) bool {
	if root == nil {
		return false
	}

	type entry struct {
		el    *Element
		depth int
	}

	queue := []entry{{root, 0}}
	// The children graph is not always a tree. An application can list an
	// element under more than one parent, or under its own descendant. A hash
	// collision only skips an element, and the next poll walks again.
	visited := make(map[uint64]struct{})
	taken := 0

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

		if ctx.Err() != nil {
			releaseVisited(cur.el)
			drainQueue()

			return false
		}

		hash, hashErr := cur.el.Hash()
		if hashErr == nil {
			if _, seen := visited[hash]; seen {
				releaseVisited(cur.el)

				continue
			}

			visited[hash] = struct{}{}
		}

		info, err := cur.el.Info()
		if err != nil || info == nil {
			releaseVisited(cur.el)

			continue
		}

		role := info.Role()

		// Stop before the menu subtree rather than after: asking for its
		// children is itself the side effect.
		if _, isMenu := menuRoles[role]; isMenu {
			releaseVisited(cur.el)

			continue
		}

		if _, ready := readyRoles[role]; ready {
			logger.Debug("Found usable accessibility tree", zap.String("role", role))
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

		taken += len(children)
		if taken > budget {
			logger.Debug("Accessibility tree reached the priming node budget",
				zap.Int("budget", budget))
			ReleaseAll(children)
			drainQueue()

			return true
		}

		for _, child := range children {
			queue = append(queue, entry{child, cur.depth + 1})
		}
	}

	return false
}
