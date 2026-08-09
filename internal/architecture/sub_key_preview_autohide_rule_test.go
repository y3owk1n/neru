package architecture_test

import (
	"fmt"
	"image"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
	"github.com/y3owk1n/neru/internal/domain"
)

// The two sides of the recursive-grid sub-key-preview autohide rule.
const (
	subKeyPreviewNativeSource = "internal/adapter/platform/darwin/overlay_darwin.m"

	// subKeyPreviewNativeMethod is the Objective-C method the rule lives in,
	// named in failure messages so a reader lands on the copy rather than on
	// this test.
	subKeyPreviewNativeMethod = "drawSubKeyPreviewInCellRect:"

	// subKeyPreviewGoDeclaration names the Go side the same way.
	subKeyPreviewGoDeclaration = "recursivegrid.Style.ShowSubKeyPreviewIn"
)

// TestSubKeyPreviewAutohideRuleIsPinnedAcrossTheLanguageBoundary keeps the macOS
// overlay hiding the same sub-key previews as every other backend.
//
// Whether a recursive-grid cell is divided finely enough for the mini-grid of
// the next level's keys to be worth drawing is one question with one shared
// answer: recursivegrid.Style.ShowSubKeyPreviewIn, which the Cairo and GDI
// backends both call. macOS asks it in Objective-C, inside
// drawSubKeyPreviewInCellRect:, and cannot call Go. This is ADR 0007's
// deliberate exception to the one-implementation rule
// (docs/adr/0007-a-shared-derivation-has-one-implementation.md): where the
// second implementation is in another language, what the rule asks for is a test
// holding the copies together rather than a deletion.
//
// It read both copies out of their sources until #1297, because the Go one sat
// behind `//go:build linux && cgo` and a test running on the macOS host could
// not link it. Converging the Windows backend on the same mini-grid gave the
// predicate an untagged home, so the shared side is now run rather than parsed —
// which pins its behavior rather than its shape, the way the label-autohide pin
// next door always has.
//
// The Objective-C copy is a rule and not a constant, so it is pinned by running
// it too. It is read into something this test can evaluate, and the two are asked
// the same questions over cases that straddle every edge the rule has: the
// multiplier that disables it, the threshold itself, each sub-cell dimension one
// pixel under it, and a cell that clears the threshold whole while its sub-cells
// do not. A disagreement is one configuration drawing a preview on macOS and
// leaving the cell bare everywhere else.
func TestSubKeyPreviewAutohideRuleIsPinnedAcrossTheLanguageBoundary(t *testing.T) {
	t.Parallel()

	native := readNativeSubKeyPreviewRule(t)

	for _, disagreement := range subKeyPreviewDisagreements(native) {
		t.Errorf(
			"%s: %s and %s disagree on %s\n\t%s draws the preview: %t\n\t%s draws it: %t\n\tthe rule read from %s is: %s",
			subKeyPreviewNativeSource,
			subKeyPreviewNativeMethod,
			subKeyPreviewGoDeclaration,
			disagreement.testCase.describe(),
			subKeyPreviewNativeMethod,
			disagreement.native,
			subKeyPreviewGoDeclaration,
			disagreement.shared,
			subKeyPreviewNativeSource,
			native,
		)
	}
}

