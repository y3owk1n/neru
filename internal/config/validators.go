package config

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// validateClickableRoles rejects role entries that name neither a semantic role
// nor a native role in a known vocabulary. Entries that are well-formed but
// belong to another platform are accepted silently here and reported by
// `neru roles` / `neru doctor`, so one config can serve several machines.
func validateClickableRoles(field string, roles []string) error {
	resolution := element.ResolveRolesForCurrentPlatform(roles)

	messages := resolution.FatalMessages()
	if len(messages) == 0 {
		return nil
	}

	return derrors.Newf(
		derrors.CodeInvalidConfig,
		"%s: %s (run `neru roles` for the accepted role names)",
		field,
		strings.Join(messages, "; "),
	)
}

// ValidateHints checks the hints configuration, reporting the first problem it
// finds.
//
// The order the checks run in is the order a user sees their mistakes, so it is
// listed here rather than left implicit in one long body. It is also why the
// search-input checks come in two parts with the boundary-highlight checks
// between them: that is where they have always run, and moving them would change
// which problem a config with several is told about first.
func (c *Config) ValidateHints() error {
	checks := []func() error{
		c.validateHintClickableRoles,
		c.validateHintCharacters,
		c.validateHintColors,
		c.validateHintLabelUI,
		c.validateHintSearchInputGeometry,
		c.validateHintBoundaryHighlight,
		c.validateHintSearchInputPlacement,
		c.validateHintScanDepth,
		c.validateHintMissionControl,
		c.validateHintVocabulary,
		func() error { return validateHintsVisionConfig(c.Hints.Vision) },
	}

	for _, check := range checks {
		checkErr := check()
		if checkErr != nil {
			return checkErr
		}
	}

	return nil
}

// validateHintClickableRoles checks the roles hints are drawn for, including the
// extra roles an individual application adds.
func (c *Config) validateHintClickableRoles() error {
	if c.Hints.Enabled && len(c.Hints.ClickableRoles) == 0 {
		return derrors.New(derrors.CodeInvalidConfig,
			"hints.clickable_roles cannot be empty when hints are enabled")
	}

	rolesErr := validateClickableRoles("hints.clickable_roles", c.Hints.ClickableRoles)
	if rolesErr != nil {
		return rolesErr
	}

	for _, appConfig := range c.Hints.AppConfigs {
		appRolesErr := validateClickableRoles(
			"hints.app_configs.additional_clickable_roles",
			appConfig.AdditionalClickable,
		)
		if appRolesErr != nil {
			return appRolesErr
		}
	}

	return nil
}

// validateHintCharacters checks the alphabet hint labels are drawn from.
//
// Labels are typed, so the alphabet has to be typeable and unambiguous: at least
// two characters to build labels out of, ASCII so every keyboard can produce
// them, and no character twice once case is folded, since matching is
// case-insensitive and a repeat would make two labels indistinguishable.
func (c *Config) validateHintCharacters() error {
	if strings.TrimSpace(c.Hints.HintCharacters) == "" {
		return derrors.New(derrors.CodeInvalidConfig, "hint_characters cannot be empty")
	}

	if len(c.Hints.HintCharacters) < MinCharactersLength {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hint_characters must contain at least 2 characters",
		)
	}

	for _, char := range c.Hints.HintCharacters {
		if char > unicode.MaxASCII {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"hint_characters can only contain ASCII characters",
			)
		}
	}

	seen := make(map[rune]struct{}, len(c.Hints.HintCharacters))

	for _, char := range c.Hints.HintCharacters {
		upper := unicode.ToUpper(char)

		_, duplicate := seen[upper]
		if duplicate {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hint_characters contains duplicate character %q",
				char,
			)
		}

		seen[upper] = struct{}{}
	}

	return nil
}

