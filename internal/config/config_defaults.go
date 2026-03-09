package config

import "time"

const (
	// DefaultHintFontSize is the default font size for hints.
	DefaultHintFontSize = 10
	// DefaultHintBorderRadius is the default border radius for hints (-1 = auto).
	DefaultHintBorderRadius = -1
	// DefaultHintPaddingX is the default horizontal padding for hints (-1 = auto).
	DefaultHintPaddingX = -1
	// DefaultHintPaddingY is the default vertical padding for hints (-1 = auto).
	DefaultHintPaddingY = -1

	// DefaultMouseActionRefreshDelay is the default delay before refreshing hints after mouse actions.
	DefaultMouseActionRefreshDelay = 0

	// MaxMouseActionRefreshDelay is the maximum delay before refreshing hints after mouse actions (10 seconds).
	MaxMouseActionRefreshDelay = 10000

	// DefaultGridFontSize is the default font size for grid.
	DefaultGridFontSize = 10

	// DefaultScrollStep is the default scroll step.
	DefaultScrollStep = 50
	// DefaultScrollStepHalf is the default scroll step half.
	DefaultScrollStepHalf = 500
	// DefaultScrollStepFull is the default scroll step full.
	DefaultScrollStepFull = 1000000

	// DefaultScrollFontSize is the default font size for scroll indicator.
	DefaultScrollFontSize = 10
	// DefaultScrollPaddingX is the default horizontal padding for scroll indicator (-1 = auto).
	DefaultScrollPaddingX = -1
	// DefaultScrollPaddingY is the default vertical padding for scroll indicator (-1 = auto).
	DefaultScrollPaddingY = -1
	// DefaultScrollBorderRadius is the default border radius for scroll indicator (-1 = auto).
	DefaultScrollBorderRadius = -1

	// DefaultScrollIndicatorXOffset is the default X offset for scroll indicator.
	DefaultScrollIndicatorXOffset = 20
	// DefaultScrollIndicatorYOffset is the default Y offset for scroll indicator.
	DefaultScrollIndicatorYOffset = 20

	// DefaultScrollSequenceTimeout is the timeout for multi-key sequences.
	DefaultScrollSequenceTimeout = 500 * time.Millisecond

	// DefaultMaxFileSize is the default max file size for logs (10MB).
	DefaultMaxFileSize = 10
	// DefaultMaxBackups is the default max backups for logs.
	DefaultMaxBackups = 5
	// DefaultMaxAge is the default max age for logs (30 days).
	DefaultMaxAge = 30

	// DefaultSmoothCursorSteps is the default smooth cursor steps.
	DefaultSmoothCursorSteps = 10

	// DefaultMoveMouseStep is the default step size for keyboard-driven mouse movement.
	DefaultMoveMouseStep = 10

	// DefaultIPCTimeout is the default IPC timeout.
	DefaultIPCTimeout = 5
	// DefaultAppWatcherTimeout is the default app watcher timeout.
	DefaultAppWatcherTimeout = 10
	// DefaultModeTimeout is the default mode timeout.
	DefaultModeTimeout = 5
	// DefaultValidationTimeout is the default validation timeout.
	DefaultValidationTimeout = 2

	// DefaultCacheSize is the default cache size.
	DefaultCacheSize = 100
	// DefaultCallbackMapSize is the default callback map size.
	DefaultCallbackMapSize = 8
	// DefaultSubscriberMapSize is the default subscriber map size.
	DefaultSubscriberMapSize = 4

	// DefaultParallelThreshold is the default parallel threshold.
	DefaultParallelThreshold = 20
	// DefaultMaxParallelDepth is the default max parallel depth.
	DefaultMaxParallelDepth = 4

	// DefaultMaxDepth is the default max depth for accessibility tree traversal.
	DefaultMaxDepth = 50

	// DefaultChildrenCapacity is the default children capacity.
	DefaultChildrenCapacity = 8

	// DefaultGridLinesCount is the default grid lines count.
	DefaultGridLinesCount = 4

	// DefaultTimerDuration is the default timer duration.
	DefaultTimerDuration = 2 * time.Second

	// DefaultIPCReadTimeout is the default IPC read timeout.
	DefaultIPCReadTimeout = 30 * time.Second

	// DefaultPingTimeout is the default ping timeout.
	DefaultPingTimeout = 500 * time.Millisecond

	// DefaultConfigCacheTTL is the default cache TTL for config.
	DefaultConfigCacheTTL = 5 * time.Second

	// DefaultDirPerms is the default directory permissions.
	DefaultDirPerms = 0o750
	// DefaultSocketPerms is the default socket permissions.
	DefaultSocketPerms = 0o600

	// MinCharactersLength is the minimum characters length.
	MinCharactersLength = 2

	// LabelLength2 is the grid label length 2.
	LabelLength2 = 2
	// LabelLength3 is the grid label length 3.
	LabelLength3 = 3
	// LabelLength4 is the grid label length 4.
	LabelLength4 = 4

	// MinGridCols is the minimum grid columns.
	MinGridCols = 2
	// MinGridRows is the minimum grid rows.
	MinGridRows = 2

	// MaxKeyIndex is the maximum key index.
	MaxKeyIndex = 9

	// RoundingFactor is the rounding factor.
	RoundingFactor = 0.5

	// CenterDivisor is the center calculation divisor.
	CenterDivisor = 2

	// ScoreWeight is the score weight.
	ScoreWeight = 0.1

	// AspectRatioAdjustment is the aspect ratio adjustment.
	AspectRatioAdjustment = 1.2

	// StringBuilderGrow2 is the string builder growth factor 2.
	StringBuilderGrow2 = 2
	// StringBuilderGrow3 is the string builder growth factor 3.
	StringBuilderGrow3 = 3
	// StringBuilderGrow4 is the string builder growth factor 4.
	StringBuilderGrow4 = 4

	// CountsCapacity is the counts capacity.
	CountsCapacity = 5

	// LabelLengthCheck is the label length check.
	LabelLengthCheck = 2

	// PrefixLengthCheck is the prefix length check.
	PrefixLengthCheck = 2

	// CtrlD is the byte value for Ctrl+D.
	CtrlD = 4
	// CtrlU is the byte value for Ctrl+U.
	CtrlU = 21

	// CacheCleanupDivisor is the cache cleanup interval divisor.
	CacheCleanupDivisor = 2

	// CacheDeletionEstimate is the cache deletion estimate.
	CacheDeletionEstimate = 4

	// OverlayTimerDuration is the timer duration for overlays.
	OverlayTimerDuration = 2 * time.Second

	// GridMaxChars is the grid max chars.
	GridMaxChars = 9

	// IPCTimeoutSeconds is the IPC timeout seconds.
	IPCTimeoutSeconds = 5

	// DefaultRecursiveGridMinSizeWidth is the default minimum cell width in pixels.
	DefaultRecursiveGridMinSizeWidth = 25
	// DefaultRecursiveGridMinSizeHeight is the default minimum cell height in pixels.
	DefaultRecursiveGridMinSizeHeight = 25
	// DefaultRecursiveGridMaxDepth is the default maximum recursion depth.
	DefaultRecursiveGridMaxDepth = 10
	// DefaultRecursiveGridMinGridCols is the minimum allowed grid columns.
	DefaultRecursiveGridMinGridCols = 2
	// DefaultRecursiveGridMinGridRows is the minimum allowed grid rows.
	DefaultRecursiveGridMinGridRows = 2
	// DefaultRecursiveGridLineWidth is the default line width for grid lines.
	DefaultRecursiveGridLineWidth = 1
	// DefaultRecursiveGridFontSize is the default font size for cell labels.
	DefaultRecursiveGridFontSize = 10

	// HintsBackgroundColorLight is the light mode background color for hints.
	HintsBackgroundColorLight = "#F200CFCF"
	// HintsBackgroundColorDark is the dark mode background color for hints.
	HintsBackgroundColorDark = "#F2007A9E"
	// HintsTextColorLight is the light mode text color for hints.
	HintsTextColorLight = "#FF003554"
	// HintsTextColorDark is the dark mode text color for hints.
	HintsTextColorDark = "#FFFFFFFF"
	// HintsMatchedTextColorLight is the light mode matched text color for hints.
	HintsMatchedTextColorLight = "#FFAAEEFF"
	// HintsMatchedTextColorDark is the dark mode matched text color for hints.
	HintsMatchedTextColorDark = "#FF003554"
	// HintsBorderColorLight is the light mode border color for hints.
	HintsBorderColorLight = "#FF008A8A"
	// HintsBorderColorDark is the dark mode border color for hints.
	HintsBorderColorDark = "#FF00B4D8"

	// GridBackgroundColorLight is the light mode background color for grid cells.
	GridBackgroundColorLight = "#9900B4D8"
	// GridBackgroundColorDark is the dark mode background color for grid cells.
	GridBackgroundColorDark = "#99003554"
	// GridTextColorLight is the light mode text color for grid labels.
	GridTextColorLight = "#FF003554"
	// GridTextColorDark is the dark mode text color for grid labels.
	GridTextColorDark = "#FFB3E8F5"
	// GridMatchedTextColorLight is the light mode matched text color for grid cells.
	GridMatchedTextColorLight = "#FFAAEEFF"
	// GridMatchedTextColorDark is the dark mode matched text color for grid cells.
	GridMatchedTextColorDark = "#FFFFFFFF"
	// GridMatchedBackgroundColorLight is the light mode matched background color for grid cells.
	GridMatchedBackgroundColorLight = "#B300CFCF"
	// GridMatchedBackgroundColorDark is the dark mode matched background color for grid cells.
	GridMatchedBackgroundColorDark = "#B300B4D8"
	// GridMatchedBorderColorLight is the light mode matched border color for grid cells.
	GridMatchedBorderColorLight = "#B300CFCF"
	// GridMatchedBorderColorDark is the dark mode matched border color for grid cells.
	GridMatchedBorderColorDark = "#B300B4D8"
	// GridBorderColorLight is the light mode border color for grid cells.
	GridBorderColorLight = "#9900B4D8"
	// GridBorderColorDark is the dark mode border color for grid cells.
	GridBorderColorDark = "#99003554"

	// RecursiveGridLineColorLight is the light mode line color for recursive grid.
	RecursiveGridLineColorLight = "#FF007A9E"
	// RecursiveGridLineColorDark is the dark mode line color for recursive grid.
	RecursiveGridLineColorDark = "#FF00CFCF"
	// RecursiveGridHighlightColorLight is the light mode highlight color for recursive grid.
	RecursiveGridHighlightColorLight = "#4D007A9E"
	// RecursiveGridHighlightColorDark is the dark mode highlight color for recursive grid.
	RecursiveGridHighlightColorDark = "#4D00CFCF"
	// RecursiveGridTextColorLight is the default text color for Light Mode for recursive grid.
	RecursiveGridTextColorLight = "#FF007A9E"
	// RecursiveGridTextColorDark is the default text color for Dark Mode for recursive grid.
	RecursiveGridTextColorDark = "#FF00CFCF"
	// RecursiveGridLabelBackgroundColorLight is the default label badge color for Light Mode.
	RecursiveGridLabelBackgroundColorLight = "#FFAAEEFF"
	// RecursiveGridLabelBackgroundColorDark is the default label badge color for Dark Mode.
	RecursiveGridLabelBackgroundColorDark = "#FF003554"
	// DefaultRecursiveGridLabelBackgroundPaddingX preserves automatic horizontal badge padding.
	DefaultRecursiveGridLabelBackgroundPaddingX = -1
	// DefaultRecursiveGridLabelBackgroundPaddingY preserves automatic vertical badge padding.
	DefaultRecursiveGridLabelBackgroundPaddingY = -1
	// DefaultRecursiveGridLabelBackgroundBorderRadius preserves the automatic pill radius.
	DefaultRecursiveGridLabelBackgroundBorderRadius = -1
	// DefaultRecursiveGridLabelBackgroundBorderWidth is the default label badge border width.
	DefaultRecursiveGridLabelBackgroundBorderWidth = 1

	// DefaultRecursiveGridSubKeyPreview controls whether the sub-key mini-grid is shown inside each cell.
	DefaultRecursiveGridSubKeyPreview = false
	// DefaultRecursiveGridSubKeyPreviewFontSize is the default font size for sub-key preview labels.
	DefaultRecursiveGridSubKeyPreviewFontSize = 6
	// RecursiveGridSubKeyPreviewTextColorLight is the default Light Mode color for sub-key preview labels.
	RecursiveGridSubKeyPreviewTextColorLight = "#66007A9E"
	// RecursiveGridSubKeyPreviewTextColorDark is the default Dark Mode color for sub-key preview labels.
	RecursiveGridSubKeyPreviewTextColorDark = "#6600CFCF"

	// ModeIndicatorBackgroundColorLight is the light mode background color for the mode indicator.
	ModeIndicatorBackgroundColorLight = "#F200CFCF"
	// ModeIndicatorBackgroundColorDark is the dark mode background color for the mode indicator.
	ModeIndicatorBackgroundColorDark = "#F200CFCF"
	// ModeIndicatorTextColorLight is the light mode text color for the mode indicator.
	ModeIndicatorTextColorLight = "#FF003554"
	// ModeIndicatorTextColorDark is the dark mode text color for the mode indicator.
	ModeIndicatorTextColorDark = "#FF003554"
	// ModeIndicatorBorderColorLight is the light mode border color for the mode indicator.
	ModeIndicatorBorderColorLight = "#FF007A9E"
	// ModeIndicatorBorderColorDark is the dark mode border color for the mode indicator.
	ModeIndicatorBorderColorDark = "#FF007A9E"
)

