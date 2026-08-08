package architecture_test

import (
	"fmt"
	"image"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
)

// The two sides of the recursive-grid label-autohide rule.
const (
	labelAutohideNativeSource = "internal/adapter/platform/darwin/overlay_darwin.m"

	// labelAutohideNativeMethod is the Objective-C method the rule lives in,
	// named in failure messages so a reader lands on the copy rather than on
	// this test.
	labelAutohideNativeMethod = "drawGridLabel:"

	// labelAutohideGoDeclaration names the Go side the same way.
	labelAutohideGoDeclaration = "recursivegrid.Style.ShowLabelIn"
)

// TestLabelAutohideRuleIsPinnedAcrossTheLanguageBoundary keeps the macOS
// overlay hiding the same labels as every other backend.
//
// Whether a recursive-grid cell is big enough for its key label is one question
// with one shared answer: recursivegrid.Style.ShowLabelIn, which the Cairo and
// GDI backends both call. macOS asks it in Objective-C, inside drawGridLabel:,
// and cannot call Go. This is ADR 0007's deliberate exception to the
// one-implementation rule
// (docs/adr/0007-a-shared-derivation-has-one-implementation.md): where the
// second implementation is in another language, what the rule asks for is a
// test holding the copies together rather than a deletion.
//
// The copy is a rule and not a constant, so it is pinned by running it. The
// Objective-C source is read into something this test can evaluate, and its
// answer is compared against the shared Go one over cases that straddle every
// edge the rule has: the multiplier that disables it, the threshold itself, and
// each cell dimension one pixel under. A disagreement is one configuration
// labeling a cell on macOS and leaving it blank everywhere else.
func TestLabelAutohideRuleIsPinnedAcrossTheLanguageBoundary(t *testing.T) {
	t.Parallel()

	rule := readNativeLabelAutohideRule(t)

	for _, disagreement := range labelAutohideDisagreements(rule) {
		t.Errorf(
			"%s: %s and %s disagree on %s\n\t%s draws the label: %t\n\t%s draws it: %t\n\tthe rule read from %s is: %s",
			labelAutohideNativeSource,
			labelAutohideNativeMethod,
			labelAutohideGoDeclaration,
			disagreement.testCase.describe(),
			labelAutohideNativeMethod,
			disagreement.native,
			labelAutohideGoDeclaration,
			disagreement.shared,
			labelAutohideNativeSource,
			rule,
		)
	}
}

// TestLabelAutohideRulePinCatchesNativeDrift keeps the pin above from passing
// over a rule that has moved.
//
// A pin is only worth its line count if the cases it runs can tell the copies
// apart, and the cases are chosen by hand: drop the width comparison and every
// square cell still agrees. So each way the Objective-C rule could plausibly
// drift is applied to the rule this pin actually read, and the mutant has to
// disagree with the shared implementation somewhere. Mutating the rule rather
// than the source text keeps this honest across a reformat of the .m file, and
// keeps "we broke it by hand once and watched it fail" from being the only
// evidence the guardrail has teeth.
func TestLabelAutohideRulePinCatchesNativeDrift(t *testing.T) {
	t.Parallel()

	rule := readNativeLabelAutohideRule(t)

	drifted := []struct {
		name  string
		apply func(nativeLabelAutohideRule) nativeLabelAutohideRule
	}{
		{
			name:  "the first cell dimension no longer compared",
			apply: func(rule nativeLabelAutohideRule) nativeLabelAutohideRule { return rule.withoutDimension(0) },
		},
		{
			name:  "the second cell dimension no longer compared",
			apply: func(rule nativeLabelAutohideRule) nativeLabelAutohideRule { return rule.withoutDimension(1) },
		},
		{
			name:  "a cell exactly on the threshold hidden instead of drawn",
			apply: func(rule nativeLabelAutohideRule) nativeLabelAutohideRule { return rule.withHideOperator("<=") },
		},
		{
			name: "the multiplier check inverted, so autohide applies when it is disabled",
			apply: func(rule nativeLabelAutohideRule) nativeLabelAutohideRule {
				return rule.withGuardOperator("<=")
			},
		},
		{
			name: "the threshold taken from the font size alone",
			apply: func(rule nativeLabelAutohideRule) nativeLabelAutohideRule {
				return rule.withThresholdFactor(1, "1")
			},
		},
		{
			name: "the multiplier admitted only above 1, so smaller ones stop hiding anything",
			apply: func(rule nativeLabelAutohideRule) nativeLabelAutohideRule {
				return rule.withGuardBound("1")
			},
		},
		{
			name: "the two dimensions joined by && , so a cell hides only when both are under",
			apply: func(rule nativeLabelAutohideRule) nativeLabelAutohideRule {
				return rule.withHideJoin("&&")
			},
		},
		{
			name: "the dimensions compared for equality instead of order",
			apply: func(rule nativeLabelAutohideRule) nativeLabelAutohideRule {
				return rule.withHideOperator("==")
			},
		},
	}

	for _, drift := range drifted {
		mutant := drift.apply(rule)

		if len(labelAutohideDisagreements(mutant)) == 0 {
			t.Errorf(
				"no case tells %s apart from %s: %s would pass the pin\n\tthe drifted rule is: %s",
				drift.name, labelAutohideGoDeclaration, labelAutohideNativeSource, mutant,
			)
		}
	}
}

