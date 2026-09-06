package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
)

// ValidateMacros checks the [macros] table and every macro call in the
// configuration.
//
// Calls are checked at load rather than when a key is pressed because a
// binding runs in the background: a mistyped macro name would otherwise
// produce nothing but a log line, with the key appearing to do nothing at all.
//
// Definitions before calls, so a call is checked against a table already known
// to be sound: a call that names a macro whose own definition is malformed
// should be told the definition is wrong, not that the arity does not match.
func (c *Config) ValidateMacros() error {
	// The [macros] table first.
	for name, steps := range c.Macros {
		err := validateMacroDefinition(name, steps)
		if err != nil {
			return err
		}
	}

	// Then every call to it, wherever the configuration makes one.
	return c.eachBindingAction(func(field, actionStr string) error {
		return c.validateMacroCall(field, actionStr)
	})
}

// validateMacroDefinition checks one entry of the [macros] table.
func validateMacroDefinition(name string, steps []string) error {
	if !IsValidMacroName(name) {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"macros.%s is not a valid macro name: use letters, digits, '_' and '-', starting with a letter",
			name,
		)
	}

	if len(steps) == 0 {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"macros.%s must have at least one step",
			name,
		)
	}

	for idx, step := range steps {
		field := fmt.Sprintf("macros.%s[%d]", name, idx)

		if strings.TrimSpace(step) == "" {
			return derrors.Newf(derrors.CodeInvalidConfig, "%s must not be empty", field)
		}

		// A step is validated with its placeholders still in place. They only
		// ever appear in flag values, which this check does not look at.
		err := validateHotkeyActionString(step)
		if err != nil {
			return derrors.Wrapf(err, derrors.CodeInvalidConfig, "%s", field)
		}
	}

	return nil
}

// maxValidatedNesting bounds how deep validation follows a step into the
// sequences it carries. It mirrors the runtime nesting limit; a sequence deeper
// than this is refused when it runs, so there is nothing to check below it.
const maxValidatedNesting = 5

// validateMacroCall checks a single action string, and the steps it carries.
//
// A step is not always a leaf: "run" holds its own steps, and a mode command
// holds the steps of its --on-exit. A macro invoked from either of those is
// still a macro this configuration will run, so it is checked the same way —
// otherwise the load-time guarantee would hold everywhere except the two
// places a sequence nests.
func (c *Config) validateMacroCall(field, actionStr string) error {
	return c.validateActionStep(field, actionStr, 0)
}

// validateActionStep checks one step and descends into any it carries.
func (c *Config) validateActionStep(field, step string, depth int) error {
	if depth >= maxValidatedNesting {
		return nil
	}

	if stepCommand(step) == CmdRun {
		for _, nested := range SplitStepArgs(strings.TrimSpace(step))[1:] {
			err := c.validateActionStep(field, nested, depth+1)
			if err != nil {
				return err
			}
		}

		return nil
	}

	for _, nested := range onExitSteps(step) {
		err := c.validateActionStep(field, nested, depth+1)
		if err != nil {
			return err
		}
	}

	// A nested step is a step: it has to name a real command, exactly like one
	// written directly in the binding. Reaching it only through this walk is
	// why a step inside a run or an --on-exit went unchecked before.
	if depth > 0 {
		err := validateHotkeyActionString(step)
		if err != nil {
			return derrors.Wrapf(err, derrors.CodeInvalidConfig, "%s", field)
		}
	}

	return c.checkMacroCall(field, step)
}

// stepCommand returns the command word of a step, ignoring its arguments.
func stepCommand(step string) string {
	fields := strings.Fields(strings.TrimSpace(step))
	if len(fields) == 0 {
		return ""
	}

	return fields[0]
}

// onExitSteps returns the steps a mode command carries in its --on-exit flags,
// in either the separated or the "=" spelling. The flag is repeatable, so a
// mode command may carry several.
func onExitSteps(step string) []string {
	if !strings.Contains(step, OnExitFlag) {
		return nil
	}

	tokens := SplitStepArgs(strings.TrimSpace(step))

	var steps []string

	for idx := 0; idx < len(tokens); idx++ {
		switch {
		case tokens[idx] == OnExitFlag && idx+1 < len(tokens):
			idx++

			steps = append(steps, tokens[idx])
		case strings.HasPrefix(tokens[idx], OnExitFlag+"="):
			steps = append(steps, strings.TrimPrefix(tokens[idx], OnExitFlag+"="))
		}
	}

	return steps
}

