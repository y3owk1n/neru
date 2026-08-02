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

// ValidateHints validates the hints configuration.
func (c *Config) ValidateHints() error {
	if c.Hints.Enabled && len(c.Hints.ClickableRoles) == 0 {
		return derrors.New(derrors.CodeInvalidConfig,
			"hints.clickable_roles cannot be empty when hints are enabled")
	}

	err := validateClickableRoles("hints.clickable_roles", c.Hints.ClickableRoles)
	if err != nil {
		return err
	}

	for _, appConfig := range c.Hints.AppConfigs {
		err = validateClickableRoles(
			"hints.app_configs.additional_clickable_roles",
			appConfig.AdditionalClickable,
		)
		if err != nil {
			return err
		}
	}

	if strings.TrimSpace(c.Hints.HintCharacters) == "" {
		return derrors.New(derrors.CodeInvalidConfig, "hint_characters cannot be empty")
	}

	if len(c.Hints.HintCharacters) < MinCharactersLength {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hint_characters must contain at least 2 characters",
		)
	}

	for _, r := range c.Hints.HintCharacters {
		if r > unicode.MaxASCII {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"hint_characters can only contain ASCII characters",
			)
		}
	}

	seen := make(map[rune]struct{}, len(c.Hints.HintCharacters))
	for _, char := range c.Hints.HintCharacters {
		upper := unicode.ToUpper(char)

		if _, ok := seen[upper]; ok {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hint_characters contains duplicate character %q",
				char,
			)
		}

		seen[upper] = struct{}{}
	}

	err = validateColors([]colorField{
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
	if err != nil {
		return err
	}

	if c.Hints.UI.FontSize < 1 || c.Hints.UI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	err = validateMinValue(c.Hints.UI.BorderRadius, -1, "hints.ui.border_radius")
	if err != nil {
		return err
	}

	err = validateMinValue(c.Hints.UI.PaddingX, -1, "hints.ui.padding_x")
	if err != nil {
		return err
	}

	err = validateMinValue(c.Hints.UI.PaddingY, -1, "hints.ui.padding_y")
	if err != nil {
		return err
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

	if c.Hints.SearchInputUI.FontSize < 1 || c.Hints.SearchInputUI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.search_input_ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	err = validateMinValue(
		c.Hints.SearchInputUI.BorderRadius,
		-1,
		"hints.search_input_ui.border_radius",
	)
	if err != nil {
		return err
	}

	err = validateMinValue(c.Hints.SearchInputUI.PaddingX, -1, "hints.search_input_ui.padding_x")
	if err != nil {
		return err
	}

	err = validateMinValue(c.Hints.SearchInputUI.PaddingY, -1, "hints.search_input_ui.padding_y")
	if err != nil {
		return err
	}

	if c.Hints.SearchInputUI.BorderWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.search_input_ui.border_width must be non-negative",
		)
	}

	if c.Hints.BoundaryHighlight.BorderWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.boundary_highlight.border_width must be non-negative",
		)
	}

	err = validateMinValue(
		c.Hints.BoundaryHighlight.BorderRadius,
		-1,
		"hints.boundary_highlight.border_radius",
	)
	if err != nil {
		return err
	}

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

	err = validateMinValue(c.Hints.SearchInputUI.Width, 1, "hints.search_input_ui.width")
	if err != nil {
		return err
	}

	err = validateMinValue(c.Hints.MaxDepth, 0, "hints.max_depth")
	if err != nil {
		return err
	}

	if (len(c.Hints.OnMissionControlActivated) > 0 || len(c.Hints.OnMissionControlDeactivated) > 0) &&
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

	for idx, actionStr := range c.Hints.OnMissionControlActivated {
		trimmed := strings.TrimSpace(actionStr)
		if trimmed == "" {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hints.on_mission_control_activated[%d] cannot be empty",
				idx,
			)
		}

		err := validateHotkeyActionString(trimmed)
		if err != nil {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hints.on_mission_control_activated[%d]: %v",
				idx,
				err,
			)
		}
	}

	for idx, actionStr := range c.Hints.OnMissionControlDeactivated {
		trimmed := strings.TrimSpace(actionStr)
		if trimmed == "" {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hints.on_mission_control_deactivated[%d] cannot be empty",
				idx,
			)
		}

		err := validateHotkeyActionString(trimmed)
		if err != nil {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hints.on_mission_control_deactivated[%d]: %v",
				idx,
				err,
			)
		}
	}

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

	err = validateHintsVisionConfig(c.Hints.Vision)
	if err != nil {
		return err
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