// TestSubKeyPreviewAutohideRulePinCatchesNativeDrift keeps the pin above from
// passing over an Objective-C rule that has moved.
//
// A pin is only worth its line count if the cases it runs can tell the copies
// apart, and the cases are chosen by hand: drop the width comparison and every
// square sub-cell still agrees, measure the whole cell instead of a sub-cell
// and every undivided dimension still agrees. So each way the Objective-C rule could
// plausibly drift is applied to the rule this pin actually read, and the mutant
// has to disagree with the shared implementation somewhere. Mutating the rule
// rather than the source text keeps this honest across a reformat of the .m file,
// and keeps "we broke it by hand once and watched it fail" from being the only
// evidence the guardrail has teeth.
func TestSubKeyPreviewAutohideRulePinCatchesNativeDrift(t *testing.T) {
	t.Parallel()

	native := readNativeSubKeyPreviewRule(t)

	drifted := []struct {
		name  string
		apply func(subKeyPreviewRule) subKeyPreviewRule
	}{
		{
			name:  "the first sub-cell dimension no longer compared",
			apply: func(rule subKeyPreviewRule) subKeyPreviewRule { return rule.withoutDimension(0) },
		},
		{
			name:  "the second sub-cell dimension no longer compared",
			apply: func(rule subKeyPreviewRule) subKeyPreviewRule { return rule.withoutDimension(1) },
		},
		{
			name: "a sub-cell exactly on the threshold hidden instead of drawn",
			apply: func(rule subKeyPreviewRule) subKeyPreviewRule {
				return rule.withVerdictOperator("<=")
			},
		},
		{
			name: "the dimensions compared for equality instead of order",
			apply: func(rule subKeyPreviewRule) subKeyPreviewRule {
				return rule.withVerdictOperator("==")
			},
		},
		{
			name: "the two dimensions joined by &&, so a cell hides only when both are under",
			apply: func(rule subKeyPreviewRule) subKeyPreviewRule {
				return rule.withVerdictJoin("&&")
			},
		},
		{
			name: "the multiplier check inverted, so autohide applies when it is disabled",
			apply: func(rule subKeyPreviewRule) subKeyPreviewRule {
				return rule.withGuardOperator("<=")
			},
		},
		{
			name: "the multiplier admitted only above 1, so smaller ones stop hiding anything",
			apply: func(rule subKeyPreviewRule) subKeyPreviewRule {
				return rule.withGuardBound("1")
			},
		},
		{
			name: "the threshold taken from the font size alone",
			apply: func(rule subKeyPreviewRule) subKeyPreviewRule {
				return rule.withProductsFromTheirFirstFactor()
			},
		},
		{
			name: "the whole cell measured instead of a sub-cell",
			apply: func(rule subKeyPreviewRule) subKeyPreviewRule {
				return rule.withQuotientsUndivided()
			},
		},
		{
			name: "the width divided by the row count and the height by the column count",
			apply: func(rule subKeyPreviewRule) subKeyPreviewRule {
				return rule.withQuotientDivisorsSwapped()
			},
		},
	}

	for _, drift := range drifted {
		mutant := drift.apply(native)

		if len(subKeyPreviewDisagreements(mutant)) == 0 {
			t.Errorf(
				"no case tells %s apart from %s: %s would pass the pin\n\tthe drifted rule is: %s",
				drift.name, subKeyPreviewGoDeclaration, subKeyPreviewNativeSource, mutant,
			)
		}
	}
}

// TestSubKeyPreviewAutohideRulePinReportsARuleItCannotRead pins the other half
// of the guardrail: a native rule this pin cannot read must be reported, never
// skipped. A pin that reads nothing and passes is worse than no pin, because it
// reads as coverage.
//
// This is where the pin's one deliberate cost sits. It reads one shape, and a
// rewrite that keeps the behavior — unfolding the Objective-C guard into the
// nested form drawGridLabel: writes it, say — fails here rather than being
// understood. Teaching it every equivalent spelling of the same rule is more
// machinery than the copy is worth; failing loudly and naming the shape it
// expected leaves the next author a one-line change to this file, which the same
// author is already making to the .m one.
func TestSubKeyPreviewAutohideRulePinReportsARuleItCannotRead(t *testing.T) {
	t.Parallel()

	unreadable := []struct {
		name   string
		source string
	}{
		{
			name:   "the method renamed",
			source: "- (void)drawSubGridPreviewInCellRect:(NSRect)cellRect {\n\treturn;\n}\n",
		},
		{
			name: "the guard unfolded into a nested if",
			source: nativeSubKeyPreviewMethodSource(
				"\tCGFloat subCellWidth = cellRect.size.width / cols;\n" +
					"\tCGFloat subCellHeight = cellRect.size.height / rows;\n" +
					"\tCGFloat minSubCell = subFont.pointSize * self.gridSubKeyAutohideMultiplier;\n" +
					"\tif (self.gridSubKeyAutohideMultiplier > 0) {\n" +
					"\t\tif (subCellWidth < minSubCell || subCellHeight < minSubCell)\n" +
					"\t\t\treturn;\n" +
					"\t}",
			),
		},
		{
			name: "a sub-cell measured from an operand this pin cannot value",
			source: nativeSubKeyPreviewMethodSource(
				"\tCGFloat subCellWidth = cellRect.size.width / self.gridSubKeyColumns;\n" +
					"\tCGFloat subCellHeight = cellRect.size.height / rows;\n" +
					"\tCGFloat minSubCell = subFont.pointSize * self.gridSubKeyAutohideMultiplier;\n" +
					"\tif (self.gridSubKeyAutohideMultiplier > 0 && (subCellWidth < minSubCell || subCellHeight < minSubCell))\n" +
					"\t\treturn;",
			),
		},
		{
			name: "the skip condition mixing || and &&",
			source: nativeSubKeyPreviewMethodSource(
				"\tCGFloat subCellWidth = cellRect.size.width / cols;\n" +
					"\tCGFloat subCellHeight = cellRect.size.height / rows;\n" +
					"\tCGFloat minSubCell = subFont.pointSize * self.gridSubKeyAutohideMultiplier;\n" +
					"\tif (self.gridSubKeyAutohideMultiplier > 0 && (subCellWidth < minSubCell || subCellHeight < minSubCell && cols > 0))\n" +
					"\t\treturn;",
			),
		},
	}

	for _, source := range unreadable {
		if _, problem := parseNativeSubKeyPreviewRule(source.source); problem == "" {
			t.Errorf(
				"parsing accepted an Objective-C source with %s; the pin would then run a rule it never read",
				source.name,
			)
		}
	}
}

