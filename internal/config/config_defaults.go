package config

import (
	"slices"
	"time"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
)

const (
	// DefaultHintFontSize is the default font size for hints.
	DefaultHintFontSize = 10
	// DefaultHintBorderRadius is the default border radius for hints (-1 = auto).
	DefaultHintBorderRadius = -1
	// DefaultHintPaddingX is the default horizontal padding for hints (-1 = auto).
	DefaultHintPaddingX = -1
	// DefaultHintPaddingY is the default vertical padding for hints (-1 = auto).
	DefaultHintPaddingY = -1
	// DefaultHintBoundaryBorderRadius is the default radius for hint target boundaries.
	DefaultHintBoundaryBorderRadius = -1
	// DefaultHintBoundaryBorderWidth is the default stroke width for hint target boundaries.
	DefaultHintBoundaryBorderWidth = 1
	// DefaultVisionRequestTimeoutMS is the default Vision request timeout.
	DefaultVisionRequestTimeoutMS = 5000
	// DefaultVisionMinimumConfidence keeps all Vision observations by default.
	DefaultVisionMinimumConfidence = 0.0
	// DefaultVisionMergeIOUThreshold is the default overlap threshold for non-maximum suppression.
	DefaultVisionMergeIOUThreshold = 0.5
	// DefaultVisionRectangleMaxCandidates is the default maximum rectangle observations.
	DefaultVisionRectangleMaxCandidates = 100
	// DefaultVisionRectangleMinSize is the default minimum normalized rectangle size.
	DefaultVisionRectangleMinSize = 0.01
	// DefaultVisionRectangleMinAspect is the default minimum rectangle aspect ratio.
	DefaultVisionRectangleMinAspect = 0.3
	// DefaultVisionRectangleMaxAspect is the default maximum rectangle aspect ratio.
	DefaultVisionRectangleMaxAspect = 10.0
	// DefaultVisionButtonMinConfidence is the default button confidence threshold.
	DefaultVisionButtonMinConfidence = 0.3
	// DefaultVisionButtonMinAspect is the default minimum button aspect ratio.
	DefaultVisionButtonMinAspect = 0.8
	// DefaultVisionButtonMaxAspect is the default maximum button aspect ratio.
	DefaultVisionButtonMaxAspect = 8.0
	// DefaultVisionButtonIconMaxSize is the default max square button/icon size.
	DefaultVisionButtonIconMaxSize = 48
	// DefaultVisionLinkMinAspect is the default minimum link aspect ratio.
	DefaultVisionLinkMinAspect = 5.0
	// DefaultVisionLinkMaxHeight is the default maximum link text height.
	DefaultVisionLinkMaxHeight = 40
	// DefaultVisionLinkMinWidth is the default minimum link text width.
	DefaultVisionLinkMinWidth = 50
	// DefaultVisionImageMinSize is the default minimum size for image classification.
	DefaultVisionImageMinSize = 48
	// DefaultVisionCheckboxMaxSize is the default maximum size for checkbox classification.
	DefaultVisionCheckboxMaxSize = 32
	// DefaultVisionGenericClickableMinConfidence is the default generic clickable confidence threshold.
	DefaultVisionGenericClickableMinConfidence = 0.5

	// DefaultSearchInputYOffset is the default Y offset for search input.
	DefaultSearchInputYOffset = 24
	// DefaultSearchInputWidth is the default width for search input.
	DefaultSearchInputWidth = 320

	// DefaultSearchInputMinPaddingY is the minimum padding for search input height.
	DefaultSearchInputMinPaddingY = 5
	// DefaultSearchInputPaddingMultiplier is the padding multiplier for search input height.
	DefaultSearchInputPaddingMultiplier = 2
	// DefaultSearchInputHeightPadding is the height padding constant for search input.
	DefaultSearchInputHeightPadding = 6

	// DefaultSearchInputCenterDivisor is the divisor for centering search input horizontally or vertically.
	DefaultSearchInputCenterDivisor = 2

	// DefaultGridFontSize is the default font size for grid.
	DefaultGridFontSize = 10

	// DefaultScrollInvert is the default scroll inversion setting.
	DefaultScrollInvert = false

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

	// DefaultExecShell is the default shell used for exec commands.
	DefaultExecShell = "/bin/bash"

	// DefaultExecShellFlag is the shell flag that causes the shell to read
	// commands from the following string argument (e.g. "-c" or "-lc").
	DefaultExecShellFlag = "-lc"

	// DefaultSmoothCursorSteps is the default smooth cursor steps.
	DefaultSmoothCursorSteps = 10

	// DefaultSmoothCursorMaxDuration is the default max duration for smooth cursor animation (ms).
	DefaultSmoothCursorMaxDuration = 200

	// DefaultSmoothCursorDurationPerPixel is the default ms per pixel for adaptive duration.
	DefaultSmoothCursorDurationPerPixel = 0.1

	// DefaultSmoothCursorRelativeMovementDuration is the default fixed
	// animation duration for relative moves (ms). Matches
	// DefaultHeldRepeatInterval so animations chain seamlessly under held-key
	// repeat.
	DefaultSmoothCursorRelativeMovementDuration = 50

	// MinSmoothCursorAnimationDuration is the floor for every smooth cursor
	// animation in ms — the animator raises shorter animations to it. It is
	// also the smallest accepted relative_movement_duration: anything below
	// would be silently rounded up rather than honored, so the validator
	// rejects it.
	MinSmoothCursorAnimationDuration = 10

	// DefaultSmoothScrollSteps is the default smooth scroll steps.
	DefaultSmoothScrollSteps = 20

	// DefaultSmoothScrollMaxDuration is the default max duration for smooth scroll animation (ms).
	DefaultSmoothScrollMaxDuration = 180

	// DefaultSmoothScrollDurationPerPixel is the default ms per pixel for adaptive duration.
	DefaultSmoothScrollDurationPerPixel = 1.0
	// DefaultHeldRepeatInitialDelay is the default held-key initial delay in ms.
	DefaultHeldRepeatInitialDelay = 50
	// DefaultHeldRepeatInterval is the default held-key repeat interval in ms.
	DefaultHeldRepeatInterval = 50
	// DefaultHeldRepeatAccelRampMs is the default ramp duration, in ms.
	DefaultHeldRepeatAccelRampMs = 500
	// DefaultHeldRepeatAccelMaxMultiplier is the default multiplier at full ramp.
	DefaultHeldRepeatAccelMaxMultiplier = 4.0
	// MaxHeldRepeatAccelMultiplier bounds accel_max_multiplier.
	MaxHeldRepeatAccelMultiplier = 100

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
	DefaultRecursiveGridMinSizeWidth = 1
	// DefaultRecursiveGridMinSizeHeight is the default minimum cell height in pixels.
	DefaultRecursiveGridMinSizeHeight = 1
	// DefaultRecursiveGridMaxDepth is the default maximum recursion depth.
	DefaultRecursiveGridMaxDepth = 10
	// DefaultRecursiveGridMinGridCols is the minimum allowed grid columns.
	DefaultRecursiveGridMinGridCols = 1
	// DefaultRecursiveGridMinGridRows is the minimum allowed grid rows.
	DefaultRecursiveGridMinGridRows = 1
	// DefaultRecursiveGridMinTotalCells is the minimum allowed total cells.
	DefaultRecursiveGridMinTotalCells = 2
	// DefaultRecursiveGridAnimationDurationMS is the default native recursive-grid transition duration in milliseconds.
	DefaultRecursiveGridAnimationDurationMS = 50
	// DefaultRecursiveGridLineWidth is the default line width for grid lines.
	DefaultRecursiveGridLineWidth = 1
	// DefaultRecursiveGridFontSize is the default font size for cell labels.
	DefaultRecursiveGridFontSize = 10
	// DefaultVirtualPointerChar is the default character displayed by the virtual pointer.
	DefaultVirtualPointerChar = "\u25CF" // "●"
	// DefaultVirtualPointerFontSize is the default font size for the virtual pointer char.
	DefaultVirtualPointerFontSize = 8
	// DefaultVirtualPointerFontFamily is the default font family for the virtual pointer char.
	DefaultVirtualPointerFontFamily = ""
	// DefaultMouseActionIndicatorSize is the default mouse action indicator diameter in points.
	DefaultMouseActionIndicatorSize = 36
	// DefaultMouseActionIndicatorBorderWidth is the default mouse action indicator border width.
	DefaultMouseActionIndicatorBorderWidth = 2
	// DefaultMouseActionIndicatorDurationMS is the default mouse action indicator animation duration.
	DefaultMouseActionIndicatorDurationMS = 260
	// DefaultMouseActionIndicatorStartScale is the default starting scale for mouse action indicators.
	DefaultMouseActionIndicatorStartScale = 0.55
	// DefaultMouseActionIndicatorEndScale is the default ending scale for mouse action indicators.
	DefaultMouseActionIndicatorEndScale = 1.35
	// DefaultMouseActionIndicatorStartOpacity is the default starting opacity for mouse action indicators.
	DefaultMouseActionIndicatorStartOpacity = 0.85
	// DefaultMouseActionIndicatorEndOpacity is the default ending opacity for mouse action indicators.
	DefaultMouseActionIndicatorEndOpacity = 0.0
	// DefaultRecursiveGridLabelBackgroundPaddingX preserves automatic horizontal badge padding.
	DefaultRecursiveGridLabelBackgroundPaddingX = -1
	// DefaultRecursiveGridLabelBackgroundPaddingY preserves automatic vertical badge padding.
	DefaultRecursiveGridLabelBackgroundPaddingY = -1
	// DefaultRecursiveGridLabelBackgroundBorderRadius preserves the automatic pill radius.
	DefaultRecursiveGridLabelBackgroundBorderRadius = -1
	// DefaultRecursiveGridLabelBackgroundBorderWidth is the default label badge border width.
	DefaultRecursiveGridLabelBackgroundBorderWidth = 1

	// DefaultRecursiveGridLabelChar is the default label character override (empty = use key character).
	DefaultRecursiveGridLabelChar = ""
	// DefaultRecursiveGridSubKeyPreviewLabelChar is the default sub-key preview label character override (empty = use key character).
	DefaultRecursiveGridSubKeyPreviewLabelChar = ""
	// DefaultRecursiveGridLabelAutohideMultiplier is the default minimum cell size multiplier
	// for main label autohide. Set to 0 to disable autohide.
	DefaultRecursiveGridLabelAutohideMultiplier = 1.5
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

	// DefaultMonitorSelectFontSize is the default font size for monitor select labels.
	DefaultMonitorSelectFontSize = 96
	// DefaultMonitorSelectSubtitleFontSize is the default font size for monitor select subtitles.
	DefaultMonitorSelectSubtitleFontSize = 18
	// DefaultMonitorSelectCharacters is the default characters for monitor select labels.
	DefaultMonitorSelectCharacters = "123456789"
	// DefaultMonitorSelectBorderRadius disables automatic radius for monitor select badges (-1 = auto).
	DefaultMonitorSelectBorderRadius = -1
	// DefaultMonitorSelectPaddingX disables automatic horizontal padding (-1 = auto).
	DefaultMonitorSelectPaddingX = -1
	// DefaultMonitorSelectPaddingY disables automatic vertical padding (-1 = auto).
	DefaultMonitorSelectPaddingY = -1
)

