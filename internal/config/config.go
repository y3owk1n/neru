package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// Accessibility role constants.
const (
	RoleMenuBarItem = "AXMenuBarItem"
	RoleDockItem    = "AXDockItem"
)

// ActionConfig defines the visual and behavioral settings for action mode.
type ActionConfig struct {
	HighlightColor string `json:"highlightColor" toml:"highlight_color"`
	HighlightWidth int    `json:"highlightWidth" toml:"highlight_width"`

	LeftClickKey   string `json:"leftClickKey"   toml:"left_click_key"`
	RightClickKey  string `json:"rightClickKey"  toml:"right_click_key"`
	MiddleClickKey string `json:"middleClickKey" toml:"middle_click_key"`
	MouseDownKey   string `json:"mouseDownKey"   toml:"mouse_down_key"`
	MouseUpKey     string `json:"mouseUpKey"     toml:"mouse_up_key"`
}

// Config represents the complete application configuration structure.
type Config struct {
	General      GeneralConfig      `json:"general"      toml:"general"`
	Hotkeys      HotkeysConfig      `json:"hotkeys"      toml:"hotkeys"`
	Hints        HintsConfig        `json:"hints"        toml:"hints"`
	Grid         GridConfig         `json:"grid"         toml:"grid"`
	Scroll       ScrollConfig       `json:"scroll"       toml:"scroll"`
	Action       ActionConfig       `json:"action"       toml:"action"`
	Logging      LoggingConfig      `json:"logging"      toml:"logging"`
	SmoothCursor SmoothCursorConfig `json:"smoothCursor" toml:"smooth_cursor"`
	Metrics      MetricsConfig      `json:"metrics"      toml:"metrics"`
}

// GeneralConfig defines general application-wide settings.
type GeneralConfig struct {
	ExcludedApps              []string `json:"excludedApps"              toml:"excluded_apps"`
	AccessibilityCheckOnStart bool     `json:"accessibilityCheckOnStart" toml:"accessibility_check_on_start"`
	RestoreCursorPosition     bool     `json:"restoreCursorPosition"     toml:"restore_cursor_position"`
}

// AppConfig defines application-specific settings for role customization.
type AppConfig struct {
	BundleID             string   `json:"bundleId"             toml:"bundle_id"`
	AdditionalClickable  []string `json:"additionalClickable"  toml:"additional_clickable_roles"`
	IgnoreClickableCheck bool     `json:"ignoreClickableCheck" toml:"ignore_clickable_check"`
}

// HotkeysConfig defines hotkey mappings and their associated actions.
type HotkeysConfig struct {
	// Bindings holds hotkey -> action mappings parsed from the [hotkeys] table.
	// Supported TOML format (preferred):
	// [hotkeys]
	// "Cmd+Shift+Space" = "hints"
	// Values are strings. The special exec prefix is supported: "exec /usr/bin/say hi"
	Bindings map[string]string `json:"bindings" toml:"bindings"`
}

// ScrollConfig defines the behavior and appearance settings for scroll mode.
type ScrollConfig struct {
	ScrollStep          int                 `json:"scrollStep"          toml:"scroll_step"`
	ScrollStepHalf      int                 `json:"scrollStepHalf"      toml:"scroll_step_half"`
	ScrollStepFull      int                 `json:"scrollStepFull"      toml:"scroll_step_full"`
	HighlightScrollArea bool                `json:"highlightScrollArea" toml:"highlight_scroll_area"`
	HighlightColor      string              `json:"highlightColor"      toml:"highlight_color"`
	HighlightWidth      int                 `json:"highlightWidth"      toml:"highlight_width"`
	KeyBindings         map[string][]string `json:"keyBindings"         toml:"key_bindings"`
}

// HintsConfig defines the visual and behavioral settings for hints mode.
type HintsConfig struct {
	Enabled        bool    `json:"enabled"        toml:"enabled"`
	HintCharacters string  `json:"hintCharacters" toml:"hint_characters"`
	FontSize       int     `json:"fontSize"       toml:"font_size"`
	FontFamily     string  `json:"fontFamily"     toml:"font_family"`
	BorderRadius   int     `json:"borderRadius"   toml:"border_radius"`
	Padding        int     `json:"padding"        toml:"padding"`
	BorderWidth    int     `json:"borderWidth"    toml:"border_width"`
	Opacity        float64 `json:"opacity"        toml:"opacity"`

	BackgroundColor  string `json:"backgroundColor"  toml:"background_color"`
	TextColor        string `json:"textColor"        toml:"text_color"`
	MatchedTextColor string `json:"matchedTextColor" toml:"matched_text_color"`
	BorderColor      string `json:"borderColor"      toml:"border_color"`

	IncludeMenubarHints           bool     `json:"includeMenubarHints"           toml:"include_menubar_hints"`
	AdditionalMenubarHintsTargets []string `json:"additionalMenubarHintsTargets" toml:"additional_menubar_hints_targets"`
	IncludeDockHints              bool     `json:"includeDockHints"              toml:"include_dock_hints"`
	IncludeNCHints                bool     `json:"includeNcHints"                toml:"include_nc_hints"`

	ClickableRoles       []string `json:"clickableRoles"       toml:"clickable_roles"`
	IgnoreClickableCheck bool     `json:"ignoreClickableCheck" toml:"ignore_clickable_check"`

	AppConfigs []AppConfig `json:"appConfigs" toml:"app_configs"`

	AdditionalAXSupport AdditionalAXSupport `json:"additionalAxSupport" toml:"additional_ax_support"`
}