// checkMacroCall checks one step that invokes a macro. It is a no-op for any
// other action.
func (c *Config) checkMacroCall(field, actionStr string) error {
	name, args, isCall := ParseMacroCall(actionStr)
	if !isCall {
		return nil
	}

	if name == "" {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s: macro requires a name (e.g. \"macro window_click 100 70\")",
			field,
		)
	}

	steps, defined := c.Macros[name]
	if !defined {
		return derrors.Newf(derrors.CodeInvalidConfig, "%s: no macro named %q", field, name)
	}

	arity := MacroArity(steps)
	if len(args) != arity {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s: macro %q takes %d argument(s), got %d",
			field,
			name,
			arity,
			len(args),
		)
	}

	return nil
}

// eachBindingAction calls visit for every action string the configuration can
// dispatch: the global bindings, each mode's bindings, the per-app overrides of
// both, and the bodies of the macros themselves.
//
// A macro can be invoked from any of these, so all of them have to be walked
// for a call to be checked at load time.
func (c *Config) eachBindingAction(visit func(field, actionStr string) error) error {
	for key, actions := range c.Hotkeys.Bindings {
		err := visitActions("hotkeys."+key, actions, visit)
		if err != nil {
			return err
		}
	}

	for idx, appConfig := range c.AppConfigs {
		field := fmt.Sprintf("app_configs[%d].hotkeys", idx)

		err := visitTable(field, appConfig.Hotkeys, visit)
		if err != nil {
			return err
		}
	}

	for _, mode := range c.modeBindingTables() {
		err := visitTable(mode.field, mode.table, visit)
		if err != nil {
			return err
		}
	}

	// The Mission Control hooks are action sequences like any binding — they
	// are dispatched through the same executor when the transition fires, so a
	// macro they name has to exist by the time it does.
	hooks := []struct {
		steps []string
		field string
	}{
		{field: "hints.on_mission_control_activated", steps: c.Hints.OnMissionControlActivated},
		{field: "hints.on_mission_control_deactivated", steps: c.Hints.OnMissionControlDeactivated},
	}

	for _, hook := range hooks {
		err := visitActions(hook.field, hook.steps, visit)
		if err != nil {
			return err
		}
	}

	for name, steps := range c.Macros {
		err := visitActions("macros."+name, steps, visit)
		if err != nil {
			return err
		}
	}

	return nil
}

// modeBindingTable names one table of bindings for reporting.
type modeBindingTable struct {
	table map[string]StringOrStringArray
	field string
}

// modeBindingTables lists every per-mode binding table, including the per-app
// overrides, so a walk cannot silently miss one.
func (c *Config) modeBindingTables() []modeBindingTable {
	tables := []modeBindingTable{
		{field: ModeNameHints + ".hotkeys", table: c.Hints.Hotkeys},
		{field: ModeNameGrid + ".hotkeys", table: c.Grid.Hotkeys},
		{field: ModeNameRecursiveGrid + ".hotkeys", table: c.RecursiveGrid.Hotkeys},
		{field: ModeNameScroll + ".hotkeys", table: c.Scroll.Hotkeys},
		{field: ModeNameMonitorSelect + ".hotkeys", table: c.MonitorSelect.Hotkeys},
	}

	type appTable struct {
		configs  []AppConfig
		modeName string
	}

	appTables := make([]appTable, 0, builtInModeCount+len(c.Modes))
	appTables = append(appTables,
		appTable{configs: c.Hints.AppConfigs, modeName: ModeNameHints},
		appTable{configs: c.Grid.AppConfigs, modeName: ModeNameGrid},
		appTable{configs: c.RecursiveGrid.AppConfigs, modeName: ModeNameRecursiveGrid},
		appTable{configs: c.Scroll.AppConfigs, modeName: ModeNameScroll},
	)

	// Declared modes are walked in name order so a fault is reported at the
	// same binding on every load.
	for _, name := range slices.Sorted(maps.Keys(c.Modes)) {
		tables = append(tables, modeBindingTable{
			field: customModeField(name) + ".hotkeys",
			table: c.Modes[name].Hotkeys,
		})
		appTables = append(appTables, appTable{
			configs:  c.Modes[name].AppConfigs,
			modeName: customModeField(name),
		})
	}

	for _, mode := range appTables {
		for idx, appConfig := range mode.configs {
			tables = append(tables, modeBindingTable{
				field: fmt.Sprintf("%s.app_configs[%d].hotkeys", mode.modeName, idx),
				table: appConfig.Hotkeys,
			})
		}
	}

	return tables
}

// visitTable applies visit to every action in a binding table.
func visitTable(
	field string,
	table map[string]StringOrStringArray,
	visit func(field, actionStr string) error,
) error {
	for key, actions := range table {
		err := visitActions(field+"."+key, actions, visit)
		if err != nil {
			return err
		}
	}

	return nil
}

// visitActions applies visit to every non-blank action in one binding.
func visitActions(field string, actions []string, visit func(field, actionStr string) error) error {
	for _, actionStr := range actions {
		if strings.TrimSpace(actionStr) == "" {
			continue
		}

		err := visit(field, actionStr)
		if err != nil {
			return err
		}
	}

	return nil
}