// TestLabelAutohideRulePinReportsARuleItCannotRead pins the other half of the
// guardrail: a native rule this pin cannot read must be reported, never
// skipped. A pin that reads nothing and passes is worse than no pin, because
// it reads as coverage.
//
// This is where the pin's one deliberate cost sits. It reads a single shape,
// and a rewrite that keeps the behavior — folding the multiplier check into
// the comparison the way drawSubKeyPreviewInCellRect: writes it, say — fails
// here rather than being understood. Teaching it every equivalent spelling of
// the same rule is more machinery than the copy is worth; failing loudly and
// naming the shape it expected leaves the next author a one-line change to
// this file, which the same author is already making to the .m one.
func TestLabelAutohideRulePinReportsARuleItCannotRead(t *testing.T) {
	t.Parallel()

	unreadable := []struct {
		name   string
		source string
	}{
		{
			name:   "the method renamed",
			source: "- (void)drawGridCaption:(NSString *)label {\n\treturn;\n}\n",
		},
		{
			name: "the guard folded into the comparison",
			source: nativeLabelAutohideMethodSource(
				"\tCGFloat minCell = self.gridFont.pointSize * self.gridLabelAutohideMultiplier;\n" +
					"\tif (self.gridLabelAutohideMultiplier > 0 && (cellRect.size.width < minCell || cellRect.size.height < minCell))\n" +
					"\t\treturn;",
			),
		},
		{
			name: "the threshold measured against an operand this pin cannot value",
			source: nativeLabelAutohideMethodSource(
				"\tif (self.gridLabelAutohideMultiplier > 0) {\n" +
					"\t\tCGFloat minCell = self.gridSubKeyFont.pointSize * self.gridLabelAutohideMultiplier;\n" +
					"\t\tif (cellRect.size.width < minCell || cellRect.size.height < minCell)\n" +
					"\t\t\treturn;\n" +
					"\t}",
			),
		},
	}

	for _, source := range unreadable {
		if _, problem := parseNativeLabelAutohideRule(source.source); problem == "" {
			t.Errorf(
				"parsing accepted a source with %s; the pin would then run a rule it never read",
				source.name,
			)
		}
	}
}

// labelAutohideInputs are the four values the rule is a function of.
type labelAutohideInputs struct {
	fontSize   float64
	multiplier float64
	cellWidth  float64
	cellHeight float64
}

