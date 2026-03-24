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

// StringOrStringArray is a type that can unmarshal from either a TOML string
// or a TOML array of strings. Used for backward compatibility.
type StringOrStringArray []string

// UnmarshalTOML implements custom unmarshaling for TOML compatibility.
// It accepts both single string values and arrays of strings.
func (s *StringOrStringArray) UnmarshalTOML(value any) error {
	switch val := value.(type) {
	case string:
		*s = []string{val}

	case []any:
		*s = make([]string, 0, len(val))
		for _, a := range val {
			actionStr, ok := a.(string)
			if !ok {
				return derrors.Newf(derrors.CodeInvalidConfig, "expected string, got %T", a)
			}

			*s = append(*s, actionStr)
		}

	case []string:
		*s = val

	default:
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"cannot unmarshal %T into StringOrStringArray",
			value,
		)
	}

	return nil
}

// Accessibility role constants.
const (
	RoleMenuBarItem = "AXMenuBarItem"
	RoleDockItem    = "AXDockItem"
)

// Mode name constants used in config lookups (ResolvedExitKeys, CustomHotkeysForMode, validation).
// These mirror domain.ModeName* but are defined here to avoid a circular import.
const (
	modeNameHints         = "hints"
	modeNameGrid          = "grid"
	modeNameRecursiveGrid = "recursive_grid"
	modeNameScroll        = "scroll"
)

// Key name constants for normalization.
// These are the canonical lowercase forms used throughout the codebase.
const (
	minimumModifierParts = 2
	KeyNameEscape        = "escape"
	KeyNameReturn        = "return"
	KeyNameTab           = "tab"
	KeyNameSpace         = "space"
	KeyNameBackspace     = "backspace"
	KeyNameDelete        = "delete"
	KeyNameHome          = "home"
	KeyNameEnd           = "end"
	KeyNamePageUp        = "pageup"
	KeyNamePageDown      = "pagedown"
	KeyNameUp            = "up"
	KeyNameDown          = "down"
	KeyNameLeft          = "left"
	KeyNameRight         = "right"
)

// validNamedKeys is the canonical set of all named keys the system supports.
// Every validator, normalizer, and key parser should reference this set via the
// public helpers (IsValidNamedKey, CanonicalNamedKeyForm) instead of maintaining
// its own ad-hoc list. The keys are stored in their display form (the casing
// that the event tap / config files use).
//
// The variable is unexported to prevent accidental mutation by other packages.
var validNamedKeys = map[string]bool{
	// Special keys
	"Space":     true,
	"Return":    true,
	"Enter":     true, // alias for Return
	"Escape":    true,
	"Tab":       true,
	"Delete":    true,
	"Backspace": true, // alias for Delete on macOS
	// Navigation keys
	"Up":       true,
	"Down":     true,
	"Left":     true,
	"Right":    true,
	"Home":     true,
	"End":      true,
	"PageUp":   true,
	"PageDown": true,
	// Function keys
	"F1":  true,
	"F2":  true,
	"F3":  true,
	"F4":  true,
	"F5":  true,
	"F6":  true,
	"F7":  true,
	"F8":  true,
	"F9":  true,
	"F10": true,
	"F11": true,
	"F12": true,
	"F13": true,
	"F14": true,
	"F15": true,
	"F16": true,
	"F17": true,
	"F18": true,
	"F19": true,
	"F20": true,
}

// validNamedKeysLower is a precomputed lowercase lookup for IsValidNamedKey.
var validNamedKeysLower map[string]bool

// namedKeyDisplayForm maps lowercase key names to their canonical display form
// (e.g. "pagedown" → "PageDown", "f1" → "F1"). Used by CanonicalNamedKeyForm.
var namedKeyDisplayForm map[string]string

func init() {
	validNamedKeysLower = make(map[string]bool, len(validNamedKeys))
	namedKeyDisplayForm = make(map[string]string, len(validNamedKeys))

	for k := range validNamedKeys {
		lower := strings.ToLower(k)
		validNamedKeysLower[lower] = true
		namedKeyDisplayForm[lower] = k
	}
}

// IsValidNamedKey checks whether a key name is a recognized named key (case-insensitive).
func IsValidNamedKey(key string) bool {
	return validNamedKeysLower[strings.ToLower(key)]
}

// CanonicalNamedKeyForm returns the canonical display form of a named key
// (e.g. "pagedown" → "PageDown", "UP" → "Up", "f1" → "F1").
// If the key is not a recognized named key, it returns the input unchanged
// and false as the second return value.
func CanonicalNamedKeyForm(key string) (string, bool) {
	display, displayOk := namedKeyDisplayForm[strings.ToLower(key)]

	if !displayOk {
		return key, false
	}

	return display, displayOk
}