// validateHintColors checks every color the hints overlay draws with.
func (c *Config) validateHintColors() error {
	return validateColors([]colorField{
		{c.Hints.UI.BackgroundColor, "hints.ui.background_color"},
		{c.Hints.UI.TextColor, "hints.ui.text_color"},
		{c.Hints.UI.MatchedTextColor, "hints.ui.matched_text_color"},
		{c.Hints.UI.BorderColor, "hints.ui.border_color"},
		{c.Hints.SearchInputUI.BackgroundColor, "hints.search_input_ui.background_color"},
		{c.Hints.SearchInputUI.TextColor, "hints.search_input_ui.text_color"},
		{c.Hints.SearchInputUI.BorderColor, "hints.search_input_ui.border_color"},
		{c.Hints.BoundaryHighlight.BackgroundColor, "hints.boundary_highlight.background_color"},
		{c.Hints.BoundaryHighlight.BorderColor, "hints.boundary_highlight.border_color"},
	})
}

// validateHintLabelUI checks the size and shape of a hint label.
//
// It also settles the placement of the label relative to its element, defaulting
// an unset placement rather than refusing it.
func (c *Config) validateHintLabelUI() error {
	if c.Hints.UI.FontSize < 1 || c.Hints.UI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	radiusErr := validateMinValue(c.Hints.UI.BorderRadius, -1, "hints.ui.border_radius")
	if radiusErr != nil {
		return radiusErr
	}

	paddingXErr := validateMinValue(c.Hints.UI.PaddingX, -1, "hints.ui.padding_x")
	if paddingXErr != nil {
		return paddingXErr
	}

	paddingYErr := validateMinValue(c.Hints.UI.PaddingY, -1, "hints.ui.padding_y")
	if paddingYErr != nil {
		return paddingYErr
	}

	if c.Hints.UI.BorderWidth < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "hints.ui.border_width must be non-negative")
	}

	switch c.Hints.UI.Placement {
	case "top", "center", placementBottom:
	case "":
		c.Hints.UI.Placement = placementBottom
	default:
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.ui.placement must be one of top, center, "+placementBottom,
		)
	}

	return nil
}

// validateHintSearchInputGeometry checks the size and shape of the search input.
// Where it sits on screen is checked separately, after the boundary highlight.
func (c *Config) validateHintSearchInputGeometry() error {
	if c.Hints.SearchInputUI.FontSize < 1 || c.Hints.SearchInputUI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.search_input_ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	radiusErr := validateMinValue(
		c.Hints.SearchInputUI.BorderRadius,
		-1,
		"hints.search_input_ui.border_radius",
	)
	if radiusErr != nil {
		return radiusErr
	}

	paddingXErr := validateMinValue(
		c.Hints.SearchInputUI.PaddingX,
		-1,
		"hints.search_input_ui.padding_x",
	)
	if paddingXErr != nil {
		return paddingXErr
	}

	paddingYErr := validateMinValue(
		c.Hints.SearchInputUI.PaddingY,
		-1,
		"hints.search_input_ui.padding_y",
	)
	if paddingYErr != nil {
		return paddingYErr
	}

	if c.Hints.SearchInputUI.BorderWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.search_input_ui.border_width must be non-negative",
		)
	}

	return nil
}

// validateHintBoundaryHighlight checks the outline drawn around the element a
// hint points at.
func (c *Config) validateHintBoundaryHighlight() error {
	if c.Hints.BoundaryHighlight.BorderWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.boundary_highlight.border_width must be non-negative",
		)
	}

	return validateMinValue(
		c.Hints.BoundaryHighlight.BorderRadius,
		-1,
		"hints.boundary_highlight.border_radius",
	)
}

// validateHintSearchInputPlacement checks where the search input sits and how
// wide it is.
func (c *Config) validateHintSearchInputPlacement() error {
	switch c.Hints.SearchInputUI.Position {
	case "top_left", "top_center", "top_right",
		"center",
		"bottom_left", "bottom_center", "bottom_right":
	default:
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.search_input_ui.position must be one of top_left, top_center, top_right, center, bottom_left, bottom_center, bottom_right",
		)
	}

	return validateMinValue(c.Hints.SearchInputUI.Width, 1, "hints.search_input_ui.width")
}

// validateHintScanDepth checks how deep the accessibility tree walk may go.
func (c *Config) validateHintScanDepth() error {
	return validateMinValue(c.Hints.MaxDepth, 0, "hints.max_depth")
}