// labelAutohideOperands binds every name the Objective-C rule is allowed to
// mention to the input it stands for. It is the pin's vocabulary: a rule
// reading anything else is one this test cannot evaluate, and parsing says so
// rather than guessing a value.
var labelAutohideOperands = map[string]func(labelAutohideInputs) float64{
	"self.gridLabelAutohideMultiplier": func(inputs labelAutohideInputs) float64 {
		return inputs.multiplier
	},
	"self.gridFont.pointSize": func(inputs labelAutohideInputs) float64 { return inputs.fontSize },
	"cellRect.size.width":     func(inputs labelAutohideInputs) float64 { return inputs.cellWidth },
	"cellRect.size.height":    func(inputs labelAutohideInputs) float64 { return inputs.cellHeight },
}

// labelAutohideCase is one question put to both implementations of the rule.
//
// It carries integers where labelAutohideInputs carries floats, because these
// are the units the shared implementation is actually asked in — a cell is a
// pixel rectangle and a font size is a whole number of points. Widening them
// here would let a case be written that no configuration can produce.
type labelAutohideCase struct {
	name       string
	fontSize   int
	multiplier float64
	cellWidth  int
	cellHeight int
}

// inputs values the case for the native rule.
func (testCase labelAutohideCase) inputs() labelAutohideInputs {
	return labelAutohideInputs{
		fontSize:   float64(testCase.fontSize),
		multiplier: testCase.multiplier,
		cellWidth:  float64(testCase.cellWidth),
		cellHeight: float64(testCase.cellHeight),
	}
}

// describe spells the case out in full, so a failure carries the configuration
// that produced it rather than a case name to go looking for.
func (testCase labelAutohideCase) describe() string {
	return fmt.Sprintf(
		"%s (label font size %d, multiplier %g, cell %dx%d)",
		testCase.name, testCase.fontSize, testCase.multiplier,
		testCase.cellWidth, testCase.cellHeight,
	)
}

// labelAutohideDisagreement is one case the two implementations answer
// differently.
type labelAutohideDisagreement struct {
	testCase labelAutohideCase
	native   bool
	shared   bool
}

// labelAutohideCases are the questions both implementations are asked.
//
// They exist to separate the rules that could plausibly be written here, not to
// cover an input space: each dimension is taken one pixel under the threshold
// on its own, because a rule comparing only one of them agrees on every square
// cell; the threshold is landed on exactly, because >= and > differ nowhere
// else; and a multiplier below 1 is asked, because a guard that admitted only
// multipliers above 1 agrees everywhere else.
// TestLabelAutohideRulePinCatchesNativeDrift is what keeps this list honest.
//
// One part of the rule no case here can reach: with a cell size and a font size
// both non-negative, dropping the multiplier guard entirely answers exactly as
// the guard does, on every input either implementation can be handed. That half
// is pinned by shape instead — parsing requires the guard to be there, and
// reports its absence.
func labelAutohideCases() []labelAutohideCase {
	return []labelAutohideCase{
		{
			name:       "a zero multiplier disables autohide",
			fontSize:   100,
			multiplier: 0,
			cellWidth:  1,
			cellHeight: 1,
		},
		{
			name:       "a negative multiplier disables autohide",
			fontSize:   100,
			multiplier: -2,
			cellWidth:  1,
			cellHeight: 1,
		},
		{
			name:       "a cell exactly on the threshold",
			fontSize:   20,
			multiplier: 1.5,
			cellWidth:  30,
			cellHeight: 30,
		},
		{
			name:       "a cell one pixel under on width",
			fontSize:   20,
			multiplier: 1.5,
			cellWidth:  29,
			cellHeight: 30,
		},
		{
			name:       "a cell one pixel under on height",
			fontSize:   20,
			multiplier: 1.5,
			cellWidth:  30,
			cellHeight: 29,
		},
		{
			name:       "a cell under on both dimensions",
			fontSize:   20,
			multiplier: 1.5,
			cellWidth:  4,
			cellHeight: 4,
		},
		{
			name:       "a cell far over the threshold",
			fontSize:   20,
			multiplier: 1.5,
			cellWidth:  400,
			cellHeight: 400,
		},
		{name: "a wide, short cell", fontSize: 10, multiplier: 2, cellWidth: 400, cellHeight: 19},
		{name: "a tall, narrow cell", fontSize: 10, multiplier: 2, cellWidth: 19, cellHeight: 400},
		{
			name:       "a threshold falling between two pixels, cell under it",
			fontSize:   10,
			multiplier: 1.55,
			cellWidth:  15,
			cellHeight: 16,
		},
		{
			name:       "a threshold falling between two pixels, cell over it",
			fontSize:   10,
			multiplier: 1.55,
			cellWidth:  16,
			cellHeight: 16,
		},
		{
			name:       "a multiplier no cell on screen clears",
			fontSize:   20,
			multiplier: 100,
			cellWidth:  400,
			cellHeight: 400,
		},
		{
			name:       "a multiplier below 1, cell under the threshold it sets",
			fontSize:   20,
			multiplier: 0.5,
			cellWidth:  8,
			cellHeight: 8,
		},
		{
			name:       "a multiplier below 1, cell over the threshold it sets",
			fontSize:   20,
			multiplier: 0.5,
			cellWidth:  12,
			cellHeight: 12,
		},
	}
}