// GridConfig defines the visual and behavioral settings for grid mode.
type GridConfig struct {
	Enabled bool `json:"enabled" toml:"enabled"`

	Characters   string `json:"characters"   toml:"characters"`
	SublayerKeys string `json:"sublayerKeys" toml:"sublayer_keys"`

	// Optional custom labels for rows and columns
	// If not provided, labels will be inferred from characters
	RowLabels string `json:"rowLabels" toml:"row_labels"`
	ColLabels string `json:"colLabels" toml:"col_labels"`

	FontSize    int     `json:"fontSize"    toml:"font_size"`
	FontFamily  string  `json:"fontFamily"  toml:"font_family"`
	Opacity     float64 `json:"opacity"     toml:"opacity"`
	BorderWidth int     `json:"borderWidth" toml:"border_width"`

	BackgroundColor        string `json:"backgroundColor"        toml:"background_color"`
	TextColor              string `json:"textColor"              toml:"text_color"`
	MatchedTextColor       string `json:"matchedTextColor"       toml:"matched_text_color"`
	MatchedBackgroundColor string `json:"matchedBackgroundColor" toml:"matched_background_color"`
	MatchedBorderColor     string `json:"matchedBorderColor"     toml:"matched_border_color"`
	BorderColor            string `json:"borderColor"            toml:"border_color"`

	LiveMatchUpdate bool `json:"liveMatchUpdate" toml:"live_match_update"`
	HideUnmatched   bool `json:"hideUnmatched"   toml:"hide_unmatched"`
	PrewarmEnabled  bool `json:"prewarmEnabled"  toml:"prewarm_enabled"`
	EnableGC        bool `json:"enableGc"        toml:"enable_gc"`
}

// LoggingConfig defines the logging behavior and file management settings.
type LoggingConfig struct {
	LogLevel          string `json:"logLevel"          toml:"log_level"`
	LogFile           string `json:"logFile"           toml:"log_file"`
	StructuredLogging bool   `json:"structuredLogging" toml:"structured_logging"`

	// New options for log rotation and file logging control
	DisableFileLogging bool `json:"disableFileLogging" toml:"disable_file_logging"`
	MaxFileSize        int  `json:"maxFileSize"        toml:"max_file_size"` // Size in MB
	MaxBackups         int  `json:"maxBackups"         toml:"max_backups"`   // Maximum number of old log files to retain
	MaxAge             int  `json:"maxAge"             toml:"max_age"`       // Maximum number of days to retain old log files
}

// SmoothCursorConfig defines the smooth cursor movement settings.
type SmoothCursorConfig struct {
	MoveMouseEnabled bool `json:"moveMouseEnabled" toml:"move_mouse_enabled"`
	Steps            int  `json:"steps"            toml:"steps"`
	Delay            int  `json:"delay"            toml:"delay"` // Delay in milliseconds
}

// AdditionalAXSupport defines accessibility support for specific application frameworks.
type AdditionalAXSupport struct {
	Enable                    bool     `json:"enable"                    toml:"enable"`
	AdditionalElectronBundles []string `json:"additionalElectronBundles" toml:"additional_electron_bundles"`
	AdditionalChromiumBundles []string `json:"additionalChromiumBundles" toml:"additional_chromium_bundles"`
	AdditionalFirefoxBundles  []string `json:"additionalFirefoxBundles"  toml:"additional_firefox_bundles"`
}

// MetricsConfig defines metrics collection settings.
type MetricsConfig struct {
	Enabled bool `json:"enabled" toml:"enabled"`
}