// NormalizeKeyForComparison converts escape sequences and key names to a canonical form for comparison.
// This ensures that "\x1b" and "escape" are treated as the same key, and provides case-insensitive
// matching for all keys (e.g. "q" matches "Q", "Ctrl+R" matches "ctrl+r").
// On macOS, both "backspace" and "delete" are treated as synonyms for the DEL key (\x7f).
// Named keys (arrows, function keys, nav keys) are normalized to their canonical lowercase form.
// Also normalizes fullwidth CJK characters to their halfwidth ASCII equivalents.
func NormalizeKeyForComparison(key string) string {
	// Normalize fullwidth CJK characters first, before lowercasing and canonical matching.
	// This ensures e.g. fullwidth space (U+3000) → " " → "space" in a single pass.
	key = normalizeFullwidthChars(key)
	key = strings.ToLower(key)

	// Handle escape sequences and aliases that map to a different canonical form.
	switch key {
	case "\x1b", "esc":
		return KeyNameEscape
	case "\r", "enter":
		return KeyNameReturn
	case "\t":
		return KeyNameTab
	case " ":
		return KeyNameSpace
	case "\x08", "\x7f", KeyNameBackspace:
		// On macOS, the Delete key (above Return) sends \x7f.
		// \x08 is the ASCII BS control character (rarely generated on macOS but included for completeness).
		// Treat "delete", "backspace", \x7f, and \x08 as synonyms for user-friendly matching.
		return KeyNameDelete
	}

	// Strip Right/Left modifier prefixes so that e.g. "rightcmd+l" normalizes to "cmd+l",
	// matching the unprefixed modifier names the event tap always produces at runtime.
	key = StripModifierPrefixes(key)

	// Normalize key aliases inside modifier combos.
	// The switch above only handles bare "enter" / "backspace" etc., but users may
	// write "Shift+Enter" which lowercases to "shift+enter". The event tap always
	// produces the canonical form "shift+return", so we must resolve the alias here.
	key = normalizeKeyAliasesInCombo(key)

	// All other keys (named keys, plain characters, modifier combos) are already
	// lowercased by strings.ToLower above and pass through as-is.
	return key
}

// HasPassthroughModifier reports whether the key contains a modifier that can
// be allowed through to macOS while a mode is active. Shift-only combos are
// excluded because they are commonly used inside modes.
func HasPassthroughModifier(key string) bool {
	parts := strings.Split(NormalizeKeyForComparison(key), "+")
	if len(parts) < minimumModifierParts {
		return false
	}

	for _, part := range parts[:len(parts)-1] {
		trimmed := strings.TrimSpace(part)
		switch trimmed {
		case "cmd", "ctrl", "alt", "option":
			return true
		}
	}

	return false
}

var modifierPrefixReplacer = strings.NewReplacer(
	"rightcmd+", "cmd+",
	"leftcmd+", "cmd+",
	"rightctrl+", "ctrl+",
	"leftctrl+", "ctrl+",
	"rightalt+", "alt+",
	"leftalt+", "alt+",
	"rightoption+", "option+",
	"leftoption+", "option+",
	"rightshift+", "shift+",
	"leftshift+", "shift+",
)

// StripModifierPrefixes removes Right/Left prefixes from modifier names in a
// lowercased key string. This is a no-op when the key contains no such prefix.
func StripModifierPrefixes(key string) string {
	if !strings.Contains(key, "+") {
		return key
	}

	return modifierPrefixReplacer.Replace(key)
}

// comboKeyAliases maps alias key names to their canonical forms.
// Used by normalizeKeyAliasesInCombo to resolve the final segment of compound keys.
var comboKeyAliases = map[string]string{
	"enter":     "return",
	"backspace": "delete",
	"esc":       "escape",
}

// normalizeKeyAliasesInCombo resolves key name aliases inside modifier combos.
// e.g. "shift+enter" → "shift+return", "cmd+backspace" → "cmd+delete".
// Only applies to compound keys (containing "+"); bare keys are handled by the
// switch in NormalizeKeyForComparison.
// Splits on the last "+" and only normalizes the final segment to avoid mangling
// modifier names or canonical forms that share a prefix (e.g. "escape" vs "esc").
func normalizeKeyAliasesInCombo(key string) string {
	idx := strings.LastIndex(key, "+")
	if idx < 0 {
		return key
	}

	prefix, suffix := key[:idx+1], key[idx+1:]
	if canonical, ok := comboKeyAliases[suffix]; ok {
		return prefix + canonical
	}

	return key
}

// normalizeFullwidthChars converts fullwidth CJK characters (U+FF01-U+FF5E)
// to their halfwidth ASCII equivalents (U+0021-U+007E).
// This ensures keys work correctly when using CJK input methods.
// Uses strings.Map for efficiency - only allocates when transformation occurs.
func normalizeFullwidthChars(key string) string {
	const (
		fullwidthStart  = 0xFF01 // Fullwidth exclamation mark
		fullwidthEnd    = 0xFF5E // Fullwidth tilde
		halfwidthOffset = 0xFEE0 // Difference between fullwidth and halfwidth
		fullwidthSpace  = 0x3000 // CJK fullwidth space
	)

	return strings.Map(func(char rune) rune {
		switch {
		case char >= fullwidthStart && char <= fullwidthEnd:
			// Convert fullwidth to halfwidth
			return char - halfwidthOffset
		case char == fullwidthSpace:
			// Fullwidth space -> regular space
			return ' '
		default:
			// Return unchanged (strings.Map optimizes this case)
			return char
		}
	}, key)
}

// IsExitKey checks if a key matches any configured exit key (with normalization).
// This handles comparison between escape sequences (e.g. "\x1b") and key names (e.g. "escape").
func IsExitKey(key string, exitKeys []string) bool {
	if len(exitKeys) == 0 {
		return false
	}

	normalizedKey := NormalizeKeyForComparison(key)
	for _, exitKey := range exitKeys {
		if normalizedKey == NormalizeKeyForComparison(exitKey) {
			return true
		}
	}

	return false
}

// IsResetKey checks if a key matches the configured reset key (with normalization).
// This handles comparison between single characters and modifier combos with case-insensitive matching.
func IsResetKey(key, resetKey string) bool {
	if resetKey == "" {
		return false
	}

	return NormalizeKeyForComparison(key) == NormalizeKeyForComparison(resetKey)
}

