package config

import (
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/event"
)

// HookField is the configuration path of the hook for name.
func HookField(name event.Name) string {
	return "hooks.on_" + string(name)
}

// ValidateHooks checks every hook's steps, and warns when a Mission Control
// or sticky modifier hook is set but the feature that fires it is off, when a
// quit hook holds a step that is not exec, or a Mission Control hook is
// written under [hints], where it is deprecated.
func (c *Config) ValidateHooks(warnings *Warnings) error {
	for _, name := range event.All() {
		err := validateHookSteps(HookField(name), c.Hooks.Steps(name))
		if err != nil {
			return err
		}
	}

	if (len(c.Hooks.OnMissionControlActivated) > 0 ||
		len(c.Hooks.OnMissionControlDeactivated) > 0) &&
		!c.Hints.DetectMissionControl {
		warnings.Addf(
			"hooks.on_mission_control_activated/deactivated never run while " +
				"hints.detect_mission_control is false",
		)
	}

	for index, step := range c.Hooks.OnQuit {
		if !action.IsExecStep(step) {
			warnings.Addf("hooks.on_quit step %d never runs: only exec steps run on quit", index+1)
		}
	}

	if len(c.Hooks.OnStickyModifiers) > 0 && !c.StickyModifiers.Enabled {
		warnings.Addf(
			"hooks.on_sticky_modifiers never runs while sticky_modifiers.enabled is false",
		)
	}

	deprecated := []struct {
		field string
		steps StringOrStringArray
	}{
		{field: "on_mission_control_activated", steps: c.Hints.OnMissionControlActivated},
		{field: "on_mission_control_deactivated", steps: c.Hints.OnMissionControlDeactivated},
	}

	for _, hook := range deprecated {
		if len(hook.steps) > 0 {
			warnings.Addf(
				"hints.%s is deprecated and is removed in v2. Move it to hooks.%s",
				hook.field,
				hook.field,
			)
		}
	}

	return nil
}