func newDefaultConfig() *Config {
	return &Config{
		General:         defaultGeneral(),
		Theme:           defaultThemeConfig(),
		Hotkeys:         defaultHotkeys(),
		Hints:           defaultHints(),
		Grid:            defaultGrid(),
		RecursiveGrid:   defaultRecursiveGrid(),
		VirtualPointer:  defaultVirtualPointer(),
		MouseAction:     defaultMouseAction(),
		ModeIndicator:   defaultModeIndicator(),
		StickyModifiers: defaultStickyModifiers(),
		MonitorSelect:   defaultMonitorSelect(),
		Scroll:          defaultScroll(),
		Logging:         defaultLogging(),
		SmoothCursor:    defaultSmoothCursor(),
		SmoothScroll:    defaultSmoothScroll(),
		HeldRepeat:      defaultHeldRepeat(),
		Systray:         defaultSystray(),
	}
}

func defaultGeneral() GeneralConfig {
	return GeneralConfig{
		ExcludedApps:                      []string{},
		PassthroughUnboundedKeys:          false,
		ShouldExitAfterPassthrough:        false,
		PassthroughUnboundedKeysBlacklist: []string{},
		HideOverlayInScreenShare:          false,
		KBLayoutToUse:                     "",
		ExecShell:                         DefaultExecShell,
		ExecShellArgs:                     []string{DefaultExecShellFlag},
	}
}

