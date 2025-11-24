package accessibility

import (
	"errors"
	"fmt"
	"image"
	"strings"
	"sync"
	"time"

	"github.com/y3owk1n/neru/internal/infra/logger"
	"go.uber.org/zap"
)

var (
	globalCache *InfoCache
	cacheOnce   sync.Once

	// Pre-allocated common errors.
	errNoFrontmostWindow = errors.New("no frontmost window found")
)

func rectFromInfo(info *ElementInfo) image.Rectangle {
	return image.Rect(
		info.Position.X,
		info.Position.Y,
		info.Position.X+info.Size.X,
		info.Position.Y+info.Size.Y,
	)
}

// PrintTree outputs the accessibility tree structure to the log for debugging purposes.
func PrintTree(node *TreeNode, depth int) {
	if node == nil || node.Info == nil {
		return
	}

	var indent strings.Builder
	for range depth {
		indent.WriteString("  ")
	}

	logger.Info(fmt.Sprintf("%sRole: %s, Title: %s, Size: %dx%d",
		indent.String(), node.Info.Role, node.Info.Title, node.Info.Size.X, node.Info.Size.Y))

	for _, child := range node.Children {
		PrintTree(child, depth+1)
	}
}

// GetClickableElements retrieves all clickable UI elements in the frontmost window.
func GetClickableElements() ([]*TreeNode, error) {
	logger.Debug("Getting clickable elements for frontmost window")

	cacheOnce.Do(func() {
		globalCache = NewInfoCache(5 * time.Second)
	})

	window := GetFrontmostWindow()
	if window == nil {
		logger.Warn("No frontmost window found")

		return nil, errNoFrontmostWindow
	}
	defer window.Release()

	opts := DefaultTreeOptions()
	opts.Cache = globalCache

	tree, err := BuildTree(window, opts)
	if err != nil {
		logger.Error("Failed to build tree for frontmost window", zap.Error(err))

		return nil, err
	}

	elements := tree.FindClickableElements()
	logger.Debug("Found clickable elements", zap.Int("count", len(elements)))

	return elements, nil
}

// GetScrollableElements retrieves all scrollable UI elements in the frontmost window.

// GetMenuBarClickableElements retrieves clickable UI elements from the focused application's menu bar.
func GetMenuBarClickableElements() ([]*TreeNode, error) {
	logger.Debug("Getting clickable elements for menu bar")

	cacheOnce.Do(func() {
		globalCache = NewInfoCache(5 * time.Second)
	})

	app := GetFocusedApplication()
	if app == nil {
		logger.Debug("No focused application found")

		return []*TreeNode{}, nil
	}
	defer app.Release()

	menubar := app.GetMenuBar()
	if menubar == nil {
		logger.Debug("No menu bar found")

		return []*TreeNode{}, nil
	}
	defer menubar.Release()

	opts := DefaultTreeOptions()
	opts.Cache = globalCache

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

// GetClickableElementsFromBundleID retrieves clickable UI elements from the application identified by bundle ID.
func GetClickableElementsFromBundleID(bundleID string) ([]*TreeNode, error) {
	logger.Debug("Getting clickable elements for bundle ID", zap.String("bundle_id", bundleID))

	cacheOnce.Do(func() {
		globalCache = NewInfoCache(5 * time.Second)
	})

	app := GetApplicationByBundleID(bundleID)
	if app == nil {
		logger.Debug("Application not found for bundle ID", zap.String("bundle_id", bundleID))

		return []*TreeNode{}, nil
	}
	defer app.Release()

	opts := DefaultTreeOptions()
	opts.Cache = globalCache
	opts.IncludeOutOfBounds = true

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