// subKeyPreviewInputs are the values both rules are functions of.
//
// The two sides name them differently — cellRect.size.width against cell.Dx(),
// self.gridSubKeyAutohideMultiplier against a method on the style — so the
// Objective-C side brings a vocabulary binding its names to these, and the Go
// side is handed them as the style and the arguments it actually takes.
type subKeyPreviewInputs struct {
	fontSize   int
	multiplier float64
	cell       image.Rectangle
	subCols    int
	subRows    int
}

// style builds the resolved style the shared rule reads its inputs through.
//
// The preview is switched on in every case, because the Objective-C method has
// no counterpart to that check inside it — its caller gates on
// self.gridDrawSubKeyPreview before calling it — and enabled is the only state in
// which the two rules are answering the same question.
func (inputs subKeyPreviewInputs) style() recursivegrid.Style {
	return recursivegrid.NewStyle(recursivegrid.StyleOptions{
		SubKeyPreview:                   true,
		SubKeyPreviewFontSize:           inputs.fontSize,
		SubKeyPreviewAutohideMultiplier: inputs.multiplier,
	})
}

// showsPreview asks the shared Go rule the case.
func (inputs subKeyPreviewInputs) showsPreview() bool {
	return inputs.style().ShowSubKeyPreviewIn(
		inputs.cell,
		domain.GridDimensions{Rows: inputs.subRows, Cols: inputs.subCols},
	)
}

// subKeyPreviewNativeOperands binds every name the Objective-C rule is allowed to
// mention to the input it stands for. It is that side's vocabulary: a rule
// reading anything else is one this test cannot evaluate, and parsing says so
// rather than guessing a value.
//
// Its names are bound to what the cgo bridge hands the overlay, which this pin
// asserts rather than checks: the multiplier and the sub-grid dimensions travel
// as fields on the style struct, and the font point size is the configured size
// rather than the value recursivegrid.Style clamps to at least 1. Those two
// agree for every configuration, because validation refuses a preview font size
// below 1, so the clamp is inert wherever this pin can be asked.
var subKeyPreviewNativeOperands = map[string]func(subKeyPreviewInputs) float64{
	"self.gridSubKeyAutohideMultiplier": func(inputs subKeyPreviewInputs) float64 {
		return inputs.style().SubKeyPreviewAutohideMultiplier()
	},
	"subFont.pointSize": func(inputs subKeyPreviewInputs) float64 {
		return float64(inputs.style().SubKeyPreviewFontSize())
	},
	"cellRect.size.width": func(inputs subKeyPreviewInputs) float64 {
		return float64(inputs.cell.Dx())
	},
	"cellRect.size.height": func(inputs subKeyPreviewInputs) float64 {
		return float64(inputs.cell.Dy())
	},
	"cols": func(inputs subKeyPreviewInputs) float64 { return float64(inputs.subCols) },
	"rows": func(inputs subKeyPreviewInputs) float64 { return float64(inputs.subRows) },
}

// subKeyPreviewArithmetic are the operators a measured value may be built with.
// The Objective-C rule builds every one of its measurements from exactly two
// operands, so nothing here needs precedence.
var subKeyPreviewArithmetic = map[string]func(left, right float64) float64{
	"*": func(left, right float64) float64 { return left * right },
	"/": func(left, right float64) float64 { return left / right },
}

