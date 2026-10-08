package config

import (
	"github.com/y3owk1n/neru/internal/domain/event"
)

// HookField is the configuration path of the hook for name.
func HookField(name event.Name) string {
	return "hooks.on_" + string(name)
}

// ValidateHooks checks every hook's steps, and warns when a Mission Control
// hook is set but the detection that fires it is off, or is written under
// [hints], where it is deprecated.
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
