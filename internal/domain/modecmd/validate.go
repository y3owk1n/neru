package modecmd

import (
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
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

	dependencyErr := validateDependencies(activation)
	if dependencyErr != nil {
		return dependencyErr
	}

	return validateAction(activation)
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

// validateDependencies refuses the flags that are only meaningful alongside
// another one. They are checked after parsing because a flag may be written
// before the one it depends on.
func validateDependencies(activation Activation) error {
	hasAction := activation.Action != nil

	if isTrue(activation.Repeat) && !hasAction {
		return invalid(msgRepeatRequiresAction)
	}

	if activation.OnExit != nil && !hasAction {
		return invalid(msgOnExitRequiresAction)
	}

	if isTrue(activation.HideOnEmptySearch) && !isTrue(activation.Search) {
		return invalid(msgHideOnEmptySearchRequiresSearch)
	}

	if activation.Modifier == nil {
		return nil
	}

	if !hasAction {
		return invalid(msgModifierRequiresAction)
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

// isTrue reads a presence-only flag: absent and false are the same answer.
func isTrue(value *bool) bool {
	return value != nil && *value
}
