package config

import (
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// ValidateModeCommands reads every mode command in the configuration through
// the grammar that will run it, so a flag that would be refused when the key
// is pressed is found before it ever is.
//
// What is refused here and what is only warned about is ADR 0002's split. A
// refused configuration is replaced by the defaults in full, so that is spent
// only on a binding that could not have run at all — one naming a flag nothing
// knows, or a value no flag can carry. A binding that runs minus one flag is
// left loading and reported through warnings, which reach the user through
// `neru config validate` rather than only the log.
//
// A step nested inside another one — the steps an --on-exit carries, or the
// steps of a "run" — is checked for the command it names, by
// validateHotkeyActionString, and not for its flags. Reading those would mean
// reading a step out of the argument list it was quoted into, which is the
// runtime's job at the moment it dispatches them.
func (c *Config) ValidateModeCommands(warnings *Warnings) error {
	return c.eachBindingAction(func(field, actionStr string) error {
		mode, args, isModeCommand := parseModeCommand(actionStr)
		if !isModeCommand {
			return nil
		}

		activation, reported, err := modecmd.Diagnose(mode, args)
		if err != nil {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s: %s",
				field,
				derrors.Message(err),
			)
		}

		// The grammar reads the name; only the configuration knows whether a
		// mode by that name was declared. A binding that enters one that was
		// not is a key that does nothing, so it is refused like a typo'd flag.
		if mode == domain.ModeCustom {
			if _, declared := c.Modes[activation.Name]; !declared {
				return derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s: %s",
					field,
					derrors.Message(modecmd.NotDeclared(activation.Name)),
				)
			}
		}

		for _, warning := range reported {
			warnings.Addf("%s: %s", field, derrors.Message(warning))
		}

		return nil
	})
}

// parseModeCommand splits a step into the mode it enters and the arguments it
// enters it with. The last return value is false for a step that is not a mode
// command at all, which most steps are not.
//
// Tokenising with SplitStepArgs is what makes this the same reading the daemon
// does: a step is dispatched by splitting it exactly this way and handing the
// rest to the grammar.
func parseModeCommand(actionStr string) (domain.Mode, []string, bool) {
	step := strings.TrimSpace(actionStr)

	// A macro body is only whole once it is called, and what fills its holes is
	// unknown here. A placeholder only ever appears in a flag's value, so a
	// step carrying one is left to be read when it is expanded.
	if hasPlaceholder(step) {
		return domain.ModeIdle, nil, false
	}

	tokens := SplitStepArgs(step)
	if len(tokens) == 0 {
		return domain.ModeIdle, nil, false
	}

	mode, isMode := modecmd.LookupMode(tokens[0])
	if !isMode {
		return domain.ModeIdle, nil, false
	}

	return mode, tokens[1:], true
}

// hasPlaceholder reports whether a step is written with a macro argument in it.
func hasPlaceholder(step string) bool {
	found := false

	eachPlaceholder(step, func(_, _, _ int) { found = true })

	return found
}
