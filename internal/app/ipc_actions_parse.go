package app

import (
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/infra/ipc"
)

// parsedActionArgs holds the parsed arguments from an action IPC command.
type parsedActionArgs struct {
	xVal, yVal     int
	deltaX, deltaY int
	hasX, hasY     bool
	hasDX, hasDY   bool
	hasCenter      bool
	hasWindow      bool
	useSelection   bool
	useBare        bool
	monitorName    string
	hasMonitorName bool
	usePrevious    bool
	useBackward    bool
	modifierStr    string
	stepsOverride  int
	hasSteps       bool
	useBail        bool
	stateStr       string
	hasState       bool
	useToggle      bool
	directionStr   string
	hasDirection   bool
	countVal       int
	hasCount       bool

	// present records every flag that appeared, keyed by its canonical
	// spelling, so rejectUnsupportedFlags can refuse flags the action does
	// not accept without each handler keeping its own list.
	present map[string]bool
}

func shouldClearSelectionAfterMoveMouse(parsed parsedActionArgs, targetsSelection bool) bool {
	if targetsSelection {
		return false
	}

	return (parsed.hasX && parsed.hasY) ||
		parsed.hasCenter ||
		parsed.hasWindow ||
		(parsed.hasDX && parsed.hasDY) ||
		parsed.useBare
}

// extractStringFlag extracts a string value from --flag=value or --flag value
// form. Used by handleSleepAction, which parses its own argument because sleep
// runs before the shared flag parsing.
// It returns the value, the updated index, and whether the extraction succeeded.
func extractStringFlag(rawArgs []string, idx int, prefix string) (string, int, bool) {
	arg := rawArgs[idx]
	if after, ok := strings.CutPrefix(arg, prefix+"="); ok {
		return after, idx, true
	}

	if arg == prefix {
		if idx+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[idx+1], "--") {
			return rawArgs[idx+1], idx + 1, true
		}

		return "", idx, false
	}

	return "", idx, false
}

// flagSpec describes how one action flag is parsed.
//
// The parser is driven by actionFlagSpecs rather than a switch so that
// recording the flag as present cannot be forgotten: parseActionArgs marks
// every flag it recognizes in one place. A flag that is parsed but never
// marked would slip past rejectUnsupportedFlags and be silently ignored.
type flagSpec struct {
	// takesValue reports whether the flag consumes a value, written either as
	// --flag=value or as --flag value.
	takesValue bool
	// apply records the flag on parsed and reports whether value is acceptable.
	// For valueless flags, value is empty.
	apply func(parsed *parsedActionArgs, value string) bool
}

// actionFlagSpecs is the set of flags `neru action` understands. Adding an
// entry here is all it takes to parse a new flag; declaring which actions
// accept it is a separate, mandatory step in actionFlagSupport.
var actionFlagSpecs = map[string]flagSpec{
	flagX: {
		takesValue: true,
		apply:      intoInt(func(p *parsedActionArgs, v int) { p.xVal, p.hasX = v, true }),
	},
	flagY: {
		takesValue: true,
		apply:      intoInt(func(p *parsedActionArgs, v int) { p.yVal, p.hasY = v, true }),
	},
	flagDX: {
		takesValue: true,
		apply:      intoInt(func(p *parsedActionArgs, v int) { p.deltaX, p.hasDX = v, true }),
	},
	flagDY: {
		takesValue: true,
		apply:      intoInt(func(p *parsedActionArgs, v int) { p.deltaY, p.hasDY = v, true }),
	},

	// --steps and --count are counts, so zero and negatives are rejected.
	flagSteps: {takesValue: true, apply: intoPositiveInt(func(p *parsedActionArgs, v int) {
		p.stepsOverride, p.hasSteps = v, true
	})},
	flagCount: {takesValue: true, apply: intoPositiveInt(func(p *parsedActionArgs, v int) {
		p.countVal, p.hasCount = v, true
	})},

	flagModifier: {takesValue: true, apply: intoString(func(p *parsedActionArgs, v string) {
		p.modifierStr = v
	})},
	flagName: {takesValue: true, apply: intoString(func(p *parsedActionArgs, v string) {
		p.monitorName, p.hasMonitorName = v, true
	})},
	flagState: {takesValue: true, apply: intoString(func(p *parsedActionArgs, v string) {
		p.stateStr, p.hasState = v, true
	})},
	flagDirection: {takesValue: true, apply: intoString(func(p *parsedActionArgs, v string) {
		p.directionStr, p.hasDirection = v, true
	})},

	flagCenter:    {apply: intoFlag(func(p *parsedActionArgs) { p.hasCenter = true })},
	flagWindow:    {apply: intoFlag(func(p *parsedActionArgs) { p.hasWindow = true })},
	flagSelection: {apply: intoFlag(func(p *parsedActionArgs) { p.useSelection = true })},
	flagBare:      {apply: intoFlag(func(p *parsedActionArgs) { p.useBare = true })},
	flagToggle:    {apply: intoFlag(func(p *parsedActionArgs) { p.useToggle = true })},
	flagPrevious:  {apply: intoFlag(func(p *parsedActionArgs) { p.usePrevious = true })},
	flagBackward:  {apply: intoFlag(func(p *parsedActionArgs) { p.useBackward = true })},
	flagBail:      {apply: intoFlag(func(p *parsedActionArgs) { p.useBail = true })},
}

