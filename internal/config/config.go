package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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

// Mode name constants used in config lookups (HotkeysForMode, validation).
// These mirror domain.ModeName* but are defined here to avoid a circular import.
const (
	ModeNameHints         = "hints"
	ModeNameGrid          = "grid"
	ModeNameRecursiveGrid = "recursive_grid"
	ModeNameScroll        = "scroll"
	ModeNameMonitorSelect = "monitor_select"
)

// Display-form key name constants used in config files and maps.
const (
	KeyDisplaySpace     = "Space"
	KeyDisplayEscape    = "Escape"
	KeyDisplayBackspace = "Backspace"
	KeyDisplayDown      = "Down"
	KeyDisplayLeft      = "Left"
	KeyDisplayRight     = "Right"
	KeyReturn           = "Return"
)

// Common action command constants.
const (
	CmdToggleCursorFollowSelection = "toggle-cursor-follow-selection"
	CmdMoveMouseUp                 = "action move_mouse_relative --dx=0 --dy=-10"
)

// Hotkey modifier combo key names.
const (
	KeyComboShiftL = "Shift+L"
	KeyComboShiftR = "Shift+R"
	KeyComboShiftM = "Shift+M"
	KeyComboShiftI = "Shift+I"
	KeyComboShiftU = "Shift+U"
)

// Common action strings.
const (
	CmdIdle           = "idle"
	CmdLeftClick      = "action left_click"
	CmdRightClick     = "action right_click"
	CmdMiddleClick    = "action middle_click"
	CmdMouseDown      = "action mouse_down"
	CmdMouseUp        = "action mouse_up"
	CmdGoTop          = "action go_top"
	CmdBackspace      = "action backspace"
	CmdMoveMouseDown  = "action move_mouse_relative --dx=0 --dy=10"
	CmdMoveMouseLeft  = "action move_mouse_relative --dx=-10 --dy=0"
	CmdMoveMouseRight = "action move_mouse_relative --dx=10 --dy=0"
)

// Placement strings for UI configuration.
const placementBottom = "bottom"

// DisabledSentinel is a special action value that removes a default hotkey binding.
// Use it in [hotkeys] or [<mode>.hotkeys] to disable a specific default:
//
//	[scroll.hotkeys]
//	"j" = "__disabled__"   # removes the default "j" = "action scroll_down"
const DisabledSentinel = "__disabled__"

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
	modifierNameCmd      = "cmd"
	modifierNameCtrl     = "ctrl"
	modifierNameAlt      = "alt"
	modifierNameShift    = "shift"
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
	KeyDisplaySpace:     true,
	KeyReturn:           true,
	"Enter":             true, // alias for Return
	KeyDisplayEscape:    true,
	"Tab":               true,
	"Delete":            true,
	KeyDisplayBackspace: true, // alias for Delete on macOS
	// Navigation keys
	"Up":            true,
	KeyDisplayDown:  true,
	KeyDisplayLeft:  true,
	KeyDisplayRight: true,
	"Home":          true,
	"End":           true,
	"PageUp":        true,
	"PageDown":      true,
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

	// Normalize key aliases inside modifier combos.
	// The switch above only handles bare "enter" / "backspace" etc., but users may
	// write "Shift+Enter" which lowercases to "shift+enter". The event tap always
	// produces the canonical form "shift+return", so we must resolve the alias here.
	key = normalizeKeyAliasesInCombo(key)

	// Normalize modifier aliases like "Primary" to the platform-native token
	// so shared config can map to Cmd on macOS and Ctrl elsewhere.
	key = normalizeModifierAliasesInCombo(key, runtime.GOOS)

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
		case modifierNameCmd, modifierNameCtrl, modifierNameAlt:
			return true
		}
	}

	return false
}

func primaryModifierTokenForOS(goos string) string {
	if goos == "darwin" {
		return "cmd"
	}

	return modifierNameCtrl
}

