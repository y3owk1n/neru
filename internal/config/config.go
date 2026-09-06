package config

import (
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// Semantic role names injected into the clickable role list when the
// corresponding hints flags are enabled. They are resolved to native role
// names alongside the user's own entries; on platforms with no menu bar or
// dock they resolve to nothing.
const (
	RoleMenuBarItem = string(element.SemanticMenubarItem)
	RoleDockItem    = string(element.SemanticDockItem)
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

// Display-form key name constants used in config files and maps. The spellings
// come from internal/domain/keyvocab, which declares the named-key vocabulary.
const (
	KeyDisplaySpace     = keyvocab.KeySpace
	KeyDisplayEscape    = keyvocab.KeyEscape
	KeyDisplayBackspace = keyvocab.KeyBackspace
	KeyDisplayDown      = keyvocab.KeyDown
	KeyDisplayLeft      = keyvocab.KeyLeft
	KeyDisplayRight     = keyvocab.KeyRight
	KeyReturn           = keyvocab.KeyReturn
)

// Common action command constants.
const (
	// CmdRun runs the rest of the binding as an ordered action sequence.
	CmdRun = "run"
	// CmdMacro runs a named sequence from the [macros] table.
	CmdMacro = MacroCommand
	// OnExitFlag carries a step a mode runs once its action is fulfilled. It is
	// repeatable, and its values are steps in their own right.
	OnExitFlag                     = "--" + string(modecmd.FlagOnExit)
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
	CmdLeftMouseDown  = "action left_click --state down"
	CmdLeftMouseUp    = "action left_click --state up"
	CmdGoTop          = "action go_top"
	CmdBackspace      = "action backspace"
	CmdMoveMouseDown  = "action move_mouse_relative --dx=0 --dy=10"
	CmdMoveMouseLeft  = "action move_mouse_relative --dx=-10 --dy=0"
	CmdMoveMouseRight = "action move_mouse_relative --dx=10 --dy=0"
)

// DisabledSentinel is a special action value that removes a default hotkey binding.
// Use it in [hotkeys] or [<mode>.hotkeys] to disable a specific default:
//
//	[scroll.hotkeys]
//	"j" = "__disabled__"   # removes the default "j" = "action scroll_down"
const DisabledSentinel = "__disabled__"

// OverrideSuffix is the suffix appended to a config file's base name to
// derive the override file path. For example, "config.toml" produces
// "config.override.toml", and "my-neru.toml" produces "my-neru.override.toml".
const OverrideSuffix = ".override.toml"

// The comparison forms the control characters a tap delivers resolve to, and
// the modifier names. These are not a listing of the named keys —
// internal/domain/keyvocab declares those, and every other named key reaches
// its comparison form by being lowercased rather than by being named here.
const (
	minimumModifierParts = 2
	KeyNameEscape        = "escape"
	KeyNameReturn        = "return"
	KeyNameTab           = "tab"
	KeyNameSpace         = "space"
	KeyNameDelete        = "delete"
	modifierNameCmd      = "cmd"
	modifierNameCtrl     = "ctrl"
	modifierNameAlt      = "alt"
	modifierNameShift    = "shift"
)

// Config is the complete application configuration structure.
type Config struct {
	General         GeneralConfig                  `json:"general"         toml:"general"`
	Theme           ThemeConfig                    `json:"theme"           toml:"theme"`
	Hotkeys         HotkeysConfig                  `json:"hotkeys"         toml:"-"`
	Macros          map[string]StringOrStringArray `json:"macros"          toml:"macros"`
	Hints           HintsConfig                    `json:"hints"           toml:"hints"`
	Grid            GridConfig                     `json:"grid"            toml:"grid"`
	RecursiveGrid   RecursiveGridConfig            `json:"recursiveGrid"   toml:"recursive_grid"`
	MonitorSelect   MonitorSelectConfig            `json:"monitorSelect"   toml:"monitor_select"`
	VirtualPointer  VirtualPointerConfig           `json:"virtualPointer"  toml:"virtual_pointer"`
	MouseAction     MouseActionConfig              `json:"mouseAction"     toml:"mouse_action_indicator"`
	Scroll          ScrollConfig                   `json:"scroll"          toml:"scroll"`
	ModeIndicator   ModeIndicatorConfig            `json:"modeIndicator"   toml:"mode_indicator"`
	StickyModifiers StickyModifiersConfig          `json:"stickyModifiers" toml:"sticky_modifiers"`
	Logging         LoggingConfig                  `json:"logging"         toml:"logging"`
	SmoothCursor    SmoothCursorConfig             `json:"smoothCursor"    toml:"smooth_cursor"`
	SmoothScroll    SmoothScrollConfig             `json:"smoothScroll"    toml:"smooth_scroll"`
	HeldRepeat      HeldRepeatConfig               `json:"heldRepeat"      toml:"held_repeat"`
	Systray         SystrayConfig                  `json:"systray"         toml:"systray"`

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
	CaptureScope         string                         `json:"captureScope"         toml:"capture_scope"`
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

// HintsConfig defines the visual and behavioral settings for hints mode.
type HintsConfig struct {
	Enabled           bool                `json:"enabled"           toml:"enabled"`
	Strategy          string              `json:"strategy"          toml:"strategy"`
	CaptureScope      string              `json:"captureScope"      toml:"capture_scope"`
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
	Enabled        bool   `json:"enabled"        toml:"enabled"`
	Characters     string `json:"characters"     toml:"characters"`
	SublayerKeys   string `json:"sublayerKeys"   toml:"sublayer_keys"`
	MaxLabelLength int    `json:"maxLabelLength" toml:"max_label_length"`
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
	// Grid dimensions: columns and rows (default: 3x3)
	GridCols int `json:"gridCols" toml:"grid_cols"`
	GridRows int `json:"gridRows" toml:"grid_rows"`
	// Key bindings, one per cell, left to right then top to bottom
	// (default: rtyfghvbn for the 3x3 grid above)
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

// VirtualPointerConfig styles the standalone pointer drawn when the system
// cursor is hidden, plus the in-frame grid and recursive-grid indicators.
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
//
// RelativeMovementDuration applies to relative (keyboard-driven) movements
// such as move_mouse_relative. Unlike jumps, these arrive as rapid small
// deltas under key repeat, so their animation uses a fixed duration per move
// — speed then scales with the delta — instead of the distance-proportional
// duration used for jumps, which would clamp every delta to the same constant
// velocity.
type SmoothCursorConfig struct {
	MoveMouseEnabled         bool    `json:"moveMouseEnabled"         toml:"move_mouse_enabled"`
	Steps                    int     `json:"steps"                    toml:"steps"`
	MaxDuration              int     `json:"maxDuration"              toml:"max_duration"`               // Max animation duration in ms
	DurationPerPixel         float64 `json:"durationPerPixel"         toml:"duration_per_pixel"`         // Ms per pixel for adaptive duration
	RelativeMovementDuration int     `json:"relativeMovementDuration" toml:"relative_movement_duration"` // Fixed duration per relative move in ms
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
	Enabled            bool     `json:"enabled"            toml:"enabled"`              // Master toggle for held-key repeat
	InitialDelay       int      `json:"initialDelay"       toml:"initial_delay_ms"`     // Delay before first repeat fires (ms)
	Interval           int      `json:"interval"           toml:"interval_ms"`          // Interval between subsequent repeats (ms)
	AccelEnabled       bool     `json:"accelEnabled"       toml:"accel_enabled"`        // Ramp the glide's speed up the longer the key stays held
	AccelRampMs        int      `json:"accelRampMs"        toml:"accel_ramp_ms"`        // Hold time to reach accel_max_multiplier (ms)
	AccelMaxMultiplier float64  `json:"accelMaxMultiplier" toml:"accel_max_multiplier"` // Speed multiplier at full ramp
	AccelTargets       []string `json:"accelTargets"       toml:"accel_targets"`        // Action names eligible for acceleration
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

	// Written is Config before any derived value was settled: the
	// configuration the user wrote, every layer of it, and nothing the daemon
	// worked out for them. Settling is one-way — a resolved grid label reads
	// exactly like one somebody typed — so a change to a *source* option can
	// only be re-derived from a configuration that never had the derivation
	// applied. It is what `neru config set` starts from; keeping it is the
	// price of that command not re-reading the file.
	Written *Config

	// Warnings are the parts of the loaded configuration that will not do what
	// they say. The configuration loaded regardless — that is what separates
	// one of these from ValidationError, which leaves the defaults running
	// instead — so they are reported rather than acted on. They are empty
	// whenever ValidationError is set: the configuration they described is not
	// the one that ended up loaded.
	Warnings []string

	// Inert are the words the user's files write that do nothing on this
	// platform, as [InertWords] found them, or on the display backend it is
	// running, as [X11InertWords] found them. The warnings above say the same
	// thing in a sentence; this is the same finding with its platform column
	// and its reason still attached, which is what `neru doctor` prints
	// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md).
	Inert parity.Declaration
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

// LabelDirectionForApp returns the label direction for the given bundle ID.
// Delegates to MergedForApp to handle the root→app-config override chain.
// An empty result is normalized to the default "normal".
func (c *HintsConfig) LabelDirectionForApp(bundleID string) string {
	dir := c.MergedForApp(bundleID).LabelDirection
	if dir == "" {
		return domain.LabelDirectionNormal
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
