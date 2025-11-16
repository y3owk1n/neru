// Package config provides configuration functionality for the Neru application.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/y3owk1n/neru/internal/logger"
	"go.uber.org/zap"
)

// ActionConfig represents the configuration for action mode.
type ActionConfig struct {
	HighlightColor string `toml:"highlight_color"`
	HighlightWidth int    `toml:"highlight_width"`

	// Action key mappings
	LeftClickKey   string `toml:"left_click_key"`
	RightClickKey  string `toml:"right_click_key"`
	MiddleClickKey string `toml:"middle_click_key"`
	MouseDownKey   string `toml:"mouse_down_key"`
	MouseUpKey     string `toml:"mouse_up_key"`
}

// Config represents the complete application configuration.
type Config struct {
	General GeneralConfig `toml:"general"`
	Hotkeys HotkeysConfig `toml:"hotkeys"`
	Hints   HintsConfig   `toml:"hints"`
	Grid    GridConfig    `toml:"grid"`
	Scroll  ScrollConfig  `toml:"scroll"`
	Action  ActionConfig  `toml:"action"`
	Logging LoggingConfig `toml:"logging"`
}

// GeneralConfig represents general application configuration.
type GeneralConfig struct {
	ExcludedApps              []string `toml:"excluded_apps"`
	AccessibilityCheckOnStart bool     `toml:"accessibility_check_on_start"` // Moved from AccessibilityConfig
}

// AppConfig represents application-specific configuration.
type AppConfig struct {
	BundleID             string   `toml:"bundle_id"`
	AdditionalClickable  []string `toml:"additional_clickable_roles"`
	IgnoreClickableCheck bool     `toml:"ignore_clickable_check"`
}

// HotkeysConfig represents hotkey configuration.
type HotkeysConfig struct {
	// Bindings holds hotkey -> action mappings parsed from the [hotkeys] table.
	// Supported TOML format (preferred):
	// [hotkeys]
	// "Cmd+Shift+Space" = "hints"
	// Values are strings. The special exec prefix is supported: "exec /usr/bin/say hi"
	Bindings map[string]string `toml:"bindings"`
}

// ScrollConfig represents scroll mode configuration.
type ScrollConfig struct {
	ScrollStep          int    `toml:"scroll_step"`
	ScrollStepHalf      int    `toml:"scroll_step_half"`
	ScrollStepFull      int    `toml:"scroll_step_full"`
	HighlightScrollArea bool   `toml:"highlight_scroll_area"`
	HighlightColor      string `toml:"highlight_color"`
	HighlightWidth      int    `toml:"highlight_width"`
}

// HintsConfig represents hints mode configuration.
type HintsConfig struct {
	// General configurations
	Enabled        bool    `toml:"enabled"`
	HintCharacters string  `toml:"hint_characters"`
	FontSize       int     `toml:"font_size"`
	FontFamily     string  `toml:"font_family"`
	BorderRadius   int     `toml:"border_radius"`
	Padding        int     `toml:"padding"`
	BorderWidth    int     `toml:"border_width"`
	Opacity        float64 `toml:"opacity"`

	BackgroundColor  string `toml:"background_color"`
	TextColor        string `toml:"text_color"`
	MatchedTextColor string `toml:"matched_text_color"`
	BorderColor      string `toml:"border_color"`

	// Addidiotanl hints
	IncludeMenubarHints           bool     `toml:"include_menubar_hints"`
	AdditionalMenubarHintsTargets []string `toml:"additional_menubar_hints_targets"`
	IncludeDockHints              bool     `toml:"include_dock_hints"`
	IncludeNCHints                bool     `toml:"include_nc_hints"`

	// Roles and clicks
	ClickableRoles       []string `toml:"clickable_roles"`
	IgnoreClickableCheck bool     `toml:"ignore_clickable_check"`

	// App specific configs for roles and clicks
	AppConfigs []AppConfig `toml:"app_configs"`

	// AX support
	AdditionalAXSupport AdditionalAXSupport `toml:"additional_ax_support"`
}