// IsBackspaceKey checks if a key is a backspace/delete key.
// This normalizes all variations: "\x7f", "delete", "backspace", "Delete", "Backspace", etc.
func IsBackspaceKey(key string) bool {
	return NormalizeKeyForComparison(key) == KeyNameDelete
}

// IsConfiguredBackspaceKey checks if a key matches the configured backspace key.
// If configuredKey is empty, it falls back to the default backspace/delete check.
func IsConfiguredBackspaceKey(key, configuredKey string) bool {
	if configuredKey == "" {
		return IsBackspaceKey(key)
	}

	return NormalizeKeyForComparison(key) == NormalizeKeyForComparison(configuredKey)
}

// ActionConfig defines the visual and behavioral settings for action mode.
type ActionConfig struct {
	KeyBindings   ActionKeyBindingsCfg `json:"keyBindings"   toml:"key_bindings"`
	MoveMouseStep int                  `json:"moveMouseStep" toml:"move_mouse_step"`
}

// ActionKeyBindingsCfg defines direct action keybindings for use in hints/grid/scroll mode.
type ActionKeyBindingsCfg struct {
	LeftClick      string `json:"leftClick"      toml:"left_click"`
	RightClick     string `json:"rightClick"     toml:"right_click"`
	MiddleClick    string `json:"middleClick"    toml:"middle_click"`
	MouseDown      string `json:"mouseDown"      toml:"mouse_down"`
	MouseUp        string `json:"mouseUp"        toml:"mouse_up"`
	MoveMouseUp    string `json:"moveMouseUp"    toml:"move_mouse_up"`
	MoveMouseDown  string `json:"moveMouseDown"  toml:"move_mouse_down"`
	MoveMouseLeft  string `json:"moveMouseLeft"  toml:"move_mouse_left"`
	MoveMouseRight string `json:"moveMouseRight" toml:"move_mouse_right"`
}

// Config represents the complete application configuration structure.
type Config struct {
	General         GeneralConfig         `json:"general"         toml:"general"`
	Hotkeys         HotkeysConfig         `json:"hotkeys"         toml:"hotkeys"`
	Hints           HintsConfig           `json:"hints"           toml:"hints"`
	Grid            GridConfig            `json:"grid"            toml:"grid"`
	RecursiveGrid   RecursiveGridConfig   `json:"recursiveGrid"   toml:"recursive_grid"`
	Scroll          ScrollConfig          `json:"scroll"          toml:"scroll"`
	Action          ActionConfig          `json:"action"          toml:"action"`
	ModeIndicator   ModeIndicatorConfig   `json:"modeIndicator"   toml:"mode_indicator"`
	StickyModifiers StickyModifiersConfig `json:"stickyModifiers" toml:"sticky_modifiers"`
	Logging         LoggingConfig         `json:"logging"         toml:"logging"`
	SmoothCursor    SmoothCursorConfig    `json:"smoothCursor"    toml:"smooth_cursor"`
	Systray         SystrayConfig         `json:"systray"         toml:"systray"`
}

// GeneralConfig defines general application-wide settings.
type GeneralConfig struct {
	ExcludedApps                      []string `json:"excludedApps"                      toml:"excluded_apps"`
	AccessibilityCheckOnStart         bool     `json:"accessibilityCheckOnStart"         toml:"accessibility_check_on_start"`
	RestoreCursorPosition             bool     `json:"restoreCursorPosition"             toml:"restore_cursor_position"`
	CenterCursorPosition              bool     `json:"centerCursorPosition"              toml:"center_cursor_position"`
	ModeExitKeys                      []string `json:"modeExitKeys"                      toml:"mode_exit_keys"`
	PassthroughUnboundedKeys          bool     `json:"passthroughUnboundedKeys"          toml:"passthrough_unbounded_keys"`
	ShouldExitAfterPassthrough        bool     `json:"shouldExitAfterPassthrough"        toml:"should_exit_after_passthrough"`
	PassthroughUnboundedKeysBlacklist []string `json:"passthroughUnboundedKeysBlacklist" toml:"passthrough_unbounded_keys_blacklist"`
	HideOverlayInScreenShare          bool     `json:"hideOverlayInScreenShare"          toml:"hide_overlay_in_screen_share"`
	KBLayoutToUse                     string   `json:"kbLayoutToUse"                     toml:"kb_layout_to_use"`
}

// ModeIndicatorUI defines the visual/appearance settings for the mode indicator.
type ModeIndicatorUI struct {
	FontSize             int    `json:"fontSize"             toml:"font_size"`
	FontFamily           string `json:"fontFamily"           toml:"font_family"`
	BackgroundColorLight string `json:"backgroundColorLight" toml:"background_color_light"`
	BackgroundColorDark  string `json:"backgroundColorDark"  toml:"background_color_dark"`
	TextColorLight       string `json:"textColorLight"       toml:"text_color_light"`
	TextColorDark        string `json:"textColorDark"        toml:"text_color_dark"`
	BorderColorLight     string `json:"borderColorLight"     toml:"border_color_light"`
	BorderColorDark      string `json:"borderColorDark"      toml:"border_color_dark"`
	BorderWidth          int    `json:"borderWidth"          toml:"border_width"`
	PaddingX             int    `json:"paddingX"             toml:"padding_x"`
	PaddingY             int    `json:"paddingY"             toml:"padding_y"`
	BorderRadius         int    `json:"borderRadius"         toml:"border_radius"`
	IndicatorXOffset     int    `json:"indicatorXOffset"     toml:"indicator_x_offset"`
	IndicatorYOffset     int    `json:"indicatorYOffset"     toml:"indicator_y_offset"`
}