// labelAutohideDisagreements runs every case through the native rule and
// through the shared Go implementation, and returns the cases they answer
// differently.
func labelAutohideDisagreements(rule nativeLabelAutohideRule) []labelAutohideDisagreement {
	var disagreements []labelAutohideDisagreement

	for _, testCase := range labelAutohideCases() {
		style := recursivegrid.NewStyle(recursivegrid.StyleOptions{
			FontSize:                testCase.fontSize,
			LabelAutohideMultiplier: testCase.multiplier,
		})

		shared := style.ShowLabelIn(image.Rect(0, 0, testCase.cellWidth, testCase.cellHeight))

		native := rule.showsLabel(testCase.inputs())
		if native == shared {
			continue
		}

		disagreements = append(disagreements, labelAutohideDisagreement{
			testCase: testCase,
			native:   native,
			shared:   shared,
		})
	}

	return disagreements
}

// nativeLabelAutohideRule is the Objective-C label-autohide rule in the only
// form a Go test can hold it to: something it can run.
//
// Nothing here is an expectation. Every field is read out of the .m file, and
// the shared Go implementation is the only expectation this pin has — which is
// what makes the pin bidirectional: change either side alone and the two stop
// answering alike.
type nativeLabelAutohideRule struct {
	// guard decides whether autohide applies at all; when it does not hold, the
	// label is drawn whatever size the cell is.
	guard nativeRuleComparison

	// thresholdName is what the rule calls the size a cell must reach, and
	// thresholdFactors are the two things it multiplies to get there.
	thresholdName    string
	thresholdFactors [2]string

	// hideWhen are the comparisons that skip the label, joined by hideJoin
	// ("||" or "&&").
	hideWhen []nativeRuleComparison
	hideJoin string
}

// String renders the rule back as one sentence, so a failure shows what was
// read out of the source rather than only that it disagreed.
func (rule nativeLabelAutohideRule) String() string {
	skips := make([]string, 0, len(rule.hideWhen))
	for _, comparison := range rule.hideWhen {
		skips = append(skips, comparison.String())
	}

	return fmt.Sprintf(
		"when %s, %s = %s * %s and the label is skipped if %s",
		rule.guard, rule.thresholdName,
		rule.thresholdFactors[0], rule.thresholdFactors[1],
		strings.Join(skips, " "+rule.hideJoin+" "),
	)
}

// showsLabel answers the question the shared implementation answers: is this
// cell big enough for its label to be drawn?
func (rule nativeLabelAutohideRule) showsLabel(inputs labelAutohideInputs) bool {
	values := make(map[string]float64, len(labelAutohideOperands)+1)
	for name, read := range labelAutohideOperands {
		values[name] = read(inputs)
	}

	if !rule.guard.holds(values) {
		return true
	}

	values[rule.thresholdName] = nativeRuleValue(rule.thresholdFactors[0], values) *
		nativeRuleValue(rule.thresholdFactors[1], values)

	return !rule.hidesLabel(values)
}