// validateHintMissionControl checks the steps run as Mission Control opens and
// closes, and the settings they depend on.
func (c *Config) validateHintMissionControl() error {
	if (len(c.Hints.OnMissionControlActivated) > 0 ||
		len(c.Hints.OnMissionControlDeactivated) > 0) &&
		!c.Hints.DetectMissionControl {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.on_mission_control_activated/deactivated requires hints.detect_mission_control = true",
		)
	}

	if c.Hints.DetectMissionControl && !c.Hints.IncludeDockHints {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.detect_mission_control requires hints.include_dock_hints = true "+
				"(dock windows are the only element source available during Mission Control)",
		)
	}

	activatedErr := validateMissionControlSteps(
		"hints.on_mission_control_activated",
		c.Hints.OnMissionControlActivated,
	)
	if activatedErr != nil {
		return activatedErr
	}

	return validateMissionControlSteps(
		"hints.on_mission_control_deactivated",
		c.Hints.OnMissionControlDeactivated,
	)
}

// validateMissionControlSteps checks one list of Mission Control steps.
func validateMissionControlSteps(field string, steps []string) error {
	for idx, actionStr := range steps {
		trimmed := strings.TrimSpace(actionStr)
		if trimmed == "" {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s[%d] cannot be empty",
				field,
				idx,
			)
		}

		stepErr := validateHotkeyActionString(trimmed)
		if stepErr != nil {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s[%d]: %v",
				field,
				idx,
				stepErr,
			)
		}
	}

	return nil
}

// validateHintVocabulary checks the settings that name one of a fixed set of
// behaviors: how elements are found, and how labels are enumerated.
func (c *Config) validateHintVocabulary() error {
	switch c.Hints.Strategy {
	case StrategyAXTree, StrategyVision, "":
	default:
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.strategy must be %q or %q",
			StrategyAXTree, StrategyVision,
		)
	}

	switch c.Hints.LabelDirection {
	case LabelDirectionReverse, LabelDirectionNormal, "":
	default:
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.label_direction must be %q or %q",
			LabelDirectionReverse, LabelDirectionNormal,
		)
	}

	return nil
}

func validateHintsVisionConfig(vision HintsVisionConfig) error {
	if !vision.DetectText && !vision.DetectRectangles {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.vision must enable detect_text or detect_rectangles",
		)
	}

	if vision.RequestTimeoutMS <= 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.vision.request_timeout_ms must be greater than 0",
		)
	}

	if vision.RectangleMaxCandidates <= 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.vision.rectangle_max_candidates must be greater than 0",
		)
	}

	err := validateUnitFloat(
		"hints.vision.minimum_confidence",
		vision.MinimumConfidence,
	)
	if err != nil {
		return err
	}

	err = validatePositiveUnitFloat(
		"hints.vision.merge_iou_threshold",
		vision.MergeIOUThreshold,
	)
	if err != nil {
		return err
	}

	err = validateUnitFloat(
		"hints.vision.rectangle_min_size",
		vision.RectangleMinSize,
	)
	if err != nil {
		return err
	}

	err = validatePositiveUnitFloat(
		"hints.vision.button_min_confidence",
		vision.ButtonMinConfidence,
	)
	if err != nil {
		return err
	}

	err = validatePositiveUnitFloat(
		"hints.vision.generic_clickable_min_confidence",
		vision.GenericClickableMinConfidence,
	)
	if err != nil {
		return err
	}

	if vision.RectangleMinAspect <= 0 || vision.RectangleMaxAspect <= 0 ||
		vision.RectangleMinAspect > vision.RectangleMaxAspect {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.vision rectangle aspect limits must be > 0 and min <= max",
		)
	}

	if vision.ButtonMinAspect <= 0 || vision.ButtonMaxAspect <= 0 ||
		vision.ButtonMinAspect > vision.ButtonMaxAspect {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.vision button aspect limits must be > 0 and min <= max",
		)
	}

	if vision.ButtonIconMaxSize <= 0 || vision.LinkMaxHeight <= 0 ||
		vision.LinkMinWidth <= 0 || vision.ImageMinSize <= 0 ||
		vision.CheckboxMaxSize <= 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.vision size thresholds must be greater than 0",
		)
	}

	if vision.LinkMinAspect <= 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.vision.link_min_aspect must be greater than 0",
		)
	}

	return nil
}