// ModeIndicatorModeConfig defines per-mode settings for the mode indicator.
// Text is the label shown in the indicator; empty string hides the label.
// Color overrides are optional; empty string inherits from [mode_indicator.ui].
type ModeIndicatorModeConfig struct {
	Enabled              bool   `json:"enabled"              toml:"enabled"`
	Text                 string `json:"text"                 toml:"text"`
	BackgroundColorLight string `json:"backgroundColorLight" toml:"background_color_light"`
	BackgroundColorDark  string `json:"backgroundColorDark"  toml:"background_color_dark"`
	TextColorLight       string `json:"textColorLight"       toml:"text_color_light"`
	TextColorDark        string `json:"textColorDark"        toml:"text_color_dark"`
	BorderColorLight     string `json:"borderColorLight"     toml:"border_color_light"`
	BorderColorDark      string `json:"borderColorDark"      toml:"border_color_dark"`
}

// ModeIndicatorConfig defines per-mode indicator visibility and appearance.
type ModeIndicatorConfig struct {
	Scroll        ModeIndicatorModeConfig `json:"scroll"        toml:"scroll"`
	Hints         ModeIndicatorModeConfig `json:"hints"         toml:"hints"`
	Grid          ModeIndicatorModeConfig `json:"grid"          toml:"grid"`
	RecursiveGrid ModeIndicatorModeConfig `json:"recursiveGrid" toml:"recursive_grid"`
	UI            ModeIndicatorUI         `json:"ui"            toml:"ui"`
}

// StickyModifiersUI defines the visual/appearance settings for the sticky modifiers indicator.
type StickyModifiersUI struct {
	FontSize             int    `json:"fontSize"             toml:"font_size"`
	FontFamily           string `json:"fontFamily"           toml:"font_family"`
	BackgroundColorLight string `json:"backgroundColorLight" toml:"background_color_light"`
	BackgroundColorDark  string `json:"backgroundColorDark"  toml:"background_color_dark"`
	TextColorLight       string `json:"textColorLight"       toml:"text_color_light"`
	TextColorDark        string `json:"textColorDark"        toml:"text_color_dark"`
	BorderColorLight     string `json:"borderColorLight"     toml:"border_color_light"`
	BorderColorDark      string `json:"borderColorDark"      toml:"border_color_dark"`
	BorderWidth          int    `json:"borderWidth"          toml:"border_width"`
	PaddingX             int    `json:"paddingX"             toml:"padding_x"`
	PaddingY             int    `json:"paddingY"             toml:"padding_y"`
	BorderRadius         int    `json:"borderRadius"         toml:"border_radius"`
	IndicatorXOffset     int    `json:"indicatorXOffset"     toml:"indicator_x_offset"`
	IndicatorYOffset     int    `json:"indicatorYOffset"     toml:"indicator_y_offset"`
}

// StickyModifiersConfig defines settings for the sticky modifiers feature.
type StickyModifiersConfig struct {
	Enabled        bool              `json:"enabled"        toml:"enabled"`
	TapMaxDuration int               `json:"tapMaxDuration" toml:"tap_max_duration"`
	TapCooldown    int               `json:"tapCooldown"    toml:"tap_cooldown"`
	UI             StickyModifiersUI `json:"ui"             toml:"ui"`
}

// AppConfig defines application-specific settings for role customization.
type AppConfig struct {
	BundleID                string   `json:"bundleId"                toml:"bundle_id"`
	AdditionalClickable     []string `json:"additionalClickable"     toml:"additional_clickable_roles"`
	IgnoreClickableCheck    bool     `json:"ignoreClickableCheck"    toml:"ignore_clickable_check"`
	MouseActionRefreshDelay *int     `json:"mouseActionRefreshDelay" toml:"mouse_action_refresh_delay"`
}

// HotkeysConfig defines hotkey mappings and their associated actions.
type HotkeysConfig struct {
	// Bindings holds hotkey -> action mappings parsed from the [hotkeys] table.
	// Supported TOML format (preferred):
	// [hotkeys]
	// "Cmd+Shift+Space" = "hints"
	// Values can be a single string or an array of strings:
	// "PageUp" = ["action go_top", "action scroll_down"]
	// The special exec prefix is supported: "exec /usr/bin/say hi"
	Bindings map[string][]string `json:"bindings" toml:"bindings"`
}

// ScrollConfig defines the behavior and appearance settings for scroll mode.
type ScrollConfig struct {
	ScrollStep      int      `json:"scrollStep"      toml:"scroll_step"`
	ScrollStepHalf  int      `json:"scrollStepHalf"  toml:"scroll_step_half"`
	ScrollStepFull  int      `json:"scrollStepFull"  toml:"scroll_step_full"`
	AutoExitActions []string `json:"autoExitActions" toml:"auto_exit_actions"`
	ModeExitKeys    []string `json:"modeExitKeys"    toml:"mode_exit_keys"`

	KeyBindings map[string][]string `json:"keyBindings" toml:"key_bindings"`

	CustomHotkeys map[string]StringOrStringArray `json:"customHotkeys" toml:"custom_hotkeys"`
}