// subKeyPreviewCase is one question put to both implementations of the rule.
//
// It carries integers where subKeyPreviewInputs carries a rectangle and a font
// size, because these are the units the rules are actually asked in — a cell is
// a pixel rectangle, a font size is a whole number of points, and a sub-grid is
// a whole number of columns by rows. Widening them would let a case be written
// that no configuration can produce.
//
// The multipliers are all exactly representable in 32 bits, because that is how
// one reaches the Objective-C side: the bridge passes it as a C float. A
// multiplier like 1.55 would be a different number on each side of the boundary
// by a rounding error, and this pin is not the place to discover that.
type subKeyPreviewCase struct {
	name       string
	fontSize   int
	multiplier float64
	cellWidth  int
	cellHeight int
	subCols    int
	subRows    int
}

// inputs values the case for either rule.
func (testCase subKeyPreviewCase) inputs() subKeyPreviewInputs {
	return subKeyPreviewInputs{
		fontSize:   testCase.fontSize,
		multiplier: testCase.multiplier,
		cell:       image.Rect(0, 0, testCase.cellWidth, testCase.cellHeight),
		subCols:    testCase.subCols,
		subRows:    testCase.subRows,
	}
}

// describe spells the case out in full, so a failure carries the configuration
// that produced it rather than a case name to go looking for.
func (testCase subKeyPreviewCase) describe() string {
	return fmt.Sprintf(
		"%s (preview font size %d, multiplier %g, cell %dx%d divided %dx%d)",
		testCase.name, testCase.fontSize, testCase.multiplier,
		testCase.cellWidth, testCase.cellHeight, testCase.subCols, testCase.subRows,
	)
}

// subKeyPreviewDisagreement is one case the two implementations answer
// differently.
type subKeyPreviewDisagreement struct {
	testCase subKeyPreviewCase
	native   bool
	shared   bool
}

