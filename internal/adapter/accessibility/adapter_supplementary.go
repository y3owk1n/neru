package accessibility

import (
	"context"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/domain/element"
	infra "github.com/y3owk1n/neru/internal/infra/accessibility"
	"go.uber.org/zap"
)

// addSupplementaryElements adds menubar, dock, and notification center elements based on filter.
func (a *Adapter) addSupplementaryElements(
	ctx context.Context,
	elements []*element.Element,
	filter ports.ElementFilter,
) []*element.Element {
	// Check if Mission Control is active
	missionControlActive := infra.IsMissionControlActive()

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

	return elements
}

// addMenubarElements adds menubar clickable elements.
func (a *Adapter) addMenubarElements(
	ctx context.Context,
	elements []*element.Element,
	filter ports.ElementFilter,
) []*element.Element {
	a.logger.Debug("Adding menubar elements")

	// Temporarily add AXMenuBarItem to clickable roles
	originalRoles := infra.GetClickableRoles()
	menubarRoles := append(originalRoles, "AXMenuBarItem")
	infra.SetClickableRoles(menubarRoles)
	defer infra.SetClickableRoles(originalRoles) // Restore original roles when done

	// Get menubar elements
	mbNodes, err := infra.GetMenuBarClickableElements()
	if err != nil {
		a.logger.Warn("Failed to get menubar elements", zap.Error(err))
	} else {
		for _, node := range mbNodes {
			elem, err := a.convertToDomainElement(node)
			if err != nil {
				a.logger.Warn("Failed to convert menubar element", zap.Error(err))
				continue
			}
			if a.matchesFilter(elem, filter) {
				elements = append(elements, elem)
			}
		}
		a.logger.Debug("Included menubar elements", zap.Int("count", len(mbNodes)))
	}

	// Get additional menubar targets
	for _, bundleID := range filter.AdditionalMenubarTargets {
		additionalNodes, err := infra.GetClickableElementsFromBundleID(bundleID)
		if err != nil {
			a.logger.Warn("Failed to get additional menubar elements",
				zap.String("bundle_id", bundleID),
				zap.Error(err))
			continue
		}
		for _, node := range additionalNodes {
			elem, err := a.convertToDomainElement(node)
			if err != nil {
				a.logger.Warn("Failed to convert additional menubar element", zap.Error(err))
				continue
			}
			if a.matchesFilter(elem, filter) {
				elements = append(elements, elem)
			}
		}
		a.logger.Debug("Included additional menubar elements",
			zap.String("bundle_id", bundleID),
			zap.Int("count", len(additionalNodes)))
	}

	return elements
}

// addDockElements adds dock clickable elements.
func (a *Adapter) addDockElements(
	ctx context.Context,
	elements []*element.Element,
) []*element.Element {
	const dockBundleID = "com.apple.dock"

	// Temporarily add AXDockItem to clickable roles
	originalRoles := infra.GetClickableRoles()
	dockRoles := append(originalRoles, "AXDockItem")
	infra.SetClickableRoles(dockRoles)
	defer infra.SetClickableRoles(originalRoles) // Restore original roles when done

	// Get dock application
	dockApp := infra.GetApplicationByBundleID(dockBundleID)
	if dockApp == nil {
		a.logger.Debug("Dock application not found")
		return elements
	}
	defer dockApp.Release()

	// Validate we got the correct application element (not a stale menu item)
	appInfo, err := dockApp.GetInfo()
	if err != nil {
		a.logger.Warn("Failed to get dock application info", zap.Error(err))
		return elements
	}

	if appInfo.Role != "AXApplication" {
		a.logger.Warn("Got incorrect element for dock, expected AXApplication",
			zap.String("actual_role", appInfo.Role),
			zap.String("title", appInfo.Title))
		return elements
	}

	// Build tree and find clickable elements
	opts := infra.DefaultTreeOptions()
	opts.IncludeOutOfBounds = true

	tree, err := infra.BuildTree(dockApp, opts)
	if err != nil {
		a.logger.Warn("Failed to build tree for dock", zap.Error(err))
		return elements
	}

	if tree == nil {
		a.logger.Debug("No tree built for dock")
		return elements
	}

	dockNodes := tree.FindClickableElements()

	for _, node := range dockNodes {
		elem, err := a.convertToDomainElement(node)
		if err != nil {
			a.logger.Warn("Failed to convert dock element", zap.Error(err))
			continue
		}
		elements = append(elements, elem)
	}

	a.logger.Debug("Included dock elements", zap.Int("count", len(dockNodes)))
	return elements
}

// addNotificationCenterElements adds notification center clickable elements.
func (a *Adapter) addNotificationCenterElements(
	ctx context.Context,
	elements []*element.Element,
) []*element.Element {
	const ncBundleID = "com.apple.notificationcenterui"

	a.logger.Debug("Adding notification center elements")

	ncNodes, err := infra.GetClickableElementsFromBundleID(ncBundleID)
	if err != nil {
		a.logger.Warn("Failed to get notification center elements", zap.Error(err))
		return elements
	}

	for _, node := range ncNodes {
		elem, err := a.convertToDomainElement(node)
		if err != nil {
			a.logger.Warn("Failed to convert notification center element", zap.Error(err))
			continue
		}
		elements = append(elements, elem)
	}

	a.logger.Debug("Included notification center elements", zap.Int("count", len(ncNodes)))
	return elements
}