// GridConfig represents grid mode configuration.
type GridConfig struct {
	// General configurations
	Enabled        bool `toml:"enabled"`
	SubgridEnabled bool `toml:"subgrid_enabled"`

	// Keys and characters
	Characters   string `toml:"characters"`
	SublayerKeys string `toml:"sublayer_keys"`

	// Appearance
	FontSize    int     `toml:"font_size"`
	FontFamily  string  `toml:"font_family"`
	Opacity     float64 `toml:"opacity"`
	BorderWidth int     `toml:"border_width"`

	BackgroundColor        string `toml:"background_color"`
	TextColor              string `toml:"text_color"`
	MatchedTextColor       string `toml:"matched_text_color"`
	MatchedBackgroundColor string `toml:"matched_background_color"`
	MatchedBorderColor     string `toml:"matched_border_color"`
	BorderColor            string `toml:"border_color"`

	// Behavior
	LiveMatchUpdate bool `toml:"live_match_update"`
	HideUnmatched   bool `toml:"hide_unmatched"`
}

// LoggingConfig represents logging configuration.
type LoggingConfig struct {
	LogLevel          string `toml:"log_level"`
	LogFile           string `toml:"log_file"`
	StructuredLogging bool   `toml:"structured_logging"`

	// New options for log rotation and file logging control
	DisableFileLogging bool `toml:"disable_file_logging"`
	MaxFileSize        int  `toml:"max_file_size"` // Size in MB
	MaxBackups         int  `toml:"max_backups"`   // Maximum number of old log files to retain
	MaxAge             int  `toml:"max_age"`       // Maximum number of days to retain old log files
}