// ValidateGrid validates the grid configuration.
func (c *Config) ValidateGrid() error {
	if !c.Grid.Enabled {
		return nil
	}

	if strings.TrimSpace(c.Grid.Characters) == "" {
		return derrors.New(derrors.CodeInvalidConfig, "grid.characters cannot be empty")
	}

	for _, r := range c.Grid.Characters {
		if r > unicode.MaxASCII {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"grid.characters can only contain ASCII characters",
			)
		}
	}

	if c.Grid.SublayerKeys != "" {
		for _, r := range c.Grid.SublayerKeys {
			if r > unicode.MaxASCII {
				return derrors.New(
					derrors.CodeInvalidConfig,
					"grid.sublayer_keys can only contain ASCII characters",
				)
			}
		}
	}

	err := validateColors([]colorField{
		{c.Grid.UI.BackgroundColor, "grid.ui.background_color"},
		{c.Grid.UI.TextColor, "grid.ui.text_color"},
		{c.Grid.UI.MatchedTextColor, "grid.ui.matched_text_color"},
		{c.Grid.UI.MatchedBackgroundColor, "grid.ui.matched_background_color"},
		{c.Grid.UI.MatchedBorderColor, "grid.ui.matched_border_color"},
		{c.Grid.UI.BorderColor, "grid.ui.border_color"},
	})
	if err != nil {
		return err
	}

	if c.Grid.UI.FontSize < 1 || c.Grid.UI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"grid.ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	if c.Grid.UI.BorderWidth < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "grid.ui.border_width must be non-negative")
	}

	err = validateAppConfigsWithCallback(
		"grid",
		c.Grid.AppConfigs,
		rejectModeSpecificFields("grid"),
	)
	if err != nil {
		return err
	}

	return nil
}

// ValidateMonitorSelect validates the monitor_select configuration.
func (c *Config) ValidateMonitorSelect() error {
	if !c.MonitorSelect.Enabled {
		return nil
	}

	if c.MonitorSelect.Characters == "" {
		return derrors.New(derrors.CodeInvalidConfig, "monitor_select.characters cannot be empty")
	}

	if utf8.RuneCountInString(c.MonitorSelect.Characters) < MinCharactersLength {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"monitor_select.characters must contain at least 2 characters",
		)
	}

	err := validateColors([]colorField{
		{c.MonitorSelect.UI.BackgroundColor, "monitor_select.ui.background_color"},
		{c.MonitorSelect.UI.TextColor, "monitor_select.ui.text_color"},
		{c.MonitorSelect.UI.MatchedTextColor, "monitor_select.ui.matched_text_color"},
		{c.MonitorSelect.UI.BorderColor, "monitor_select.ui.border_color"},
		{c.MonitorSelect.UI.BackdropColor, "monitor_select.ui.backdrop_color"},
		{c.MonitorSelect.UI.SubtitleTextColor, "monitor_select.ui.subtitle_text_color"},
	})
	if err != nil {
		return err
	}

	if c.MonitorSelect.UI.FontSize < 1 || c.MonitorSelect.UI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"monitor_select.ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	if c.MonitorSelect.UI.SubtitleFontSize < 1 ||
		c.MonitorSelect.UI.SubtitleFontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"monitor_select.ui.subtitle_font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	return nil
}

// ValidateStickyModifiers validates sticky modifier settings.
func (c *Config) ValidateStickyModifiers() error {
	if !c.StickyModifiers.Enabled {
		return nil
	}

	if c.StickyModifiers.TapMaxDuration < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"sticky_modifiers.tap_max_duration must be >= 0",
		)
	}

	if c.StickyModifiers.UI.FontSize < 1 || c.StickyModifiers.UI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"sticky_modifiers.ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	return validateColors([]colorField{
		{c.StickyModifiers.UI.BackgroundColor, "sticky_modifiers.ui.background_color"},
		{c.StickyModifiers.UI.TextColor, "sticky_modifiers.ui.text_color"},
		{c.StickyModifiers.UI.BorderColor, "sticky_modifiers.ui.border_color"},
	})
}

