package architecture_test

import (
	"fmt"
	"regexp"
)

// This file is how a language-boundary pin finds the native definition its
// copy lives in.
//
// ADR 0007 (docs/adr/0007-a-shared-derivation-has-one-implementation.md) lets a
// native copy of a Go declaration exist and asks for a test holding the copies
// together. readNativeSource in native_constants_test.go is the entry point
// that reads the file. This is how a pin then addresses one definition in it.

// nativeRuleMethodEndPattern matches the closing brace of a native definition,
// which sits in the first column in the Objective-C methods and the C functions
// these pins read alike.
var nativeRuleMethodEndPattern = regexp.MustCompile(`(?m)^\}`)

// nativeRuleMethodBody returns the body of the native definition a pinned copy
// lives in — an Objective-C method or a C function — found by the pattern
// matching its opening line. Addressing it by name means a rename surfaces as a
// failure rather than as a silent pass over a definition that no longer exists,
// so a source the opening pattern does not match is reported with the spelling
// it was looked for by.
//
// named_key_tables_test.go reads its tables out of two methods this way, and
// wayland_keypad_folds_test.go reads the keypad fold table out of a C
// function.
func nativeRuleMethodBody(
	source string,
	opening *regexp.Regexp,
	method, expected string,
) (string, string) {
	header := opening.FindStringIndex(source)
	if header == nil {
		return "", fmt.Sprintf(
			"no `%s` definition to read the rule from (renamed?)",
			expected,
		)
	}

	body := source[header[1]:]

	end := nativeRuleMethodEndPattern.FindStringIndex(body)
	if end == nil {
		return "", fmt.Sprintf("the %s definition is never closed", method)
	}

	return body[:end[0]], ""
}
