package accessibility

import (
	"context"

	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// addSupplementaryElements adds menubar, dock, and notification center elements based on filter.
func (a *Adapter) addSupplementaryElements(
	ctx context.Context,
	elements []*element.Element,
	filter ports.ElementFilter,
) []*element.Element {
	// Check if Mission Control is active
	missionControlActive := a.client.IsMissionControlActive()

	a.logger.Debug("Adding supplementary elements",
		zap.Bool("mission_control_active", missionControlActive),
		zap.Bool("include_menubar", filter.IncludeMenubar),
		zap.Bool("include_dock", filter.IncludeDock),
		zap.Bool("include_nc", filter.IncludeNotificationCenter))

	// Add menubar elements
	if !missionControlActive && filter.IncludeMenubar {
		elements = a.addMenubarElements(ctx, elements, filter)
	}

	// Add dock elements
	if filter.IncludeDock {
		elements = a.addDockElements(ctx, elements)
	}

	// Add notification center elements (only when Mission Control is active)
	if missionControlActive && filter.IncludeNotificationCenter {
		elements = a.addNotificationCenterElements(ctx, elements)
	}

	// Add stage manager elements
	if filter.IncludeStageManager {
		elements = a.addStageManagerElements(ctx, elements)
	}

	return elements
}

// addMenubarElements adds menubar clickable elements.
// Temporarily modifies clickable roles to include AXMenuBarItem, collects menubar elements,
// and processes additional menubar targets from configuration.
func (a *Adapter) addMenubarElements(
	_ context.Context,
	elements []*element.Element,
	filter ports.ElementFilter,
) []*element.Element {
	a.logger.Debug("Adding menubar elements")

	// Create local allowed roles including AXMenuBarItem for additional targets
	originalRoles := a.client.ClickableRoles()
	menubarRoles := make([]string, len(originalRoles)+1)
	copy(menubarRoles, originalRoles)
	menubarRoles[len(originalRoles)] = "AXMenuBarItem"

	// Get menubar elements
	menubarNodes, menubarNodesErr := a.client.MenuBarClickableElements()
	if menubarNodesErr != nil {
		a.logger.Warn("Failed to get menubar elements", zap.Error(menubarNodesErr))
	} else {
		for _, node := range menubarNodes {
			element, elementErr := a.convertToDomainElement(node)
			if elementErr != nil {
				a.logger.Debug("Failed to convert menubar element", zap.Error(elementErr))

				continue
			}

			if a.MatchesFilter(element, filter) {
				elements = append(elements, element)
			}
		}

		a.logger.Debug("Included menubar elements", zap.Int("count", len(menubarNodes)))
	}

	// Get additional menubar targets
	for _, bundleID := range filter.AdditionalMenubarTargets {
		additionalNodes, err := a.client.ClickableElementsFromBundleID(bundleID, menubarRoles)
		if err != nil {
			a.logger.Warn("Failed to get additional menubar elements",
				zap.String("bundle_id", bundleID),
				zap.Error(err))

			continue
		}

		for _, node := range additionalNodes {
			element, elementErr := a.convertToDomainElement(node)
			if elementErr != nil {
				a.logger.Debug(
					"Failed to convert additional menubar element",
					zap.Error(elementErr),
				)

				continue
			}

			if a.MatchesFilter(element, filter) {
				elements = append(elements, element)
			}
		}

		a.logger.Debug("Included additional menubar elements",
			zap.String("bundle_id", bundleID),
			zap.Int("count", len(additionalNodes)))
	}

	return elements
}

// addDockElements adds dock clickable elements.
// Temporarily modifies clickable roles to include AXDockItem, finds the dock application,
// validates it, and collects clickable elements from the dock tree.
func (a *Adapter) addDockElements(
	_ context.Context,
	elements []*element.Element,
) []*element.Element {
	const dockBundleID = "com.apple.dock"

	// Create local allowed roles including AXDockItem
	originalRoles := a.client.ClickableRoles()
	dockRoles := make([]string, len(originalRoles)+1)
	copy(dockRoles, originalRoles)
	dockRoles[len(originalRoles)] = "AXDockItem"

	// Get dock application by bundle ID
	dockApp, dockAppErr := a.client.ApplicationByBundleID(dockBundleID)
	if dockAppErr != nil || dockApp == nil {
		a.logger.Debug("Dock application not found")

		return elements
	}
	defer dockApp.Release()

	// Validate we got the correct application element (not a stale menu item)
	appInfo, appInfoErr := dockApp.Info()
	if appInfoErr != nil {
		a.logger.Warn("Failed to get dock application info", zap.Error(appInfoErr))

		return elements
	}

	if appInfo.Role != "AXApplication" {
		a.logger.Warn("Got incorrect element for dock, expected AXApplication",
			zap.String("actual_role", appInfo.Role),
			zap.String("title", appInfo.Title))

		return elements
	}

	// Build tree and find clickable elements
	dockNodes, dockNodesErr := a.client.ClickableNodes(dockApp, true, dockRoles)
	if dockNodesErr != nil {
		a.logger.Warn("Failed to get dock elements", zap.Error(dockNodesErr))

		return elements
	}

	for _, node := range dockNodes {
		element, elementErr := a.convertToDomainElement(node)
		if elementErr != nil {
			a.logger.Warn("Failed to convert dock element", zap.Error(elementErr))

			continue
		}

		elements = append(elements, element)
	}

	a.logger.Debug("Included dock elements", zap.Int("count", len(dockNodes)))

	return elements
}

// addNotificationCenterElements adds notification center clickable elements.
func (a *Adapter) addNotificationCenterElements(
	_ context.Context,
	elements []*element.Element,
) []*element.Element {
	const ncBundleID = "com.apple.notificationcenterui"

	a.logger.Debug("Adding notification center elements")

	ncNodes, ncNodesErr := a.client.ClickableElementsFromBundleID(ncBundleID, nil)
	if ncNodesErr != nil {
		a.logger.Warn("Failed to get notification center elements", zap.Error(ncNodesErr))

		return elements
	}

	for _, node := range ncNodes {
		element, elementErr := a.convertToDomainElement(node)
		if elementErr != nil {
			a.logger.Warn("Failed to convert notification center element", zap.Error(elementErr))

			continue
		}

		elements = append(elements, element)
	}

	a.logger.Debug("Included notification center elements", zap.Int("count", len(ncNodes)))

	return elements
}

// addStageManagerElements adds stage manager center clickable elements.
func (a *Adapter) addStageManagerElements(
	_ context.Context,
	elements []*element.Element,
) []*element.Element {
	const wmBundleID = "com.apple.WindowManager"

	// Get window manager application by bundle ID
	wmApp, wmAppErr := a.client.ApplicationByBundleID(wmBundleID)
	if wmAppErr != nil || wmApp == nil {
		a.logger.Debug("Window manager application not found")

		return elements
	}
	defer wmApp.Release()

	// Validate we got the correct application element (not a stale menu item)
	appInfo, appInfoErr := wmApp.Info()
	if appInfoErr != nil {
		a.logger.Warn("Failed to get window manager application info", zap.Error(appInfoErr))

		return elements
	}

	if appInfo.Role != "AXApplication" {
		a.logger.Warn("Got incorrect element for window manager, expected AXApplication",
			zap.String("actual_role", appInfo.Role),
			zap.String("title", appInfo.Title))

		return elements
	}

	// Build tree and find clickable elements
	wmNodes, wmNodesErr := a.client.ClickableNodes(wmApp, true, nil)
	if wmNodesErr != nil {
		a.logger.Warn("Failed to get window manager elements", zap.Error(wmNodesErr))

		return elements
	}

	for _, node := range wmNodes {
		element, elementErr := a.convertToDomainElement(node)
		if elementErr != nil {
			a.logger.Warn("Failed to convert window manager element", zap.Error(elementErr))

			continue
		}

		elements = append(elements, element)
	}

	a.logger.Debug("Included window manager elements", zap.Int("count", len(wmNodes)))

	return elements
}
