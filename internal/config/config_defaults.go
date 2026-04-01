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

	// DefaultMaxFileSize is the default max file size for logs (10MB).
	DefaultMaxFileSize = 10
	// DefaultMaxBackups is the default max backups for logs.
	DefaultMaxBackups = 5
	// DefaultMaxAge is the default max age for logs (30 days).
	DefaultMaxAge = 30

	// DefaultSmoothCursorSteps is the default smooth cursor steps.
	DefaultSmoothCursorSteps = 10

	// DefaultSmoothCursorMaxDuration is the default max duration for smooth cursor animation (ms).
	DefaultSmoothCursorMaxDuration = 200

	// DefaultSmoothCursorDurationPerPixel is the default ms per pixel for adaptive duration.
	DefaultSmoothCursorDurationPerPixel = 0.1

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
	// DefaultFilePerms is the default file permissions.
	DefaultFilePerms = 0o644
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
	// DefaultRecursiveGridAnimationDurationMS is the default native recursive-grid transition duration in milliseconds.
	DefaultRecursiveGridAnimationDurationMS = 180
	// DefaultRecursiveGridLineWidth is the default line width for grid lines.
	DefaultRecursiveGridLineWidth = 1
	// DefaultRecursiveGridFontSize is the default font size for cell labels.
	DefaultRecursiveGridFontSize = 10
	// DefaultVirtualPointerSize is the default virtual pointer dot radius in points.
	DefaultVirtualPointerSize = 3
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
	DefaultRecursiveGridSubKeyPreviewFontSize = 8
	// DefaultRecursiveGridSubKeyPreviewAutohideMultiplier is the default minimum cell size multiplier
	// for sub-key preview autohide. Set to 0 to disable autohide.
	DefaultRecursiveGridSubKeyPreviewAutohideMultiplier = 1.5
	// DefaultStickyModifiersTapMaxDuration is the default maximum hold duration (ms)
	// for a modifier key press to be considered a "tap" for sticky toggle.
	// If held longer than this, the release will not toggle the sticky modifier.
	// 0 means no threshold (always toggle on release).
	DefaultStickyModifiersTapMaxDuration = 300

	// DefaultStickyModifiersFontSize is the default font size for the sticky modifiers indicator.
	DefaultStickyModifiersFontSize = 10
	// DefaultStickyModifiersBorderWidth is the default border width for the sticky modifiers indicator.
	DefaultStickyModifiersBorderWidth = 1
	// DefaultStickyModifiersPaddingX is the default horizontal padding for the sticky modifiers indicator (-1 = auto).
	DefaultStickyModifiersPaddingX = -1
	// DefaultStickyModifiersPaddingY is the default vertical padding for the sticky modifiers indicator (-1 = auto).
	DefaultStickyModifiersPaddingY = -1
	// DefaultStickyModifiersBorderRadius is the default border radius for the sticky modifiers indicator (-1 = auto).
	DefaultStickyModifiersBorderRadius = -1
	// DefaultStickyModifiersXOffset is the default X offset for the sticky modifiers indicator (negative = left of cursor).
	DefaultStickyModifiersXOffset = -40
	// DefaultStickyModifiersYOffset is the default Y offset for the sticky modifiers indicator.
	DefaultStickyModifiersYOffset = 20
)