// subKeyPreviewCases are the questions both implementations are asked.
//
// They exist to separate the rules that could plausibly be written here, not to
// cover an input space: each sub-cell dimension is taken one pixel under the
// threshold on its own, because a rule comparing only one of them agrees on
// every square sub-cell; the threshold is landed on exactly, because >= and >
// differ nowhere else; a cell that clears the threshold whole while its
// sub-cells do not is asked, because that is the shape the Windows backend used
// to measure and the one no copy may drift back into; the division is asked with
// unequal columns and rows, because a rule dividing the width by the row count
// agrees on every square division; and a multiplier below 1 is asked, because a
// guard that admitted only multipliers above 1 agrees everywhere else.
// TestSubKeyPreviewAutohideRulePinCatchesNativeDrift is what keeps this list
// honest.
//
// One part of the rule no case here can reach: with a cell size and a font size
// both non-negative, dropping the multiplier guard entirely answers exactly as
// the guard does, on every input either implementation can be handed. That half
// is pinned by shape instead — parsing requires the guard to be there, and
// reports its absence.
func subKeyPreviewCases() []subKeyPreviewCase {
	return []subKeyPreviewCase{
		{
			name:       "a zero multiplier disables autohide",
			fontSize:   10,
			multiplier: 0,
			cellWidth:  4,
			cellHeight: 4,
			subCols:    3,
			subRows:    3,
		},
		{
			name:       "a negative multiplier disables autohide",
			fontSize:   10,
			multiplier: -2,
			cellWidth:  4,
			cellHeight: 4,
			subCols:    3,
			subRows:    3,
		},
		{
			name:       "sub-cells exactly on the threshold",
			fontSize:   10,
			multiplier: 1.5,
			cellWidth:  45,
			cellHeight: 30,
			subCols:    3,
			subRows:    2,
		},
		{
			name:       "a sub-cell one pixel under on width",
			fontSize:   10,
			multiplier: 1.5,
			cellWidth:  42,
			cellHeight: 30,
			subCols:    3,
			subRows:    2,
		},
		{
			name:       "a sub-cell one pixel under on height",
			fontSize:   10,
			multiplier: 1.5,
			cellWidth:  45,
			cellHeight: 28,
			subCols:    3,
			subRows:    2,
		},
		{
			name:       "sub-cells under on both dimensions",
			fontSize:   10,
			multiplier: 1.5,
			cellWidth:  30,
			cellHeight: 20,
			subCols:    3,
			subRows:    2,
		},
		{
			name:       "sub-cells far over the threshold",
			fontSize:   10,
			multiplier: 1.5,
			cellWidth:  900,
			cellHeight: 600,
			subCols:    3,
			subRows:    2,
		},
		{
			name:       "a cell far over the threshold divided into sub-cells that are not",
			fontSize:   10,
			multiplier: 1.5,
			cellWidth:  40,
			cellHeight: 40,
			subCols:    5,
			subRows:    5,
		},
		{
			name:       "sub-cells over the font size but under the threshold",
			fontSize:   10,
			multiplier: 1.5,
			cellWidth:  36,
			cellHeight: 24,
			subCols:    3,
			subRows:    2,
		},
		{
			name:       "a wide, short cell",
			fontSize:   10,
			multiplier: 2,
			cellWidth:  400,
			cellHeight: 38,
			subCols:    2,
			subRows:    2,
		},
		{
			name:       "a tall, narrow cell",
			fontSize:   10,
			multiplier: 2,
			cellWidth:  38,
			cellHeight: 400,
			subCols:    2,
			subRows:    2,
		},
		{
			name:       "a threshold falling between two pixels, sub-cells under it",
			fontSize:   10,
			multiplier: 1.25,
			cellWidth:  36,
			cellHeight: 24,
			subCols:    3,
			subRows:    2,
		},
		{
			name:       "a threshold falling between two pixels, sub-cells over it",
			fontSize:   10,
			multiplier: 1.25,
			cellWidth:  39,
			cellHeight: 26,
			subCols:    3,
			subRows:    2,
		},
		{
			name:       "a multiplier no cell on screen clears",
			fontSize:   20,
			multiplier: 100,
			cellWidth:  900,
			cellHeight: 600,
			subCols:    3,
			subRows:    2,
		},
		{
			name:       "a multiplier below 1, sub-cells under the threshold it sets",
			fontSize:   20,
			multiplier: 0.5,
			cellWidth:  24,
			cellHeight: 16,
			subCols:    3,
			subRows:    2,
		},
		{
			name:       "a multiplier below 1, sub-cells over the threshold it sets",
			fontSize:   20,
			multiplier: 0.5,
			cellWidth:  36,
			cellHeight: 24,
			subCols:    3,
			subRows:    2,
		},
		{
			name:       "a single-column division, where the width is not divided at all",
			fontSize:   10,
			multiplier: 1.5,
			cellWidth:  14,
			cellHeight: 28,
			subCols:    1,
			subRows:    2,
		},
	}
}

// subKeyPreviewDisagreements runs every case through the native rule and through
// the shared Go implementation, and returns the cases they answer differently.
func subKeyPreviewDisagreements(native subKeyPreviewRule) []subKeyPreviewDisagreement {
	var disagreements []subKeyPreviewDisagreement

	for _, testCase := range subKeyPreviewCases() {
		inputs := testCase.inputs()

		nativeAnswer := native.showsPreview(inputs)

		sharedAnswer := inputs.showsPreview()
		if nativeAnswer == sharedAnswer {
			continue
		}

		disagreements = append(disagreements, subKeyPreviewDisagreement{
			testCase: testCase,
			native:   nativeAnswer,
			shared:   sharedAnswer,
		})
	}

	return disagreements
}

// subKeyPreviewMeasure is one value the native rule works out before comparing
// anything: the threshold, and each sub-cell dimension.
type subKeyPreviewMeasure struct {
	name  string
	left  string
	op    string
	right string
}

// String spells the measurement the way its source writes it.
func (measure subKeyPreviewMeasure) String() string {
	return fmt.Sprintf("%s = %s %s %s", measure.name, measure.left, measure.op, measure.right)
}

// value evaluates the measurement against the values bound so far.
func (measure subKeyPreviewMeasure) value(values map[string]float64) float64 {
	return subKeyPreviewArithmetic[measure.op](
		nativeRuleValue(measure.left, values),
		nativeRuleValue(measure.right, values),
	)
}

