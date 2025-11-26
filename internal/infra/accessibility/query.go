package accessibility

import (
	"fmt"
	"image"
	"strings"
	"sync"
	"time"

	derrors "github.com/y3owk1n/neru/internal/errors"
	"github.com/y3owk1n/neru/internal/infra/logger"
	"go.uber.org/zap"
)

const (
	// DefaultCacheTTL is the default cache TTL.
	DefaultCacheTTL = 5 * time.Second
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

// PrintTree outputs the accessibility tree structure to the log for debugging purposes.
func PrintTree(node *TreeNode, depth int) {
	if node == nil || node.Info() == nil {
		return
	}

	var indent strings.Builder
	for range depth {
		indent.WriteString("  ")
	}

	size := node.Info().Size()
	logger.Info(fmt.Sprintf("%sRole: %s, Title: %s, Size: %dx%d",
		indent.String(), node.Info().Role(), node.Info().Title(), size.X, size.Y))

	for _, child := range node.Children() {
		PrintTree(child, depth+1)
	}
}

// ClickableElements retrieves all clickable UI elements in the frontmost window.
func ClickableElements() ([]*TreeNode, error) {
	logger.Debug("Getting clickable elements for frontmost window")

	cacheOnce.Do(func() {
		globalCache = NewInfoCache(DefaultCacheTTL)
	})

	window := FrontmostWindow()
	if window == nil {
		logger.Warn("No frontmost window found")

		return nil, errNoFrontmostWindow
	}
	defer window.Release()

	opts := DefaultTreeOptions()
	opts.cache = globalCache

	tree, err := BuildTree(window, opts)
	if err != nil {
		logger.Error("Failed to build tree for frontmost window", zap.Error(err))

		return nil, err
	}

	elements := tree.FindClickableElements()
	logger.Debug("Found clickable elements", zap.Int("count", len(elements)))

	return elements, nil
}

// MenuBarClickableElements retrieves clickable UI elements from the focused application's menu bar.
func MenuBarClickableElements() ([]*TreeNode, error) {
	logger.Debug("Getting clickable elements for menu bar")

	cacheOnce.Do(func() {
		globalCache = NewInfoCache(DefaultCacheTTL)
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

	opts := DefaultTreeOptions()
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

	elements := tree.FindClickableElements()
	logger.Debug("Found menu bar clickable elements", zap.Int("count", len(elements)))

	return elements, nil
}

// ClickableElementsFromBundleID retrieves clickable UI elements from the application identified by bundle ID.
func ClickableElementsFromBundleID(bundleID string) ([]*TreeNode, error) {
	logger.Debug("Getting clickable elements for bundle ID", zap.String("bundle_id", bundleID))

	cacheOnce.Do(func() {
		globalCache = NewInfoCache(DefaultCacheTTL)
	})

	app := ApplicationByBundleID(bundleID)
	if app == nil {
		logger.Debug("Application not found for bundle ID", zap.String("bundle_id", bundleID))

		return []*TreeNode{}, nil
	}
	defer app.Release()

	opts := DefaultTreeOptions()
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

	elements := tree.FindClickableElements()
	logger.Debug("Found clickable elements for application",
		zap.String("bundle_id", bundleID),
		zap.Int("count", len(elements)))

	return elements, nil
}