// HintsUI defines the visual/appearance settings for hints mode.
type HintsUI struct {
	FontSize              int    `json:"fontSize"              toml:"font_size"`
	FontFamily            string `json:"fontFamily"            toml:"font_family"`
	BorderRadius          int    `json:"borderRadius"          toml:"border_radius"`
	PaddingX              int    `json:"paddingX"              toml:"padding_x"`
	PaddingY              int    `json:"paddingY"              toml:"padding_y"`
	BorderWidth           int    `json:"borderWidth"           toml:"border_width"`
	BackgroundColorLight  string `json:"backgroundColorLight"  toml:"background_color_light"`
	BackgroundColorDark   string `json:"backgroundColorDark"   toml:"background_color_dark"`
	TextColorLight        string `json:"textColorLight"        toml:"text_color_light"`
	TextColorDark         string `json:"textColorDark"         toml:"text_color_dark"`
	MatchedTextColorLight string `json:"matchedTextColorLight" toml:"matched_text_color_light"`
	MatchedTextColorDark  string `json:"matchedTextColorDark"  toml:"matched_text_color_dark"`
	BorderColorLight      string `json:"borderColorLight"      toml:"border_color_light"`
	BorderColorDark       string `json:"borderColorDark"       toml:"border_color_dark"`
}

// HintsConfig defines the visual and behavioral settings for hints mode.
type HintsConfig struct {
	Enabled                 bool     `json:"enabled"                 toml:"enabled"`
	AutoExitActions         []string `json:"autoExitActions"         toml:"auto_exit_actions"`
	ModeExitKeys            []string `json:"modeExitKeys"            toml:"mode_exit_keys"`
	HintCharacters          string   `json:"hintCharacters"          toml:"hint_characters"`
	BackspaceKey            string   `json:"backspaceKey"            toml:"backspace_key"`
	MouseActionRefreshDelay int      `json:"mouseActionRefreshDelay" toml:"mouse_action_refresh_delay"`
	MaxDepth                int      `json:"maxDepth"                toml:"max_depth"`
	ParallelThreshold       int      `json:"parallelThreshold"       toml:"parallel_threshold"`
	UI                      HintsUI  `json:"ui"                      toml:"ui"`

	IncludeMenubarHints           bool     `json:"includeMenubarHints"           toml:"include_menubar_hints"`
	AdditionalMenubarHintsTargets []string `json:"additionalMenubarHintsTargets" toml:"additional_menubar_hints_targets"`
	IncludeDockHints              bool     `json:"includeDockHints"              toml:"include_dock_hints"`
	IncludeNCHints                bool     `json:"includeNcHints"                toml:"include_nc_hints"`
	IncludeStageManagerHints      bool     `json:"includeStageManagerHints"      toml:"include_stage_manager_hints"`
	DetectMissionControl          bool     `json:"detectMissionControl"          toml:"detect_mission_control"`

	ClickableRoles       []string `json:"clickableRoles"       toml:"clickable_roles"`
	IgnoreClickableCheck bool     `json:"ignoreClickableCheck" toml:"ignore_clickable_check"`

	AppConfigs []AppConfig `json:"appConfigs" toml:"app_configs"`

	AdditionalAXSupport AdditionalAXSupport `json:"additionalAxSupport" toml:"additional_ax_support"`

	CustomHotkeys map[string]StringOrStringArray `json:"customHotkeys" toml:"custom_hotkeys"`
}

// GridUI defines the visual/appearance settings for grid mode.
type GridUI struct {
	FontSize    int    `json:"fontSize"    toml:"font_size"`
	FontFamily  string `json:"fontFamily"  toml:"font_family"`
	BorderWidth int    `json:"borderWidth" toml:"border_width"`

	BackgroundColorLight        string `json:"backgroundColorLight"        toml:"background_color_light"`
	BackgroundColorDark         string `json:"backgroundColorDark"         toml:"background_color_dark"`
	TextColorLight              string `json:"textColorLight"              toml:"text_color_light"`
	TextColorDark               string `json:"textColorDark"               toml:"text_color_dark"`
	MatchedTextColorLight       string `json:"matchedTextColorLight"       toml:"matched_text_color_light"`
	MatchedTextColorDark        string `json:"matchedTextColorDark"        toml:"matched_text_color_dark"`
	MatchedBackgroundColorLight string `json:"matchedBackgroundColorLight" toml:"matched_background_color_light"`
	MatchedBackgroundColorDark  string `json:"matchedBackgroundColorDark"  toml:"matched_background_color_dark"`
	MatchedBorderColorLight     string `json:"matchedBorderColorLight"     toml:"matched_border_color_light"`
	MatchedBorderColorDark      string `json:"matchedBorderColorDark"      toml:"matched_border_color_dark"`
	BorderColorLight            string `json:"borderColorLight"            toml:"border_color_light"`
	BorderColorDark             string `json:"borderColorDark"             toml:"border_color_dark"`
}

// GridConfig defines the visual and behavioral settings for grid mode.
type GridConfig struct {
	Enabled         bool     `json:"enabled"         toml:"enabled"`
	AutoExitActions []string `json:"autoExitActions" toml:"auto_exit_actions"`
	ModeExitKeys    []string `json:"modeExitKeys"    toml:"mode_exit_keys"`
	Characters      string   `json:"characters"      toml:"characters"`
	SublayerKeys    string   `json:"sublayerKeys"    toml:"sublayer_keys"`
	BackspaceKey    string   `json:"backspaceKey"    toml:"backspace_key"`
	// Optional custom labels for rows and columns
	// If not provided, labels will be inferred from characters
	RowLabels       string `json:"rowLabels"       toml:"row_labels"`
	ColLabels       string `json:"colLabels"       toml:"col_labels"`
	UI              GridUI `json:"ui"              toml:"ui"`
	LiveMatchUpdate bool   `json:"liveMatchUpdate" toml:"live_match_update"`
	HideUnmatched   bool   `json:"hideUnmatched"   toml:"hide_unmatched"`
	PrewarmEnabled  bool   `json:"prewarmEnabled"  toml:"prewarm_enabled"`
	EnableGC        bool   `json:"enableGc"        toml:"enable_gc"`
	ResetKey        string `json:"resetKey"        toml:"reset_key"`

	CustomHotkeys map[string]StringOrStringArray `json:"customHotkeys" toml:"custom_hotkeys"`
}