func newDefaultConfig() *Config {
	return &Config{
		General: GeneralConfig{
			ExcludedApps:                      []string{},
			AccessibilityCheckOnStart:         true,
			PassthroughUnboundedKeys:          false,
			ShouldExitAfterPassthrough:        false,
			PassthroughUnboundedKeysBlacklist: []string{},
			HideOverlayInScreenShare:          false,
			KBLayoutToUse:                     "",
		},
		Theme: defaultThemeConfig(),
		Hotkeys: HotkeysConfig{
			Bindings: map[string][]string{
				"Cmd+Shift+Space": {"hints"},
				"Cmd+Shift+G":     {"grid"},
				"Cmd+Shift+C":     {"recursive_grid"},
				"Cmd+Shift+S":     {"scroll"},
			},
		},
		Hints: HintsConfig{
			Enabled:           true,
			HintCharacters:    "asdfghjkl",
			MaxDepth:          DefaultMaxDepth,
			ParallelThreshold: DefaultParallelThreshold,
			Hotkeys: map[string]StringOrStringArray{
				"Escape":    {"idle"},
				"Backspace": {"action backspace"},
				"Shift+L":   {"action left_click"},
				"Shift+R":   {"action right_click"},
				"Shift+M":   {"action middle_click"},
				"Shift+I":   {"action mouse_down"},
				"Shift+U":   {"action mouse_up"},
				"Up":        {"action move_mouse_relative --dx=0 --dy=-10"},
				"Down":      {"action move_mouse_relative --dx=0 --dy=10"},
				"Left":      {"action move_mouse_relative --dx=-10 --dy=0"},
				"Right":     {"action move_mouse_relative --dx=10 --dy=0"},
			},

			UI: HintsUI{
				FontSize:         DefaultHintFontSize,
				FontFamily:       "",
				BorderRadius:     DefaultHintBorderRadius,
				PaddingX:         DefaultHintPaddingX,
				PaddingY:         DefaultHintPaddingY,
				BorderWidth:      1,
				BackgroundColor:  Color{},
				TextColor:        Color{},
				MatchedTextColor: Color{},
				BorderColor:      Color{},
			},

			IncludeMenubarHints:           false,
			AdditionalMenubarHintsTargets: []string{},
			IncludeDockHints:              false,
			IncludeNCHints:                false,
			IncludeStageManagerHints:      false,
			DetectMissionControl:          false,

			ClickableRoles: []string{},

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
			Hotkeys: map[string]StringOrStringArray{
				"Escape":    {"idle"},
				"`":         {"toggle-cursor-follow-selection"},
				"Space":     {"action reset"},
				"Backspace": {"action backspace"},
				"Shift+L":   {"action left_click"},
				"Shift+R":   {"action right_click"},
				"Shift+M":   {"action middle_click"},
				"Shift+I":   {"action mouse_down"},
				"Shift+U":   {"action mouse_up"},
				"Up":        {"action move_mouse_relative --dx=0 --dy=-10"},
				"Down":      {"action move_mouse_relative --dx=0 --dy=10"},
				"Left":      {"action move_mouse_relative --dx=-10 --dy=0"},
				"Right":     {"action move_mouse_relative --dx=10 --dy=0"},
			},

			UI: GridUI{
				FontSize:               DefaultGridFontSize,
				FontFamily:             "",
				BorderWidth:            1,
				BackgroundColor:        Color{},
				TextColor:              Color{},
				MatchedTextColor:       Color{},
				MatchedBackgroundColor: Color{},
				MatchedBorderColor:     Color{},
				BorderColor:            Color{},
			},

			LiveMatchUpdate: true,
			HideUnmatched:   true,
			PrewarmEnabled:  true,
			EnableGC:        false,
		},
		RecursiveGrid: RecursiveGridConfig{
			Enabled:             true,
			Animate:             false,
			AnimationDurationMS: DefaultRecursiveGridAnimationDurationMS,
			GridCols:            2, //nolint:mnd
			GridRows:            2, //nolint:mnd

			Keys: "uijk", // warpd convention: u=TL, i=TR, j=BL, k=BR
			Hotkeys: map[string]StringOrStringArray{
				"Escape":    {"idle"},
				"`":         {"toggle-cursor-follow-selection"},
				"Space":     {"action reset"},
				"Backspace": {"action backspace"},
				"Shift+L":   {"action left_click"},
				"Shift+R":   {"action right_click"},
				"Shift+M":   {"action middle_click"},
				"Shift+I":   {"action mouse_down"},
				"Shift+U":   {"action mouse_up"},
				"Up":        {"action move_mouse_relative --dx=0 --dy=-10"},
				"Down":      {"action move_mouse_relative --dx=0 --dy=10"},
				"Left":      {"action move_mouse_relative --dx=-10 --dy=0"},
				"Right":     {"action move_mouse_relative --dx=10 --dy=0"},
			},

			UI: RecursiveGridUI{
				LineColor:                       Color{},
				LineWidth:                       DefaultRecursiveGridLineWidth,
				HighlightColor:                  Color{},
				TextColor:                       Color{},
				FontSize:                        DefaultRecursiveGridFontSize,
				FontFamily:                      "",
				LabelBackgroundColor:            Color{},
				LabelBackgroundPaddingX:         DefaultRecursiveGridLabelBackgroundPaddingX,
				LabelBackgroundPaddingY:         DefaultRecursiveGridLabelBackgroundPaddingY,
				LabelBackgroundBorderRadius:     DefaultRecursiveGridLabelBackgroundBorderRadius,
				LabelBackgroundBorderWidth:      DefaultRecursiveGridLabelBackgroundBorderWidth,
				LabelBackground:                 false,
				SubKeyPreview:                   DefaultRecursiveGridSubKeyPreview,
				SubKeyPreviewFontSize:           DefaultRecursiveGridSubKeyPreviewFontSize,
				SubKeyPreviewAutohideMultiplier: DefaultRecursiveGridSubKeyPreviewAutohideMultiplier,
				SubKeyPreviewTextColor:          Color{},
			},

			MinSizeWidth:  DefaultRecursiveGridMinSizeWidth,
			MinSizeHeight: DefaultRecursiveGridMinSizeHeight,
			MaxDepth:      DefaultRecursiveGridMaxDepth,
		},
		VirtualPointer: VirtualPointerConfig{
			Enabled: true,
			UI: VirtualPointerUI{
				Size:  DefaultVirtualPointerSize,
				Color: Color{},
			},
		},
		ModeIndicator: ModeIndicatorConfig{
			Scroll: ModeIndicatorModeConfig{
				Enabled: true,
				Text:    "Scroll",
			},
			Hints: ModeIndicatorModeConfig{
				Enabled: false,
				Text:    "Hints",
			},
			Grid: ModeIndicatorModeConfig{
				Enabled: false,
				Text:    "Grid",
			},
			RecursiveGrid: ModeIndicatorModeConfig{
				Enabled: false,
				Text:    "Recursive Grid",
			},
			UI: ModeIndicatorUI{
				FontSize:         DefaultScrollFontSize,
				FontFamily:       "",
				BackgroundColor:  Color{},
				TextColor:        Color{},
				BorderColor:      Color{},
				BorderWidth:      1,
				PaddingX:         DefaultScrollPaddingX,
				PaddingY:         DefaultScrollPaddingY,
				BorderRadius:     DefaultScrollBorderRadius,
				IndicatorXOffset: DefaultScrollIndicatorXOffset,
				IndicatorYOffset: DefaultScrollIndicatorYOffset,
			},
		},
		StickyModifiers: StickyModifiersConfig{
			Enabled:        true,
			TapMaxDuration: DefaultStickyModifiersTapMaxDuration,
			UI: StickyModifiersUI{
				FontSize:         DefaultStickyModifiersFontSize,
				FontFamily:       "",
				BackgroundColor:  Color{},
				TextColor:        Color{},
				BorderColor:      Color{},
				BorderWidth:      DefaultStickyModifiersBorderWidth,
				PaddingX:         DefaultStickyModifiersPaddingX,
				PaddingY:         DefaultStickyModifiersPaddingY,
				BorderRadius:     DefaultStickyModifiersBorderRadius,
				IndicatorXOffset: DefaultStickyModifiersXOffset,
				IndicatorYOffset: DefaultStickyModifiersYOffset,
			},
		},
		Scroll: ScrollConfig{
			ScrollStep:     DefaultScrollStep,
			ScrollStepHalf: DefaultScrollStepHalf,
			ScrollStepFull: DefaultScrollStepFull,
			Hotkeys: map[string]StringOrStringArray{
				"Escape":   {"idle"},
				"k":        {"action scroll_up"},
				"j":        {"action scroll_down"},
				"h":        {"action scroll_left"},
				"l":        {"action scroll_right"},
				"gg":       {"action go_top"},
				"Shift+G":  {"action go_bottom"},
				"u":        {"action page_up"},
				"PageUp":   {"action page_up"},
				"d":        {"action page_down"},
				"PageDown": {"action page_down"},
				"Shift+L":  {"action left_click"},
				"Shift+R":  {"action right_click"},
				"Shift+M":  {"action middle_click"},
				"Shift+I":  {"action mouse_down"},
				"Shift+U":  {"action mouse_up"},
				"Up":       {"action move_mouse_relative --dx=0 --dy=-10"},
				"Down":     {"action move_mouse_relative --dx=0 --dy=10"},
				"Left":     {"action move_mouse_relative --dx=-10 --dy=0"},
				"Right":    {"action move_mouse_relative --dx=10 --dy=0"},
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
			MaxDuration:      DefaultSmoothCursorMaxDuration,
			DurationPerPixel: DefaultSmoothCursorDurationPerPixel,
		},
		Systray: SystrayConfig{
			Enabled: true, // Enabled by default
		},
	}
}
