package config

import "time"

const (
	// DefaultHintFontSize is the default font size for hints.
	DefaultHintFontSize = 12
	// DefaultHintBorderRadius is the default border radius for hints.
	DefaultHintBorderRadius = 4
	// DefaultHintPadding is the default padding for hints.
	DefaultHintPadding = 4

	// DefaultMouseActionRefreshDelay is the default delay before refreshing hints after mouse actions.
	DefaultMouseActionRefreshDelay = 0

	// MaxMouseActionRefreshDelay is the maximum delay before refreshing hints after mouse actions (10 seconds).
	MaxMouseActionRefreshDelay = 10000

	// DefaultGridFontSize is the default font size for grid.
	DefaultGridFontSize = 12

	// DefaultScrollStep is the default scroll step.
	DefaultScrollStep = 50
	// DefaultScrollStepHalf is the default scroll step half.
	DefaultScrollStepHalf = 500
	// DefaultScrollStepFull is the default scroll step full.
	DefaultScrollStepFull = 1000000

	// DefaultScrollFontSize is the default font size for scroll indicator.
	DefaultScrollFontSize = 12
	// DefaultScrollPadding is the default padding for scroll indicator.
	DefaultScrollPadding = 4
	// DefaultScrollBorderRadius is the default border radius for scroll indicator.
	DefaultScrollBorderRadius = 4

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
	DefaultParallelThreshold = 100
	// DefaultMaxParallelDepth is the default max parallel depth.
	DefaultMaxParallelDepth = 4

	// DefaultMetricsCapacity is the default metrics capacity.
	DefaultMetricsCapacity = 1000

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

	// DefaultRecursiveGridMinSize is the default minimum cell size in pixels.
	DefaultRecursiveGridMinSize = 25
	// DefaultRecursiveGridMaxDepth is the default maximum recursion depth.
	DefaultRecursiveGridMaxDepth = 10
	// DefaultRecursiveGridMinGridSize is the minimum allowed grid size (NxN).
	DefaultRecursiveGridMinGridSize = 2
	// DefaultRecursiveGridLineWidth is the default line width for grid lines.
	DefaultRecursiveGridLineWidth = 1
	// DefaultRecursiveGridLabelFontSize is the default font size for cell labels.
	DefaultRecursiveGridLabelFontSize = 12
)

// DefaultConfig returns the default application configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		General: GeneralConfig{
			ExcludedApps:              []string{},
			AccessibilityCheckOnStart: true,
			RestoreCursorPosition:     false,
			ModeExitKeys:              []string{"escape"},
			HideOverlayInScreenShare:  false,
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
			FontSize:                DefaultHintFontSize,
			FontFamily:              "SF Mono",
			BorderRadius:            DefaultHintBorderRadius,
			Padding:                 DefaultHintPadding,
			BorderWidth:             1,
			MouseActionRefreshDelay: DefaultMouseActionRefreshDelay,

			BackgroundColor:  "#F2FFD700", // Gold, alpha F2 ≈ 95% opacity
			TextColor:        "#FF000000", // Black
			MatchedTextColor: "#FF737373", // Gray with 100% opacity (matched text)
			BorderColor:      "#FF000000", // Black

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

			FontSize:    DefaultGridFontSize,
			FontFamily:  "SF Mono",
			BorderWidth: 1,

			BackgroundColor:        "#B3ABE9B3", // Light green, alpha B3 ≈ 70% opacity
			TextColor:              "#FF000000", // Black
			MatchedTextColor:       "#FFF8BD96", // Orange, alpha FF = 100% opacity
			MatchedBackgroundColor: "#B3F8BD96", // Orange, alpha B3 ≈ 70% opacity (matches bg)
			MatchedBorderColor:     "#B3F8BD96", // Orange, alpha B3 ≈ 70% opacity
			BorderColor:            "#B3ABE9B3", // Light green, alpha B3 ≈ 70% opacity

			LiveMatchUpdate: true,
			HideUnmatched:   true,
			PrewarmEnabled:  true,
			EnableGC:        false,
			ResetKey:        ",",
		},
		RecursiveGrid: RecursiveGridConfig{
			Enabled:  true,
			GridSize: 2, //nolint:mnd

			Keys: "uijk", // warpd convention: u=TL, i=TR, j=BL, k=BR

			LineColor:       "#FF8EE2FF", // Light blue, alpha FF = 100% opacity
			LineWidth:       DefaultRecursiveGridLineWidth,
			HighlightColor:  "#4D00BFFF", // Deep sky blue, alpha 4D ≈ 30% opacity
			LabelColor:      "#FFFFFFFF", // White, alpha FF = 100% opacity
			LabelFontSize:   DefaultRecursiveGridLabelFontSize,
			LabelFontFamily: "SF Mono",

			MinSize:  DefaultRecursiveGridMinSize,
			MaxDepth: DefaultRecursiveGridMaxDepth,
			ResetKey: ",",
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

			// New styling defaults
			FontSize:         DefaultScrollFontSize,
			FontFamily:       "SF Mono",
			BackgroundColor:  "#F2FFD700", // Gold with 95% opacity
			TextColor:        "#FF000000", // Black
			BorderColor:      "#FF000000", // Black
			BorderWidth:      1,
			Padding:          DefaultScrollPadding,
			BorderRadius:     DefaultScrollBorderRadius,
			IndicatorXOffset: DefaultScrollIndicatorXOffset,
			IndicatorYOffset: DefaultScrollIndicatorYOffset,
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
			DisableFileLogging: false,
			MaxFileSize:        DefaultMaxFileSize,
			MaxBackups:         DefaultMaxBackups,
			MaxAge:             DefaultMaxAge,
		},
		SmoothCursor: SmoothCursorConfig{
			MoveMouseEnabled: false,
			Steps:            DefaultSmoothCursorSteps,
			Delay:            1, // 1ms delay between steps
		},
		Metrics: MetricsConfig{
			Enabled: false, // Disabled by default
		},
		Systray: SystrayConfig{
			Enabled: true, // Enabled by default
		},
	}
}
