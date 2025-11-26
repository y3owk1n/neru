package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	derrors "github.com/y3owk1n/neru/internal/errors"
)

// ActionConfig defines the visual and behavioral settings for action mode.
type ActionConfig struct {
	HighlightColor string `toml:"highlight_color"`
	HighlightWidth int    `toml:"highlight_width"`

	LeftClickKey   string `toml:"left_click_key"`
	RightClickKey  string `toml:"right_click_key"`
	MiddleClickKey string `toml:"middle_click_key"`
	MouseDownKey   string `toml:"mouse_down_key"`
	MouseUpKey     string `toml:"mouse_up_key"`
}

// Config represents the complete application configuration structure.
type Config struct {
	General      GeneralConfig      `toml:"general"`
	Hotkeys      HotkeysConfig      `toml:"hotkeys"`
	Hints        HintsConfig        `toml:"hints"`
	Grid         GridConfig         `toml:"grid"`
	Scroll       ScrollConfig       `toml:"scroll"`
	Action       ActionConfig       `toml:"action"`
	Logging      LoggingConfig      `toml:"logging"`
	SmoothCursor SmoothCursorConfig `toml:"smooth_cursor"`
	Metrics      MetricsConfig      `toml:"metrics"`
}

// GeneralConfig defines general application-wide settings.
type GeneralConfig struct {
	ExcludedApps              []string `toml:"excluded_apps"`
	AccessibilityCheckOnStart bool     `toml:"accessibility_check_on_start"`
	RestoreCursorPosition     bool     `toml:"restore_cursor_position"`
}

// AppConfig defines application-specific settings for role customization.
type AppConfig struct {
	BundleID             string   `toml:"bundle_id"`
	AdditionalClickable  []string `toml:"additional_clickable_roles"`
	IgnoreClickableCheck bool     `toml:"ignore_clickable_check"`
}

// HotkeysConfig defines hotkey mappings and their associated actions.
type HotkeysConfig struct {
	// Bindings holds hotkey -> action mappings parsed from the [hotkeys] table.
	// Supported TOML format (preferred):
	// [hotkeys]
	// "Cmd+Shift+Space" = "hints"
	// Values are strings. The special exec prefix is supported: "exec /usr/bin/say hi"
	Bindings map[string]string `toml:"bindings"`
}

// ScrollConfig defines the behavior and appearance settings for scroll mode.
type ScrollConfig struct {
	ScrollStep          int    `toml:"scroll_step"`
	ScrollStepHalf      int    `toml:"scroll_step_half"`
	ScrollStepFull      int    `toml:"scroll_step_full"`
	HighlightScrollArea bool   `toml:"highlight_scroll_area"`
	HighlightColor      string `toml:"highlight_color"`
	HighlightWidth      int    `toml:"highlight_width"`
}

// HintsConfig defines the visual and behavioral settings for hints mode.
type HintsConfig struct {
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

	IncludeMenubarHints           bool     `toml:"include_menubar_hints"`
	AdditionalMenubarHintsTargets []string `toml:"additional_menubar_hints_targets"`
	IncludeDockHints              bool     `toml:"include_dock_hints"`
	IncludeNCHints                bool     `toml:"include_nc_hints"`

	ClickableRoles       []string `toml:"clickable_roles"`
	IgnoreClickableCheck bool     `toml:"ignore_clickable_check"`

	AppConfigs []AppConfig `toml:"app_configs"`

	AdditionalAXSupport AdditionalAXSupport `toml:"additional_ax_support"`
}

// GridConfig defines the visual and behavioral settings for grid mode.
type GridConfig struct {
	Enabled bool `toml:"enabled"`

	Characters   string `toml:"characters"`
	SublayerKeys string `toml:"sublayer_keys"`

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

	LiveMatchUpdate bool `toml:"live_match_update"`
	HideUnmatched   bool `toml:"hide_unmatched"`
}

// LoggingConfig defines the logging behavior and file management settings.
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

// SmoothCursorConfig defines the smooth cursor movement settings.
type SmoothCursorConfig struct {
	MoveMouseEnabled bool `toml:"move_mouse_enabled"`
	Steps            int  `toml:"steps"`
	Delay            int  `toml:"delay"` // Delay in milliseconds
}

// AdditionalAXSupport defines accessibility support for specific application frameworks.
type AdditionalAXSupport struct {
	Enable                    bool     `toml:"enable"`
	AdditionalElectronBundles []string `toml:"additional_electron_bundles"`
	AdditionalChromiumBundles []string `toml:"additional_chromium_bundles"`
	AdditionalFirefoxBundles  []string `toml:"additional_firefox_bundles"`
}

// MetricsConfig defines metrics collection settings.
type MetricsConfig struct {
	Enabled bool `toml:"enabled"`
}

// LoadResult contains the result of loading a configuration file.
type LoadResult struct {
	Config          *Config
	ValidationError error
	ConfigPath      string
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// At least one mode must be enabled
	if !c.Hints.Enabled && !c.Grid.Enabled {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"at least one mode must be enabled: hints.enabled or grid.enabled",
		)
	}

	// Validate hints configuration
	validateErr := c.ValidateHints()
	if validateErr != nil {
		return validateErr
	}

	// Validate log level
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

	// Validate scroll settings
	if c.Scroll.ScrollStep < 1 {
		return derrors.New(derrors.CodeInvalidConfig, "scroll.scroll_speed must be at least 1")
	}

	if c.Scroll.ScrollStepHalf < 1 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"scroll.half_page_multiplier must be at least 1",
		)
	}

	if c.Scroll.ScrollStepFull < 1 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"scroll.full_page_multiplier must be at least 1",
		)
	}

	// Validate app configs
	validateAppConfigsErr := c.ValidateAppConfigs()
	if validateAppConfigsErr != nil {
		return validateAppConfigsErr
	}

	// Validate grid settings
	validateGridErr := c.ValidateGrid()
	if validateGridErr != nil {
		return validateGridErr
	}

	// Validate action settings
	validateActionErr := c.ValidateAction()
	if validateActionErr != nil {
		return validateActionErr
	}

	// Validate smooth cursor settings
	validateSmoothCursorErr := c.ValidateSmoothCursor()
	if validateSmoothCursorErr != nil {
		return validateSmoothCursorErr
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

// ClickableRolesForApp returns the merged clickable roles for a specific app.
func (c *Config) ClickableRolesForApp(bundleID string) []string {
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
		rolesMap["AXMenuBarItem"] = struct{}{}
	}

	if c.Hints.IncludeDockHints {
		rolesMap["AXDockItem"] = struct{}{}
	}

	roles := make([]string, 0, len(rolesMap))
	for role := range rolesMap {
		roles = append(roles, role)
	}

	return roles
}
