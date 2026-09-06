package config

import (
	"math/bits"
	"strings"
)

// HotkeysForModeAndApp returns the effective per-mode hotkeys map for the given mode
// and focused app bundle ID. For modes without app-specific overrides, it returns the
// base mode hotkeys unchanged. Hints, Grid, RecursiveGrid, and Scroll modes support
// per-app hotkey overrides through [[<mode>.app_configs]].
//
// It is the merge itself. Everything that dispatches a key reads a settled
// Keymap instead (ResolveKeymap, keymap.go); what is left here is validation,
// which checks a table rather than matching against one.
func (c *Config) HotkeysForModeAndApp(
	modeName, bundleID string,
) map[string]StringOrStringArray {
	base := c.baseHotkeysForMode(modeName)
	if bundleID == "" {
		return base
	}

	var appConfig *AppConfig
	switch modeName {
	case ModeNameHints:
		appConfig = c.Hints.AppConfigForBundleID(bundleID)
	case ModeNameGrid:
		appConfig = c.Grid.AppConfigForBundleID(bundleID)
	case ModeNameRecursiveGrid:
		appConfig = c.RecursiveGrid.AppConfigForBundleID(bundleID)
	case ModeNameScroll:
		appConfig = c.Scroll.AppConfigForBundleID(bundleID)
	case ModeNameMonitorSelect:
		return base
	default:
		appConfig = c.Modes[modeName].AppConfigForBundleID(bundleID)
	}

	if appConfig == nil || len(appConfig.Hotkeys) == 0 {
		return base
	}

	baseLen := len(base)
	appHotkeysLen := len(appConfig.Hotkeys)

	sum, carry := bits.Add(uint(baseLen), uint(appHotkeysLen), 0)
	maxInt := ^uint(0) >> 1

	size := baseLen
	if carry == 0 && sum <= maxInt {
		size = int(sum)
	}

	merged := make(map[string]StringOrStringArray, size)
	for key, actions := range base {
		copied := make(StringOrStringArray, len(actions))
		copy(copied, actions)
		merged[key] = copied
	}

	for key, actions := range appConfig.Hotkeys {
		canonicalKey := FindNormalizedMapKey(merged, key)
		if len(actions) == 1 && actions[0] == DisabledSentinel {
			delete(merged, canonicalKey)

			continue
		}

		delete(merged, canonicalKey)

		copied := make(StringOrStringArray, len(actions))
		copy(copied, actions)
		merged[key] = copied
	}

	return merged
}

// GlobalHotkeysForApp returns the effective global hotkey bindings for the given
// focused app bundle ID. When the bundle ID matches an entry in [[app_configs]],
// the app-specific hotkeys are merged on top of the base [hotkeys] bindings.
// The __disabled__ sentinel removes a base binding.
// Returns the base bindings unchanged when bundleID is empty or no matching
// app config has hotkey overrides.
func (c *Config) GlobalHotkeysForApp(bundleID string) map[string][]string {
	base := c.Hotkeys.Bindings
	if bundleID == "" || !c.HasGlobalAppHotkeyOverrides() {
		return base
	}

	lowerBundleID := strings.ToLower(strings.TrimSpace(bundleID))
	for idx := range c.AppConfigs {
		if strings.ToLower(strings.TrimSpace(c.AppConfigs[idx].BundleID)) == lowerBundleID {
			appConfig := &c.AppConfigs[idx]
			if len(appConfig.Hotkeys) == 0 {
				return base
			}

			merged := make(map[string][]string, len(base))
			for key, actions := range base {
				copied := make([]string, len(actions))
				copy(copied, actions)
				merged[key] = copied
			}

			for key, sosa := range appConfig.Hotkeys {
				canonicalKey := FindNormalizedMapKey(merged, key)
				if len(sosa) == 1 && sosa[0] == DisabledSentinel {
					delete(merged, canonicalKey)

					continue
				}

				delete(merged, canonicalKey)
				merged[key] = []string(sosa)
			}

			return merged
		}
	}

	return base
}

// HasGlobalAppHotkeyOverrides reports whether any [[app_configs]] entry
// has a non-empty Hotkeys map. Callers can use this to skip expensive
// operations (e.g. accessibility API calls) when no per-app hotkey overrides
// are configured.
func (c *Config) HasGlobalAppHotkeyOverrides() bool {
	for idx := range c.AppConfigs {
		if len(c.AppConfigs[idx].Hotkeys) > 0 {
			return true
		}
	}

	return false
}

// HasAppHotkeyOverrides reports whether any [[hints.app_configs]] entry has a
// non-empty Hotkeys map. Callers can use this to skip expensive operations
// (e.g. accessibility API calls) when no per-app hotkey overrides are configured.
func (c *HintsConfig) HasAppHotkeyOverrides() bool {
	for idx := range c.AppConfigs {
		if len(c.AppConfigs[idx].Hotkeys) > 0 {
			return true
		}
	}

	return false
}