// subKeyPreviewRule is the Objective-C sub-key-preview autohide rule in the only
// form a Go test can hold it to: something it can run.
//
// Nothing here is an expectation. Every field is read out of the .m file, and the
// shared Go implementation is the only expectation this pin has — which is what
// makes the pin bidirectional: change either side alone and the two stop
// answering alike.
type subKeyPreviewRule struct {
	// guard decides whether autohide applies at all. The multiplier check is a
	// conjunct of the condition that returns early, so it holding is what makes
	// the size check run.
	guard nativeRuleComparison

	// measures are the values the rule works out before comparing, in the order
	// it declares them.
	measures []subKeyPreviewMeasure

	// verdict are the comparisons that decide the answer, joined by verdictJoin
	// ("||" or "&&"). The condition returns out of the method, so it holding is
	// what skips the preview.
	verdict     []nativeRuleComparison
	verdictJoin string
}

// String renders the rule back as one sentence, so a failure shows what was read
// out of the source rather than only that it disagreed.
func (rule subKeyPreviewRule) String() string {
	measures := make([]string, 0, len(rule.measures))
	for _, measure := range rule.measures {
		measures = append(measures, measure.String())
	}

	comparisons := make([]string, 0, len(rule.verdict))
	for _, comparison := range rule.verdict {
		comparisons = append(comparisons, comparison.String())
	}

	return fmt.Sprintf(
		"autohide applies when %s; %s; and the preview is skipped when %s",
		rule.guard,
		strings.Join(measures, ", "),
		strings.Join(comparisons, " "+rule.verdictJoin+" "),
	)
}

// showsPreview answers the question the shared implementation answers: is this
// cell divided finely enough for the sub-key preview to be drawn in it?
func (rule subKeyPreviewRule) showsPreview(inputs subKeyPreviewInputs) bool {
	values := make(map[string]float64, len(subKeyPreviewNativeOperands)+len(rule.measures))
	for name, read := range subKeyPreviewNativeOperands {
		values[name] = read(inputs)
	}

	if !rule.guard.holds(values) {
		return true
	}

	for _, measure := range rule.measures {
		values[measure.name] = measure.value(values)
	}

	return !rule.verdictHolds(values)
}

// verdictHolds folds the deciding comparisons together the way the source joins
// them.
func (rule subKeyPreviewRule) verdictHolds(values map[string]float64) bool {
	all := rule.verdictJoin == "&&"
	held := all

	for _, comparison := range rule.verdict {
		if all {
			held = held && comparison.holds(values)

			continue
		}

		held = held || comparison.holds(values)
	}

	return held
}

// withoutDimension drops one of the compared sub-cell dimensions, standing for
// a rule that stopped measuring it.
func (rule subKeyPreviewRule) withoutDimension(index int) subKeyPreviewRule {
	kept := make([]nativeRuleComparison, 0, len(rule.verdict))

	for position, comparison := range rule.verdict {
		if position != index {
			kept = append(kept, comparison)
		}
	}

	rule.verdict = kept

	return rule
}

// withVerdictOperator rewrites how every sub-cell dimension is compared against
// the threshold.
func (rule subKeyPreviewRule) withVerdictOperator(op string) subKeyPreviewRule {
	rewritten := make([]nativeRuleComparison, 0, len(rule.verdict))

	for _, comparison := range rule.verdict {
		comparison.op = op
		rewritten = append(rewritten, comparison)
	}

	rule.verdict = rewritten

	return rule
}

// withVerdictJoin rewrites how the dimensions are joined, standing for a rule
// that decides on both together rather than on either.
func (rule subKeyPreviewRule) withVerdictJoin(join string) subKeyPreviewRule {
	rule.verdictJoin = join

	return rule
}

// withGuardOperator rewrites how the multiplier decides whether autohide
// applies.
func (rule subKeyPreviewRule) withGuardOperator(op string) subKeyPreviewRule {
	rule.guard.op = op

	return rule
}

// withGuardBound rewrites the value the multiplier is measured against to
// decide whether autohide applies.
func (rule subKeyPreviewRule) withGuardBound(bound string) subKeyPreviewRule {
	rule.guard.right = bound

	return rule
}

// withProductsFromTheirFirstFactor drops the second factor of every product the
// rule measures, standing for a threshold that stopped applying the multiplier.
func (rule subKeyPreviewRule) withProductsFromTheirFirstFactor() subKeyPreviewRule {
	return rule.withMeasuresRewritten("*", func(measure subKeyPreviewMeasure) subKeyPreviewMeasure {
		measure.right = "1"

		return measure
	})
}