// ValidateSmoothCursor validates smooth cursor settings.
func (c *Config) ValidateSmoothCursor() error {
	if c.SmoothCursor.Steps < 1 {
		return derrors.New(derrors.CodeInvalidConfig, "smooth_cursor.steps must be >= 1")
	}

	if c.SmoothCursor.MaxDuration < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "smooth_cursor.max_duration must be >= 0")
	}

	if c.SmoothCursor.DurationPerPixel < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"smooth_cursor.duration_per_pixel must be >= 0",
		)
	}

	return nil
}

// ValidateSmoothScroll validates smooth scroll settings.
func (c *Config) ValidateSmoothScroll() error {
	if c.SmoothScroll.Steps < 1 {
		return derrors.New(derrors.CodeInvalidConfig, "smooth_scroll.steps must be >= 1")
	}

	if c.SmoothScroll.MaxDuration < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "smooth_scroll.max_duration must be >= 0")
	}

	if c.SmoothScroll.DurationPerPixel < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"smooth_scroll.duration_per_pixel must be >= 0",
		)
	}

	return nil
}

// ValidateHeldRepeat validates held-key repeat settings.
func (c *Config) ValidateHeldRepeat() error {
	if c.HeldRepeat.InitialDelay < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "held_repeat.initial_delay_ms must be >= 0")
	}

	if c.HeldRepeat.Interval < 1 {
		return derrors.New(derrors.CodeInvalidConfig, "held_repeat.interval_ms must be >= 1")
	}

	return nil
}

// ValidateRecursiveGrid validates recursive grid configuration.
func (c *Config) ValidateRecursiveGrid() error {
	if !c.RecursiveGrid.Enabled {
		return nil
	}

	if c.RecursiveGrid.GridCols < DefaultRecursiveGridMinGridCols {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"recursive_grid.grid_cols must be >= %d",
			DefaultRecursiveGridMinGridCols,
		)
	}

	if c.RecursiveGrid.GridRows < DefaultRecursiveGridMinGridRows {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"recursive_grid.grid_rows must be >= %d",
			DefaultRecursiveGridMinGridRows,
		)
	}

	if c.RecursiveGrid.GridCols*c.RecursiveGrid.GridRows < DefaultRecursiveGridMinTotalCells {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid grid must have at least 2 cells (grid_cols * grid_rows >= 2); a 1x1 grid cannot subdivide",
		)
	}

	if c.RecursiveGrid.MaxDepth < 1 {
		return derrors.New(derrors.CodeInvalidConfig, "recursive_grid.max_depth must be >= 1")
	}

	if c.RecursiveGrid.Animation.DurationMS < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.animation.duration_ms must be non-negative",
		)
	}

	expectedKeys := c.RecursiveGrid.GridCols * c.RecursiveGrid.GridRows
	if utf8.RuneCountInString(c.RecursiveGrid.Keys) != expectedKeys {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"recursive_grid.keys must have %d characters",
			expectedKeys,
		)
	}

	for _, layer := range c.RecursiveGrid.Layers {
		if layer.Depth < 0 {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"recursive_grid.layers.depth must be >= 0",
			)
		}

		if layer.GridCols < DefaultRecursiveGridMinGridCols ||
			layer.GridRows < DefaultRecursiveGridMinGridRows {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"recursive_grid.layers grid dimensions must be >= 1",
			)
		}

		if layer.GridCols*layer.GridRows < DefaultRecursiveGridMinTotalCells {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"recursive_grid.layers depth %d must have at least 2 cells (grid_cols * grid_rows >= 2); a 1x1 grid cannot subdivide",
				layer.Depth,
			)
		}

		if utf8.RuneCountInString(layer.Keys) != layer.GridCols*layer.GridRows {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"recursive_grid.layers depth %d keys length mismatch",
				layer.Depth,
			)
		}
	}

	err := validateColors([]colorField{
		{c.RecursiveGrid.UI.LineColor, "recursive_grid.ui.line_color"},
		{c.RecursiveGrid.UI.HighlightColor, "recursive_grid.ui.highlight_color"},
		{c.RecursiveGrid.UI.TextColor, "recursive_grid.ui.text_color"},
		{c.RecursiveGrid.UI.LabelBackgroundColor, "recursive_grid.ui.label_background_color"},
		{c.RecursiveGrid.UI.SubKeyPreviewTextColor, "recursive_grid.ui.sub_key_preview_text_color"},
	})
	if err != nil {
		return err
	}

	if c.RecursiveGrid.UI.LineWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.line_width must be non-negative",
		)
	}

	if c.RecursiveGrid.UI.FontSize < 1 || c.RecursiveGrid.UI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	if c.RecursiveGrid.UI.SubKeyPreviewFontSize < 1 ||
		c.RecursiveGrid.UI.SubKeyPreviewFontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.sub_key_preview_font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	if utf8.RuneCountInString(c.RecursiveGrid.UI.LabelChar) > 1 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.label_char must be empty or a single character",
		)
	}

	if utf8.RuneCountInString(c.RecursiveGrid.UI.SubKeyPreviewLabelChar) > 1 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"recursive_grid.ui.sub_key_preview_label_char must be empty or a single character",
		)
	}

	err = validateAppConfigsWithCallback(
		"recursive_grid",
		c.RecursiveGrid.AppConfigs,
		rejectModeSpecificFields("recursive_grid"),
	)
	if err != nil {
		return err
	}

	return nil
}