// AdditionalAXSupport represents additional accessibility support configuration.
type AdditionalAXSupport struct {
	Enable                    bool     `toml:"enable"`
	AdditionalElectronBundles []string `toml:"additional_electron_bundles"`
	AdditionalChromiumBundles []string `toml:"additional_chromium_bundles"`
	AdditionalFirefoxBundles  []string `toml:"additional_firefox_bundles"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		General: GeneralConfig{
			ExcludedApps:              []string{},
			AccessibilityCheckOnStart: true,
		},
		Hotkeys: HotkeysConfig{
			Bindings: map[string]string{
				"Cmd+Shift+Space": "hints",
				"Cmd+Shift+G":     "grid",
				"Cmd+Shift+S":     "action scroll",
			},
		},
		Hints: HintsConfig{
			Enabled:        true,
			HintCharacters: "asdfghjkl",
			FontSize:       12,
			FontFamily:     "SF Mono",
			BorderRadius:   4,
			Padding:        4,
			BorderWidth:    1,
			Opacity:        0.95,

			BackgroundColor:  "#FFD700",
			TextColor:        "#000000",
			MatchedTextColor: "#737373",
			BorderColor:      "#000000",

			IncludeMenubarHints: false,
			AdditionalMenubarHintsTargets: []string{
				"com.apple.TextInputMenuAgent",
				"com.apple.controlcenter",
				"com.apple.systemuiserver",
			},
			IncludeDockHints: false,
			IncludeNCHints:   false,

			ClickableRoles: []string{
				"AXButton",
				"AXComboBox",
				"AXCheckBox",
				"AXRadioButton",
				"AXLink",
				"AXPopUpButton",
				"AXTextField",
				"AXSlider",
				"AXTabButton",
				"AXSwitch",
				"AXDisclosureTriangle",
				"AXTextArea",
				"AXMenuButton",
				"AXMenuItem",
				"AXCell",
				"AXRow",
			},
			IgnoreClickableCheck: false,

			AppConfigs: []AppConfig{}, // Moved from AccessibilityConfig

			AdditionalAXSupport: AdditionalAXSupport{
				Enable:                    false,
				AdditionalElectronBundles: []string{},
				AdditionalChromiumBundles: []string{},
				AdditionalFirefoxBundles:  []string{},
			},
		},
		Grid: GridConfig{
			Enabled:        true,
			SubgridEnabled: true,

			Characters:   "abcdefghijklmnpqrstuvwxyz",
			SublayerKeys: "abcdefghijklmnpqrstuvwxyz",

			FontSize:    12,
			FontFamily:  "SF Mono",
			Opacity:     0.7,
			BorderWidth: 1,

			BackgroundColor:        "#abe9b3",
			TextColor:              "#000000",
			MatchedTextColor:       "#f8bd96",
			MatchedBackgroundColor: "#f8bd96",
			MatchedBorderColor:     "#f8bd96",
			BorderColor:            "#abe9b3",

			LiveMatchUpdate: true,
			HideUnmatched:   true,
		},
		Scroll: ScrollConfig{
			ScrollStep:          50,
			ScrollStepHalf:      500,
			ScrollStepFull:      1000000,
			HighlightScrollArea: true,
			HighlightColor:      "#FF0000",
			HighlightWidth:      2,
		},
		Action: ActionConfig{
			HighlightColor: "#00FF00",
			HighlightWidth: 3,

			// Default action key mappings
			LeftClickKey:   "l",
			RightClickKey:  "r",
			MiddleClickKey: "m",
			MouseDownKey:   "i",
			MouseUpKey:     "u",
		},
		Logging: LoggingConfig{
			LogLevel:           "info",
			LogFile:            "",
			StructuredLogging:  true,
			DisableFileLogging: false,
			MaxFileSize:        10, // 10MB
			MaxBackups:         5,  // Keep 5 old log files
			MaxAge:             30, // Keep log files for 30 days
		},
	}
}

// Load loads configuration from the specified path.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// If path is empty, try default locations
	if path == "" {
		path = FindConfigFile()
	}

	logger.Info("Loading config from", zap.String("path", path))

	// If config file doesn't exist, return default config
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		logger.Info("Config file not found, using default configuration")
		return cfg, nil
	}

	// Parse TOML file into the typed config
	_, err = toml.DecodeFile(path, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Decode the hotkeys table into a generic map and populate cfg.Hotkeys.Bindings.
	var raw map[string]map[string]any
	_, err = toml.DecodeFile(path, &raw)
	if err == nil {
		if hot, ok := raw["hotkeys"]; ok {
			// Clear default bindings and initialize with empty map when user provides hotkeys config
			if len(hot) > 0 {
				cfg.Hotkeys.Bindings = map[string]string{}
			}
			for key, value := range hot {
				// Only accept string values for actions
				str, ok := value.(string)
				if !ok {
					return nil, fmt.Errorf("hotkeys.%s must be a string action", key)
				}
				cfg.Hotkeys.Bindings[key] = str
			}
		}
	}
	// Validate configuration.
	err = cfg.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	logger.Info("Configuration loaded successfully")
	return cfg, nil
}

// FindConfigFile searches for config file in default locations.
func FindConfigFile() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Try ~/.config/neru/config.toml
	configPath := filepath.Join(homeDir, ".config", "neru", "config.toml")
	_, err = os.Stat(configPath)
	if err == nil {
		logger.Info("Found config at", zap.String("path", configPath))
		return configPath
	}

	// Try ~/Library/Application Support/neru/config.toml
	configPath = filepath.Join(homeDir, "Library", "Application Support", "neru", "config.toml")
	_, err = os.Stat(configPath)
	if err == nil {
		logger.Info("Found config at", zap.String("path", configPath))
		return configPath
	}

	logger.Info("No config file found in default locations")
	return ""
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// At least one mode must be enabled
	if !c.Hints.Enabled && !c.Grid.Enabled {
		return errors.New("at least one mode must be enabled: hints.enabled or grid.enabled")
	}

	// Validate hints configuration
	err := c.validateHints()
	if err != nil {
		return err
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.Logging.LogLevel] {
		return errors.New("log_level must be one of: debug, info, warn, error")
	}

	// Validate scroll settings
	if c.Scroll.ScrollStep < 1 {
		return errors.New("scroll.scroll_speed must be at least 1")
	}
	if c.Scroll.ScrollStepHalf < 1 {
		return errors.New("scroll.half_page_multiplier must be at least 1")
	}
	if c.Scroll.ScrollStepFull < 1 {
		return errors.New("scroll.full_page_multiplier must be at least 1")
	}

	// Validate app configs
	err = c.validateAppConfigs()
	if err != nil {
		return err
	}

	// Validate grid settings
	err = c.validateGrid()
	if err != nil {
		return err
	}

	// Validate action settings
	err = c.validateAction()
	if err != nil {
		return err
	}

	return nil
}

// Save saves the configuration to the specified path.
func (c *Config) Save(path string) error {
	// Create directory if it doesn't exist
	var err error
	dir := filepath.Dir(path)
	err = os.MkdirAll(dir, 0o750)
	if err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create file
	var closeErr error
	// #nosec G304 -- Path is validated and controlled by the application
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer func() {
		cerr := file.Close()
		if cerr != nil && closeErr == nil {
			closeErr = fmt.Errorf("failed to close config file: %w", cerr)
		}
	}()

	// Encode to TOML
	encoder := toml.NewEncoder(file)
	err = encoder.Encode(c)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
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

// GetClickableRolesForApp returns the merged clickable roles for a specific app.
// It combines global clickable roles with app-specific additional roles.
func (c *Config) GetClickableRolesForApp(bundleID string) []string {
	// Start with global roles
	rolesMap := make(map[string]struct{})
	for _, role := range c.Hints.ClickableRoles {
		trimmed := strings.TrimSpace(role)
		if trimmed != "" {
			rolesMap[trimmed] = struct{}{}
		}
	}

	// Add app-specific roles
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

	// Add menubar roles if enabled
	if c.Hints.IncludeMenubarHints {
		rolesMap["AXMenuBarItem"] = struct{}{}
	}

	// Add dock roles if enabled
	if c.Hints.IncludeDockHints {
		rolesMap["AXDockItem"] = struct{}{}
	}

	// Convert map to slice
	roles := make([]string, 0, len(rolesMap))
	for role := range rolesMap {
		roles = append(roles, role)
	}
	return roles
}

// validateHints validates the hints configuration.
func (c *Config) validateHints() error {
	var err error
	// Validate hint characters
	if strings.TrimSpace(c.Hints.HintCharacters) == "" {
		return errors.New("hint_characters cannot be empty")
	}
	if len(c.Hints.HintCharacters) < 2 {
		return errors.New("hint_characters must contain at least 2 characters")
	}

	// Validate opacity values
	if c.Hints.Opacity < 0 || c.Hints.Opacity > 1 {
		return errors.New("hints.opacity must be between 0 and 1")
	}

	// Validate colors
	err = validateColor(c.Hints.BackgroundColor, "hints.background_color")
	if err != nil {
		return err
	}
	err = validateColor(c.Hints.TextColor, "hints.text_color")
	if err != nil {
		return err
	}
	err = validateColor(c.Hints.MatchedTextColor, "hints.matched_text_color")
	if err != nil {
		return err
	}
	err = validateColor(c.Hints.BorderColor, "hints.border_color")
	if err != nil {
		return err
	}

	// Validate hints settings
	if c.Hints.FontSize < 6 || c.Hints.FontSize > 72 {
		return errors.New("hints.font_size must be between 6 and 72")
	}
	if c.Hints.BorderRadius < 0 {
		return errors.New("hints.border_radius must be non-negative")
	}
	if c.Hints.Padding < 0 {
		return errors.New("hints.padding must be non-negative")
	}
	if c.Hints.BorderWidth < 0 {
		return errors.New("hints.border_width must be non-negative")
	}

	for _, role := range c.Hints.ClickableRoles {
		if strings.TrimSpace(role) == "" {
			return errors.New("hints.clickable_roles cannot contain empty values")
		}
	}

	for _, bundle := range c.Hints.AdditionalAXSupport.AdditionalElectronBundles {
		if strings.TrimSpace(bundle) == "" {
			return errors.New(
				"hints.electron_support.additional_electron_bundles cannot contain empty values",
			)
		}
	}

	for _, bundle := range c.Hints.AdditionalAXSupport.AdditionalChromiumBundles {
		if strings.TrimSpace(bundle) == "" {
			return errors.New(
				"hints.electron_support.additional_chromium_bundles cannot contain empty values",
			)
		}
	}

	for _, bundle := range c.Hints.AdditionalAXSupport.AdditionalFirefoxBundles {
		if strings.TrimSpace(bundle) == "" {
			return errors.New(
				"hints.electron_support.additional_firefox_bundles cannot contain empty values",
			)
		}
	}

	return nil
}

// validateAppConfigs validates the app configurations.
func (c *Config) validateAppConfigs() error {
	var err error
	for index, appConfig := range c.Hints.AppConfigs {
		if strings.TrimSpace(appConfig.BundleID) == "" {
			return fmt.Errorf("hints.app_configs[%d].bundle_id cannot be empty", index)
		}

		// Validate hotkey bindings
		for key, value := range c.Hotkeys.Bindings {
			if strings.TrimSpace(key) == "" {
				return errors.New("hotkeys.bindings contains an empty key")
			}
			err = validateHotkey(key, "hotkeys.bindings")
			if err != nil {
				return err
			}
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("hotkeys.bindings[%s] cannot be empty", key)
			}
		}
		for _, role := range appConfig.AdditionalClickable {
			if strings.TrimSpace(role) == "" {
				return fmt.Errorf(
					"hints.app_configs[%d].additional_clickable_roles cannot contain empty values",
					index,
				)
			}
		}
	}
	return nil
}

// validateGrid validates the grid configuration.
func (c *Config) validateGrid() error {
	var err error
	if strings.TrimSpace(c.Grid.Characters) == "" {
		return errors.New("grid.characters cannot be empty")
	}
	if len(c.Grid.Characters) < 2 {
		return errors.New("grid.characters must contain at least 2 characters")
	}
	if c.Grid.FontSize < 6 || c.Grid.FontSize > 72 {
		return errors.New("grid.font_size must be between 6 and 72")
	}
	if c.Grid.BorderWidth < 0 {
		return errors.New("grid.border_width must be non-negative")
	}
	if c.Grid.Opacity < 0 || c.Grid.Opacity > 1 {
		return errors.New("grid.opacity must be between 0 and 1")
	}

	// Validate per-action grid colors
	err = validateColor(c.Grid.BackgroundColor, "grid.background_color")
	if err != nil {
		return err
	}
	err = validateColor(c.Grid.TextColor, "grid.text_color")
	if err != nil {
		return err
	}
	err = validateColor(c.Grid.MatchedTextColor, "grid.matched_text_color")
	if err != nil {
		return err
	}
	err = validateColor(c.Grid.MatchedBackgroundColor, "grid.matched_background_color")
	if err != nil {
		return err
	}
	err = validateColor(c.Grid.MatchedBorderColor, "grid.matched_border_color")
	if err != nil {
		return err
	}
	err = validateColor(c.Grid.BorderColor, "grid.border_color")
	if err != nil {
		return err
	}

	if c.Grid.SubgridEnabled {
		// Validate sublayer keys length (fallback to grid.characters) for 3x3 subgrid
		keys := strings.TrimSpace(c.Grid.SublayerKeys)
		if keys == "" {
			keys = c.Grid.Characters
		}
		// Subgrid is always 3x3, requiring at least 9 characters
		const required = 9
		if len([]rune(keys)) < required {
			return fmt.Errorf(
				"grid.sublayer_keys must contain at least %d characters for 3x3 subgrid selection",
				required,
			)
		}
	}
	return nil
}

// validateAction validates the action configuration.
func (c *Config) validateAction() error {
	var err error
	if c.Action.HighlightWidth < 1 {
		return errors.New("action.highlight_width must be at least 1")
	}
	err = validateColor(c.Action.HighlightColor, "action.highlight_color")
	if err != nil {
		return err
	}
	return nil
}

// validateHotkey validates a hotkey string format.
func validateHotkey(hotkey, fieldName string) error {
	if strings.TrimSpace(hotkey) == "" {
		return nil // Allow empty hotkey to disable the action
	}

	// Hotkey format: [Modifier+]*Key
	// Valid modifiers: Cmd, Ctrl, Alt, Shift, Option
	// Examples: "Cmd+Shift+Space", "Ctrl+D", "F1"

	parts := strings.Split(hotkey, "+")
	if len(parts) == 0 {
		return fmt.Errorf("%s has invalid format: %s", fieldName, hotkey)
	}

	validModifiers := map[string]bool{
		"Cmd":    true,
		"Ctrl":   true,
		"Alt":    true,
		"Shift":  true,
		"Option": true,
	}

	// Check all parts except the last (which is the key)
	for i := range parts[:len(parts)-1] {
		modifier := strings.TrimSpace(parts[i])
		if !validModifiers[modifier] {
			return fmt.Errorf(
				"%s has invalid modifier '%s' in: %s (valid: Cmd, Ctrl, Alt, Shift, Option)",
				fieldName,
				modifier,
				hotkey,
			)
		}
	}

	// Last part should be the key (non-empty)
	key := strings.TrimSpace(parts[len(parts)-1])
	if key == "" {
		return fmt.Errorf("%s has empty key in: %s", fieldName, hotkey)
	}

	return nil
}

// validateColor validates a color string (hex format).
func validateColor(color, fieldName string) error {
	if strings.TrimSpace(color) == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	// Match hex color format: #RGB, #RRGGBB, #RRGGBBAA
	hexColorRegex := regexp.MustCompile(`^#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6}|[0-9A-Fa-f]{8})$`)

	if !hexColorRegex.MatchString(color) {
		return fmt.Errorf(
			"%s has invalid hex color format: %s (expected #RGB, #RRGGBB, or #RRGGBBAA)",
			fieldName,
			color,
		)
	}

	return nil
}