// DefaultConfig returns the default application configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		General: GeneralConfig{
			ExcludedApps:              []string{},
			AccessibilityCheckOnStart: true,
			RestoreCursorPosition:     false,
			CenterCursorPosition:      false,
			ModeExitKeys:              []string{"Escape"},
			HideOverlayInScreenShare:  false,
			KBLayoutToUse:             "",
		},
		Hotkeys: HotkeysConfig{
			Bindings: map[string]string{
				"Cmd+Shift+Space": "hints",
				"Cmd+Shift+G":     "grid",
				"Cmd+Shift+C":     "recursive_grid",
				"Cmd+Shift+S":     "scroll",
			},
		},
		Hints: HintsConfig{
			Enabled:                 true,
			HintCharacters:          "asdfghjkl",
			BackspaceKey:            "Backspace",
			MouseActionRefreshDelay: DefaultMouseActionRefreshDelay,
			MaxDepth:                DefaultMaxDepth,
			ParallelThreshold:       DefaultParallelThreshold,

			UI: HintsUI{
				FontSize:              DefaultHintFontSize,
				FontFamily:            "",
				BorderRadius:          DefaultHintBorderRadius,
				PaddingX:              DefaultHintPaddingX,
				PaddingY:              DefaultHintPaddingY,
				BorderWidth:           1,
				BackgroundColorLight:  HintsBackgroundColorLight,
				BackgroundColorDark:   HintsBackgroundColorDark,
				TextColorLight:        HintsTextColorLight,
				TextColorDark:         HintsTextColorDark,
				MatchedTextColorLight: HintsMatchedTextColorLight,
				MatchedTextColorDark:  HintsMatchedTextColorDark,
				BorderColorLight:      HintsBorderColorLight,
				BorderColorDark:       HintsBorderColorDark,
			},

			IncludeMenubarHints: false,
			AdditionalMenubarHintsTargets: []string{
				"com.apple.TextInputMenuAgent",
				"com.apple.controlcenter",
				"com.apple.systemuiserver",
			},
			IncludeDockHints:         false,
			IncludeNCHints:           false,
			IncludeStageManagerHints: false,
			DetectMissionControl:     false,

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

			AppConfigs: []AppConfig{},

			AdditionalAXSupport: AdditionalAXSupport{
				Enable:                    false,
				AdditionalElectronBundles: []string{},
				AdditionalChromiumBundles: []string{},
				AdditionalFirefoxBundles:  []string{},
			},
		},
		Grid: GridConfig{
			Enabled: true,

			Characters:   "abcdefghijklmnpqrstuvwxyz",
			SublayerKeys: "abcdefghijklmnpqrstuvwxyz",
			BackspaceKey: "Backspace",

			UI: GridUI{
				FontSize:                    DefaultGridFontSize,
				FontFamily:                  "",
				BorderWidth:                 1,
				BackgroundColorLight:        GridBackgroundColorLight,
				BackgroundColorDark:         GridBackgroundColorDark,
				TextColorLight:              GridTextColorLight,
				TextColorDark:               GridTextColorDark,
				MatchedTextColorLight:       GridMatchedTextColorLight,
				MatchedTextColorDark:        GridMatchedTextColorDark,
				MatchedBackgroundColorLight: GridMatchedBackgroundColorLight,
				MatchedBackgroundColorDark:  GridMatchedBackgroundColorDark,
				MatchedBorderColorLight:     GridMatchedBorderColorLight,
				MatchedBorderColorDark:      GridMatchedBorderColorDark,
				BorderColorLight:            GridBorderColorLight,
				BorderColorDark:             GridBorderColorDark,
			},

			LiveMatchUpdate: true,
			HideUnmatched:   true,
			PrewarmEnabled:  true,
			EnableGC:        false,
			ResetKey:        " ",
		},
		RecursiveGrid: RecursiveGridConfig{
			Enabled:  true,
			GridCols: 2, //nolint:mnd
			GridRows: 2, //nolint:mnd

			Keys:         "uijk", // warpd convention: u=TL, i=TR, j=BL, k=BR
			BackspaceKey: "Backspace",

			UI: RecursiveGridUI{
				LineColorLight:              RecursiveGridLineColorLight,
				LineColorDark:               RecursiveGridLineColorDark,
				LineWidth:                   DefaultRecursiveGridLineWidth,
				HighlightColorLight:         RecursiveGridHighlightColorLight,
				HighlightColorDark:          RecursiveGridHighlightColorDark,
				TextColorLight:              RecursiveGridTextColorLight,
				TextColorDark:               RecursiveGridTextColorDark,
				FontSize:                    DefaultRecursiveGridFontSize,
				FontFamily:                  "",
				LabelBackgroundColorLight:   RecursiveGridLabelBackgroundColorLight,
				LabelBackgroundColorDark:    RecursiveGridLabelBackgroundColorDark,
				LabelBackgroundPaddingX:     DefaultRecursiveGridLabelBackgroundPaddingX,
				LabelBackgroundPaddingY:     DefaultRecursiveGridLabelBackgroundPaddingY,
				LabelBackgroundBorderRadius: DefaultRecursiveGridLabelBackgroundBorderRadius,
				LabelBackgroundBorderWidth:  DefaultRecursiveGridLabelBackgroundBorderWidth,
				LabelBackground:             false,
				SubKeyPreview:               DefaultRecursiveGridSubKeyPreview,
				SubKeyPreviewFontSize:       DefaultRecursiveGridSubKeyPreviewFontSize,
				SubKeyPreviewTextColorLight: RecursiveGridSubKeyPreviewTextColorLight,
				SubKeyPreviewTextColorDark:  RecursiveGridSubKeyPreviewTextColorDark,
			},

			MinSizeWidth:  DefaultRecursiveGridMinSizeWidth,
			MinSizeHeight: DefaultRecursiveGridMinSizeHeight,
			MaxDepth:      DefaultRecursiveGridMaxDepth,
			ResetKey:      " ",
		},
		ModeIndicator: ModeIndicatorConfig{
			ScrollEnabled:        true,
			HintsEnabled:         false,
			GridEnabled:          false,
			RecursiveGridEnabled: false,

			UI: ModeIndicatorUI{
				FontSize:             DefaultScrollFontSize,
				FontFamily:           "",
				BackgroundColorLight: ModeIndicatorBackgroundColorLight,
				BackgroundColorDark:  ModeIndicatorBackgroundColorDark,
				TextColorLight:       ModeIndicatorTextColorLight,
				TextColorDark:        ModeIndicatorTextColorDark,
				BorderColorLight:     ModeIndicatorBorderColorLight,
				BorderColorDark:      ModeIndicatorBorderColorDark,
				BorderWidth:          1,
				PaddingX:             DefaultScrollPaddingX,
				PaddingY:             DefaultScrollPaddingY,
				BorderRadius:         DefaultScrollBorderRadius,
				IndicatorXOffset:     DefaultScrollIndicatorXOffset,
				IndicatorYOffset:     DefaultScrollIndicatorYOffset,
			},
		},
		Scroll: ScrollConfig{
			ScrollStep:     DefaultScrollStep,
			ScrollStepHalf: DefaultScrollStepHalf,
			ScrollStepFull: DefaultScrollStepFull,

			KeyBindings: map[string][]string{
				"scroll_up":    {"k", "Up"},
				"scroll_down":  {"j", "Down"},
				"scroll_left":  {"h", "Left"},
				"scroll_right": {"l", "Right"},
				"go_top":       {"gg", "Cmd+Up"},
				"go_bottom":    {"Shift+G", "Cmd+Down"},
				"page_up":      {"Ctrl+U", "PageUp"},
				"page_down":    {"Ctrl+D", "PageDown"},
			},
		},
		Action: ActionConfig{
			MoveMouseStep: DefaultMoveMouseStep,
			KeyBindings: ActionKeyBindingsCfg{
				LeftClick:      "Shift+L",
				RightClick:     "Shift+R",
				MiddleClick:    "Shift+M",
				MouseDown:      "Shift+I",
				MouseUp:        "Shift+U",
				MoveMouseUp:    "Up",
				MoveMouseDown:  "Down",
				MoveMouseLeft:  "Left",
				MoveMouseRight: "Right",
			},
		},
		Logging: LoggingConfig{
			LogLevel:           "info",
			LogFile:            "",
			StructuredLogging:  true,
			DisableFileLogging: true,
			MaxFileSize:        DefaultMaxFileSize,
			MaxBackups:         DefaultMaxBackups,
			MaxAge:             DefaultMaxAge,
		},
		SmoothCursor: SmoothCursorConfig{
			MoveMouseEnabled: false,
			Steps:            DefaultSmoothCursorSteps,
			Delay:            1, // 1ms delay between steps
		},
		Systray: SystrayConfig{
			Enabled: true, // Enabled by default
		},
	}
}
