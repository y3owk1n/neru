package config

import (
	"maps"
	"slices"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// customModeField is the TOML path a declared mode is reported under.
func customModeField(name string) string {
	return "modes." + name
}

// ValidateCustomModes checks every [modes.<name>] declaration: that the name
// is one a mode command can carry, that it is not a built-in mode's, and that
// its per-app overrides are shaped like grid's, which is the mode that takes
// no field of its own.
//
// The hotkey tables themselves are checked by ValidateHotkeys alongside the
// built-in ones, and the steps in them by the binding walks that close the
// ladder, so nothing about a binding is judged twice.
func (c *Config) ValidateCustomModes() error {
	for _, name := range slices.Sorted(maps.Keys(c.Modes)) {
		if !modecmd.ValidModeName(name) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s: a mode name starts with a letter and continues with letters, digits, _ or -",
				customModeField(name),
			)
		}

		// A built-in mode's name is answered by the built-in mode wherever a
		// name is looked up, so a declaration under it could never be
		// entered. "idle" and "mode" are in the same lookup and refused for
		// the same reason.
		if _, builtIn := modecmd.LookupMode(name); builtIn {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s: %q is a built-in mode command and cannot be declared",
				customModeField(name),
				name,
			)
		}

		err := validateAppConfigsWithCallback(
			customModeField(name),
			c.Modes[name].AppConfigs,
			rejectModeSpecificFields(customModeField(name)),
		)
		if err != nil {
			return err
		}
	}

	return nil
}
