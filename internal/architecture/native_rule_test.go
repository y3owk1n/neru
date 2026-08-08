package architecture_test

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// This file is what the rule-shaped language-boundary pins have in common.
//
// ADR 0007 (docs/adr/0007-a-shared-derivation-has-one-implementation.md) lets a
// native copy of a Go rule exist and asks for a test holding the copies
// together. Two such pins exist — label_autohide_rule_test.go and
// sub_key_preview_autohide_rule_test.go — and each brings its own pattern for
// the shape its rule is written in, because the shapes genuinely differ. What
// they must not each bring is a second answer to "what does `<=` mean" or "how
// is `a || b` split", which is the divergence that ADR is about, and which
// would be worse here than anywhere: two guardrails reading the same operator
// differently is a guardrail nobody can trust.
//
// So the vocabulary lives here — the comparisons, how a condition is split into
// them, how a token is valued, and how an Objective-C method body is found —
// and the shape of each rule stays with the pin that reads it.
//
// readNativeSource in native_constants_test.go is the entry point that reads
// the file; this is what a pin does with what it read.

// nativeRuleComparisonOperators is the alternation every comparison in a native
// rule is read with; the longer spellings come first so `<=` is never read as
// `<`.
const nativeRuleComparisonOperators = `(<=|>=|==|!=|<|>)`

// nativeRuleComparators are the comparisons a pinned rule may be written with.
// Parsing rejects anything outside this set, so evaluating a parsed comparison
// never has to decide what an unknown operator means.
var nativeRuleComparators = map[string]func(left, right float64) bool{
	"<":  func(left, right float64) bool { return left < right },
	"<=": func(left, right float64) bool { return left <= right },
	">":  func(left, right float64) bool { return left > right },
	">=": func(left, right float64) bool { return left >= right },
	"==": func(left, right float64) bool { return left == right },
	"!=": func(left, right float64) bool { return left != right },
}

// nativeRuleComparison is one `a < b` of a pinned rule. Each side is an operand
// in that rule's vocabulary, a value the rule works out for itself, or a
// numeric literal.
type nativeRuleComparison struct {
	left  string
	op    string
	right string
}

// String spells the comparison the way its source writes it.
func (comparison nativeRuleComparison) String() string {
	return fmt.Sprintf("%s %s %s", comparison.left, comparison.op, comparison.right)
}

// holds evaluates the comparison against the values bound so far.
func (comparison nativeRuleComparison) holds(values map[string]float64) bool {
	return nativeRuleComparators[comparison.op](
		nativeRuleValue(comparison.left, values),
		nativeRuleValue(comparison.right, values),
	)
}

// nativeRuleValue reads a token that parsing has already accepted: a bound
// name, or a numeric literal.
func nativeRuleValue(token string, values map[string]float64) float64 {
	if value, bound := values[token]; bound {
		return value
	}

	literal, _ := strconv.ParseFloat(token, 64)

	return literal
}

// nativeRuleTermPattern matches one comparison of a condition.
var nativeRuleTermPattern = regexp.MustCompile(
	`^[ \t]*([-\w.]+)[ \t]*` + nativeRuleComparisonOperators + `[ \t]*([-\w.]+)[ \t]*$`,
)

// parseNativeRuleCondition splits a condition into its comparisons and the
// operator joining them. The third result describes why it could not be read,
// and is empty when it could.
func parseNativeRuleCondition(condition string) ([]nativeRuleComparison, string, string) {
	join := "||"
	if !strings.Contains(condition, join) && strings.Contains(condition, "&&") {
		join = "&&"
	}

	if strings.Contains(condition, "||") && strings.Contains(condition, "&&") {
		return nil, "", fmt.Sprintf(
			"the condition `%s` mixes || and &&, which this pin does not read",
			strings.TrimSpace(condition),
		)
	}

	var comparisons []nativeRuleComparison

	for term := range strings.SplitSeq(condition, join) {
		parsed := nativeRuleTermPattern.FindStringSubmatch(term)
		if parsed == nil {
			return nil, "", fmt.Sprintf(
				"`%s` in the condition is not a comparison this pin reads",
				strings.TrimSpace(term),
			)
		}

		comparisons = append(comparisons, nativeRuleComparison{
			left: parsed[1], op: parsed[2], right: parsed[3],
		})
	}

	return comparisons, join, ""
}

// nativeRuleMethodEndPattern matches the closing brace of an Objective-C method
// definition, which sits in the first column.
var nativeRuleMethodEndPattern = regexp.MustCompile(`(?m)^\}`)

// nativeRuleMethodBody returns the body of the Objective-C method a rule lives
// in, found by the pattern matching its opening line. Addressing a method by
// name means a rename surfaces as a failure rather than as a silent pass over a
// method that no longer exists, so a source the opening pattern does not match
// is reported with the spelling it was looked for by.
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

// sortedNativeRuleOperands lists a pin's vocabulary in a stable order for
// failure messages. Each pin binds its operand names to its own input type, so
// the map's value type is the pin's business and only the names are shared.
func sortedNativeRuleOperands[Reader any](operands map[string]Reader) []string {
	names := make([]string, 0, len(operands))
	for name := range operands {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
