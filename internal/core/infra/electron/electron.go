package electron

import (
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/infra/accessibility"
)

var (
	axPIDsMu      sync.Mutex
	axEnabledPIDs = make(map[int]struct{})
)

const (
	accessibilityRetryCount = 10
	accessibilityRetryDelay = 100 * time.Millisecond
	maxAccessibilityDepth   = 10
)

// EnsureAccessibility ensures accessibility is enabled for the provided bundle ID.
func EnsureAccessibility(bundleID string, logger *zap.Logger) bool {
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

	axPIDsMu.Lock()
	_, already := axEnabledPIDs[pid]
	axPIDsMu.Unlock()

	if already {
		return true
	}

	if waitForAccessibility(app, logger) {
		markPIDEnabled(&axPIDsMu, axEnabledPIDs, pid)

		return true
	}

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