// HasAppHotkeyOverrides reports whether any [[grid.app_configs]] entry has a
// non-empty Hotkeys map.
func (c *GridConfig) HasAppHotkeyOverrides() bool {
	for idx := range c.AppConfigs {
		if len(c.AppConfigs[idx].Hotkeys) > 0 {
			return true
		}
	}

	return false
}

// HasAppHotkeyOverrides reports whether any [[recursive_grid.app_configs]] entry has a
// non-empty Hotkeys map.
func (c *RecursiveGridConfig) HasAppHotkeyOverrides() bool {
	for idx := range c.AppConfigs {
		if len(c.AppConfigs[idx].Hotkeys) > 0 {
			return true
		}
	}

	return false
}

// HasAppHotkeyOverrides reports whether any [[scroll.app_configs]] entry has a
// non-empty Hotkeys map.
func (c *ScrollConfig) HasAppHotkeyOverrides() bool {
	for idx := range c.AppConfigs {
		if len(c.AppConfigs[idx].Hotkeys) > 0 {
			return true
		}
	}

	return false
}

// CustomModeIndicators returns the indicator label of every declared mode, by
// name, leaving out the ones declared with none: an absent name is what tells
// the overlay to draw nothing.
func (c *Config) CustomModeIndicators() map[string]string {
	labels := make(map[string]string, len(c.Modes))

	for name, mode := range c.Modes {
		if mode.Indicator != "" {
			labels[name] = mode.Indicator
		}
	}

	return labels
}

// HasAppHotkeyOverrides reports whether any [[modes.<name>.app_configs]] entry
// has a non-empty Hotkeys map. While it is false the focused app cannot change
// what the mode binds, which is what lets the keymap settle without asking
// the platform which app is focused.
func (m CustomModeConfig) HasAppHotkeyOverrides() bool {
	for idx := range m.AppConfigs {
		if len(m.AppConfigs[idx].Hotkeys) > 0 {
			return true
		}
	}

	return false
}

// AppConfigForBundleID returns the matching app config of a declared mode for
// the given bundle ID, nil when none matches. Bundle ID matching is
// case-insensitive after trimming whitespace, as it is for every mode.
func (m CustomModeConfig) AppConfigForBundleID(bundleID string) *AppConfig {
	lowerBundleID := strings.ToLower(strings.TrimSpace(bundleID))

	for idx := range m.AppConfigs {
		if strings.ToLower(strings.TrimSpace(m.AppConfigs[idx].BundleID)) == lowerBundleID {
			return &m.AppConfigs[idx]
		}
	}

	return nil
}

// AppConfigForBundleID returns the matching hints app config for the given bundle ID.
// Bundle ID matching is case-insensitive (after trimming whitespace).
func (c *HintsConfig) AppConfigForBundleID(bundleID string) *AppConfig {
	lowerBundleID := strings.ToLower(strings.TrimSpace(bundleID))

	for idx := range c.AppConfigs {
		if strings.ToLower(strings.TrimSpace(c.AppConfigs[idx].BundleID)) == lowerBundleID {
			return &c.AppConfigs[idx]
		}
	}

	return nil
}

// AppConfigForBundleID returns the matching grid app config for the given bundle ID.
// Bundle ID matching is case-insensitive (after trimming whitespace).
func (c *GridConfig) AppConfigForBundleID(bundleID string) *AppConfig {
	lowerBundleID := strings.ToLower(strings.TrimSpace(bundleID))

	for idx := range c.AppConfigs {
		if strings.ToLower(strings.TrimSpace(c.AppConfigs[idx].BundleID)) == lowerBundleID {
			return &c.AppConfigs[idx]
		}
	}

	return nil
}

// AppConfigForBundleID returns the matching recursive grid app config for the given bundle ID.
// Bundle ID matching is case-insensitive (after trimming whitespace).
func (c *RecursiveGridConfig) AppConfigForBundleID(bundleID string) *AppConfig {
	lowerBundleID := strings.ToLower(strings.TrimSpace(bundleID))

	for idx := range c.AppConfigs {
		if strings.ToLower(strings.TrimSpace(c.AppConfigs[idx].BundleID)) == lowerBundleID {
			return &c.AppConfigs[idx]
		}
	}

	return nil
}

// AppConfigForBundleID returns the matching scroll app config for the given bundle ID.
// Bundle ID matching is case-insensitive (after trimming whitespace).
func (c *ScrollConfig) AppConfigForBundleID(bundleID string) *AppConfig {
	lowerBundleID := strings.ToLower(strings.TrimSpace(bundleID))

	for idx := range c.AppConfigs {
		if strings.ToLower(strings.TrimSpace(c.AppConfigs[idx].BundleID)) == lowerBundleID {
			return &c.AppConfigs[idx]
		}
	}

	return nil
}