// LoadResult contains the result of loading a configuration file.
type LoadResult struct {
	Config          *Config
	ValidationError error
	ConfigPath      string
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c == nil {
		return derrors.New(derrors.CodeInvalidConfig, "configuration cannot be nil")
	}

	err := c.ValidateModes()
	if err != nil {
		return err
	}

	// Validate hints configuration
	err = c.ValidateHints()
	if err != nil {
		return err
	}

	err = c.ValidateLogging()
	if err != nil {
		return err
	}

	err = c.ValidateScroll()
	if err != nil {
		return err
	}

	// Validate app configs
	err = c.ValidateAppConfigs()
	if err != nil {
		return err
	}

	// Validate grid settings
	err = c.ValidateGrid()
	if err != nil {
		return err
	}

	// Validate action settings
	err = c.ValidateAction()
	if err != nil {
		return err
	}

	// Validate smooth cursor settings
	err = c.ValidateSmoothCursor()
	if err != nil {
		return err
	}

	return nil
}

// Save saves the configuration to the specified path.
func (c *Config) Save(path string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)

	mkdirErr := os.MkdirAll(dir, DefaultDirPerms)
	if mkdirErr != nil {
		return derrors.Wrap(
			mkdirErr,
			derrors.CodeConfigIOFailed,
			"failed to create config directory",
		)
	}

	// Create file
	var closeErr error
	// #nosec G304 -- Path is validated and controlled by the application
	file, fileErr := os.Create(path)
	if fileErr != nil {
		return derrors.Wrap(fileErr, derrors.CodeConfigIOFailed, "failed to create config file")
	}

	defer func() {
		cerr := file.Close()
		if cerr != nil && closeErr == nil {
			closeErr = derrors.Wrap(cerr, derrors.CodeConfigIOFailed, "failed to close config file")
		}
	}()

	// Encode to TOML
	encoder := toml.NewEncoder(file)

	encodeErr := encoder.Encode(c)
	if encodeErr != nil {
		return derrors.Wrap(encodeErr, derrors.CodeSerializationFailed, "failed to encode config")
	}

	return closeErr
}

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

// ClickableRolesForApp returns the clickable roles for a specific app bundle ID.
func (c *Config) ClickableRolesForApp(bundleID string) []string {
	rolesMap := c.buildRolesMap(bundleID)

	return rolesMapToSlice(rolesMap)
}

// ShouldIgnoreClickableCheckForApp returns whether clickable check should be ignored for a specific app bundle ID.
// It first checks for app-specific configuration, then falls back to the global setting.
func (c *Config) ShouldIgnoreClickableCheckForApp(bundleID string) bool {
	// Check if the app has an app-specific ignore_clickable_check
	if c.Hints.AppConfigs != nil {
		if len(c.Hints.AppConfigs) > 0 {
			for _, appConfig := range c.Hints.AppConfigs {
				if appConfig.BundleID == bundleID {
					return appConfig.IgnoreClickableCheck
				}
			}
		}
	}

	// Fall back to global ignore_clickable_check
	return c.Hints.IgnoreClickableCheck
}

// rolesMapToSlice converts a roles map to a slice.
func rolesMapToSlice(rolesMap map[string]struct{}) []string {
	roles := make([]string, 0, len(rolesMap))
	for role := range rolesMap {
		roles = append(roles, role)
	}

	return roles
}

// ValidateModes validates that at least one mode is enabled.
func (c *Config) ValidateModes() error {
	if !c.Hints.Enabled && !c.Grid.Enabled {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"at least one mode must be enabled: hints.enabled or grid.enabled",
		)
	}

	return nil
}

// ValidateLogging validates the logging configuration.
func (c *Config) ValidateLogging() error {
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.Logging.LogLevel] {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"log_level must be one of: debug, info, warn, error",
		)
	}

	return nil
}

// validateMinValue validates that a value is at least the minimum.
func validateMinValue(value int, minimum int, fieldName string) error {
	if value < minimum {
		return derrors.New(
			derrors.CodeInvalidConfig,
			fieldName+" must be at least "+strconv.Itoa(minimum),
		)
	}

	return nil
}

// ValidateScroll validates the scroll configuration.
func (c *Config) ValidateScroll() error {
	err := validateMinValue(c.Scroll.ScrollStep, 1, "scroll.scroll_step")
	if err != nil {
		return err
	}

	err = validateMinValue(c.Scroll.ScrollStepHalf, 1, "scroll.scroll_step_half")
	if err != nil {
		return err
	}

	err = validateMinValue(c.Scroll.ScrollStepFull, 1, "scroll.scroll_step_full")
	if err != nil {
		return err
	}

	err = c.ValidateScrollKeyBindings()
	if err != nil {
		return err
	}

	return nil
}