// hidesLabel folds the skip comparisons together the way the source joins them.
func (rule nativeLabelAutohideRule) hidesLabel(values map[string]float64) bool {
	all := rule.hideJoin == "&&"
	hides := all

	for _, comparison := range rule.hideWhen {
		if all {
			hides = hides && comparison.holds(values)

			continue
		}

		hides = hides || comparison.holds(values)
	}

	return hides
}

// withoutDimension drops one of the compared cell dimensions, standing for a
// rule that stopped measuring it.
func (rule nativeLabelAutohideRule) withoutDimension(index int) nativeLabelAutohideRule {
	kept := make([]nativeRuleComparison, 0, len(rule.hideWhen))

	for position, comparison := range rule.hideWhen {
		if position != index {
			kept = append(kept, comparison)
		}
	}

	rule.hideWhen = kept

	return rule
}

// withHideOperator rewrites how every cell dimension is compared against the
// threshold.
func (rule nativeLabelAutohideRule) withHideOperator(op string) nativeLabelAutohideRule {
	rewritten := make([]nativeRuleComparison, 0, len(rule.hideWhen))

	for _, comparison := range rule.hideWhen {
		comparison.op = op
		rewritten = append(rewritten, comparison)
	}

	rule.hideWhen = rewritten

	return rule
}

// withHideJoin rewrites how the cell dimensions are joined, standing for a rule
// that hides a cell only when both are under the threshold rather than either.
func (rule nativeLabelAutohideRule) withHideJoin(join string) nativeLabelAutohideRule {
	rule.hideJoin = join

	return rule
}

// withGuardOperator rewrites how the multiplier decides whether autohide
// applies.
func (rule nativeLabelAutohideRule) withGuardOperator(op string) nativeLabelAutohideRule {
	rule.guard.op = op

	return rule
}

// withGuardBound rewrites the value the multiplier is measured against to
// decide whether autohide applies.
func (rule nativeLabelAutohideRule) withGuardBound(bound string) nativeLabelAutohideRule {
	rule.guard.right = bound

	return rule
}

// withThresholdFactor rewrites one of the two things the threshold is a product
// of.
func (rule nativeLabelAutohideRule) withThresholdFactor(
	index int,
	factor string,
) nativeLabelAutohideRule {
	rule.thresholdFactors[index] = factor

	return rule
}

// nativeLabelAutohideMethodPattern matches the opening line of the
// drawGridLabel: definition that carries the rule. The four-argument
// forwarder above it and the two declarations in the @interface are excluded
// by the alpha: argument and by refusing to cross a `;` or a brace.
var nativeLabelAutohideMethodPattern = regexp.MustCompile(
	`(?m)^- \(void\)drawGridLabel:[^{};]*alpha:\(CGFloat\)alpha[ \t]*\{`,
)

// nativeLabelAutohideRulePattern matches the whole guard: the multiplier check,
// the threshold it declares, and the comparison that skips the label.
var nativeLabelAutohideRulePattern = regexp.MustCompile(
	`if[ \t]*\([ \t]*([\w.]+)[ \t]*` + nativeRuleComparisonOperators + `[ \t]*([-\w.]+)[ \t]*\)[ \t]*\{\s*` +
		`CGFloat[ \t]+(\w+)[ \t]*=[ \t]*([\w.]+)[ \t]*\*[ \t]*([\w.]+)[ \t]*;\s*` +
		`if[ \t]*\(([^()]+)\)\s*\{?\s*return[ \t]*;\s*\}?\s*\}`,
)

// readNativeLabelAutohideRule reads the rule out of the macOS overlay, failing
// the test when it cannot — a rule this pin cannot read is a rule it cannot
// hold to the shared one, and passing quietly there would be worse than having
// no pin at all.
func readNativeLabelAutohideRule(t *testing.T) nativeLabelAutohideRule {
	t.Helper()

	rule, problem := parseNativeLabelAutohideRule(readNativeSource(t, labelAutohideNativeSource))
	if problem != "" {
		t.Fatalf("%s: %s", labelAutohideNativeSource, problem)
	}

	return rule
}