func defaultHotkeys() HotkeysConfig {
	return HotkeysConfig{
		Bindings: map[string][]string{
			"Primary+Shift+Space": {ModeNameHints},
			"Primary+Shift+G":     {ModeNameGrid},
			"Primary+Shift+C":     {ModeNameRecursiveGrid},
			"Primary+Shift+S":     {ModeNameScroll},
		},
	}
}

func defaultHints() HintsConfig {
	return HintsConfig{
		Enabled:        true,
		Strategy:       domain.StrategyAXTree,
		CaptureScope:   domain.CaptureScopeWindow,
		HintCharacters: "asdfghjkl",
		LabelDirection: domain.LabelDirectionNormal,
		MaxDepth:       DefaultMaxDepth,
		Hotkeys: map[string]StringOrStringArray{
			KeyDisplayEscape:    {CmdIdle},
			"/":                 {"action search_hints"},
			KeyDisplayBackspace: {CmdBackspace},
			"Tab":               {"action cycle_hint"},
			"Shift+Tab":         {"action cycle_hint --backward"},
			KeyComboShiftL:      {CmdLeftClick},
			KeyComboShiftR:      {CmdRightClick},
			KeyComboShiftM:      {CmdMiddleClick},
			KeyComboShiftI:      {CmdLeftMouseDown},
			KeyComboShiftU:      {CmdLeftMouseUp},
			"Up":                {CmdMoveMouseUp},
			KeyDisplayDown:      {CmdMoveMouseDown},
			KeyDisplayLeft:      {CmdMoveMouseLeft},
			KeyDisplayRight:     {CmdMoveMouseRight},
		},

		UI: HintsUI{
			FontSize:         DefaultHintFontSize,
			FontFamily:       "",
			BorderRadius:     DefaultHintBorderRadius,
			PaddingX:         DefaultHintPaddingX,
			PaddingY:         DefaultHintPaddingY,
			BorderWidth:      1,
			Placement:        HintPlacementDefault,
			BackgroundColor:  Color{},
			TextColor:        Color{},
			MatchedTextColor: Color{},
			BorderColor:      Color{},
		},
		SearchInputUI: SearchInputUI{
			FontSize:        DefaultHintFontSize,
			FontFamily:      "",
			BorderRadius:    DefaultHintBorderRadius,
			PaddingX:        DefaultHintPaddingX,
			PaddingY:        DefaultHintPaddingY,
			BorderWidth:     1,
			Position:        SearchInputPositionDefault,
			XOffset:         0,
			YOffset:         DefaultSearchInputYOffset,
			Width:           DefaultSearchInputWidth,
			BackgroundColor: Color{},
			TextColor:       Color{},
			BorderColor:     Color{},
		},
		BoundaryHighlight: BoundaryHighlightUI{
			Enabled:         false,
			BorderWidth:     DefaultHintBoundaryBorderWidth,
			BorderRadius:    DefaultHintBoundaryBorderRadius,
			BorderColor:     Color{},
			BackgroundColor: Color{},
		},
		Vision: HintsVisionConfig{
			DetectText:                    true,
			DetectRectangles:              true,
			RequestTimeoutMS:              DefaultVisionRequestTimeoutMS,
			MinimumConfidence:             DefaultVisionMinimumConfidence,
			MergeIOUThreshold:             DefaultVisionMergeIOUThreshold,
			RectangleMaxCandidates:        DefaultVisionRectangleMaxCandidates,
			RectangleMinSize:              DefaultVisionRectangleMinSize,
			RectangleMinAspect:            DefaultVisionRectangleMinAspect,
			RectangleMaxAspect:            DefaultVisionRectangleMaxAspect,
			ButtonMinConfidence:           DefaultVisionButtonMinConfidence,
			ButtonMinAspect:               DefaultVisionButtonMinAspect,
			ButtonMaxAspect:               DefaultVisionButtonMaxAspect,
			ButtonIconMaxSize:             DefaultVisionButtonIconMaxSize,
			LinkMinAspect:                 DefaultVisionLinkMinAspect,
			LinkMaxHeight:                 DefaultVisionLinkMaxHeight,
			LinkMinWidth:                  DefaultVisionLinkMinWidth,
			ImageMinSize:                  DefaultVisionImageMinSize,
			CheckboxMaxSize:               DefaultVisionCheckboxMaxSize,
			GenericClickableMinConfidence: DefaultVisionGenericClickableMinConfidence,
		},

		IncludeMenubarHints:           false,
		AdditionalMenubarHintsTargets: []string{},
		IncludeDockHints:              false,
		IncludeNCHints:                false,
		IncludeStageManagerHints:      false,
		IncludePIPHints:               false,
		IncludeScreenCaptureHints:     false,
		DetectMissionControl:          false,
		OnMissionControlActivated:     nil,
		OnMissionControlDeactivated:   nil,

		// Semantic role names resolve to each platform's native
		// accessibility vocabulary at load time, so one default serves
		// every platform. See internal/domain/element/vocabulary.go.
		ClickableRoles: slices.Clone(element.DefaultClickableRoles),

		IgnoreClickableCheck: false,
		VisibleCheckEnabled:  false,

		AppConfigs: []AppConfig{},
	}
}

