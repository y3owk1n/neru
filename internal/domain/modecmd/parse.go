package modecmd

import (
	"strings"

	"github.com/y3owk1n/neru/internal/domain"
)

// The messages a mode command's shape gives, as opposed to any one flag's.
const (
	msgUnknownFlag       = "unknown flag: "
	msgUnexpectedArg     = "unexpected argument: "
	msgDoesNotAcceptVerb = " does not accept "
)

// Parse reads a mode command's arguments into the activation they describe.
//
// The arguments are what a user wrote after the mode's name, in either shape
// the wire carries: a caller modeled on the CLI's own traffic repeats the
// mode name as the first argument, and a binding does not. Both reach the same
// activation.
//
// It validates before returning, so no caller can forget to. [Validate] is
// exported separately for the caller that builds an activation from typed
// flags and never parses a string.
func Parse(mode domain.Mode, args []string) (Activation, error) {
	activation, unsupported, err := read(mode, args)
	if err != nil {
		return Activation{}, err
	}

	if len(unsupported) > 0 {
		return Activation{}, notAccepted(mode, unsupported[0])
	}

	err = Validate(activation)
	if err != nil {
		return Activation{}, err
	}

	return activation, nil
}

// read reads the arguments into the activation they describe, and hands back
// the flags the named mode does not accept rather than stopping at the first
// one.
//
// Reading past such a flag is what lets [Diagnose] weigh a whole command:
// every caller that enters a mode refuses the first of them, and the one
// caller that does not — a configuration deciding what a mistake costs — needs
// all of them.
func read(mode domain.Mode, args []string) (Activation, []Flag, error) {
	activation := Activation{Mode: mode}
	rest := withoutModeName(mode, args)

	var unsupported []Flag

	for index := 0; index < len(rest); index++ {
		arg := rest[index]

		descriptor, known := match(arg)

		switch {
		case known && !descriptor.AcceptedBy(mode):
			// Read for its shape and then dropped. Stepping over the value it
			// was written with is what keeps the rest of the command readable:
			// left where it is, that value would be taken as the positional
			// action.
			unsupported = append(unsupported, descriptor.name)
			index += skipped(descriptor, rest, index)

		case known:
			consumed, err := apply(descriptor, &activation, rest, index)
			if err != nil {
				return Activation{}, nil, err
			}

			index += consumed

		case looksLikeAFlag(arg):
			// Taking it as the positional action instead is how a typo used to
			// travel all the way to the mode as an unusable action name.
			return Activation{}, nil, invalid(msgUnknownFlag + arg)

		case mode == domain.ModeCustom && activation.Name == "":
			// The first argument of a custom activation is the declared mode
			// it enters, which is the one thing a bare word can mean there:
			// a custom mode makes no selection, so it takes no action.
			activation.Name = arg

		case activation.Action == nil && accepts(FlagAction, mode):
			// An argument matching no flag is the positional action, which is
			// how "hints left_click" is written. Once an action is set there
			// is nothing left for a stray argument to mean.
			positional := arg
			activation.Action = &positional

		default:
			return Activation{}, nil, invalid(msgUnexpectedArg + arg)
		}
	}

	return activation, unsupported, nil
}

// apply reads the flag positioned at index into the activation and reports how
// many further arguments it consumed.
func apply(
	descriptor Descriptor,
	activation *Activation,
	args []string,
	index int,
) (int, error) {
	if !descriptor.TakesValue() {
		// A value attached to a flag that carries none is the flag's own to
		// refuse. Reading it here is what stops it from being dropped: the
		// alternative is "--toggle=false" toggling.
		_, attached, _ := strings.Cut(args[index], "=")

		return 0, descriptor.set(activation, attached)
	}

	value, consumed, err := valueAt(descriptor, args, index)
	if err != nil {
		return 0, err
	}

	return consumed, descriptor.set(activation, value)
}

// valueAt reads the value belonging to the flag at index, in whichever form it
// was written, and reports whether it consumed the following argument.
//
// A flag that carries a value can be written "--flag=value" or "--flag value",
// and reading the second form means looking ahead — including the bounds check
// whose absence would be a panic rather than a refusal.
func valueAt(descriptor Descriptor, args []string, index int) (string, int, error) {
	if _, after, attached := strings.Cut(args[index], "="); attached {
		return after, 0, nil
	}

	if index+1 >= len(args) {
		return "", 0, invalid(descriptor.valueMessage)
	}

	return args[index+1], 1, nil
}

// skipped reports how many further arguments a flag occupies whose value is
// never going to be read, which is the flag the mode does not accept.
//
// It reads the value the same way applying the flag would, so the two cannot
// disagree about where it ends, and stops short of one case: another flag is
// never this one's value, whatever it was written next to. Stepping over
// "--search" in "grid --strategy --search" would drop a flag in silence, which
// is the failure this package exists to remove.
func skipped(descriptor Descriptor, args []string, index int) int {
	if !descriptor.TakesValue() {
		return 0
	}

	_, consumed, err := valueAt(descriptor, args, index)
	if err != nil || consumed == 0 {
		return 0
	}

	if _, isAFlag := match(args[index+consumed]); isAFlag {
		return 0
	}

	return consumed
}

// withoutModeName drops the mode's own name when the caller repeated it as the
// first argument.
//
// The CLI sends it — "neru grid --action left_click" arrives as
// ["grid", "--action", "left_click"] — and a binding does not, so both shapes
// have to reach the same parse.
func withoutModeName(mode domain.Mode, args []string) []string {
	if len(args) > 0 && args[0] == domain.ModeString(mode) {
		return args[1:]
	}

	return args
}

// match returns the descriptor for an argument, in any spelling.
func match(arg string) (Descriptor, bool) {
	for _, descriptor := range descriptors {
		if descriptor.Match(arg) {
			return descriptor, true
		}
	}

	return Descriptor{}, false
}

// looksLikeAFlag reports whether an argument was written as one. No action
// name starts with a dash, so anything that does was meant to be a flag.
func looksLikeAFlag(arg string) bool {
	return strings.HasPrefix(arg, "-")
}

// accepts reports whether a mode accepts a flag.
func accepts(name Flag, mode domain.Mode) bool {
	descriptor, known := Lookup(name)

	return known && descriptor.AcceptedBy(mode)
}

// notAccepted builds the refusal for a flag the named mode has no use for.
// Accepting it and dropping it is what made a flag in a binding look like it
// worked.
func notAccepted(mode domain.Mode, name Flag) error {
	return invalid(domain.ModeString(mode) + msgDoesNotAcceptVerb + name.Long())
}
