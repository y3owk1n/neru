package config

import (
	"math/bits"
	"strings"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// IsAppExcluded checks if the given bundle ID is in the excluded apps list.
func (c *Config) IsAppExcluded(bundleID string) bool {
	if bundleID == "" {
		return false
	}

	// Normalize bundle ID for case-insensitive comparison
	bundleID = strings.ToLower(strings.TrimSpace(bundleID))

	for _, excludedApp := range c.General.ExcludedApps {
		excludedApp = strings.ToLower(strings.TrimSpace(excludedApp))
		if excludedApp == bundleID {
			return true
		}
	}

	return false
}

// MergedForApp returns a copy of the HintsConfig with app-specific overrides
// merged on top. Only fields explicitly set in the [[hints.app_configs]] entry
// override the base; unset fields inherit from the root [hints] config.
// Hotkeys are handled separately via HotkeysForModeAndApp since they require
// key-level merge semantics (__disabled__ sentinel, etc.).
func (c *HintsConfig) MergedForApp(bundleID string) HintsConfig {
	appConfig := c.AppConfigForBundleID(bundleID)

	merged := *c // shallow copy

	if appConfig == nil {
		// No app config: still filter empty roles from the base.
		if hasEmptyRoles(merged.ClickableRoles) {
			merged.ClickableRoles = mergeClickableRoles(merged.ClickableRoles, nil)
		}

		return merged
	}

	if appConfig.Strategy != "" {
		merged.Strategy = appConfig.Strategy
	}

	if appConfig.CaptureScope != "" {
		merged.CaptureScope = appConfig.CaptureScope
	}

	if appConfig.LabelDirection != "" {
		merged.LabelDirection = appConfig.LabelDirection
	}

	if appConfig.IgnoreClickableCheck != nil {
		merged.IgnoreClickableCheck = *appConfig.IgnoreClickableCheck
	}

	if appConfig.VisibleCheckEnabled != nil {
		merged.VisibleCheckEnabled = *appConfig.VisibleCheckEnabled
	}

	// Always rebuild ClickableRoles with filtering + deduplication so that
	// empty and whitespace-only entries from the base config are removed,
	// and additional roles from the app config are merged without duplicates.
	merged.ClickableRoles = mergeClickableRoles(
		merged.ClickableRoles,
		appConfig.AdditionalClickable,
	)

	return merged
}

// ClickableRolesForApp returns the clickable roles for a specific app bundle ID.
func (c *Config) ClickableRolesForApp(bundleID string) []string {
	return c.Hints.ClickableRolesForApp(bundleID)
}

// ShouldIgnoreClickableCheckForApp returns whether clickable check should be
// ignored for a specific app bundle ID. Delegates to MergedForApp to handle
// the root→app-config override chain.
func (c *Config) ShouldIgnoreClickableCheckForApp(bundleID string) bool {
	return c.Hints.MergedForApp(bundleID).IgnoreClickableCheck
}

// ShouldEnableVisibleCheckForApp returns whether the visibility hit-test check
// should be performed for a specific app bundle ID. Delegates to MergedForApp
// to handle the root→app-config override chain.
func (c *Config) ShouldEnableVisibleCheckForApp(bundleID string) bool {
	return c.Hints.MergedForApp(bundleID).VisibleCheckEnabled
}

// hasEmptyRoles reports whether the slice contains any empty or
// whitespace-only entries. Used by MergedForApp to decide whether to
// rebuild ClickableRoles for filtering.
func hasEmptyRoles(roles []string) bool {
	for _, role := range roles {
		if strings.TrimSpace(role) == "" {
			return true
		}
	}

	return false
}

// mergeClickableRoles combines base roles with additional roles, deduplicating
// and filtering out empty/whitespace-only entries. Base roles are preserved in
// their original order (minus empties), then additional roles are appended.
func mergeClickableRoles(base, additional []string) []string {
	baseLen := len(base)
	additionalLen := len(additional)

	sum, carry := bits.Add(uint(baseLen), uint(additionalLen), 0)
	maxInt := ^uint(0) >> 1

	size := baseLen
	if carry == 0 && sum <= maxInt {
		size = int(sum)
	}

	seen := make(map[string]struct{}, size)
	result := make([]string, 0, size)

	for _, role := range base {
		trimmed := strings.TrimSpace(role)
		if trimmed == "" {
			continue
		}

		if _, exists := seen[trimmed]; !exists {
			seen[trimmed] = struct{}{}
			result = append(result, trimmed)
		}
	}

	for _, role := range additional {
		trimmed := strings.TrimSpace(role)
		if trimmed == "" {
			continue
		}

		if _, exists := seen[trimmed]; !exists {
			seen[trimmed] = struct{}{}
			result = append(result, trimmed)
		}
	}

	return result
}

// ClickableRolesForApp returns the native accessibility role names to hint for
// a specific app bundle ID. Starts from the merged config (root + app
// overrides), appends the menubar / dock roles when the corresponding hints
// flags are enabled, then resolves the whole list against the running
// platform's accessibility vocabulary.
func (c *HintsConfig) ClickableRolesForApp(bundleID string) []string {
	merged := c.MergedForApp(bundleID)

	// Append menubar/dock roles on top of the merged roles.
	if merged.IncludeMenubarHints {
		merged.ClickableRoles = append(merged.ClickableRoles, RoleMenuBarItem)
	}

	if merged.IncludeDockHints {
		merged.ClickableRoles = append(merged.ClickableRoles, RoleDockItem)
	}

	return element.ResolveRolesForCurrentPlatform(merged.ClickableRoles).Native
}

// ResolvedClickableRoles returns the native accessibility role names for the
// root hints config, without app-specific overrides.
func (c *HintsConfig) ResolvedClickableRoles() []string {
	return element.ResolveRolesForCurrentPlatform(c.ClickableRoles).Native
}

// StrategyForApp returns the element detection strategy for the given bundle ID.
// Delegates to MergedForApp to handle the root→app-config override chain.
func (c *HintsConfig) StrategyForApp(bundleID string) string {
	return c.MergedForApp(bundleID).Strategy
}

// CaptureScopeForApp returns the region the capture strategies scan for the
// given bundle ID, following the same root to app-config override chain.
func (c *HintsConfig) CaptureScopeForApp(bundleID string) string {
	return c.MergedForApp(bundleID).CaptureScope
}