func normalizeModifierTokenForOS(token, goos string) string {
	switch strings.TrimSpace(strings.ToLower(token)) {
	case "primary":
		return primaryModifierTokenForOS(goos)
	case modifierNameCmd, "command", "super", "meta", "rightcmd", "leftcmd":
		return modifierNameCmd
	case modifierNameCtrl, "control", "rightctrl", "leftctrl":
		return modifierNameCtrl
	case modifierNameAlt, "option", "rightalt", "leftalt", "rightoption", "leftoption":
		return modifierNameAlt
	case modifierNameShift, "rightshift", "leftshift":
		return modifierNameShift
	default:
		return strings.TrimSpace(strings.ToLower(token))
	}
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

func normalizeModifierAliasesInCombo(key, goos string) string {
	parts := strings.Split(key, "+")
	if len(parts) < minimumModifierParts {
		return key
	}

	for idx := range len(parts) - 1 {
		parts[idx] = normalizeModifierTokenForOS(parts[idx], goos)
	}

	return strings.Join(parts, "+")
}

func displayModifierTokenForOS(token, goos string) string {
	switch token {
	case modifierNameCmd:
		if goos == "darwin" {
			return "Cmd"
		}

		return "Super"
	case modifierNameCtrl:
		return "Ctrl"
	case modifierNameAlt:
		return "Alt"
	case modifierNameShift:
		return "Shift"
	default:
		return token
	}
}

// CanonicalHotkeyForPlatform rewrites shared modifier aliases like "Primary"
// into the concrete tokens expected by the current platform backend.
func CanonicalHotkeyForPlatform(hotkey string) string {
	return canonicalHotkeyForOS(hotkey, runtime.GOOS)
}

func canonicalHotkeyForOS(hotkey, goos string) string {
	if hotkey == "" {
		return hotkey
	}

	parts := strings.Split(hotkey, "+")

	for idx := range len(parts) - 1 {
		parts[idx] = displayModifierTokenForOS(normalizeModifierTokenForOS(parts[idx], goos), goos)
	}

	last := strings.TrimSpace(parts[len(parts)-1])
	if canonical, ok := CanonicalNamedKeyForm(last); ok {
		parts[len(parts)-1] = canonical
	} else {
		parts[len(parts)-1] = last
	}

	return strings.Join(parts, "+")
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

// Config represents the complete application configuration structure.
type Config struct {
	General         GeneralConfig         `json:"general"         toml:"general"`
	Theme           ThemeConfig           `json:"theme"           toml:"theme"`
	Hotkeys         HotkeysConfig         `json:"hotkeys"         toml:"-"`
	Hints           HintsConfig           `json:"hints"           toml:"hints"`
	Grid            GridConfig            `json:"grid"            toml:"grid"`
	RecursiveGrid   RecursiveGridConfig   `json:"recursiveGrid"   toml:"recursive_grid"`
	MonitorSelect   MonitorSelectConfig   `json:"monitorSelect"   toml:"monitor_select"`
	VirtualPointer  VirtualPointerConfig  `json:"virtualPointer"  toml:"virtual_pointer"`
	MouseAction     MouseActionConfig     `json:"mouseAction"     toml:"mouse_action_indicator"`
	Scroll          ScrollConfig          `json:"scroll"          toml:"scroll"`
	ModeIndicator   ModeIndicatorConfig   `json:"modeIndicator"   toml:"mode_indicator"`
	StickyModifiers StickyModifiersConfig `json:"stickyModifiers" toml:"sticky_modifiers"`
	Logging         LoggingConfig         `json:"logging"         toml:"logging"`
	SmoothCursor    SmoothCursorConfig    `json:"smoothCursor"    toml:"smooth_cursor"`
	SmoothScroll    SmoothScrollConfig    `json:"smoothScroll"    toml:"smooth_scroll"`
	HeldRepeat      HeldRepeatConfig      `json:"heldRepeat"      toml:"held_repeat"`
	Systray         SystrayConfig         `json:"systray"         toml:"systray"`

	// AppConfigs holds per-application overrides for global hotkeys, similar to
	// per-mode [[<mode>.app_configs]].  Each entry can override or disable
	// specific global bindings (via __disabled__) when the identified app
	// is focused.  Populated by the TOML struct decoder from [[app_configs]].
	AppConfigs []AppConfig `json:"appConfigs" toml:"app_configs"`
}

// ThemePalette defines a semantic base palette for one appearance mode.
// Component defaults are derived from these solid colors by applying alpha.
type ThemePalette struct {
	Surface     string `json:"surface"     toml:"surface"`
	Accent      string `json:"accent"      toml:"accent"`
	AccentAlt   string `json:"accentAlt"   toml:"accent_alt"`
	OnAccentAlt string `json:"onAccentAlt" toml:"on_accent_alt"`
	Text        string `json:"text"        toml:"text"`
}

// ThemeConfig defines the light and dark palettes used to derive UI defaults.
type ThemeConfig struct {
	Light ThemePalette `json:"light" toml:"light"`
	Dark  ThemePalette `json:"dark"  toml:"dark"`
}

// GeneralConfig defines general application-wide settings.
type GeneralConfig struct {
	ExcludedApps                      []string `json:"excludedApps"                      toml:"excluded_apps"`
	PassthroughUnboundedKeys          bool     `json:"passthroughUnboundedKeys"          toml:"passthrough_unbounded_keys"`
	ShouldExitAfterPassthrough        bool     `json:"shouldExitAfterPassthrough"        toml:"should_exit_after_passthrough"`
	PassthroughUnboundedKeysBlacklist []string `json:"passthroughUnboundedKeysBlacklist" toml:"passthrough_unbounded_keys_blacklist"`
	HideOverlayInScreenShare          bool     `json:"hideOverlayInScreenShare"          toml:"hide_overlay_in_screen_share"`
	KBLayoutToUse                     string   `json:"kbLayoutToUse"                     toml:"kb_layout_to_use"`
	ExecShell                         string   `json:"execShell"                         toml:"exec_shell"`
	ExecShellArgs                     []string `json:"execShellArgs"                     toml:"exec_shell_args"`
}

// ModeIndicatorUI defines the visual/appearance settings for the mode indicator.
type ModeIndicatorUI struct {
	FontSize         int    `json:"fontSize"         toml:"font_size"`
	FontFamily       string `json:"fontFamily"       toml:"font_family"`
	BackgroundColor  Color  `json:"backgroundColor"  toml:"background_color"`
	TextColor        Color  `json:"textColor"        toml:"text_color"`
	BorderColor      Color  `json:"borderColor"      toml:"border_color"`
	BorderWidth      int    `json:"borderWidth"      toml:"border_width"`
	PaddingX         int    `json:"paddingX"         toml:"padding_x"`
	PaddingY         int    `json:"paddingY"         toml:"padding_y"`
	BorderRadius     int    `json:"borderRadius"     toml:"border_radius"`
	IndicatorXOffset int    `json:"indicatorXOffset" toml:"indicator_x_offset"`
	IndicatorYOffset int    `json:"indicatorYOffset" toml:"indicator_y_offset"`
}

// ModeIndicatorModeConfig defines per-mode settings for the mode indicator.
// Text is the label shown in the indicator; empty string hides the label.
// Color overrides are optional; empty string inherits from [mode_indicator.ui].
type ModeIndicatorModeConfig struct {
	Enabled         bool   `json:"enabled"         toml:"enabled"`
	Text            string `json:"text"            toml:"text"`
	BackgroundColor Color  `json:"backgroundColor" toml:"background_color"`
	TextColor       Color  `json:"textColor"       toml:"text_color"`
	BorderColor     Color  `json:"borderColor"     toml:"border_color"`
}

// ModeIndicatorConfig defines per-mode indicator visibility and appearance.
type ModeIndicatorConfig struct {
	Scroll        ModeIndicatorModeConfig `json:"scroll"        toml:"scroll"`
	Hints         ModeIndicatorModeConfig `json:"hints"         toml:"hints"`
	Grid          ModeIndicatorModeConfig `json:"grid"          toml:"grid"`
	RecursiveGrid ModeIndicatorModeConfig `json:"recursiveGrid" toml:"recursive_grid"`
	MonitorSelect ModeIndicatorModeConfig `json:"monitorSelect" toml:"monitor_select"`
	UI            ModeIndicatorUI         `json:"ui"            toml:"ui"`
}

// StickyModifiersUI defines the visual/appearance settings for the sticky modifiers indicator.
type StickyModifiersUI struct {
	FontSize         int    `json:"fontSize"         toml:"font_size"`
	FontFamily       string `json:"fontFamily"       toml:"font_family"`
	BackgroundColor  Color  `json:"backgroundColor"  toml:"background_color"`
	TextColor        Color  `json:"textColor"        toml:"text_color"`
	BorderColor      Color  `json:"borderColor"      toml:"border_color"`
	BorderWidth      int    `json:"borderWidth"      toml:"border_width"`
	PaddingX         int    `json:"paddingX"         toml:"padding_x"`
	PaddingY         int    `json:"paddingY"         toml:"padding_y"`
	BorderRadius     int    `json:"borderRadius"     toml:"border_radius"`
	IndicatorXOffset int    `json:"indicatorXOffset" toml:"indicator_x_offset"`
	IndicatorYOffset int    `json:"indicatorYOffset" toml:"indicator_y_offset"`
}

// StickyModifiersConfig defines settings for the sticky modifiers feature.
type StickyModifiersConfig struct {
	Enabled        bool              `json:"enabled"        toml:"enabled"`
	TapMaxDuration int               `json:"tapMaxDuration" toml:"tap_max_duration"`
	UI             StickyModifiersUI `json:"ui"             toml:"ui"`
}

// AppConfig defines application-specific settings for role customization.
type AppConfig struct {
	BundleID             string                         `json:"bundleId"             toml:"bundle_id"`
	Strategy             string                         `json:"strategy"             toml:"strategy"`
	LabelDirection       string                         `json:"labelDirection"       toml:"label_direction"`
	AdditionalClickable  []string                       `json:"additionalClickable"  toml:"additional_clickable_roles"`
	IgnoreClickableCheck *bool                          `json:"ignoreClickableCheck" toml:"ignore_clickable_check,omitempty"`
	VisibleCheckEnabled  *bool                          `json:"visibleCheckEnabled"  toml:"visible_check_enabled,omitempty"`
	ScrollStep           *int                           `json:"scrollStep"           toml:"scroll_step,omitempty"`
	ScrollStepHalf       *int                           `json:"scrollStepHalf"       toml:"scroll_step_half,omitempty"`
	ScrollStepFull       *int                           `json:"scrollStepFull"       toml:"scroll_step_full,omitempty"`
	Hotkeys              map[string]StringOrStringArray `json:"hotkeys"              toml:"hotkeys"`
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
	// Bindings is never populated by the TOML struct decoder — it is always
	// overwritten by the raw-map processing in service.go.  Both this field
	// and the parent Config.Hotkeys are tagged toml:"-" so the encoder skips
	// them entirely; Save writes the flat [hotkeys] section manually instead.
	Bindings map[string][]string `json:"bindings" toml:"-"`
}

// ScrollConfig defines the behavior and appearance settings for scroll mode.
type ScrollConfig struct {
	ScrollStep     int `json:"scrollStep"     toml:"scroll_step"`
	ScrollStepHalf int `json:"scrollStepHalf" toml:"scroll_step_half"`
	ScrollStepFull int `json:"scrollStepFull" toml:"scroll_step_full"`

	InvertScroll bool `json:"invertScroll" toml:"invert_scroll"`

	AppConfigs []AppConfig `json:"appConfigs" toml:"app_configs"`

	Hotkeys map[string]StringOrStringArray `json:"hotkeys" toml:"-"`
}

// MonitorSelectUI defines the visual/appearance settings for monitor_select mode.
type MonitorSelectUI struct {
	FontSize           int    `json:"fontSize"           toml:"font_size"`
	FontFamily         string `json:"fontFamily"         toml:"font_family"`
	BorderRadius       int    `json:"borderRadius"       toml:"border_radius"`
	PaddingX           int    `json:"paddingX"           toml:"padding_x"`
	PaddingY           int    `json:"paddingY"           toml:"padding_y"`
	BorderWidth        int    `json:"borderWidth"        toml:"border_width"`
	BackgroundColor    Color  `json:"backgroundColor"    toml:"background_color"`
	TextColor          Color  `json:"textColor"          toml:"text_color"`
	MatchedTextColor   Color  `json:"matchedTextColor"   toml:"matched_text_color"`
	BorderColor        Color  `json:"borderColor"        toml:"border_color"`
	BackdropColor      Color  `json:"backdropColor"      toml:"backdrop_color"`
	SubtitleFontSize   int    `json:"subtitleFontSize"   toml:"subtitle_font_size"`
	SubtitleFontFamily string `json:"subtitleFontFamily" toml:"subtitle_font_family"`
	SubtitleTextColor  Color  `json:"subtitleTextColor"  toml:"subtitle_text_color"`
}

// MonitorSelectConfig defines behavior and appearance settings for monitor_select mode.
type MonitorSelectConfig struct {
	Enabled    bool                           `json:"enabled"    toml:"enabled"`
	Characters string                         `json:"characters" toml:"characters"`
	UI         MonitorSelectUI                `json:"ui"         toml:"ui"`
	Hotkeys    map[string]StringOrStringArray `json:"hotkeys"    toml:"-"`
}

// HintsUI defines the visual/appearance settings for hints mode.
type HintsUI struct {
	FontSize         int    `json:"fontSize"         toml:"font_size"`
	FontFamily       string `json:"fontFamily"       toml:"font_family"`
	BorderRadius     int    `json:"borderRadius"     toml:"border_radius"`
	PaddingX         int    `json:"paddingX"         toml:"padding_x"`
	PaddingY         int    `json:"paddingY"         toml:"padding_y"`
	BorderWidth      int    `json:"borderWidth"      toml:"border_width"`
	Placement        string `json:"placement"        toml:"placement"`
	BackgroundColor  Color  `json:"backgroundColor"  toml:"background_color"`
	TextColor        Color  `json:"textColor"        toml:"text_color"`
	MatchedTextColor Color  `json:"matchedTextColor" toml:"matched_text_color"`
	BorderColor      Color  `json:"borderColor"      toml:"border_color"`
}

// BoundaryHighlightUI defines the optional target boundary highlight for hints mode.
type BoundaryHighlightUI struct {
	Enabled         bool  `json:"enabled"         toml:"enabled"`
	BorderWidth     int   `json:"borderWidth"     toml:"border_width"`
	BorderRadius    int   `json:"borderRadius"    toml:"border_radius"`
	BorderColor     Color `json:"borderColor"     toml:"border_color"`
	BackgroundColor Color `json:"backgroundColor" toml:"background_color"`
}

// SearchInputUI defines the visual/appearance settings for hints text search.
type SearchInputUI struct {
	FontSize        int    `json:"fontSize"        toml:"font_size"`
	FontFamily      string `json:"fontFamily"      toml:"font_family"`
	BorderRadius    int    `json:"borderRadius"    toml:"border_radius"`
	PaddingX        int    `json:"paddingX"        toml:"padding_x"`
	PaddingY        int    `json:"paddingY"        toml:"padding_y"`
	BorderWidth     int    `json:"borderWidth"     toml:"border_width"`
	Position        string `json:"position"        toml:"position"`
	XOffset         int    `json:"xOffset"         toml:"x_offset"`
	YOffset         int    `json:"yOffset"         toml:"y_offset"`
	Width           int    `json:"width"           toml:"width"`
	BackgroundColor Color  `json:"backgroundColor" toml:"background_color"`
	TextColor       Color  `json:"textColor"       toml:"text_color"`
	BorderColor     Color  `json:"borderColor"     toml:"border_color"`
}

// HintsVisionConfig defines tunable settings for vision-based hint detection.
type HintsVisionConfig struct {
	DetectText             bool    `json:"detectText"             toml:"detect_text"`
	DetectRectangles       bool    `json:"detectRectangles"       toml:"detect_rectangles"`
	RequestTimeoutMS       int     `json:"requestTimeoutMs"       toml:"request_timeout_ms"`
	MinimumConfidence      float64 `json:"minimumConfidence"      toml:"minimum_confidence"`
	MergeIOUThreshold      float64 `json:"mergeIouThreshold"      toml:"merge_iou_threshold"`
	RectangleMaxCandidates int     `json:"rectangleMaxCandidates" toml:"rectangle_max_candidates"`
	RectangleMinSize       float64 `json:"rectangleMinSize"       toml:"rectangle_min_size"`
	RectangleMinAspect     float64 `json:"rectangleMinAspect"     toml:"rectangle_min_aspect"`
	RectangleMaxAspect     float64 `json:"rectangleMaxAspect"     toml:"rectangle_max_aspect"`

	ButtonMinConfidence           float64 `json:"buttonMinConfidence"           toml:"button_min_confidence"`
	ButtonMinAspect               float64 `json:"buttonMinAspect"               toml:"button_min_aspect"`
	ButtonMaxAspect               float64 `json:"buttonMaxAspect"               toml:"button_max_aspect"`
	ButtonIconMaxSize             int     `json:"buttonIconMaxSize"             toml:"button_icon_max_size"`
	LinkMinAspect                 float64 `json:"linkMinAspect"                 toml:"link_min_aspect"`
	LinkMaxHeight                 int     `json:"linkMaxHeight"                 toml:"link_max_height"`
	LinkMinWidth                  int     `json:"linkMinWidth"                  toml:"link_min_width"`
	ImageMinSize                  int     `json:"imageMinSize"                  toml:"image_min_size"`
	CheckboxMaxSize               int     `json:"checkboxMaxSize"               toml:"checkbox_max_size"`
	GenericClickableMinConfidence float64 `json:"genericClickableMinConfidence" toml:"generic_clickable_min_confidence"`
}

// Strategy constants for element detection.
const (
	StrategyAXTree = "axtree"
	StrategyVision = "vision"
)

// Label direction constants for hint label enumeration.
const (
	// LabelDirectionReverse spreads labels across the alphabet by varying the
	// first character so same-prefix labels never cluster together.
	LabelDirectionReverse = "reverse"

	// LabelDirectionNormal uses the original prefix-avoidance algorithm that
	// prefers shorter labels. This is the default.
	LabelDirectionNormal = "normal"
)

// HintsConfig defines the visual and behavioral settings for hints mode.
type HintsConfig struct {
	Enabled           bool                `json:"enabled"           toml:"enabled"`
	Strategy          string              `json:"strategy"          toml:"strategy"`
	HintCharacters    string              `json:"hintCharacters"    toml:"hint_characters"`
	LabelDirection    string              `json:"labelDirection"    toml:"label_direction"`
	MaxDepth          int                 `json:"maxDepth"          toml:"max_depth"`
	UI                HintsUI             `json:"ui"                toml:"ui"`
	SearchInputUI     SearchInputUI       `json:"searchInputUi"     toml:"search_input_ui"`
	BoundaryHighlight BoundaryHighlightUI `json:"boundaryHighlight" toml:"boundary_highlight"`
	Vision            HintsVisionConfig   `json:"vision"            toml:"vision"`

	IncludeMenubarHints           bool                `json:"includeMenubarHints"           toml:"include_menubar_hints"`
	AdditionalMenubarHintsTargets []string            `json:"additionalMenubarHintsTargets" toml:"additional_menubar_hints_targets"`
	IncludeDockHints              bool                `json:"includeDockHints"              toml:"include_dock_hints"`
	IncludeNCHints                bool                `json:"includeNcHints"                toml:"include_nc_hints"`
	IncludeStageManagerHints      bool                `json:"includeStageManagerHints"      toml:"include_stage_manager_hints"`
	IncludePIPHints               bool                `json:"includePipHints"               toml:"include_pip_hints"`
	IncludeScreenCaptureHints     bool                `json:"includeScreenCaptureHints"     toml:"include_screen_capture_hints"`
	DetectMissionControl          bool                `json:"detectMissionControl"          toml:"detect_mission_control"`
	OnMissionControlActivated     StringOrStringArray `json:"onMissionControlActivated"     toml:"on_mission_control_activated"`
	OnMissionControlDeactivated   StringOrStringArray `json:"onMissionControlDeactivated"   toml:"on_mission_control_deactivated"`

	ClickableRoles       []string `json:"clickableRoles"       toml:"clickable_roles"`
	IgnoreClickableCheck bool     `json:"ignoreClickableCheck" toml:"ignore_clickable_check"`
	VisibleCheckEnabled  bool     `json:"visibleCheckEnabled"  toml:"visible_check_enabled"`

	AppConfigs []AppConfig `json:"appConfigs" toml:"app_configs"`

	Hotkeys map[string]StringOrStringArray `json:"hotkeys" toml:"-"`
}

// GridUI defines the visual/appearance settings for grid mode.
type GridUI struct {
	FontSize    int    `json:"fontSize"    toml:"font_size"`
	FontFamily  string `json:"fontFamily"  toml:"font_family"`
	BorderWidth int    `json:"borderWidth" toml:"border_width"`

	BackgroundColor        Color `json:"backgroundColor"        toml:"background_color"`
	TextColor              Color `json:"textColor"              toml:"text_color"`
	MatchedTextColor       Color `json:"matchedTextColor"       toml:"matched_text_color"`
	MatchedBackgroundColor Color `json:"matchedBackgroundColor" toml:"matched_background_color"`
	MatchedBorderColor     Color `json:"matchedBorderColor"     toml:"matched_border_color"`
	BorderColor            Color `json:"borderColor"            toml:"border_color"`
}

// GridConfig defines the visual and behavioral settings for grid mode.
type GridConfig struct {
	Enabled      bool   `json:"enabled"      toml:"enabled"`
	Characters   string `json:"characters"   toml:"characters"`
	SublayerKeys string `json:"sublayerKeys" toml:"sublayer_keys"`
	// Optional custom labels for rows and columns
	// If not provided, labels will be inferred from characters
	RowLabels       string `json:"rowLabels"       toml:"row_labels"`
	ColLabels       string `json:"colLabels"       toml:"col_labels"`
	UI              GridUI `json:"ui"              toml:"ui"`
	LiveMatchUpdate bool   `json:"liveMatchUpdate" toml:"live_match_update"`
	HideUnmatched   bool   `json:"hideUnmatched"   toml:"hide_unmatched"`
	PrewarmEnabled  bool   `json:"prewarmEnabled"  toml:"prewarm_enabled"`
	EnableGC        bool   `json:"enableGc"        toml:"enable_gc"`

	AppConfigs []AppConfig `json:"appConfigs" toml:"app_configs"`

	Hotkeys map[string]StringOrStringArray `json:"hotkeys" toml:"-"`
}

// RecursiveGridUI defines the visual/appearance settings for recursive-grid mode.
type RecursiveGridUI struct {
	LineColor                       Color   `json:"lineColor"                       toml:"line_color"`
	LineWidth                       int     `json:"lineWidth"                       toml:"line_width"`
	HighlightColor                  Color   `json:"highlightColor"                  toml:"highlight_color"`
	TextColor                       Color   `json:"textColor"                       toml:"text_color"`
	FontSize                        int     `json:"fontSize"                        toml:"font_size"`
	FontFamily                      string  `json:"fontFamily"                      toml:"font_family"`
	LabelBackground                 bool    `json:"labelBackground"                 toml:"label_background"`
	LabelBackgroundColor            Color   `json:"labelBackgroundColor"            toml:"label_background_color"`
	LabelBackgroundPaddingX         int     `json:"labelBackgroundPaddingX"         toml:"label_background_padding_x"`
	LabelBackgroundPaddingY         int     `json:"labelBackgroundPaddingY"         toml:"label_background_padding_y"`
	LabelBackgroundBorderRadius     int     `json:"labelBackgroundBorderRadius"     toml:"label_background_border_radius"`
	LabelBackgroundBorderWidth      int     `json:"labelBackgroundBorderWidth"      toml:"label_background_border_width"`
	LabelChar                       string  `json:"labelChar"                       toml:"label_char"`
	LabelAutohideMultiplier         float64 `json:"labelAutohideMultiplier"         toml:"label_autohide_multiplier"`
	SubKeyPreview                   bool    `json:"subKeyPreview"                   toml:"sub_key_preview"`
	SubKeyPreviewFontSize           int     `json:"subKeyPreviewFontSize"           toml:"sub_key_preview_font_size"`
	SubKeyPreviewAutohideMultiplier float64 `json:"subKeyPreviewAutohideMultiplier" toml:"sub_key_preview_autohide_multiplier"`
	SubKeyPreviewTextColor          Color   `json:"subKeyPreviewTextColor"          toml:"sub_key_preview_text_color"`
	SubKeyPreviewLabelChar          string  `json:"subKeyPreviewLabelChar"          toml:"sub_key_preview_label_char"`
}

// RecursiveGridLayerConfig defines per-depth overrides for the recursive grid.
// Depths not listed in the Layers slice use the top-level GridCols/GridRows/Keys defaults.
type RecursiveGridLayerConfig struct {
	Depth    int    `json:"depth"    toml:"depth"`
	GridCols int    `json:"gridCols" toml:"grid_cols"`
	GridRows int    `json:"gridRows" toml:"grid_rows"`
	Keys     string `json:"keys"     toml:"keys"`
}

// RecursiveGridAnimationConfig defines native recursive-grid animation settings.
type RecursiveGridAnimationConfig struct {
	Enabled    bool `json:"enabled"    toml:"enabled"`
	DurationMS int  `json:"durationMs" toml:"duration_ms"`
}

// RecursiveGridConfig defines the visual and behavioral settings for recursive-grid mode.
type RecursiveGridConfig struct {
	Enabled bool `json:"enabled" toml:"enabled"`
	// Animation configures native depth transition animations for recursive-grid on supported platforms.
	Animation RecursiveGridAnimationConfig `json:"animation" toml:"animation"`
	// Grid dimensions: columns and rows (default: 2x2)
	GridCols int `json:"gridCols" toml:"grid_cols"`
	GridRows int `json:"gridRows" toml:"grid_rows"`
	// Key bindings (warpd convention for 2x2: u=TL, i=TR, j=BL, k=BR)
	Keys string          `json:"keys" toml:"keys"`
	UI   RecursiveGridUI `json:"ui"   toml:"ui"`
	// Behavior
	MinSizeWidth  int `json:"minSizeWidth"  toml:"min_size_width"`  // Default: 1
	MinSizeHeight int `json:"minSizeHeight" toml:"min_size_height"` // Default: 1
	MaxDepth      int `json:"maxDepth"      toml:"max_depth"`       // Default: 10
	// Per-depth overrides for grid dimensions and keys.
	// Depths not listed here use the top-level GridCols/GridRows/Keys.
	Layers []RecursiveGridLayerConfig `json:"layers" toml:"layers"`

	AppConfigs []AppConfig `json:"appConfigs" toml:"app_configs"`

	Hotkeys map[string]StringOrStringArray `json:"hotkeys" toml:"-"`
}

// VirtualPointerUI defines the visual settings for the character-based virtual pointer.
type VirtualPointerUI struct {
	Char       string `json:"char"       toml:"char"`
	FontSize   int    `json:"fontSize"   toml:"font_size"`
	FontFamily string `json:"fontFamily" toml:"font_family"`
	TextColor  Color  `json:"textColor"  toml:"text_color"`
}

// VirtualPointerConfig defines settings for the hold-mode virtual pointer.
type VirtualPointerConfig struct {
	UI VirtualPointerUI `json:"ui" toml:"ui"`
}

// MouseActionUI defines the visual settings for mouse action indicators.
type MouseActionUI struct {
	Size            int    `json:"size"            toml:"size"`
	BorderWidth     int    `json:"borderWidth"     toml:"border_width"`
	BackgroundColor Color  `json:"backgroundColor" toml:"background_color"`
	BorderColor     Color  `json:"borderColor"     toml:"border_color"`
	Shape           string `json:"shape"           toml:"shape"`
}

// MouseActionAnimation defines animation settings for mouse action indicators.
type MouseActionAnimation struct {
	DurationMS   int     `json:"durationMs"   toml:"duration_ms"`
	StartScale   float64 `json:"startScale"   toml:"start_scale"`
	EndScale     float64 `json:"endScale"     toml:"end_scale"`
	StartOpacity float64 `json:"startOpacity" toml:"start_opacity"`
	EndOpacity   float64 `json:"endOpacity"   toml:"end_opacity"`
	Easing       string  `json:"easing"       toml:"easing"`
}

// MouseActionConfig defines settings for transient mouse action indicators.
type MouseActionConfig struct {
	Enabled   bool                 `json:"enabled"   toml:"enabled"`
	Actions   []string             `json:"actions"   toml:"actions"`
	UI        MouseActionUI        `json:"ui"        toml:"ui"`
	Animation MouseActionAnimation `json:"animation" toml:"animation"`
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
	LogLevel string `json:"logLevel" toml:"log_level"`
	LogFile  string `json:"logFile"  toml:"log_file"`

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

// SmoothScrollConfig defines smooth scroll animation settings.
type SmoothScrollConfig struct {
	Enabled          bool    `json:"enabled"          toml:"enabled"`
	Steps            int     `json:"steps"            toml:"steps"`
	MaxDuration      int     `json:"maxDuration"      toml:"max_duration"`
	DurationPerPixel float64 `json:"durationPerPixel" toml:"duration_per_pixel"`
}

// HeldRepeatConfig defines held-key repeat settings for scroll, page, and mouse-move actions.
type HeldRepeatConfig struct {
	Enabled      bool `json:"enabled"      toml:"enabled"`          // Master toggle for held-key repeat
	InitialDelay int  `json:"initialDelay" toml:"initial_delay_ms"` // Delay before first repeat fires (ms)
	Interval     int  `json:"interval"     toml:"interval_ms"`      // Interval between subsequent repeats (ms)
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

	err = c.ValidateTheme()
	if err != nil {
		return err
	}

	err = c.ValidateModes()
	if err != nil {
		return err
	}

	err = c.ValidateMonitorSelect()
	if err != nil {
		return err
	}

	err = c.ValidateModeIndicator()
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

	// Validate global hotkey app configs
	err = validateHotkeysAppConfigs("app_configs", c.AppConfigs)
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

	err = c.ValidateVirtualPointer()
	if err != nil {
		return err
	}

	err = c.ValidateMouseAction()
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

	// Validate smooth scroll settings
	err = c.ValidateSmoothScroll()
	if err != nil {
		return err
	}

	// Validate held-key repeat settings
	err = c.ValidateHeldRepeat()
	if err != nil {
		return err
	}

	// Validate top-level hotkey bindings
	err = c.ValidateHotkeyBindings()
	if err != nil {
		return err
	}

	// Validate per-mode custom hotkeys
	err = c.ValidateHotkeys()
	if err != nil {
		return err
	}

	return nil
}

// ValidateHotkeyBindings validates the top-level [hotkeys] key format and action strings.
func (c *Config) ValidateHotkeyBindings() error {
	// Check for duplicate normalized keys (mirrors checkHotkeysConflicts
	// for per-mode hotkeys). After merge, two keys that normalize identically
	// would cause ambiguous runtime behavior.
	seen := make(map[string]string, len(c.Hotkeys.Bindings))

	for key, actions := range c.Hotkeys.Bindings {
		fieldName := "hotkeys." + key
		if strings.TrimSpace(key) == "" {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"hotkeys contains an empty key",
			)
		}

		normalized := NormalizeKeyForComparison(key)
		if prev, ok := seen[normalized]; ok {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hotkeys has duplicate bindings (%q and %q)",
				prev,
				key,
			)
		}

		seen[normalized] = key

		err := ValidateHotkey(key, fieldName)
		if err != nil {
			return err
		}

		if len(actions) == 0 {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s cannot have an empty action list",
				fieldName,
			)
		}

		for actionIndex, actionStr := range actions {
			trimmed := strings.TrimSpace(actionStr)
			if trimmed == "" {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s[%d] cannot be empty",
					fieldName,
					actionIndex,
				)
			}

			err := validateHotkeyActionString(trimmed)
			if err != nil {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s[%d]: %v",
					fieldName,
					actionIndex,
					err,
				)
			}
		}
	}

	return nil
}