func defaultGrid() GridConfig {
	return GridConfig{
		Enabled: true,

		// Assigned from the domain constant rather than written out, because
		// the grid falls back to that same set when the configured characters
		// cannot label anything. Two literals is what they were, and they
		// drifted (see grid.DefaultCharacters).
		Characters:     domainGrid.DefaultCharacters,
		SublayerKeys:   domainGrid.DefaultCharacters,
		MaxLabelLength: domainGrid.DefaultMaxLabelLength,

		// Empty is the meaning, not a missing default: "" tells the grid to
		// infer its row and column labels from the characters it is drawn
		// with, so shipping a value here would take that inference away
		// from everyone who only sets characters.
		RowLabels: "",
		ColLabels: "",

		Hotkeys: map[string]StringOrStringArray{
			KeyDisplayEscape:    {CmdIdle},
			"`":                 {CmdToggleCursorFollowSelection},
			KeyDisplaySpace:     {"action reset"},
			KeyDisplayBackspace: {CmdBackspace},
			KeyComboShiftL:      {CmdLeftClick},
			KeyComboShiftR:      {CmdRightClick},
			KeyComboShiftM:      {CmdMiddleClick},
			KeyComboShiftI:      {CmdLeftMouseDown},
			KeyComboShiftU:      {CmdLeftMouseUp},
			"Up":                {CmdMoveMouseUp},
			KeyDisplayDown:      {CmdMoveMouseDown},
			KeyDisplayLeft:      {CmdMoveMouseLeft},
			KeyDisplayRight:     {CmdMoveMouseRight},
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
	}
}

func defaultRecursiveGrid() RecursiveGridConfig {
	return RecursiveGridConfig{
		Enabled: true,
		Animation: RecursiveGridAnimationConfig{
			Enabled:    true,
			DurationMS: DefaultRecursiveGridAnimationDurationMS,
		},
		GridCols: 3, //nolint:mnd
		GridRows: 3, //nolint:mnd

		Keys: "rtyfghvbn", // 3x3 grid: left-to-right, top-to-bottom
		Hotkeys: map[string]StringOrStringArray{
			KeyDisplayEscape:    {CmdIdle},
			"`":                 {CmdToggleCursorFollowSelection},
			KeyDisplaySpace:     {"action reset"},
			KeyDisplayBackspace: {CmdBackspace},
			KeyComboShiftL:      {CmdLeftClick},
			KeyComboShiftR:      {CmdRightClick},
			KeyComboShiftM:      {CmdMiddleClick},
			KeyComboShiftI:      {CmdLeftMouseDown},
			KeyComboShiftU:      {CmdLeftMouseUp},
			"Up":                {CmdMoveMouseUp},
			KeyDisplayDown:      {CmdMoveMouseDown},
			KeyDisplayLeft:      {CmdMoveMouseLeft},
			KeyDisplayRight:     {CmdMoveMouseRight},
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
			LabelChar:                       DefaultRecursiveGridLabelChar,
			LabelAutohideMultiplier:         DefaultRecursiveGridLabelAutohideMultiplier,
			SubKeyPreview:                   DefaultRecursiveGridSubKeyPreview,
			SubKeyPreviewFontSize:           DefaultRecursiveGridSubKeyPreviewFontSize,
			SubKeyPreviewAutohideMultiplier: DefaultRecursiveGridSubKeyPreviewAutohideMultiplier,
			SubKeyPreviewTextColor:          Color{},
			SubKeyPreviewLabelChar:          DefaultRecursiveGridSubKeyPreviewLabelChar,
		},

		MinSizeWidth:  DefaultRecursiveGridMinSizeWidth,
		MinSizeHeight: DefaultRecursiveGridMinSizeHeight,
		MaxDepth:      DefaultRecursiveGridMaxDepth,
	}
}

func defaultVirtualPointer() VirtualPointerConfig {
	return VirtualPointerConfig{
		UI: VirtualPointerUI{
			Char:       DefaultVirtualPointerChar,
			FontSize:   DefaultVirtualPointerFontSize,
			FontFamily: DefaultVirtualPointerFontFamily,
			TextColor:  Color{},
		},
	}
}

func defaultMouseAction() MouseActionConfig {
	return MouseActionConfig{
		Enabled: false,
		Actions: []string{
			"left_click",
			"right_click",
			"middle_click",
			"left_mouse_down",
			"left_mouse_up",
			"right_mouse_down",
			"right_mouse_up",
			"middle_mouse_down",
			"middle_mouse_up",
			"left_mouse_toggle",
			"right_mouse_toggle",
			"middle_mouse_toggle",
		},
		UI: MouseActionUI{
			Size:            DefaultMouseActionIndicatorSize,
			BorderWidth:     DefaultMouseActionIndicatorBorderWidth,
			BackgroundColor: Color{},
			BorderColor:     Color{},
			Shape:           "circle",
		},
		Animation: MouseActionAnimation{
			DurationMS:   DefaultMouseActionIndicatorDurationMS,
			StartScale:   DefaultMouseActionIndicatorStartScale,
			EndScale:     DefaultMouseActionIndicatorEndScale,
			StartOpacity: DefaultMouseActionIndicatorStartOpacity,
			EndOpacity:   DefaultMouseActionIndicatorEndOpacity,
			Easing:       "ease_out",
		},
	}
}

// Whether a mode's indicator shows by default, named so the table below reads
// as a table rather than as a column of bare booleans.
const (
	indicatorShown  = true
	indicatorHidden = false
)

// defaultModeIndicatorMode is one mode's row of the indicator table. Enabled
// and Text differ per mode; the three color overrides do not, and writing them
// once here is what keeps their default a stated one.
func defaultModeIndicatorMode(enabled bool, text string) ModeIndicatorModeConfig {
	return ModeIndicatorModeConfig{
		Enabled: enabled,
		Text:    text,

		// Empty is the meaning, not a missing default: an unset per-mode color
		// inherits the matching [mode_indicator.ui] value, light and dark leg
		// independently (Color.ForThemeWithOverride). Shipping a value here
		// would take that inheritance away from everyone who only styles the
		// shared block.
		BackgroundColor: Color{},
		TextColor:       Color{},
		BorderColor:     Color{},
	}
}

func defaultModeIndicator() ModeIndicatorConfig {
	return ModeIndicatorConfig{
		Scroll:        defaultModeIndicatorMode(indicatorShown, "Scroll"),
		Hints:         defaultModeIndicatorMode(indicatorHidden, "Hints"),
		Grid:          defaultModeIndicatorMode(indicatorHidden, "Grid"),
		RecursiveGrid: defaultModeIndicatorMode(indicatorHidden, "Recursive Grid"),
		MonitorSelect: defaultModeIndicatorMode(indicatorHidden, "Monitor Select"),
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
	}
}

func defaultStickyModifiers() StickyModifiersConfig {
	return StickyModifiersConfig{
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
	}
}

func defaultMonitorSelect() MonitorSelectConfig {
	return MonitorSelectConfig{
		Enabled:    false,
		Characters: DefaultMonitorSelectCharacters,
		Hotkeys: map[string]StringOrStringArray{
			KeyDisplayEscape: {CmdIdle},
		},
		UI: MonitorSelectUI{
			FontSize:           DefaultMonitorSelectFontSize,
			FontFamily:         "",
			BorderRadius:       DefaultMonitorSelectBorderRadius,
			PaddingX:           DefaultMonitorSelectPaddingX,
			PaddingY:           DefaultMonitorSelectPaddingY,
			BorderWidth:        1,
			BackgroundColor:    Color{},
			TextColor:          Color{},
			MatchedTextColor:   Color{},
			BorderColor:        Color{},
			BackdropColor:      Color{},
			SubtitleFontSize:   DefaultMonitorSelectSubtitleFontSize,
			SubtitleFontFamily: "",
			SubtitleTextColor:  Color{},
		},
	}
}

func defaultScroll() ScrollConfig {
	return ScrollConfig{
		ScrollStep:     DefaultScrollStep,
		ScrollStepHalf: DefaultScrollStepHalf,
		ScrollStepFull: DefaultScrollStepFull,
		InvertScroll:   DefaultScrollInvert,
		AppConfigs:     []AppConfig{},
		Hotkeys: map[string]StringOrStringArray{
			KeyDisplayEscape: {CmdIdle},
			"k":              {"action scroll_up"},
			"j":              {"action scroll_down"},
			"h":              {"action scroll_left"},
			"l":              {"action scroll_right"},
			"gg":             {CmdGoTop},
			"Shift+G":        {"action go_bottom"},
			"u":              {"action page_up"},
			"PageUp":         {"action page_up"},
			"d":              {"action page_down"},
			"PageDown":       {"action page_down"},
			KeyComboShiftL:   {CmdLeftClick},
			KeyComboShiftR:   {CmdRightClick},
			KeyComboShiftM:   {CmdMiddleClick},
			KeyComboShiftI:   {CmdLeftMouseDown},
			KeyComboShiftU:   {CmdLeftMouseUp},
			"Up":             {CmdMoveMouseUp},
			KeyDisplayDown:   {CmdMoveMouseDown},
			KeyDisplayLeft:   {CmdMoveMouseLeft},
			KeyDisplayRight:  {CmdMoveMouseRight},
		},
	}
}

func defaultLogging() LoggingConfig {
	return LoggingConfig{
		LogLevel:           "info",
		LogFile:            "",
		DisableFileLogging: true,
		MaxFileSize:        DefaultMaxFileSize,
		MaxBackups:         DefaultMaxBackups,
		MaxAge:             DefaultMaxAge,
	}
}

func defaultSmoothCursor() SmoothCursorConfig {
	return SmoothCursorConfig{
		MoveMouseEnabled:         false,
		Steps:                    DefaultSmoothCursorSteps,
		MaxDuration:              DefaultSmoothCursorMaxDuration,
		DurationPerPixel:         DefaultSmoothCursorDurationPerPixel,
		RelativeMovementDuration: DefaultSmoothCursorRelativeMovementDuration,
	}
}

func defaultSmoothScroll() SmoothScrollConfig {
	return SmoothScrollConfig{
		Enabled:          false,
		Steps:            DefaultSmoothScrollSteps,
		MaxDuration:      DefaultSmoothScrollMaxDuration,
		DurationPerPixel: DefaultSmoothScrollDurationPerPixel,
	}
}

func defaultHeldRepeat() HeldRepeatConfig {
	return HeldRepeatConfig{
		Enabled:            false,
		InitialDelay:       DefaultHeldRepeatInitialDelay,
		Interval:           DefaultHeldRepeatInterval,
		AccelEnabled:       false,
		AccelRampMs:        DefaultHeldRepeatAccelRampMs,
		AccelMaxMultiplier: DefaultHeldRepeatAccelMaxMultiplier,
		AccelTargets:       []string{string(action.NameMoveMouseRelative)},
	}
}

func defaultSystray() SystrayConfig {
	return SystrayConfig{
		Enabled: true, // Enabled by default
	}
}
