package modecmd

import (
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// The one message each cross-flag rule gives.
//
// A rule has exactly one of these wherever it is broken, so the same mistake
// cannot be described two ways depending on whether it was typed, sent over
// the socket, or written in a binding.
const (
	msgRepeatRequiresAction            = "--repeat requires --action"
	msgOnExitRequiresAction            = "--on-exit requires --action (it runs only when the action is fulfilled)"
	msgModifierRequiresAction          = "--modifier requires --action"
	msgHideOnEmptySearchRequiresSearch = "--hide-on-empty-search requires --search"
	msgModifierEmpty                   = "modifier values cannot be empty"
	msgNameRequired                    = "mode requires the name of a declared mode: mode <name>"
	msgNameInvalid                     = "a mode name starts with a letter and continues with letters, digits, _ or -"
	msgNameNotAccepted                 = " does not take a mode name; only mode <name> does"
)

// Validate applies every rule that needs more than one flag to judge: which
// flags the mode accepts, the flags that only mean something alongside another
// one, and the vocabulary an action is written in.
//
// [Parse] calls it, so a parsed command is always a validated one. It is
// exported separately for a caller that builds an activation from typed flags
// rather than from a string, which is how the CLI reads its own flags: the
// rules it is held to have to be these ones.
func Validate(activation Activation) error {
	acceptanceErr := validateAcceptance(activation)
	if acceptanceErr != nil {
		return acceptanceErr
	}

	unmet := unmetDependencies(activation)
	if len(unmet) > 0 {
		return unmet[0]
	}

	return validateValues(activation)
}

// validateAcceptance refuses a flag the named mode has no use for.
//
// Whether a flag was given is read from its own renderer rather than from a
// third closure per flag: a flag renders exactly what it was given, so
// rendering nothing is what "absent" means. A presence-only flag set
// explicitly off renders nothing too, and rightly: it asks the mode for
// nothing, so there is nothing for the mode to refuse.
func validateAcceptance(activation Activation) error {
	for _, descriptor := range descriptors {
		if len(descriptor.render(activation)) == 0 {
			continue
		}

		if !descriptor.AcceptedBy(activation.Mode) {
			return notAccepted(activation.Mode, descriptor.name)
		}
	}

	return nil
}

// dependency is a flag that only means something alongside another one: what
// it looks like to have been written alone, and the sentence that says so.
type dependency struct {
	unmet   func(Activation) bool
	message string
}

// dependencies are the co-dependency rules, in the order they are reported.
//
// They are a table rather than a run of conditions because they have two
// readers that need different amounts of them: [Validate] refuses at the
// first, and [Diagnose] weighs every one.
var dependencies = []dependency{
	{
		unmet:   func(a Activation) bool { return isTrue(a.Repeat) && a.Action == nil },
		message: msgRepeatRequiresAction,
	},
	{
		unmet:   func(a Activation) bool { return a.OnExit != nil && a.Action == nil },
		message: msgOnExitRequiresAction,
	},
	{
		unmet:   func(a Activation) bool { return isTrue(a.HideOnEmptySearch) && !isTrue(a.Search) },
		message: msgHideOnEmptySearchRequiresSearch,
	},
	{
		unmet:   func(a Activation) bool { return a.Modifier != nil && a.Action == nil },
		message: msgModifierRequiresAction,
	},
}

// unmetDependencies lists the flags that were written without the flag they
// are only meaningful alongside. They are judged after parsing because a flag
// may be written before the one it depends on.
func unmetDependencies(activation Activation) []error {
	var unmet []error

	for _, rule := range dependencies {
		if rule.unmet(activation) {
			unmet = append(unmet, invalid(rule.message))
		}
	}

	return unmet
}

// validateValues refuses a value a flag was given that it cannot carry. Unlike
// an unmet dependency, nothing about the rest of the command can make one of
// these mean anything.
func validateValues(activation Activation) error {
	nameErr := validateName(activation)
	if nameErr != nil {
		return nameErr
	}

	modifierErr := validateModifier(activation)
	if modifierErr != nil {
		return modifierErr
	}

	return validateAction(activation)
}

// validateName holds the declared name to the custom mode: required and
// well-formed there, and refused anywhere else, where a name would be dropped
// by the rendering and so could never have meant anything.
func validateName(activation Activation) error {
	if activation.Mode != domain.ModeCustom {
		if activation.Name != "" {
			return invalid(domain.ModeString(activation.Mode) + msgNameNotAccepted)
		}

		return nil
	}

	if activation.Name == "" {
		return invalid(msgNameRequired)
	}

	if !ValidModeName(activation.Name) {
		return invalid(msgNameInvalid)
	}

	return nil
}

// validateModifier refuses a --modifier value that names no modifier key. An
// empty one is named separately: it parses, and holds nothing.
func validateModifier(activation Activation) error {
	if activation.Modifier == nil {
		return nil
	}

	modifiers, err := action.ParseModifiers(*activation.Modifier)
	if err != nil {
		return err
	}

	if modifiers == 0 {
		return invalid(msgModifierEmpty)
	}

	return nil
}

// validateAction refuses an action a mode cannot fulfill.
//
// Commas chain several actions, which is how a double-click is written, so
// each entry is judged on its own. A mode action is performed on a selection,
// so it has to be a mouse button: everything else — scrolling, moving the
// cursor, pressing a key — is an action in its own right and reaches the same
// place without a mode.
func validateAction(activation Activation) error {
	if activation.Action == nil {
		return nil
	}

	for index, entry := range strings.Split(*activation.Action, ",") {
		name := strings.TrimSpace(entry)
		if name == "" {
			return derrors.Newf(
				derrors.CodeInvalidInput,
				"invalid --action at position %d: empty action in comma-separated list",
				index,
			)
		}

		if !action.IsKnownName(action.Name(name)) {
			return derrors.Newf(
				derrors.CodeInvalidInput,
				"invalid action: %s. Supported actions: %s",
				name,
				action.SupportedNamesString(),
			)
		}

		if action.IsScrollSubAction(name) {
			return derrors.Newf(
				derrors.CodeInvalidInput,
				"scroll sub-action %q cannot be used as a mode action; only mouse button actions can",
				name,
			)
		}

		actionType, err := action.Name(name).ToType()
		if err != nil || !actionType.IsMouseButton() {
			return derrors.Newf(
				derrors.CodeInvalidInput,
				"%q cannot be used as a mode action; only mouse button actions can",
				name,
			)
		}
	}

	return nil
}

// NotDeclared is the refusal for a custom activation naming a mode the
// configuration does not declare. The grammar cannot judge that itself, but it
// owns the sentence, so a binding read at load and a command sent over the
// socket are refused in the same words.
func NotDeclared(name string) error {
	return derrors.Newf(
		derrors.CodeInvalidInput,
		"mode %q is not declared; declare it as [modes.%s]",
		name,
		name,
	)
}

// isTrue reads a presence-only flag: absent and false are the same answer.
func isTrue(value *bool) bool {
	return value != nil && *value
}
