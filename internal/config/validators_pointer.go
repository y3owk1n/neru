package config

import (
	"unicode/utf8"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

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

	// The animator floors every animation at MinSmoothCursorAnimationDuration,
	// so smaller values would be silently rounded up rather than honored;
	// reject them instead.
	if c.SmoothCursor.RelativeMovementDuration < MinSmoothCursorAnimationDuration {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"smooth_cursor.relative_movement_duration must be >= %d",
			MinSmoothCursorAnimationDuration,
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

	if c.HeldRepeat.AccelRampMs < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "held_repeat.accel_ramp_ms must be >= 0")
	}

	// Negated so NaN, which compares false to everything, is rejected too.
	if !(c.HeldRepeat.AccelMaxMultiplier >= 1 &&
		c.HeldRepeat.AccelMaxMultiplier <= MaxHeldRepeatAccelMultiplier) {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"held_repeat.accel_max_multiplier must be between 1 and %d",
			MaxHeldRepeatAccelMultiplier,
		)
	}

	if c.HeldRepeat.AccelEnabled && len(c.HeldRepeat.AccelTargets) == 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"held_repeat.accel_targets must list at least one action while accel_enabled is true",
		)
	}

	// Only move_mouse_relative accepts --dx/--dy; anything else would validate
	// and then accelerate nothing.
	for _, target := range c.HeldRepeat.AccelTargets {
		if action.Name(target) != action.NameMoveMouseRelative {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"held_repeat.accel_targets only supports "+
					string(action.NameMoveMouseRelative)+", got: "+target,
			)
		}
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