// ValidateGeneral validates general settings.
func (c *Config) ValidateGeneral() error {
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

	if c.General.ExecShell == "" {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"general.exec_shell cannot be empty",
		)
	}

	if !filepath.IsAbs(c.General.ExecShell) {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"general.exec_shell must be an absolute path (got: %q)",
			c.General.ExecShell,
		)
	}

	if len(c.General.ExecShellArgs) == 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"general.exec_shell_args cannot be empty",
		)
	}

	return nil
}

// ValidateModeIndicator validates the mode indicator configuration.
func (c *Config) ValidateModeIndicator() error {
	if c.ModeIndicator.UI.FontSize < 1 || c.ModeIndicator.UI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"mode_indicator.ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	err := validateMinValue(c.ModeIndicator.UI.BorderWidth, 0, "mode_indicator.ui.border_width")
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
		{c.ModeIndicator.UI.BackgroundColor, "mode_indicator.ui.background_color"},
		{c.ModeIndicator.UI.TextColor, "mode_indicator.ui.text_color"},
		{c.ModeIndicator.UI.BorderColor, "mode_indicator.ui.border_color"},
	})
	if err != nil {
		return err
	}

	// Validate per-mode color overrides (only when non-empty).
	modes := []struct {
		cfg  ModeIndicatorModeConfig
		name string
	}{
		{c.ModeIndicator.Scroll, ModeNameScroll},
		{c.ModeIndicator.Hints, ModeNameHints},
		{c.ModeIndicator.Grid, ModeNameGrid},
		{c.ModeIndicator.RecursiveGrid, ModeNameRecursiveGrid},
	}

	for _, mode := range modes {
		err = validateColors([]colorField{
			{mode.cfg.BackgroundColor, "mode_indicator." + mode.name + ".background_color"},
			{mode.cfg.TextColor, "mode_indicator." + mode.name + ".text_color"},
			{mode.cfg.BorderColor, "mode_indicator." + mode.name + ".border_color"},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// writeStringOrStringArrayMap writes a map[string]StringOrStringArray (or
// map[string][]string) as a TOML table to the given file.  Single-action
// entries are emitted as plain strings for backward compatibility; multi-action
// entries use TOML array syntax.  The section header (e.g. "[scroll.hotkeys]")
// is always written so that an empty map round-trips correctly.
//
// When defaults is non-nil, any default key not present in _map (after
// normalization) is emitted as "__disabled__" so that Save+LoadWithValidation
// round-trips correctly under merge-on-top-of-defaults semantics.
func writeStringOrStringArrayMap(
	file *os.File,
	sectionHeader string,
	_map map[string]StringOrStringArray,
	defaults map[string]StringOrStringArray,
) error {
	_, err := fmt.Fprintf(file, "\n[%s]\n", sectionHeader)
	if err != nil {
		return derrors.Wrap(
			err, derrors.CodeConfigIOFailed, "failed to write section header",
		)
	}

	if len(_map) == 0 {
		return nil
	}

	keys := make([]string, 0, len(_map))
	for k := range _map {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, key := range keys {
		actions := _map[key]

		if len(actions) == 0 {
			continue
		}

		var line string
		if len(actions) == 1 {
			line = fmt.Sprintf("%q = %q", key, actions[0])
		} else {
			quoted := make([]string, 0, len(actions))
			for _, a := range actions {
				quoted = append(quoted, fmt.Sprintf("%q", a))
			}

			line = fmt.Sprintf("%q = [%s]", key, strings.Join(quoted, ", "))
		}

		_, err := fmt.Fprintln(file, line)
		if err != nil {
			return derrors.Wrap(
				err, derrors.CodeConfigIOFailed, "failed to write binding",
			)
		}
	}

	// Emit __disabled__ markers for default bindings that were removed.
	if defaults != nil {
		disabledKeys := make([]string, 0)
		for defaultKey := range defaults {
			found := findNormalizedMapKey(_map, defaultKey)
			if _, exists := _map[found]; !exists {
				disabledKeys = append(disabledKeys, defaultKey)
			}
		}

		sort.Strings(disabledKeys)

		for _, key := range disabledKeys {
			line := fmt.Sprintf("%q = %q", key, DisabledSentinel)

			_, disabledErr := fmt.Fprintln(file, line)
			if disabledErr != nil {
				return derrors.Wrap(
					disabledErr,
					derrors.CodeConfigIOFailed,
					"failed to write disabled binding marker",
				)
			}
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

	// Encode the main config struct to TOML.
	// The Hotkeys field is tagged toml:"-" so the encoder skips it entirely;
	// we append the flat [hotkeys] section manually afterwards.
	encoder := toml.NewEncoder(file)

	encodeErr := encoder.Encode(c)
	if encodeErr != nil {
		return derrors.Wrap(encodeErr, derrors.CodeSerializationFailed, "failed to encode config")
	}

	// Write the [hotkeys] section so that LoadWithValidation sees
	// raw["hotkeys"] and merges user entries on top of defaults.  An empty
	// section (no keys) is the documented way to disable all hotkeys.
	//
	// Convert map[string][]string → map[string]StringOrStringArray so we can
	// reuse writeStringOrStringArrayMap (StringOrStringArray is []string).
	defaults := DefaultConfig()

	hotkeysSOSA := make(map[string]StringOrStringArray, len(c.Hotkeys.Bindings))
	for k, v := range c.Hotkeys.Bindings {
		hotkeysSOSA[k] = StringOrStringArray(v)
	}

	defaultHotkeysSOSA := make(map[string]StringOrStringArray, len(defaults.Hotkeys.Bindings))
	for k, v := range defaults.Hotkeys.Bindings {
		defaultHotkeysSOSA[k] = StringOrStringArray(v)
	}

	err := writeStringOrStringArrayMap(file, "hotkeys", hotkeysSOSA, defaultHotkeysSOSA)
	if err != nil {
		return err
	}

	// Write per-mode [<mode>.hotkeys] sections.
	// These fields are tagged toml:"-" so the encoder skips them; we write
	// them manually to preserve the single-string format for single-action
	// entries (backward compatibility).
	hotkeysSections := []struct {
		header   string
		hotkeys  map[string]StringOrStringArray
		defaults map[string]StringOrStringArray
	}{
		{"scroll.hotkeys", c.Scroll.Hotkeys, defaults.Scroll.Hotkeys},
		{"hints.hotkeys", c.Hints.Hotkeys, defaults.Hints.Hotkeys},
		{"grid.hotkeys", c.Grid.Hotkeys, defaults.Grid.Hotkeys},
		{
			"recursive_grid.hotkeys",
			c.RecursiveGrid.Hotkeys,
			defaults.RecursiveGrid.Hotkeys,
		},
		{
			"monitor_select.hotkeys",
			c.MonitorSelect.Hotkeys,
			defaults.MonitorSelect.Hotkeys,
		},
	}
	for _, section := range hotkeysSections {
		err = writeStringOrStringArrayMap(file, section.header, section.hotkeys, section.defaults)
		if err != nil {
			return err
		}
	}

	return closeErr
}

// HotkeysForMode returns the hotkeys map for the given mode name.
// These are per-mode hotkeys that are only active while that mode is active,
// using the same action syntax as [hotkeys] (e.g. "exec ...", "action ...", "hints", etc.).
func (c *Config) HotkeysForMode(modeName string) map[string]StringOrStringArray {
	return c.HotkeysForModeAndApp(modeName, "")
}

// HotkeysForModeAndApp returns the effective per-mode hotkeys map for the given mode
// and focused app bundle ID. For modes without app-specific overrides, it returns the
// base mode hotkeys unchanged. Hints, Grid, RecursiveGrid, and Scroll modes support
// per-app hotkey overrides through [[<mode>.app_configs]].
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
	}

	if appConfig == nil || len(appConfig.Hotkeys) == 0 {
		return base
	}

	merged := make(map[string]StringOrStringArray, len(base)+len(appConfig.Hotkeys))
	for key, actions := range base {
		copied := make(StringOrStringArray, len(actions))
		copy(copied, actions)
		merged[key] = copied
	}

	for key, actions := range appConfig.Hotkeys {
		canonicalKey := findNormalizedMapKey(merged, key)
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
				canonicalKey := findNormalizedMapKey(merged, key)
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
	seen := make(map[string]struct{}, len(base)+len(additional))
	result := make([]string, 0, len(base)+len(additional))

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

	err = validateScrollAppConfigs("scroll", c.Scroll.AppConfigs)
	if err != nil {
		return err
	}

	return nil
}

func (c *Config) baseHotkeysForMode(modeName string) map[string]StringOrStringArray {
	switch modeName {
	case ModeNameHints:
		return c.Hints.Hotkeys
	case ModeNameGrid:
		return c.Grid.Hotkeys
	case ModeNameRecursiveGrid:
		return c.RecursiveGrid.Hotkeys
	case ModeNameScroll:
		return c.Scroll.Hotkeys
	case ModeNameMonitorSelect:
		return c.MonitorSelect.Hotkeys
	default:
		return nil
	}
}

// ClickableRolesForApp returns the clickable roles for a specific app bundle ID.
// Starts from the merged config (root + app overrides), then appends
// AXMenuBarItem / AXDockItem when the corresponding hints flags are enabled.
func (c *HintsConfig) ClickableRolesForApp(bundleID string) []string {
	merged := c.MergedForApp(bundleID)

	// Append menubar/dock roles on top of the merged roles.
	if merged.IncludeMenubarHints {
		merged.ClickableRoles = append(merged.ClickableRoles, RoleMenuBarItem)
	}

	if merged.IncludeDockHints {
		merged.ClickableRoles = append(merged.ClickableRoles, RoleDockItem)
	}

	return merged.ClickableRoles
}

// StrategyForApp returns the element detection strategy for the given bundle ID.
// Delegates to MergedForApp to handle the root→app-config override chain.
func (c *HintsConfig) StrategyForApp(bundleID string) string {
	return c.MergedForApp(bundleID).Strategy
}

// LabelDirectionForApp returns the label direction for the given bundle ID.
// Delegates to MergedForApp to handle the root→app-config override chain.
// An empty result is normalized to the default "normal".
func (c *HintsConfig) LabelDirectionForApp(bundleID string) string {
	dir := c.MergedForApp(bundleID).LabelDirection
	if dir == "" {
		return LabelDirectionNormal
	}

	return dir
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
