package config

import (
	"regexp"
	"strconv"
	"strings"
)

// MacroCommand is the step keyword that invokes a named macro:
// "macro <name> [args...]".
const MacroCommand = "macro"

// macroNamePattern is what a [macros] key may be called. Keeping names to
// identifier characters means a call can be read by splitting on spaces: the
// second token is the whole name, and everything after it is an argument.
var macroNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// IsValidMacroName reports whether name may be used as a macro name.
func IsValidMacroName(name string) bool {
	return macroNamePattern.MatchString(name)
}

// MacroArity returns the number of arguments a macro body expects, which is
// the highest positional placeholder it uses. A body with no placeholders
// takes no arguments.
func MacroArity(steps []string) int {
	arity := 0

	for _, step := range steps {
		eachPlaceholder(step, func(index int, _, _ int) {
			if index > arity {
				arity = index
			}
		})
	}

	return arity
}

// ExpandMacroSteps substitutes positional arguments into a macro body. `$1` is
// the first argument, `$$` is a literal dollar sign, and a placeholder with no
// matching argument expands to nothing — validation rejects that case before it
// can reach here.
//
// Substitution is textual and happens before the step is split into arguments,
// so a placeholder that may contain spaces should be quoted in the body
// (`exec say "$1"`), exactly as it would be in a shell.
func ExpandMacroSteps(steps, args []string) []string {
	expanded := make([]string, 0, len(steps))

	for _, step := range steps {
		expanded = append(expanded, expandMacroStep(step, args))
	}

	return expanded
}

// expandMacroStep substitutes the placeholders in one step.
func expandMacroStep(step string, args []string) string {
	var expanded strings.Builder

	last := 0

	eachPlaceholder(step, func(index, start, end int) {
		expanded.WriteString(step[last:start])

		if index == 0 {
			// The escape for a literal dollar sign.
			expanded.WriteString("$")
		} else if index <= len(args) {
			expanded.WriteString(args[index-1])
		}

		last = end
	})

	expanded.WriteString(step[last:])

	return expanded.String()
}

// eachPlaceholder calls visit for every `$N` and `$$` in step, with the
// 1-based argument index (0 for the `$$` escape) and the byte range the
// placeholder occupies. Having one scanner keeps arity counting and expansion
// from disagreeing about what a placeholder is.
func eachPlaceholder(step string, visit func(index, start, end int)) {
	for pos := 0; pos < len(step); pos++ {
		if step[pos] != '$' {
			continue
		}

		if pos+1 < len(step) && step[pos+1] == '$' {
			visit(0, pos, pos+2) //nolint:mnd // the escape is two bytes.

			pos++

			continue
		}

		digits := pos + 1
		for digits < len(step) && step[digits] >= '0' && step[digits] <= '9' {
			digits++
		}

		if digits == pos+1 {
			// A lone dollar sign is ordinary text.
			continue
		}

		index, convErr := strconv.Atoi(step[pos+1 : digits])
		if convErr != nil || index == 0 {
			// Out of int range, or "$0", which names no argument.
			continue
		}

		visit(index, pos, digits)

		pos = digits - 1
	}
}

// ParseMacroCall splits a step into the macro name and its arguments. The
// second return value is false when the step does not invoke a macro at all.
func ParseMacroCall(step string) (string, []string, bool) {
	trimmed := strings.TrimSpace(step)

	// This is asked of every step of every sequence, including on the
	// key-press path, and almost none of them are macro calls. Rejecting them
	// on a prefix keeps the step from being tokenised twice — once here and
	// once when it is dispatched.
	if !strings.HasPrefix(trimmed, MacroCommand) {
		return "", nil, false
	}

	fields := SplitStepArgs(trimmed)
	if len(fields) == 0 || fields[0] != MacroCommand {
		return "", nil, false
	}

	if len(fields) == 1 {
		return "", nil, true
	}

	return fields[1], fields[2:], true
}

// SplitStepArgs splits one action step into its arguments, honoring single
// and double quotes so an argument may contain spaces. An unclosed quote ends
// at the end of the step rather than being an error.
//
// This is the one definition of how a step is tokenised. The daemon dispatches
// steps by it and config validation reads macro calls by it, so the two cannot
// disagree about where an argument begins.
func SplitStepArgs(input string) []string {
	var args []string

	var current strings.Builder

	inSingleQuote := false
	inDoubleQuote := false

	for _, char := range input {
		switch char {
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			} else {
				current.WriteRune(char)
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			} else {
				current.WriteRune(char)
			}
		case ' ':
			if inSingleQuote || inDoubleQuote {
				current.WriteRune(char)
			} else if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}