// withQuotientsUndivided drops the divisor of every quotient the rule measures,
// standing for a rule that measures the whole cell rather than a sub-cell.
func (rule subKeyPreviewRule) withQuotientsUndivided() subKeyPreviewRule {
	return rule.withMeasuresRewritten("/", func(measure subKeyPreviewMeasure) subKeyPreviewMeasure {
		measure.right = "1"

		return measure
	})
}

// withQuotientDivisorsSwapped exchanges the divisors of the two quotients,
// standing for a rule that divides the width by the row count and the height by
// the column count.
func (rule subKeyPreviewRule) withQuotientDivisorsSwapped() subKeyPreviewRule {
	divisors := make([]string, 0, len(rule.measures))

	for _, measure := range rule.measures {
		if measure.op == "/" {
			divisors = append(divisors, measure.right)
		}
	}

	if len(divisors) != 2 {
		return rule
	}

	taken := 0

	return rule.withMeasuresRewritten("/", func(measure subKeyPreviewMeasure) subKeyPreviewMeasure {
		measure.right = divisors[len(divisors)-1-taken]
		taken++

		return measure
	})
}

// withMeasuresRewritten applies a rewrite to every measurement built with the
// given operator.
func (rule subKeyPreviewRule) withMeasuresRewritten(
	operator string,
	rewrite func(subKeyPreviewMeasure) subKeyPreviewMeasure,
) subKeyPreviewRule {
	rewritten := make([]subKeyPreviewMeasure, 0, len(rule.measures))

	for _, measure := range rule.measures {
		if measure.op == operator {
			measure = rewrite(measure)
		}

		rewritten = append(rewritten, measure)
	}

	rule.measures = rewritten

	return rule
}

// validateSubKeyPreviewRule checks that every name and operator the rule was
// written with is one this pin can evaluate. It is what stops a rewritten rule
// from being run with a value invented for it.
func validateSubKeyPreviewRule(rule subKeyPreviewRule) string {
	measured := make(map[string]bool, len(rule.measures))

	for _, measure := range rule.measures {
		if _, known := subKeyPreviewArithmetic[measure.op]; !known {
			return fmt.Sprintf(
				"`%s` is built with %q, which this pin does not read",
				measure,
				measure.op,
			)
		}

		for _, token := range []string{measure.left, measure.right} {
			if problem := subKeyPreviewToken(token, measured); problem != "" {
				return problem
			}
		}

		measured[measure.name] = true
	}

	// The guard is evaluated before anything is measured, which is also true of
	// the source it is read from. A guard naming a measured value would be valued
	// at nothing rather than reported.
	for _, token := range []string{rule.guard.left, rule.guard.right} {
		if measured[token] {
			return fmt.Sprintf(
				"the guard `%s` reads %s, which the rule only works out afterwards",
				rule.guard, token,
			)
		}
	}

	for _, comparison := range append([]nativeRuleComparison{rule.guard}, rule.verdict...) {
		if _, known := nativeRuleComparators[comparison.op]; !known {
			return fmt.Sprintf(
				"`%s` compares with %q, which this pin does not read",
				comparison,
				comparison.op,
			)
		}

		for _, token := range []string{comparison.left, comparison.right} {
			if problem := subKeyPreviewToken(token, measured); problem != "" {
				return problem
			}
		}
	}

	return ""
}

// subKeyPreviewToken reports why a token cannot be valued, and nothing when it
// can.
func subKeyPreviewToken(token string, measured map[string]bool) string {
	if _, bound := subKeyPreviewNativeOperands[token]; bound {
		return ""
	}

	if measured[token] {
		return ""
	}

	_, parseErr := strconv.ParseFloat(token, 64)
	if parseErr == nil {
		return ""
	}

	return fmt.Sprintf(
		"the rule reads %s, which this pin cannot value; it knows %s, the values the rule measures for itself, and numeric literals",
		token,
		strings.Join(sortedNativeRuleOperands(subKeyPreviewNativeOperands), ", "),
	)
}

// nativeSubKeyPreviewMethodPattern matches the opening line of the
// drawSubKeyPreviewInCellRect: definition that carries the rule. The
// declaration in the @interface is excluded by refusing to cross a `;`.
var nativeSubKeyPreviewMethodPattern = regexp.MustCompile(
	`(?m)^- \(void\)drawSubKeyPreviewInCellRect:[^{};]*\{`,
)