// RecursiveGridUI defines the visual/appearance settings for recursive-grid mode.
type RecursiveGridUI struct {
	// Visual styling
	LineColorLight              string `json:"lineColorLight"              toml:"line_color_light"`
	LineColorDark               string `json:"lineColorDark"               toml:"line_color_dark"`
	LineWidth                   int    `json:"lineWidth"                   toml:"line_width"`
	HighlightColorLight         string `json:"highlightColorLight"         toml:"highlight_color_light"`
	HighlightColorDark          string `json:"highlightColorDark"          toml:"highlight_color_dark"`
	TextColorLight              string `json:"textColorLight"              toml:"text_color_light"`
	TextColorDark               string `json:"textColorDark"               toml:"text_color_dark"`
	FontSize                    int    `json:"fontSize"                    toml:"font_size"`
	FontFamily                  string `json:"fontFamily"                  toml:"font_family"`
	LabelBackground             bool   `json:"labelBackground"             toml:"label_background"`
	LabelBackgroundColorLight   string `json:"labelBackgroundColorLight"   toml:"label_background_color_light"`
	LabelBackgroundColorDark    string `json:"labelBackgroundColorDark"    toml:"label_background_color_dark"`
	LabelBackgroundPaddingX     int    `json:"labelBackgroundPaddingX"     toml:"label_background_padding_x"`
	LabelBackgroundPaddingY     int    `json:"labelBackgroundPaddingY"     toml:"label_background_padding_y"`
	LabelBackgroundBorderRadius int    `json:"labelBackgroundBorderRadius" toml:"label_background_border_radius"`
	LabelBackgroundBorderWidth  int    `json:"labelBackgroundBorderWidth"  toml:"label_background_border_width"`
	// Sub-key preview: draw a miniature version of the key grid inside each cell
	SubKeyPreview                   bool    `json:"subKeyPreview"                   toml:"sub_key_preview"`
	SubKeyPreviewFontSize           int     `json:"subKeyPreviewFontSize"           toml:"sub_key_preview_font_size"`
	SubKeyPreviewAutohideMultiplier float64 `json:"subKeyPreviewAutohideMultiplier" toml:"sub_key_preview_autohide_multiplier"`
	SubKeyPreviewTextColorLight     string  `json:"subKeyPreviewTextColorLight"     toml:"sub_key_preview_text_color_light"`
	SubKeyPreviewTextColorDark      string  `json:"subKeyPreviewTextColorDark"      toml:"sub_key_preview_text_color_dark"`
}

// RecursiveGridLayerConfig defines per-depth overrides for the recursive grid.
// Depths not listed in the Layers slice use the top-level GridCols/GridRows/Keys defaults.
type RecursiveGridLayerConfig struct {
	Depth    int    `json:"depth"    toml:"depth"`
	GridCols int    `json:"gridCols" toml:"grid_cols"`
	GridRows int    `json:"gridRows" toml:"grid_rows"`
	Keys     string `json:"keys"     toml:"keys"`
}

// RecursiveGridConfig defines the visual and behavioral settings for recursive-grid mode.
type RecursiveGridConfig struct {
	Enabled         bool     `json:"enabled"         toml:"enabled"`
	AutoExitActions []string `json:"autoExitActions" toml:"auto_exit_actions"`
	ModeExitKeys    []string `json:"modeExitKeys"    toml:"mode_exit_keys"`
	// Grid dimensions: columns and rows (default: 2x2)
	GridCols int `json:"gridCols" toml:"grid_cols"`
	GridRows int `json:"gridRows" toml:"grid_rows"`
	// Key bindings (warpd convention for 2x2: u=TL, i=TR, j=BL, k=BR)
	Keys         string          `json:"keys"         toml:"keys"`
	BackspaceKey string          `json:"backspaceKey" toml:"backspace_key"`
	UI           RecursiveGridUI `json:"ui"           toml:"ui"`
	// Behavior
	MinSizeWidth  int    `json:"minSizeWidth"  toml:"min_size_width"`  // Default: 25
	MinSizeHeight int    `json:"minSizeHeight" toml:"min_size_height"` // Default: 25
	MaxDepth      int    `json:"maxDepth"      toml:"max_depth"`       // Default: 10
	ResetKey      string `json:"resetKey"      toml:"reset_key"`
	// Per-depth overrides for grid dimensions and keys.
	// Depths not listed here use the top-level GridCols/GridRows/Keys.
	Layers []RecursiveGridLayerConfig `json:"layers" toml:"layers"`

	CustomHotkeys map[string]StringOrStringArray `json:"customHotkeys" toml:"custom_hotkeys"`
}

// AllKeysIncludingLayers returns a combined string of all unique keys from the
// top-level config and all layers. Used for conflict validation.
func (c *RecursiveGridConfig) AllKeysIncludingLayers() string {
	seen := make(map[rune]bool)

	var result []rune
	for _, r := range c.Keys {
		if !seen[r] {
			seen[r] = true
			result = append(result, r)
		}
	}

	for _, layer := range c.Layers {
		for _, r := range layer.Keys {
			if !seen[r] {
				seen[r] = true
				result = append(result, r)
			}
		}
	}

	return string(result)
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
	MoveMouseEnabled bool    `json:"moveMouseEnabled" toml:"move_mouse_enabled"`
	Steps            int     `json:"steps"            toml:"steps"`
	MaxDuration      int     `json:"maxDuration"      toml:"max_duration"`       // Max animation duration in ms
	DurationPerPixel float64 `json:"durationPerPixel" toml:"duration_per_pixel"` // Ms per pixel for adaptive duration
}