// ValidateVirtualPointer validates virtual pointer configuration.
func (c *Config) ValidateVirtualPointer() error {
	err := validateColors([]colorField{
		{c.VirtualPointer.UI.TextColor, "virtual_pointer.ui.text_color"},
	})
	if err != nil {
		return err
	}

	charLen := utf8.RuneCountInString(c.VirtualPointer.UI.Char)
	if charLen != 1 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"virtual_pointer.ui.char must be exactly 1 character",
		)
	}

	if c.VirtualPointer.UI.FontSize < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "virtual_pointer.ui.font_size must be >= 0")
	}

	if c.VirtualPointer.UI.FontSize > maxFontSize {
		return derrors.Newf(derrors.CodeInvalidConfig,
			"virtual_pointer.ui.font_size must be <= %d", maxFontSize)
	}

	return nil
}

// ValidateMouseAction validates mouse action indicator configuration.
func (c *Config) ValidateMouseAction() error {
	err := validateColors([]colorField{
		{c.MouseAction.UI.BackgroundColor, "mouse_action_indicator.ui.background_color"},
		{c.MouseAction.UI.BorderColor, "mouse_action_indicator.ui.border_color"},
	})
	if err != nil {
		return err
	}

	if c.MouseAction.UI.Size < 1 {
		return derrors.New(derrors.CodeInvalidConfig, "mouse_action_indicator.ui.size must be >= 1")
	}

	if c.MouseAction.UI.BorderWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"mouse_action_indicator.ui.border_width must be >= 0",
		)
	}

	switch c.MouseAction.UI.Shape {
	case "", "circle", "square":
	default:
		return derrors.New(
			derrors.CodeInvalidConfig,
			"mouse_action_indicator.ui.shape must be circle or square",
		)
	}

	if c.MouseAction.Animation.DurationMS < 1 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"mouse_action_indicator.animation.duration_ms must be >= 1",
		)
	}

	if c.MouseAction.Animation.StartScale < 0 || c.MouseAction.Animation.EndScale < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"mouse_action_indicator.animation scales must be non-negative",
		)
	}

	if !validOpacity(c.MouseAction.Animation.StartOpacity) ||
		!validOpacity(c.MouseAction.Animation.EndOpacity) {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"mouse_action_indicator.animation opacity values must be between 0 and 1",
		)
	}

	switch c.MouseAction.Animation.Easing {
	case "", "linear", "ease_in", "ease_out", "ease_in_out":
	default:
		return derrors.New(
			derrors.CodeInvalidConfig,
			"mouse_action_indicator.animation.easing must be linear, ease_in, ease_out, or ease_in_out",
		)
	}

	if len(c.MouseAction.Actions) == 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"mouse_action_indicator.actions must contain at least one mouse button action",
		)
	}

	for index, actionName := range c.MouseAction.Actions {
		actionType, parseErr := action.ParseType(actionName)
		if parseErr != nil || !actionType.IsMouseButton() {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"mouse_action_indicator.actions[%d] must be a mouse button action",
				index,
			)
		}
	}

	return nil
}
