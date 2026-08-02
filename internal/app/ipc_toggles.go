package app

import (
	"fmt"
	"strings"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
)

// The toggle commands (toggle-scroll-invert, toggle-screen-share,
// toggle-cursor-follow-selection) flip a boolean. Flipping is the right default
// for a key binding, where the user sees the result and presses again if it is
// wrong, but it is the wrong primitive for a script: a driver that cannot read
// the state can only guess at it, and a binding pressed twice by mistake leaves
// the daemon somewhere the driver does not expect.
//
// --state lets those callers ask for the state they want instead. The states
// are reported by "neru status --json" under the same names, so a driver can
// check what it asked for.

// toggleStateFlag names the state a toggle command should converge on.
const toggleStateFlag = "--state"

// The values toggleStateFlag accepts.
const (
	toggleStateOn     = "on"
	toggleStateOff    = "off"
	toggleStateToggle = "toggle"
)

// parseToggleState reads the optional --state flag from a toggle command.
//
// A nil state means "flip it": what a toggle command does with no flag, and
// what --state toggle asks for explicitly. The explicit spelling exists so a
// macro can pass the value straight through — "toggle-scroll-invert --state $1"
// — without the caller having to special-case flipping.
//
// Both --state=on and --state on are accepted, matching how action flags are
// written.
func parseToggleState(command string, args []string) (*bool, *ipc.Response) {
	var desired *bool

	for idx := 0; idx < len(args); idx++ {
		arg := strings.TrimSpace(args[idx])
		if arg == "" {
			continue
		}

		name, value, hasInlineValue := strings.Cut(arg, "=")
		if name != toggleStateFlag {
			return nil, unknownToggleArgResponse(command, arg)
		}

		if !hasInlineValue {
			// A following argument is the value only when it is not itself a
			// flag, so "--state --state=on" reports a missing value rather
			// than swallowing the next flag as one.
			if idx+1 >= len(args) || strings.HasPrefix(args[idx+1], "--") {
				return nil, toggleStateValueResponse(command)
			}

			idx++
			value = strings.TrimSpace(args[idx])
		}

		parsed, ok := parseToggleStateValue(value)
		if !ok {
			return nil, toggleStateValueResponse(command)
		}

		desired = parsed
	}

	return desired, nil
}

// parseToggleStateValue resolves one --state value. A nil state with ok true is
// "toggle" — a valid request to flip, not a parse failure.
func parseToggleStateValue(value string) (*bool, bool) {
	switch value {
	case toggleStateOn:
		on := true

		return &on, true
	case toggleStateOff:
		off := false

		return &off, true
	case toggleStateToggle:
		return nil, true
	default:
		return nil, false
	}
}

// applyToggleState resolves what a toggle command should end up at: the
// requested state, or the opposite of the current one when none was requested.
//
// toggle is passed separately rather than derived from current because the
// state types toggle atomically, and a check-then-act here would reintroduce
// the race those methods exist to avoid.
func applyToggleState(desired *bool, toggle func() bool, set func(bool)) bool {
	if desired == nil {
		return toggle()
	}

	set(*desired)

	return *desired
}

// applyToggleStateWithResult is applyToggleState for state that may not exist
// to change. Both callbacks report whether they found anything to act on, and
// a false is passed straight through to the caller.
func applyToggleStateWithResult(
	desired *bool,
	toggle func() (bool, bool),
	set func(bool) (bool, bool),
) (bool, bool) {
	if desired == nil {
		return toggle()
	}

	return set(*desired)
}

// toggleStateValueResponse reports a --state that named no valid state.
func toggleStateValueResponse(command string) *ipc.Response {
	return &ipc.Response{
		Success: false,
		Message: fmt.Sprintf(
			"%s --state requires %s, %s, or %s",
			command,
			toggleStateOn,
			toggleStateOff,
			toggleStateToggle,
		),
		Code: ipc.CodeInvalidInput,
	}
}

// unknownToggleArgResponse reports an argument a toggle command does not take.
func unknownToggleArgResponse(command, arg string) *ipc.Response {
	return &ipc.Response{
		Success: false,
		Message: fmt.Sprintf(
			"%s does not accept %q (only %s %s|%s|%s)",
			command,
			arg,
			toggleStateFlag,
			toggleStateOn,
			toggleStateOff,
			toggleStateToggle,
		),
		Code: ipc.CodeInvalidInput,
	}
}