// AdditionalAXSupport defines accessibility support for specific application frameworks.
type AdditionalAXSupport struct {
	Enable                    bool     `json:"enable"                    toml:"enable"`
	AdditionalElectronBundles []string `json:"additionalElectronBundles" toml:"additional_electron_bundles"`
	AdditionalChromiumBundles []string `json:"additionalChromiumBundles" toml:"additional_chromium_bundles"`
	AdditionalFirefoxBundles  []string `json:"additionalFirefoxBundles"  toml:"additional_firefox_bundles"`
}

// SystrayConfig defines system tray settings.
type SystrayConfig struct {
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

	err := c.ValidateGeneral()
	if err != nil {
		return err
	}

	err = c.ValidateModes()
	if err != nil {
		return err
	}

	err = c.ValidateModeIndicator()
	if err != nil {
		return err
	}

	// Validate mode exit keys
	err = c.ValidateModeExitKeys()
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

	// Validate recursive-grid settings
	err = c.ValidateRecursiveGrid()
	if err != nil {
		return err
	}

	// Validate action settings
	err = c.ValidateAction()
	if err != nil {
		return err
	}

	// Validate backspace keys don't conflict with action key bindings.
	// At runtime, direct action keys are checked before backspace keys,
	// so a conflict means backspace will never fire.
	err = c.checkBackspaceKeyActionKeyConflicts()
	if err != nil {
		return err
	}

	// Validate reset keys don't conflict with action key bindings.
	// At runtime, direct action keys are checked before reset keys,
	// so a conflict means reset will never fire.
	err = c.checkResetKeyActionKeyConflicts()
	if err != nil {
		return err
	}

	// Validate per-mode exit keys don't conflict with action key bindings.
	// At runtime, exit keys are checked before action keys (in key_dispatch.go),
	// so a conflict means the action will never fire.
	err = c.checkPerModeExitKeysActionKeyConflicts()
	if err != nil {
		return err
	}

	// Validate scroll key bindings don't conflict with action key bindings.
	// At runtime, action keys are checked before scroll keys (in scroll.go),
	// so a conflict means the scroll binding will never fire.
	err = c.checkScrollKeyBindingsActionKeyConflicts()
	if err != nil {
		return err
	}

	// Validate sticky modifiers settings
	err = c.ValidateStickyModifiers()
	if err != nil {
		return err
	}

	// Validate smooth cursor settings
	err = c.ValidateSmoothCursor()
	if err != nil {
		return err
	}

	// Validate per-mode custom hotkeys
	err = c.ValidateCustomHotkeys()
	if err != nil {
		return err
	}

	return nil
}

// ValidateGeneral validates general settings.
func (c *Config) ValidateGeneral() error {
	if c.General.RestoreCursorPosition && c.General.CenterCursorPosition {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"restore_cursor_position and center_cursor_position cannot both be enabled",
		)
	}

	if c.General.KBLayoutToUse != "" && strings.TrimSpace(c.General.KBLayoutToUse) == "" {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"general.kb_layout_to_use cannot be whitespace-only",
		)
	}

	for index, key := range c.General.PassthroughUnboundedKeysBlacklist {
		fieldName := fmt.Sprintf("general.passthrough_unbounded_keys_blacklist[%d]", index)

		err := ValidateHotkey(key, fieldName)
		if err != nil {
			return err
		}

		if !HasPassthroughModifier(key) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s must include Cmd, Ctrl, Alt, or Option: %s",
				fieldName,
				key,
			)
		}
	}

	return nil
}