// nativeSubKeyPreviewGuardPattern matches the whole guard: the multiplier check
// and the condition it is folded into, which skips the preview.
var nativeSubKeyPreviewGuardPattern = regexp.MustCompile(
	`if[ \t]*\([ \t]*([\w.]+)[ \t]*` + nativeRuleComparisonOperators +
		`[ \t]*([-\w.]+)[ \t]*&&[ \t]*\(([^()]+)\)[ \t]*\)\s*return[ \t]*;`,
)

// nativeSubKeyPreviewMeasurePattern matches one `CGFloat name = a * b;` the
// rule works out before comparing anything.
var nativeSubKeyPreviewMeasurePattern = regexp.MustCompile(
	`(?m)^[ \t]*CGFloat[ \t]+(\w+)[ \t]*=[ \t]*([\w.]+)[ \t]*([*/])[ \t]*([\w.]+)[ \t]*;`,
)

// readNativeSubKeyPreviewRule reads the rule out of the macOS overlay, failing
// the test when it cannot — a rule this pin cannot read is a rule it cannot hold
// to the shared one, and passing quietly there would be worse than having no pin
// at all.
func readNativeSubKeyPreviewRule(t *testing.T) subKeyPreviewRule {
	t.Helper()

	rule, problem := parseNativeSubKeyPreviewRule(
		readNativeSource(t, subKeyPreviewNativeSource),
	)
	if problem != "" {
		t.Fatalf(
			"%s: %s\n\tuntil this reads again, nothing holds it to %s",
			subKeyPreviewNativeSource, problem, subKeyPreviewGoDeclaration,
		)
	}

	return rule
}

// parseNativeSubKeyPreviewRule reads the autohide rule out of an Objective-C
// source. The second result describes why the rule could not be read, and is
// empty when it could — an error value would buy nothing here, since the only
// caller turns it straight into a test failure.
func parseNativeSubKeyPreviewRule(source string) (subKeyPreviewRule, string) {
	body, problem := nativeRuleMethodBody(
		source,
		nativeSubKeyPreviewMethodPattern,
		subKeyPreviewNativeMethod,
		"- (void)drawSubKeyPreviewInCellRect:... {",
	)
	if problem != "" {
		return subKeyPreviewRule{}, problem
	}

	guards := nativeSubKeyPreviewGuardPattern.FindAllStringSubmatchIndex(body, -1)

	if len(guards) != 1 {
		return subKeyPreviewRule{}, fmt.Sprintf(
			"%s holds %d autohide guards shaped `if (<multiplier> <op> <literal> && (<comparisons>)) return;`, want exactly 1 (rewritten?)",
			subKeyPreviewNativeMethod,
			len(guards),
		)
	}

	guard := guards[0]
	capture := func(group int) string {
		return body[guard[2*group]:guard[2*group+1]]
	}

	rule := subKeyPreviewRule{
		guard: nativeRuleComparison{
			left:  capture(1),
			op:    capture(2),
			right: capture(3),
		},
	}

	// Only what the rule works out before the guard can be read by it.
	rule.measures = nativeSubKeyPreviewMeasures(body[:guard[0]])

	if len(rule.measures) == 0 {
		return subKeyPreviewRule{}, subKeyPreviewNativeMethod + " works out nothing shaped `CGFloat <name> = <a> <*|/> <b>;` before its autohide guard (rewritten?)"
	}

	rule.verdict, rule.verdictJoin, problem = parseNativeRuleCondition(capture(4))
	if problem != "" {
		return subKeyPreviewRule{}, problem
	}

	return rule, validateSubKeyPreviewRule(rule)
}

// nativeSubKeyPreviewMeasures reads the values the rule works out before its
// guard, in declaration order.
func nativeSubKeyPreviewMeasures(body string) []subKeyPreviewMeasure {
	matched := nativeSubKeyPreviewMeasurePattern.FindAllStringSubmatch(body, -1)
	measures := make([]subKeyPreviewMeasure, 0, len(matched))

	for _, match := range matched {
		measures = append(measures, subKeyPreviewMeasure{
			name:  match[1],
			left:  match[2],
			op:    match[3],
			right: match[4],
		})
	}

	return measures
}

// nativeSubKeyPreviewMethodSource wraps a body in the method definition the
// Objective-C parser looks for, so a test can hand it a rule shaped differently
// from the one in the tree.
func nativeSubKeyPreviewMethodSource(body string) string {
	return "- (void)drawSubKeyPreviewInCellRect:(NSRect)cellRect {\n" + body + "\n}\n"
}
