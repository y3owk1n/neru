package accessibility

import (
	"image"
	"sync"
	"time"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"go.uber.org/zap"
)

const (
	// DefaultAccessibilityCacheTTL is the default cache TTL for accessibility.
	DefaultAccessibilityCacheTTL = 5 * time.Second
)

var (
	globalCache *InfoCache
	cacheOnce   sync.Once

	// Pre-allocated common errors.
	errNoFrontmostWindow = derrors.New(derrors.CodeAccessibilityFailed, "no frontmost window found")
)

func rectFromInfo(info *ElementInfo) image.Rectangle {
	pos := info.Position()
	size := info.Size()

	return image.Rect(
		pos.X,
		pos.Y,
		pos.X+size.X,
		pos.Y+size.Y,
	)
}

// ClickableElements retrieves all clickable UI elements in the frontmost window.
func ClickableElements(logger *zap.Logger) ([]*TreeNode, error) {
	logger.Debug("Getting clickable elements for frontmost window")

	cacheOnce.Do(func() {
		globalCache = NewInfoCache(DefaultAccessibilityCacheTTL, logger)
	})

	window := FrontmostWindow()
	if window == nil {
		logger.Warn("No frontmost window found")

		return nil, errNoFrontmostWindow
	}
	defer window.Release()

	opts := DefaultTreeOptions(logger)
	opts.cache = globalCache

	tree, err := BuildTree(window, opts)
	if err != nil {
		logger.Error("Failed to build tree for frontmost window", zap.Error(err))

		return nil, err
	}

	elements := tree.FindClickableElements(nil)

	// Release tree nodes that are not part of the result to avoid
	// leaking CFRetain'd AXUIElementRefs from getChildren/getVisibleRows.
	releaseTreeExcept(tree, elements)
	logger.Debug("Found clickable elements", zap.Int("count", len(elements)))

	return elements, nil
}

// MenuBarClickableElements retrieves clickable UI elements from the focused application's menu bar.
func MenuBarClickableElements(logger *zap.Logger) ([]*TreeNode, error) {
	logger.Debug("Getting clickable elements for menu bar")

	cacheOnce.Do(func() {
		globalCache = NewInfoCache(DefaultAccessibilityCacheTTL, logger)
	})

	app := FocusedApplication()
	if app == nil {
		logger.Debug("No focused application found")

		return []*TreeNode{}, nil
	}
	defer app.Release()

	menubar := app.MenuBar()
	if menubar == nil {
		logger.Debug("No menu bar found")

		return []*TreeNode{}, nil
	}
	defer menubar.Release()

	opts := DefaultTreeOptions(logger)
	opts.cache = globalCache

	tree, err := BuildTree(menubar, opts)
	if err != nil {
		logger.Error("Failed to build tree for menu bar", zap.Error(err))

		return nil, err
	}

	if tree == nil {
		logger.Debug("No tree built for menu bar")

		return []*TreeNode{}, nil
	}

	// Create local allowed roles map including AXMenuBarItem
	allowedRoles := make(map[string]struct{})

	// Add global roles
	globalRoles := ClickableRoles()
	for _, role := range globalRoles {
		allowedRoles[role] = struct{}{}
	}

	// Add menubar specific role
	allowedRoles["AXMenuBarItem"] = struct{}{}

	elements := tree.FindClickableElements(allowedRoles)

	// Release tree nodes that are not part of the result to avoid
	// leaking CFRetain'd AXUIElementRefs from getChildren/getVisibleRows.
	releaseTreeExcept(tree, elements)

	logger.Debug("Found menu bar clickable elements", zap.Int("count", len(elements)))

	return elements, nil
}

// ClickableElementsFromBundleID retrieves clickable UI elements from the application identified by bundle ID.
func ClickableElementsFromBundleID(
	bundleID string,
	roles []string,
	logger *zap.Logger,
) ([]*TreeNode, error) {
	logger.Debug("Getting clickable elements for bundle ID",
		zap.String("bundle_id", bundleID),
		zap.Int("role_count", len(roles)))

	cacheOnce.Do(func() {
		globalCache = NewInfoCache(DefaultAccessibilityCacheTTL, logger)
	})

	app := ApplicationByBundleID(bundleID)
	if app == nil {
		logger.Debug("Application not found for bundle ID", zap.String("bundle_id", bundleID))

		return []*TreeNode{}, nil
	}
	defer app.Release()

	opts := DefaultTreeOptions(logger)
	opts.cache = globalCache
	opts.includeOutOfBounds = true

	tree, err := BuildTree(app, opts)
	if err != nil {
		logger.Error("Failed to build tree for application",
			zap.String("bundle_id", bundleID),
			zap.Error(err))

		return nil, err
	}

	if tree == nil {
		logger.Debug("No tree built for application", zap.String("bundle_id", bundleID))

		return []*TreeNode{}, nil
	}

	var allowedRoles map[string]struct{}
	if len(roles) > 0 {
		allowedRoles = make(map[string]struct{}, len(roles))
		for _, role := range roles {
			allowedRoles[role] = struct{}{}
		}
	}

	elements := tree.FindClickableElements(allowedRoles)

	// Release tree nodes that are not part of the result to avoid
	// leaking CFRetain'd AXUIElementRefs from getChildren/getVisibleRows.
	releaseTreeExcept(tree, elements)

	logger.Debug("Found clickable elements for application",
		zap.String("bundle_id", bundleID),
		zap.Int("count", len(elements)))

	return elements, nil
}

// releaseTreeExcept releases all AXUIElementRefs in the tree except those
// belonging to the keep list. This prevents leaking CFRetain'd refs from
// getChildren/getVisibleRows that are stored in tree nodes but never returned
// to callers.
func releaseTreeExcept(tree *TreeNode, keep []*TreeNode) {
	keepSet := make(map[*Element]struct{}, len(keep))
	for _, node := range keep {
		if node.Element() != nil {
			keepSet[node.Element()] = struct{}{}
		}
	}

	tree.Release(keepSet)
}