// parseNativeLabelAutohideRule reads the label-autohide rule out of an
// Objective-C source. The second result describes why the rule could not be
// read, and is empty when it could — an error value would buy nothing here,
// since the only caller turns it straight into a test failure.
func parseNativeLabelAutohideRule(source string) (nativeLabelAutohideRule, string) {
	body, problem := nativeRuleMethodBody(
		source,
		nativeLabelAutohideMethodPattern,
		labelAutohideNativeMethod,
		"- (void)drawGridLabel:...alpha:(CGFloat)alpha {",
	)
	if problem != "" {
		return nativeLabelAutohideRule{}, problem
	}

	matches := nativeLabelAutohideRulePattern.FindAllStringSubmatch(body, -1)

	if len(matches) != 1 {
		return nativeLabelAutohideRule{}, fmt.Sprintf(
			"%s holds %d autohide guards shaped `if (<multiplier> <op> <literal>) { CGFloat <threshold> = <a> * <b>; if (<comparisons>) return; }`, want exactly 1 (rewritten?)",
			labelAutohideNativeMethod,
			len(matches),
		)
	}

	match := matches[0]

	rule := nativeLabelAutohideRule{
		guard: nativeRuleComparison{
			left:  match[1],
			op:    match[2],
			right: match[3],
		},
		thresholdName:    match[4],
		thresholdFactors: [2]string{match[5], match[6]},
	}

	rule.hideWhen, rule.hideJoin, problem = parseNativeRuleCondition(match[7])
	if problem != "" {
		return nativeLabelAutohideRule{}, problem
	}

	return rule, validateNativeLabelAutohideRule(rule)
}

// validateNativeLabelAutohideRule checks that every name and operator the rule
// was written with is one this pin can evaluate. It is what stops a rewritten
// rule from being run with a value invented for it.
func validateNativeLabelAutohideRule(rule nativeLabelAutohideRule) string {
	// The guard is evaluated before the threshold exists, which is also true of
	// the Objective-C it was read from — the threshold is declared inside the
	// guard. A guard naming it would be valued at nothing rather than reported.
	if rule.guard.left == rule.thresholdName || rule.guard.right == rule.thresholdName {
		return fmt.Sprintf(
			"the guard `%s` reads %s, which the rule only declares inside it",
			rule.guard, rule.thresholdName,
		)
	}

	comparisons := append([]nativeRuleComparison{rule.guard}, rule.hideWhen...)

	tokens := []string{rule.thresholdFactors[0], rule.thresholdFactors[1]}

	for _, comparison := range comparisons {
		if _, known := nativeRuleComparators[comparison.op]; !known {
			return fmt.Sprintf(
				"`%s` compares with %q, which this pin does not read",
				comparison,
				comparison.op,
			)
		}

		tokens = append(tokens, comparison.left, comparison.right)
	}

	for _, token := range tokens {
		if _, bound := labelAutohideOperands[token]; bound {
			continue
		}

		if token == rule.thresholdName {
			continue
		}

		_, parseErr := strconv.ParseFloat(token, 64)
		if parseErr == nil {
			continue
		}

		return fmt.Sprintf(
			"the rule reads %s, which this pin cannot value; it knows %s, the threshold it declares, and numeric literals",
			token,
			strings.Join(sortedNativeRuleOperands(labelAutohideOperands), ", "),
		)
	}

	return ""
}

// nativeLabelAutohideMethodSource wraps a body in the method definition the
// parser looks for, so a test can hand it a rule shaped differently from the
// one in the tree.
func nativeLabelAutohideMethodSource(body string) string {
	return "- (void)drawGridLabel:(NSString *)label\n" +
		"             inCellRect:(NSRect)cellRect\n" +
		"              isMatched:(BOOL)isMatched\n" +
		"    matchedPrefixLength:(int)matchedPrefixLength\n" +
		"                  alpha:(CGFloat)alpha {\n" +
		body + "\n}\n"
}
