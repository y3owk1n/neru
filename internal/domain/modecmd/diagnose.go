package modecmd

import "github.com/y3owk1n/neru/internal/domain"

// Diagnose reads a mode command the way [Parse] does and reports what is wrong
// with it in two parts: the error is the command being unreadable, and the
// warnings are the ways a readable command will not do everything it says.
//
// It is for the one reader that cannot spend the two the same way. A
// configuration that fails to load is replaced by the defaults — every
// binding, theme and setting, not the offending line — so that is spent only
// on a command that could not have run at all: one naming a flag nothing
// knows, or a value no flag can carry. A command that runs minus one flag is
// left loading, and reported instead (ADR 0002).
//
// Everywhere a mode is actually entered, [Parse] is the door and every fault
// below is a refusal. The sentences are the same either way, so the same
// mistake reads the same whether it was typed, sent over the socket, or found
// in a binding.
//
// The activation is returned alongside, for the one question the grammar
// cannot answer on its own: whether the declared mode a custom activation
// names exists is the configuration's to know.
func Diagnose(mode domain.Mode, args []string) (Activation, []error, error) {
	activation, unsupported, err := read(mode, args)
	if err != nil {
		return Activation{}, nil, err
	}

	// A command holding both an unreadable value and an inert flag is
	// unreadable: there is nothing left to weigh once it cannot run.
	valueErr := validateValues(activation)
	if valueErr != nil {
		return Activation{}, nil, valueErr
	}

	warnings := make([]error, 0, len(unsupported))

	for _, name := range unsupported {
		warnings = append(warnings, notAccepted(mode, name))
	}

	return activation, append(warnings, unmetDependencies(activation)...), nil
}
