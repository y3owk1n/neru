package native

import (
	"context"
	"slices"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// MenuBarClickableElements retrieves clickable UI elements from the focused application's menu bar.
// If maxDepth is > 0, it overrides the configured tree depth.
func MenuBarClickableElements(
	ctx context.Context,
	logger *zap.Logger,
	configProvider config.Provider,
	maxDepth int,
) ([]*TreeNode, error) {
	logger.Debug("Getting clickable elements for menu bar")

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
	opts.SetConfigProvider(configProvider)

	if cfg := currentConfig(configProvider); cfg != nil {
		depth := cfg.Hints.MaxDepth
		if maxDepth > 0 {
			depth = maxDepth
		}

		opts.SetMaxDepth(depth)
	}

	tree, err := BuildTree(ctx, menubar, opts)
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
	allowedRoles[string(element.RoleMenuBarItem)] = struct{}{}

	ignoreClickableCheck := false
	if cfg := currentConfig(configProvider); cfg != nil {
		focusedBundleID := app.BundleIdentifier()
		ignoreClickableCheck = cfg.ShouldIgnoreClickableCheckForApp(focusedBundleID)
	}

	elements := tree.FindClickableElements(
		allowedRoles,
		configProvider,
		ignoreClickableCheck,
	)

	// Release tree nodes that are not part of the result to avoid
	// leaking CFRetain'd AXUIElementRefs from NeruGetChildren/NeruGetVisibleRows.
	ReleaseTreeExcept(tree, elements)

	logger.Debug("Found menu bar clickable elements", zap.Int("count", len(elements)))

	return elements, nil
}

// ClickableElementsFromBundleID retrieves clickable UI elements from the application identified by bundle ID.
// If maxDepth is > 0, it overrides the configured tree depth (useful for flat supplementary sources).
func ClickableElementsFromBundleID(
	ctx context.Context,
	bundleID string,
	roles []string,
	logger *zap.Logger,
	configProvider config.Provider,
	maxDepth int,
) ([]*TreeNode, error) {
	logger.Debug("Getting clickable elements for bundle ID",
		zap.String("bundle_id", bundleID),
		zap.Int("role_count", len(roles)))

	app := ApplicationByBundleID(bundleID)
	if app == nil {
		logger.Debug("Application not found for bundle ID", zap.String("bundle_id", bundleID))

		return []*TreeNode{}, nil
	}
	defer app.Release()

	opts := DefaultTreeOptions(logger)
	opts.SetConfigProvider(configProvider)

	if slices.Contains(roles, string(element.RoleMenuBarItem)) {
		opts.SetFilterFunc(isAdditionalMenuBarElement)
	}

	if cfg := currentConfig(configProvider); cfg != nil {
		depth := cfg.Hints.MaxDepth
		if maxDepth > 0 {
			depth = maxDepth
		}

		opts.SetMaxDepth(depth)
	}

	tree, err := BuildTree(ctx, app, opts)
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

	ignoreClickableCheck := false
	if cfg := currentConfig(configProvider); cfg != nil {
		ignoreClickableCheck = cfg.ShouldIgnoreClickableCheckForApp(bundleID)
	}

	elements := tree.FindClickableElements(
		allowedRoles,
		configProvider,
		ignoreClickableCheck,
	)

	// Release tree nodes that are not part of the result to avoid
	// leaking CFRetain'd AXUIElementRefs from NeruGetChildren/NeruGetVisibleRows.
	ReleaseTreeExcept(tree, elements)

	logger.Debug("Found clickable elements for application",
		zap.String("bundle_id", bundleID),
		zap.Int("count", len(elements)))

	return elements, nil
}

func isAdditionalMenuBarElement(info *ElementInfo) bool {
	//nolint:exhaustive
	switch element.Role(info.Role()) {
	case element.RoleMenuBar, element.RoleMenu, element.RoleMenuItem:
		return true
	case element.RoleMenuBarItem:
		return info.Subrole() == string(element.SubroleMenuExtra)
	default:
		return false
	}
}