// ValidateModeIndicator validates the mode indicator configuration.
func (c *Config) ValidateModeIndicator() error {
	err := validateMinValue(c.ModeIndicator.UI.FontSize, 1, "mode_indicator.ui.font_size")
	if err != nil {
		return err
	}

	err = validateMinValue(c.ModeIndicator.UI.BorderWidth, 0, "mode_indicator.ui.border_width")
	if err != nil {
		return err
	}

	err = validateMinValue(c.ModeIndicator.UI.PaddingX, -1, "mode_indicator.ui.padding_x")
	if err != nil {
		return err
	}

	err = validateMinValue(c.ModeIndicator.UI.PaddingY, -1, "mode_indicator.ui.padding_y")
	if err != nil {
		return err
	}

	err = validateMinValue(c.ModeIndicator.UI.BorderRadius, -1, "mode_indicator.ui.border_radius")
	if err != nil {
		return err
	}

	err = validateColors([]colorField{
		{c.ModeIndicator.UI.BackgroundColorLight, "mode_indicator.ui.background_color_light"},
		{c.ModeIndicator.UI.BackgroundColorDark, "mode_indicator.ui.background_color_dark"},
		{c.ModeIndicator.UI.TextColorLight, "mode_indicator.ui.text_color_light"},
		{c.ModeIndicator.UI.TextColorDark, "mode_indicator.ui.text_color_dark"},
		{c.ModeIndicator.UI.BorderColorLight, "mode_indicator.ui.border_color_light"},
		{c.ModeIndicator.UI.BorderColorDark, "mode_indicator.ui.border_color_dark"},
	})
	if err != nil {
		return err
	}

	// Validate per-mode color overrides (only when non-empty).
	modes := []struct {
		cfg  ModeIndicatorModeConfig
		name string
	}{
		{c.ModeIndicator.Scroll, "scroll"},
		{c.ModeIndicator.Hints, "hints"},
		{c.ModeIndicator.Grid, "grid"},
		{c.ModeIndicator.RecursiveGrid, "recursive_grid"},
	}

	for _, mode := range modes {
		err = validateColors([]colorField{
			{
				mode.cfg.BackgroundColorLight,
				"mode_indicator." + mode.name + ".background_color_light",
			},
			{
				mode.cfg.BackgroundColorDark,
				"mode_indicator." + mode.name + ".background_color_dark",
			},
			{mode.cfg.TextColorLight, "mode_indicator." + mode.name + ".text_color_light"},
			{mode.cfg.TextColorDark, "mode_indicator." + mode.name + ".text_color_dark"},
			{mode.cfg.BorderColorLight, "mode_indicator." + mode.name + ".border_color_light"},
			{mode.cfg.BorderColorDark, "mode_indicator." + mode.name + ".border_color_dark"},
		})
		if err != nil {
			return err
		}
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

// ResolvedExitKeys returns the effective exit keys for a given mode.
// Per-mode keys are merged on top of the global keys (additive).
// If the mode has no per-mode keys, the global keys are returned as-is.
// When global keys are empty, falls back to ["Escape"] to stay consistent
// with the runtime fallback in resolveExitKeysForCurrentMode.
func (c *Config) ResolvedExitKeys(modeName string) []string {
	globalKeys := c.General.ModeExitKeys

	if len(globalKeys) == 0 {
		globalKeys = []string{"Escape"}
	}

	var modeKeys []string
	switch modeName {
	case modeNameHints:
		modeKeys = c.Hints.ModeExitKeys
	case modeNameGrid:
		modeKeys = c.Grid.ModeExitKeys
	case modeNameRecursiveGrid:
		modeKeys = c.RecursiveGrid.ModeExitKeys
	case modeNameScroll:
		modeKeys = c.Scroll.ModeExitKeys
	}

	return MergeExitKeys(globalKeys, modeKeys)
}

// CustomHotkeysForMode returns the custom_hotkeys map for the given mode name.
// These are per-mode hotkeys that are only active while that mode is active,
// using the same action syntax as [hotkeys] (e.g. "exec ...", "action ...", "hints", etc.).
func (c *Config) CustomHotkeysForMode(modeName string) map[string]StringOrStringArray {
	switch modeName {
	case modeNameHints:
		return c.Hints.CustomHotkeys
	case modeNameGrid:
		return c.Grid.CustomHotkeys
	case modeNameRecursiveGrid:
		return c.RecursiveGrid.CustomHotkeys
	case modeNameScroll:
		return c.Scroll.CustomHotkeys
	default:
		return nil
	}
}

// MergeExitKeys merges global and per-mode exit keys, deduplicating by normalized form.
// Per-mode keys are appended after global keys. If modeKeys is empty, globalKeys is returned as-is.
func MergeExitKeys(globalKeys, modeKeys []string) []string {
	if len(modeKeys) == 0 {
		return globalKeys
	}

	merged := make([]string, 0, len(globalKeys)+len(modeKeys))
	merged = append(merged, globalKeys...)

	seen := make(map[string]bool, len(globalKeys))
	for _, k := range globalKeys {
		seen[NormalizeKeyForComparison(k)] = true
	}

	for _, k := range modeKeys {
		normalized := NormalizeKeyForComparison(k)
		if !seen[normalized] {
			merged = append(merged, k)
			seen[normalized] = true
		}
	}

	return merged
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

// MouseActionRefreshDelayForApp returns the mouse action refresh delay for a specific app bundle ID.
// It first checks for app-specific configuration, then falls back to the global setting.
func (c *Config) MouseActionRefreshDelayForApp(bundleID string) int {
	if len(c.Hints.AppConfigs) > 0 {
		for _, appConfig := range c.Hints.AppConfigs {
			if appConfig.BundleID == bundleID {
				if appConfig.MouseActionRefreshDelay != nil {
					return *appConfig.MouseActionRefreshDelay
				}

				break
			}
		}
	}

	return c.Hints.MouseActionRefreshDelay
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
	if !c.Hints.Enabled && !c.Grid.Enabled && !c.RecursiveGrid.Enabled {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"at least one mode must be enabled: hints.enabled, grid.enabled, or recursive_grid.enabled",
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

	err = validateAutoExitActions(
		c.Scroll.AutoExitActions,
		"scroll.auto_exit_actions",
	)
	if err != nil {
		return err
	}

	// Validate per-mode exit keys
	if len(c.Scroll.ModeExitKeys) > 0 {
		err = validatePerModeExitKeysFormat(c.Scroll.ModeExitKeys, "scroll.mode_exit_keys")
		if err != nil {
			return err
		}

		err = checkPerModeExitKeysScrollBindingConflicts(
			c.Scroll.ModeExitKeys,
			c.Scroll.KeyBindings,
			"scroll.mode_exit_keys",
		)
		if err != nil {
			return err
		}
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

		for i := range len(parts) - 1 {
			modifier := strings.TrimSpace(parts[i])
			if !isValidModifier(modifier) {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s has invalid modifier '%s' in '%s' (valid: Cmd, Ctrl, Alt, Shift, Option, and Right*/Left* variants e.g. RightCmd, LeftShift)",
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
// Uses the centralized validNamedKeys registry for named key validation.
func isValidScrollKeyName(key string) bool {
	if IsValidNamedKey(key) {
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