// ValidateScrollKeyBindings validates the scroll keybindings configuration.
func (c *Config) ValidateScrollKeyBindings() error {
	if len(c.Scroll.KeyBindings) == 0 {
		return nil
	}

	validActions := map[string]bool{
		"scroll_up":    true,
		"scroll_down":  true,
		"scroll_left":  true,
		"scroll_right": true,
		"go_top":       true,
		"go_bottom":    true,
		"page_up":      true,
		"page_down":    true,
	}

	for action, keys := range c.Scroll.KeyBindings {
		if !validActions[action] {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"scroll.key_bindings has unknown action '%s' (valid: scroll_up, scroll_down, scroll_left, scroll_right, go_top, go_bottom, page_up, page_down)",
				action,
			)
		}

		if len(keys) == 0 {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"scroll.key_bindings['%s'] has no keys specified",
				action,
			)
		}

		for _, key := range keys {
			err := validateScrollKey(key, fmt.Sprintf("scroll.key_bindings['%s']", action))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// buildRolesMap builds a map of clickable roles for the given bundle ID.
func (c *Config) buildRolesMap(bundleID string) map[string]struct{} {
	rolesMap := make(map[string]struct{})

	for _, role := range c.Hints.ClickableRoles {
		trimmed := strings.TrimSpace(role)
		if trimmed != "" {
			rolesMap[trimmed] = struct{}{}
		}
	}

	for _, appConfig := range c.Hints.AppConfigs {
		if appConfig.BundleID == bundleID {
			for _, role := range appConfig.AdditionalClickable {
				trimmed := strings.TrimSpace(role)
				if trimmed != "" {
					rolesMap[trimmed] = struct{}{}
				}
			}

			break
		}
	}

	if c.Hints.IncludeMenubarHints {
		rolesMap[RoleMenuBarItem] = struct{}{}
	}

	if c.Hints.IncludeDockHints {
		rolesMap[RoleDockItem] = struct{}{}
	}

	return rolesMap
}

// validateScrollKey validates a single scroll key binding.
func validateScrollKey(key, fieldName string) error {
	if strings.TrimSpace(key) == "" {
		return derrors.Newf(derrors.CodeInvalidConfig, "%s has empty key", fieldName)
	}

	switch {
	case strings.Contains(key, "+"):
		parts := strings.Split(key, "+")
		validModifiers := map[string]bool{
			"Cmd":    true,
			"Ctrl":   true,
			"Alt":    true,
			"Shift":  true,
			"Option": true,
		}

		for i := range len(parts) - 1 {
			modifier := strings.TrimSpace(parts[i])
			if !validModifiers[modifier] {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s has invalid modifier '%s' in '%s' (valid: Cmd, Ctrl, Alt, Shift, Option)",
					fieldName,
					modifier,
					key,
				)
			}
		}

		lastPart := strings.TrimSpace(parts[len(parts)-1])
		if lastPart == "" {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s has empty key in '%s'",
				fieldName,
				key,
			)
		}

		if !isValidScrollKeyName(lastPart) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s has invalid key name '%s' in '%s'",
				fieldName,
				lastPart,
				key,
			)
		}
	case len(key) == 2 && IsAllLetters(key):
		for _, r := range key {
			if r < 'a' || r > 'z' {
				if r < 'A' || r > 'Z' {
					return derrors.Newf(
						derrors.CodeInvalidConfig,
						"%s has invalid sequence key '%s' - sequences must be 2 letters (a-z, A-Z)",
						fieldName,
						key,
					)
				}
			}
		}
	default:
		if len(key) == 1 && key[0] <= 31 {
			return nil
		}

		if !isValidScrollKeyName(key) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s has invalid key name '%s'",
				fieldName,
				key,
			)
		}
	}

	return nil
}

// isValidScrollKeyName checks if a key name is valid for scroll keybindings.
// This validates the base key part (after modifier splitting in validateScrollKey).
func isValidScrollKeyName(key string) bool {
	validKeys := map[string]bool{
		"Space":     true,
		"Return":    true,
		"Enter":     true,
		"Escape":    true,
		"Tab":       true,
		"Delete":    true,
		"Backspace": true,
		"Home":      true,
		"End":       true,
		"PageUp":    true,
		"PageDown":  true,
		"Up":        true,
		"Down":      true,
		"Left":      true,
		"Right":     true,
		"F1":        true,
		"F2":        true,
		"F3":        true,
		"F4":        true,
		"F5":        true,
		"F6":        true,
		"F7":        true,
		"F8":        true,
		"F9":        true,
		"F10":       true,
		"F11":       true,
		"F12":       true,
	}

	if validKeys[key] {
		return true
	}

	if len(key) == 1 {
		r := rune(key[0])

		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
	}

	return false
}

// IsAllLetters checks if a string contains only letters (a-z, A-Z).
func IsAllLetters(keyStr string) bool {
	for _, r := range keyStr {
		if r < 'a' || r > 'z' {
			if r < 'A' || r > 'Z' {
				return false
			}
		}
	}

	return len(keyStr) > 0
}