// intoFlag adapts a valueless flag setter to flagSpec.apply.
func intoFlag(set func(*parsedActionArgs)) func(*parsedActionArgs, string) bool {
	return func(parsed *parsedActionArgs, _ string) bool {
		set(parsed)

		return true
	}
}

// intoString adapts a string flag setter, rejecting an empty value.
func intoString(set func(*parsedActionArgs, string)) func(*parsedActionArgs, string) bool {
	return func(parsed *parsedActionArgs, value string) bool {
		if value == "" {
			return false
		}

		set(parsed, value)

		return true
	}
}

// intoInt adapts an integer flag setter. Negative values are allowed because
// coordinates and deltas can point left or up.
func intoInt(set func(*parsedActionArgs, int)) func(*parsedActionArgs, string) bool {
	return func(parsed *parsedActionArgs, value string) bool {
		number, err := strconv.Atoi(value)
		if err != nil {
			return false
		}

		set(parsed, number)

		return true
	}
}

// intoPositiveInt adapts an integer flag setter that requires a value of at
// least one.
func intoPositiveInt(set func(*parsedActionArgs, int)) func(*parsedActionArgs, string) bool {
	return func(parsed *parsedActionArgs, value string) bool {
		number, err := strconv.Atoi(value)
		if err != nil || number <= 0 {
			return false
		}

		set(parsed, number)

		return true
	}
}

// parseActionArgs parses flag arguments from an action IPC command.
// Supports both --flag=value and --flag value forms. The second result reports
// whether any argument was malformed.
func parseActionArgs(rawArgs []string) (parsedActionArgs, bool) {
	var parsed parsedActionArgs

	parseErr := false

	for idx := 0; idx < len(rawArgs); idx++ {
		arg := rawArgs[idx]

		name, inlineValue, hasInlineValue := strings.Cut(arg, "=")

		spec, known := actionFlagSpecs[name]
		if !known {
			// Non-flag arguments are left for the action handlers; an
			// unrecognized flag is an error.
			if strings.HasPrefix(arg, "--") {
				parseErr = true
			}

			continue
		}

		value := ""

		switch {
		case !spec.takesValue:
			if hasInlineValue {
				parseErr = true

				continue
			}
		case hasInlineValue:
			value = inlineValue
		// A following argument is the value only when it is not itself a flag,
		// so a missing value cannot swallow the next flag.
		case idx+1 < len(rawArgs) && !strings.HasPrefix(rawArgs[idx+1], "--"):
			idx++
			value = rawArgs[idx]
		default:
			parseErr = true

			continue
		}

		if !spec.apply(&parsed, value) {
			parseErr = true

			continue
		}

		parsed.markPresent(name)
	}

	return parsed, parseErr
}

// validateActionFlags rejects flag *combinations* the named action cannot
// honor. Whether an action accepts a flag at all is settled earlier by
// rejectUnsupportedFlags, so everything here is about flags that are
// individually valid but contradict each other:
//
//  1. --center mixed with --window.
//  2. --selection mixed with --bare or with explicit move targeting.
//  3. move_mouse requires --x AND --y when neither --center nor --window is given.
//
// Combinations of coordinate and delta flags (--x with --dx, --center with
// --dx) are not checked here: no action declares both families, so
// rejectUnsupportedFlags refuses them first.
//
// Note: --center with --x/--y is intentionally allowed — x/y act as offsets from center.
func validateActionFlags(
	actionName string,
	parsed parsedActionArgs,
) *ipc.Response {
	isMoveMouse := actionName == string(action.NameMoveMouse)

	if parsed.hasCenter && parsed.hasWindow {
		return &ipc.Response{
			Success: false,
			Message: "--center and --window cannot be used together",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.useSelection && parsed.useBare {
		return &ipc.Response{
			Success: false,
			Message: msgSelectionAndBareCannotBeUsedTogether,
			Code:    ipc.CodeInvalidInput,
		}
	}

	if parsed.useSelection &&
		(parsed.hasCenter || parsed.hasWindow || parsed.hasX || parsed.hasY) {
		return &ipc.Response{
			Success: false,
			Message: "--selection cannot be combined with --x, --y, --center, or --window",
			Code:    ipc.CodeInvalidInput,
		}
	}

	if isMoveMouse && !parsed.hasCenter && !parsed.hasWindow &&
		!parsed.useSelection &&
		((parsed.hasX && !parsed.hasY) || (!parsed.hasX && parsed.hasY)) {
		return &ipc.Response{
			Success: false,
			Message: "move_mouse requires both --x and --y when using absolute coordinates",
			Code:    ipc.CodeInvalidInput,
		}
	}

	return nil
}
