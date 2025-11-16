// Package main provides the main entry point for the Neru application.
package main

import (
	"github.com/y3owk1n/neru/internal/accessibility"
	"go.uber.org/zap"
)

// updateRolesForCurrentApp updates clickable and scrollable roles based on the current focused app.
func (a *App) updateRolesForCurrentApp() {
	// Get the focused application
	focusedApp := accessibility.GetFocusedApplication()
	if focusedApp == nil {
		a.logger.Debug("No focused application, using global roles only")
		// Use global roles
		accessibility.SetClickableRoles(a.config.Hints.ClickableRoles)
		return
	}
	defer focusedApp.Release()

	// Get bundle ID
	bundleID := focusedApp.GetBundleIdentifier()
	if bundleID == "" {
		a.logger.Debug("Could not get bundle ID, using global roles only")
		// Use global roles
		accessibility.SetClickableRoles(a.config.Hints.ClickableRoles)
		return
	}

	// Get merged roles for this app
	clickableRoles := a.config.GetClickableRolesForApp(bundleID)

	a.logger.Debug("Updating roles for current app",
		zap.String("bundle_id", bundleID),
		zap.Int("clickable_count", len(clickableRoles)),
	)

	// Apply the merged roles
	accessibility.SetClickableRoles(clickableRoles)
}

// getFocusedBundleID returns the bundle identifier of the currently focused
// application, or an empty string if it cannot be determined.
func (a *App) getFocusedBundleID() string {
	app := accessibility.GetFocusedApplication()
	if app == nil {
		return ""
	}
	defer app.Release()
	return app.GetBundleIdentifier()
}

// isFocusedAppExcluded returns true if the currently focused application's bundle
// ID is in the excluded apps list. Logs context for debugging.
func (a *App) isFocusedAppExcluded() bool {
	bundleID := a.getFocusedBundleID()
	if bundleID != "" && a.config.IsAppExcluded(bundleID) {
		a.logger.Debug("Current app is excluded; ignoring mode activation",
			zap.String("bundle_id", bundleID))
		return true
	}
	return false
}

// collectElementsForMode collects UI elements based on the current mode.
func (a *App) collectElements() []*accessibility.TreeNode {
	var elements []*accessibility.TreeNode

	// Check if Mission Control is active - affects what we can scan
	missionControlActive := accessibility.IsMissionControlActive()

	elements = a.collectClickableElements(missionControlActive)

	elements = a.addSupplementaryElements(elements, missionControlActive)

	return elements
}

// collectClickableElements collects clickable elements from the frontmost window.
func (a *App) collectClickableElements(missionControlActive bool) []*accessibility.TreeNode {
	if missionControlActive {
		a.logger.Info("Mission Control is active, skipping frontmost window clickable elements")
		return nil
	}

	a.logger.Info("Scanning for clickable elements")
	roles := accessibility.GetClickableRoles()
	a.logger.Debug("Clickable roles", zap.Strings("roles", roles))

	clickableElements, err := accessibility.GetClickableElements()
	if err != nil {
		a.logger.Error("Failed to get clickable elements", zap.Error(err))
		return nil
	}

	a.logger.Info("Found clickable elements", zap.Int("count", len(clickableElements)))
	return clickableElements
}

// addSupplementaryElements adds menubar, dock, and notification center elements.
func (a *App) addSupplementaryElements(
	elements []*accessibility.TreeNode,
	missionControlActive bool,
) []*accessibility.TreeNode {
	// Menubar elements
	if !missionControlActive {
		elements = a.addMenubarElements(elements)
	} else {
		a.logger.Info("Mission Control is active, skipping menubar elements")
	}

	// Dock elements
	elements = a.addDockElements(elements)

	// Notification Center elements (only when Mission Control is active)
	if missionControlActive {
		elements = a.addNotificationCenterElements(elements)
	}

	return elements
}

// addMenubarElements adds menubar clickable elements.
func (a *App) addMenubarElements(elements []*accessibility.TreeNode) []*accessibility.TreeNode {
	if !a.config.Hints.IncludeMenubarHints {
		return elements
	}

	a.logger.Info("Adding menubar elements")

	// Add standard menubar elements
	var mbElems []*accessibility.TreeNode
	var err error
	mbElems, err = accessibility.GetMenuBarClickableElements()
	if err == nil {
		elements = append(elements, mbElems...)
		a.logger.Debug("Included menubar elements", zap.Int("count", len(mbElems)))
	} else {
		a.logger.Warn("Failed to get menubar elements", zap.Error(err))
	}

	// Add additional menubar elements from specific bundle IDs
	for _, bundleID := range a.config.Hints.AdditionalMenubarHintsTargets {
		var additionalElems []*accessibility.TreeNode
		var err error
		additionalElems, err = accessibility.GetClickableElementsFromBundleID(bundleID)
		if err == nil {
			elements = append(elements, additionalElems...)
			a.logger.Debug("Included additional menubar elements",
				zap.String("bundle_id", bundleID),
				zap.Int("count", len(additionalElems)))
		} else {
			a.logger.Warn("Failed to get additional menubar elements",
				zap.String("bundle_id", bundleID),
				zap.Error(err))
		}
	}

	return elements
}

// addDockElements adds dock clickable elements.
func (a *App) addDockElements(elements []*accessibility.TreeNode) []*accessibility.TreeNode {
	if !a.config.Hints.IncludeDockHints {
		return elements
	}

	var dockElems []*accessibility.TreeNode
	var err error
	dockElems, err = accessibility.GetClickableElementsFromBundleID("com.apple.dock")
	if err == nil {
		elements = append(elements, dockElems...)
		a.logger.Debug("Included dock elements", zap.Int("count", len(dockElems)))
	} else {
		a.logger.Warn("Failed to get dock elements", zap.Error(err))
	}

	return elements
}

// addNotificationCenterElements adds notification center clickable elements.
func (a *App) addNotificationCenterElements(
	elements []*accessibility.TreeNode,
) []*accessibility.TreeNode {
	if !a.config.Hints.IncludeNCHints {
		return elements
	}

	a.logger.Info("Adding notification center elements")

	var ncElems []*accessibility.TreeNode
	var err error
	ncElems, err = accessibility.GetClickableElementsFromBundleID("com.apple.notificationcenterui")
	if err == nil {
		elements = append(elements, ncElems...)
		a.logger.Debug("Included notification center elements", zap.Int("count", len(ncElems)))
	} else {
		a.logger.Warn("Failed to get notification center elements", zap.Error(err))
	}

	return elements
}
