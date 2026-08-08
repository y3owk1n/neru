package architecture_test

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"image"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/recursivegrid"
)

// The two sides of the recursive-grid sub-key-preview autohide rule.
const (
	subKeyPreviewNativeSource = "internal/adapter/platform/darwin/overlay_darwin.m"

	// subKeyPreviewNativeMethod is the Objective-C method the rule lives in,
	// named in failure messages so a reader lands on the copy rather than on
	// this test.
	subKeyPreviewNativeMethod = "drawSubKeyPreviewInCellRect:"

	// subKeyPreviewGoSource and subKeyPreviewGoDeclaration name the Go side the
	// same way.
	subKeyPreviewGoSource      = "internal/adapter/overlay/linux/cgo_helpers.go"
	subKeyPreviewGoDeclaration = "shouldShowSubKeyPreview"

	// subKeyPreviewGoEnableOperand is the check the Go rule opens with, and the
	// one part of it this pin holds constant rather than comparing: whether the
	// preview is switched on at all. The Objective-C method has no counterpart
	// inside it, because its caller gates on self.gridDrawSubKeyPreview before
	// calling it. Every case here is asked with the preview enabled, which is
	// the only state in which the two rules are answering the same question.
	subKeyPreviewGoEnableOperand = "style.SubKeyPreview()"
)

// TestSubKeyPreviewAutohideRuleIsPinnedAcrossTheLanguageBoundary keeps the
// macOS overlay hiding the same sub-key previews as the Cairo one.
//
// Whether a recursive-grid cell is divided finely enough for the mini-grid of
// the next level's keys to be worth drawing is one question with one answer:
// every sub-cell must reach sub_key_preview_autohide_multiplier x the preview
// font size, and a non-positive multiplier means "always draw". It is written
// twice — in Go for the Cairo backend and in Objective-C for macOS — and
// nothing held the two together. It is the third copy standing on ADR 0007's
// deliberate exception to the one-implementation rule
// (docs/adr/0007-a-shared-derivation-has-one-implementation.md), and the second
// that is a rule rather than a vocabulary: where the second implementation is
// in another language, what the rule asks for is a test holding the copies
// together rather than a deletion.
//
// Both copies are rules rather than constants, so both are pinned by running
// them. Each is read out of its own source into something this test can
// evaluate, and the two are asked the same questions over cases that straddle
// every edge the rule has: the multiplier that disables it, the threshold
// itself, each sub-cell dimension one pixel under it, and a cell that clears
// the threshold whole while its sub-cells do not. A disagreement is one
// configuration drawing a preview on macOS and leaving the cell bare on Linux.
//
// The Go side is read from source rather than called, which the label-autohide
// pin next door does not have to do. Its rule is a method on a shared style;
// this one sits behind `//go:build linux && cgo`, so a test running on the
// macOS host cannot link it. Giving it an untagged home is #1297's work — that
// issue converges the Windows backend, which deliberately measures the whole
// cell rather than a sub-cell (docs/CROSS_PLATFORM.md records the difference),
// and only then is there one predicate to share. Reading it here holds the two
// copies together in the meantime, and holds them in both directions: change
// either side alone and the two stop answering alike.
func TestSubKeyPreviewAutohideRuleIsPinnedAcrossTheLanguageBoundary(t *testing.T) {
	t.Parallel()

	native := readNativeSubKeyPreviewRule(t)
	shared := readGoSubKeyPreviewRule(t)

	for _, disagreement := range subKeyPreviewDisagreements(native, shared) {
		t.Errorf(
			"%s: %s and %s disagree on %s\n\t%s draws the preview: %t\n\t%s draws it: %t\n\tthe rule read from %s is: %s\n\tthe rule read from %s is: %s",
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
			subKeyPreviewGoSource,
			shared,
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
// has to disagree with the Go one somewhere. Mutating the rule rather than the
// source text keeps this honest across a reformat of the .m file, and keeps "we
// broke it by hand once and watched it fail" from being the only evidence the
// guardrail has teeth.
func TestSubKeyPreviewAutohideRulePinCatchesNativeDrift(t *testing.T) {
	t.Parallel()

	native := readNativeSubKeyPreviewRule(t)
	shared := readGoSubKeyPreviewRule(t)

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

		if len(subKeyPreviewDisagreements(mutant, shared)) == 0 {
			t.Errorf(
				"no case tells %s apart from %s: %s would pass the pin\n\tthe drifted rule is: %s",
				drift.name, subKeyPreviewGoDeclaration, subKeyPreviewNativeSource, mutant,
			)
		}
	}
}

// TestSubKeyPreviewAutohideRulePinReportsARuleItCannotRead pins the other half
// of the guardrail: a rule this pin cannot read must be reported, never
// skipped. A pin that reads nothing and passes is worse than no pin, because it
// reads as coverage.
//
// This is where the pin's one deliberate cost sits. It reads one shape per
// side, and a rewrite that keeps the behavior — unfolding the Objective-C
// guard into the nested form drawGridLabel: writes it, say — fails here rather
// than being understood. Teaching it every equivalent spelling of the same rule
// is more machinery than the copy is worth; failing loudly and naming the shape
// it expected leaves the next author a one-line change to this file, which the
// same author is already making to the source it reads.
func TestSubKeyPreviewAutohideRulePinReportsARuleItCannotRead(t *testing.T) {
	t.Parallel()

	unreadableNative := []struct {
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

	for _, source := range unreadableNative {
		if _, problem := parseNativeSubKeyPreviewRule(source.source); problem == "" {
			t.Errorf(
				"parsing accepted an Objective-C source with %s; the pin would then run a rule it never read",
				source.name,
			)
		}
	}

	unreadableGo := []struct {
		name   string
		source string
	}{
		{
			name:   "the function renamed",
			source: goSubKeyPreviewFuncSource("shouldDrawSubKeyPreview", "\treturn true\n"),
		},
		{
			name: "the enable check gone, so the pin cannot tell it is holding one constant",
			source: goSubKeyPreviewFuncSource(subKeyPreviewGoDeclaration,
				"\tif style.SubKeyPreviewAutohideMultiplier() <= 0 {\n"+
					"\t\treturn true\n"+
					"\t}\n\n"+
					"\tthreshold := style.SubKeyPreviewFontSizeF() * style.SubKeyPreviewAutohideMultiplier()\n"+
					"\tsubCellW := float64(cell.Dx()) / float64(subDims.Cols)\n"+
					"\tsubCellH := float64(cell.Dy()) / float64(subDims.Rows)\n\n"+
					"\treturn subCellW >= threshold && subCellH >= threshold\n"),
		},
		{
			name: "the guard folded into the returned expression",
			source: goSubKeyPreviewFuncSource(subKeyPreviewGoDeclaration,
				"\tif !style.SubKeyPreview() {\n"+
					"\t\treturn false\n"+
					"\t}\n\n"+
					"\tthreshold := style.SubKeyPreviewFontSizeF() * style.SubKeyPreviewAutohideMultiplier()\n"+
					"\tsubCellW := float64(cell.Dx()) / float64(subDims.Cols)\n"+
					"\tsubCellH := float64(cell.Dy()) / float64(subDims.Rows)\n\n"+
					"\treturn style.SubKeyPreviewAutohideMultiplier() <= 0 ||\n"+
					"\t\t(subCellW >= threshold && subCellH >= threshold)\n"),
		},
		{
			name: "a sub-cell measured from an operand this pin cannot value",
			source: goSubKeyPreviewFuncSource(subKeyPreviewGoDeclaration,
				"\tif !style.SubKeyPreview() {\n"+
					"\t\treturn false\n"+
					"\t}\n\n"+
					"\tif style.SubKeyPreviewAutohideMultiplier() <= 0 {\n"+
					"\t\treturn true\n"+
					"\t}\n\n"+
					"\tthreshold := style.LabelFontSize() * style.SubKeyPreviewAutohideMultiplier()\n"+
					"\tsubCellW := float64(cell.Dx()) / float64(subDims.Cols)\n"+
					"\tsubCellH := float64(cell.Dy()) / float64(subDims.Rows)\n\n"+
					"\treturn subCellW >= threshold && subCellH >= threshold\n"),
		},
	}

	for _, source := range unreadableGo {
		if _, problem := parseGoSubKeyPreviewRule(source.source); problem == "" {
			t.Errorf(
				"parsing accepted a Go source with %s; the pin would then run a rule it never read",
				source.name,
			)
		}
	}
}

// subKeyPreviewInputs are the values both rules are functions of.
//
// The two sides name them differently — cellRect.size.width against cell.Dx(),
// self.gridSubKeyAutohideMultiplier against a method on the style — so each
// side brings its own vocabulary binding its names to these.
type subKeyPreviewInputs struct {
	fontSize   int
	multiplier float64
	cell       image.Rectangle
	subCols    int
	subRows    int
}

// style builds the resolved style the Go rule reads its inputs through, so the
// pin values style.SubKeyPreviewFontSizeF() by calling it rather than by
// assuming what it returns.
func (inputs subKeyPreviewInputs) style() recursivegrid.Style {
	return recursivegrid.NewStyle(recursivegrid.StyleOptions{
		SubKeyPreview:                   true,
		SubKeyPreviewFontSize:           inputs.fontSize,
		SubKeyPreviewAutohideMultiplier: inputs.multiplier,
	})
}

// subKeyPreviewGoOperands binds every name the Go rule is allowed to mention to
// the input it stands for. It is that side's vocabulary: a rule reading
// anything else is one this test cannot evaluate, and parsing says so rather
// than guessing a value.
var subKeyPreviewGoOperands = map[string]func(subKeyPreviewInputs) float64{
	"style.SubKeyPreviewAutohideMultiplier()": func(inputs subKeyPreviewInputs) float64 {
		return inputs.style().SubKeyPreviewAutohideMultiplier()
	},
	"style.SubKeyPreviewFontSizeF()": func(inputs subKeyPreviewInputs) float64 {
		return inputs.style().SubKeyPreviewFontSizeF()
	},
	"cell.Dx()":    func(inputs subKeyPreviewInputs) float64 { return float64(inputs.cell.Dx()) },
	"cell.Dy()":    func(inputs subKeyPreviewInputs) float64 { return float64(inputs.cell.Dy()) },
	"subDims.Cols": func(inputs subKeyPreviewInputs) float64 { return float64(inputs.subCols) },
	"subDims.Rows": func(inputs subKeyPreviewInputs) float64 { return float64(inputs.subRows) },
}

// subKeyPreviewNativeOperands is the same vocabulary for the Objective-C side.
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
// Both rules build every one of theirs from exactly two operands, so nothing
// here needs precedence.
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
// sub-cells do not is asked, because that is the shape the Windows backend
// deliberately has and the one this copy must not drift into; the division is
// asked with unequal columns and rows, because a rule dividing the width by the
// row count agrees on every square division; and a multiplier below 1 is asked,
// because a guard that admitted only multipliers above 1 agrees everywhere
// else. TestSubKeyPreviewAutohideRulePinCatchesNativeDrift is what keeps this
// list honest.
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

// subKeyPreviewDisagreements runs every case through both rules and returns the
// cases they answer differently.
func subKeyPreviewDisagreements(native, shared subKeyPreviewRule) []subKeyPreviewDisagreement {
	var disagreements []subKeyPreviewDisagreement

	for _, testCase := range subKeyPreviewCases() {
		inputs := testCase.inputs()

		nativeAnswer := native.showsPreview(inputs)

		sharedAnswer := shared.showsPreview(inputs)
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

// subKeyPreviewMeasure is one value a rule works out before comparing anything:
// the threshold, and each sub-cell dimension.
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

// subKeyPreviewRule is one implementation of the sub-key-preview autohide rule
// in the only form a Go test can hold another to: something it can run.
//
// Nothing here is an expectation. Every field is read out of a source file, and
// each rule's only expectation is the other one — which is what makes the pin
// bidirectional: change either side alone and the two stop answering alike.
type subKeyPreviewRule struct {
	// operands is the vocabulary this side is written in.
	operands map[string]func(subKeyPreviewInputs) float64

	// guard decides whether autohide applies at all. guardShows says which way
	// round its source writes it: the Go rule returns early when the guard holds
	// (so holding means the preview is drawn whatever the size), the Objective-C
	// one folds the guard into the condition that skips the preview (so holding
	// means the size check runs).
	guard      nativeRuleComparison
	guardShows bool

	// measures are the values the rule works out before comparing, in the order
	// it declares them.
	measures []subKeyPreviewMeasure

	// verdict are the comparisons that decide the answer, joined by verdictJoin
	// ("||" or "&&"). verdictShows says what the joined condition holding means:
	// the Go rule returns it as the answer, the Objective-C one returns early
	// when it holds.
	verdict      []nativeRuleComparison
	verdictJoin  string
	verdictShows bool
}

// String renders the rule back as one sentence, so a failure shows what was
// read out of the source rather than only that it disagreed.
func (rule subKeyPreviewRule) String() string {
	guardClause := "autohide applies when %s"
	if rule.guardShows {
		guardClause = "autohide is off when %s"
	}

	verdictClause := "the preview is skipped when %s"
	if rule.verdictShows {
		verdictClause = "the preview is drawn when %s"
	}

	measures := make([]string, 0, len(rule.measures))
	for _, measure := range rule.measures {
		measures = append(measures, measure.String())
	}

	comparisons := make([]string, 0, len(rule.verdict))
	for _, comparison := range rule.verdict {
		comparisons = append(comparisons, comparison.String())
	}

	return fmt.Sprintf(
		guardClause+"; %s; and "+verdictClause,
		rule.guard,
		strings.Join(measures, ", "),
		strings.Join(comparisons, " "+rule.verdictJoin+" "),
	)
}

// showsPreview answers the question both implementations answer: is this cell
// divided finely enough for the sub-key preview to be drawn in it?
func (rule subKeyPreviewRule) showsPreview(inputs subKeyPreviewInputs) bool {
	values := make(map[string]float64, len(rule.operands)+len(rule.measures))
	for name, read := range rule.operands {
		values[name] = read(inputs)
	}

	if rule.guard.holds(values) == rule.guardShows {
		return true
	}

	for _, measure := range rule.measures {
		values[measure.name] = measure.value(values)
	}

	return rule.verdictHolds(values) == rule.verdictShows
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
			if problem := subKeyPreviewToken(token, rule.operands, measured); problem != "" {
				return problem
			}
		}

		measured[measure.name] = true
	}

	// The guard is evaluated before anything is measured, which is also true of
	// both sources it is read from. A guard naming a measured value would be
	// valued at nothing rather than reported.
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
			if problem := subKeyPreviewToken(token, rule.operands, measured); problem != "" {
				return problem
			}
		}
	}

	return ""
}

// subKeyPreviewToken reports why a token cannot be valued, and nothing when it
// can.
func subKeyPreviewToken(
	token string,
	operands map[string]func(subKeyPreviewInputs) float64,
	measured map[string]bool,
) string {
	if _, bound := operands[token]; bound {
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
		strings.Join(sortedNativeRuleOperands(operands), ", "),
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
// the test when it cannot — a rule this pin cannot read is a rule it cannot
// hold to the other one, and passing quietly there would be worse than having
// no pin at all.
func readNativeSubKeyPreviewRule(t *testing.T) subKeyPreviewRule {
	t.Helper()

	rule, problem := parseNativeSubKeyPreviewRule(
		readNativeSource(t, subKeyPreviewNativeSource),
	)
	if problem != "" {
		t.Fatalf(
			"%s: %s\n\tuntil this reads again, nothing holds it to %s in %s",
			subKeyPreviewNativeSource, problem,
			subKeyPreviewGoDeclaration, subKeyPreviewGoSource,
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
		operands: subKeyPreviewNativeOperands,
		guard: nativeRuleComparison{
			left:  capture(1),
			op:    capture(2),
			right: capture(3),
		},
		// The multiplier check is a conjunct of the condition that returns
		// early, so it holding is what makes the size check run.
		guardShows: false,
		// The condition returns out of the method, so it holding is what skips
		// the preview.
		verdictShows: false,
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

// readGoSubKeyPreviewRule reads the rule out of the Cairo backend, failing the
// test when it cannot, for the same reason the Objective-C reader does.
func readGoSubKeyPreviewRule(t *testing.T) subKeyPreviewRule {
	t.Helper()

	rule, problem := parseGoSubKeyPreviewRule(readNativeSource(t, subKeyPreviewGoSource))
	if problem != "" {
		t.Fatalf(
			"%s: %s\n\tuntil this reads again, nothing holds %s in %s to it",
			subKeyPreviewGoSource, problem,
			subKeyPreviewNativeMethod, subKeyPreviewNativeSource,
		)
	}

	return rule
}

// parseGoSubKeyPreviewRule reads the autohide rule out of a Go source. Go's own
// parser does the reading, so this only has to recognize the statements the
// rule is written with and refuse everything else.
func parseGoSubKeyPreviewRule(source string) (subKeyPreviewRule, string) {
	fileSet := token.NewFileSet()

	parsed, err := parser.ParseFile(fileSet, "cgo_helpers.go", source, 0)
	if err != nil {
		return subKeyPreviewRule{}, fmt.Sprintf("cannot be parsed as Go: %v", err)
	}

	body := goSubKeyPreviewFuncBody(parsed)
	if body == nil {
		return subKeyPreviewRule{}, fmt.Sprintf(
			"no `func %s` to read the autohide rule from (renamed?)",
			subKeyPreviewGoDeclaration,
		)
	}

	rule := subKeyPreviewRule{
		operands: subKeyPreviewGoOperands,
		// The guard returns out of the function with the preview drawn, so it
		// holding is what switches autohide off.
		guardShows: true,
		// The deciding comparisons are what the function returns, so them
		// holding is what draws the preview.
		verdictShows: true,
	}

	var read goSubKeyPreviewParts

	for _, statement := range body.List {
		var problem string

		switch typed := statement.(type) {
		case *ast.IfStmt:
			problem = goSubKeyPreviewGuard(fileSet, typed, &rule, &read)
		case *ast.AssignStmt:
			problem = goSubKeyPreviewMeasure(fileSet, typed, &rule)
		case *ast.ReturnStmt:
			problem = goSubKeyPreviewVerdict(fileSet, typed, &rule)
			read.answer = true
		default:
			problem = subKeyPreviewGoDeclaration + " is written with a statement this pin does not read"
		}

		if problem != "" {
			return subKeyPreviewRule{}, problem
		}
	}

	if problem := read.missing(rule); problem != "" {
		return subKeyPreviewRule{}, problem
	}

	return rule, validateSubKeyPreviewRule(rule)
}

// goSubKeyPreviewParts records which statements of the rule the parser has
// found, so the reader can say which one is missing rather than run a rule with
// a hole in it.
type goSubKeyPreviewParts struct {
	// enable is the check that the preview is switched on, guard the check that
	// autohide applies, and answer the comparison the function returns.
	enable bool
	guard  bool
	answer bool
}

// missing reports which part of the rule was never found, and nothing when all
// of them were.
func (read goSubKeyPreviewParts) missing(rule subKeyPreviewRule) string {
	switch {
	case !read.enable:
		return fmt.Sprintf(
			"%s no longer opens with `if !%s { return false }` (rewritten?)",
			subKeyPreviewGoDeclaration, subKeyPreviewGoEnableOperand,
		)
	case !read.guard:
		return subKeyPreviewGoDeclaration + " holds no autohide guard shaped `if <multiplier> <op> <literal> { return true }` (rewritten?)"
	case len(rule.measures) == 0:
		return subKeyPreviewGoDeclaration + " works out nothing shaped `<name> := <a> <*|/> <b>` before deciding (rewritten?)"
	case !read.answer:
		return subKeyPreviewGoDeclaration + " returns nothing (rewritten?)"
	}

	return ""
}

// goSubKeyPreviewFuncBody returns the body of the shouldShowSubKeyPreview
// declaration, or nil when the file does not declare it.
func goSubKeyPreviewFuncBody(file *ast.File) *ast.BlockStmt {
	for _, decl := range file.Decls {
		funcDecl, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || funcDecl.Name.Name != subKeyPreviewGoDeclaration {
			continue
		}

		return funcDecl.Body
	}

	return nil
}

// goSubKeyPreviewGuard reads one of the two early returns the rule opens with:
// the check that the preview is switched on, and the check that autohide
// applies. Which one it is read from the value returned, not assumed.
func goSubKeyPreviewGuard(
	fileSet *token.FileSet,
	statement *ast.IfStmt,
	rule *subKeyPreviewRule,
	read *goSubKeyPreviewParts,
) string {
	answer, isEarlyReturn := goSubKeyPreviewEarlyReturn(statement)
	if !isEarlyReturn {
		return fmt.Sprintf(
			"an `if` in %s does not return a plain true or false, which this pin does not read",
			subKeyPreviewGoDeclaration,
		)
	}

	if !answer {
		negated, isNegation := statement.Cond.(*ast.UnaryExpr)
		if !isNegation || negated.Op != token.NOT ||
			goSubKeyPreviewOperandName(fileSet, negated.X) != subKeyPreviewGoEnableOperand {
			return fmt.Sprintf(
				"%s returns false on something other than `!%s`, so this pin cannot tell which check it is holding constant",
				subKeyPreviewGoDeclaration,
				subKeyPreviewGoEnableOperand,
			)
		}

		read.enable = true

		return ""
	}

	comparison, problem := goSubKeyPreviewComparison(fileSet, statement.Cond)
	if problem != "" {
		return problem
	}

	rule.guard = comparison
	read.guard = true

	return ""
}

// goSubKeyPreviewEarlyReturn reports the boolean an `if` returns, and whether
// its body is a single such return at all.
func goSubKeyPreviewEarlyReturn(statement *ast.IfStmt) (bool, bool) {
	if statement.Else != nil || statement.Init != nil || len(statement.Body.List) != 1 {
		return false, false
	}

	returned, isReturn := statement.Body.List[0].(*ast.ReturnStmt)
	if !isReturn || len(returned.Results) != 1 {
		return false, false
	}

	answer, isIdent := returned.Results[0].(*ast.Ident)
	if !isIdent || (answer.Name != "true" && answer.Name != "false") {
		return false, false
	}

	return answer.Name == "true", true
}

// goSubKeyPreviewMeasure reads one `name := a * b` the rule works out before
// deciding.
func goSubKeyPreviewMeasure(
	fileSet *token.FileSet,
	statement *ast.AssignStmt,
	rule *subKeyPreviewRule,
) string {
	if statement.Tok != token.DEFINE || len(statement.Lhs) != 1 || len(statement.Rhs) != 1 {
		return fmt.Sprintf(
			"an assignment in %s is not the single `<name> := <expression>` this pin reads",
			subKeyPreviewGoDeclaration,
		)
	}

	name, isIdent := statement.Lhs[0].(*ast.Ident)
	if !isIdent {
		return fmt.Sprintf(
			"an assignment in %s does not name a single value",
			subKeyPreviewGoDeclaration,
		)
	}

	binary, isBinary := statement.Rhs[0].(*ast.BinaryExpr)
	if !isBinary {
		return name.Name + " is worked out from something other than two operands, which this pin does not read"
	}

	rule.measures = append(rule.measures, subKeyPreviewMeasure{
		name:  name.Name,
		left:  goSubKeyPreviewOperandName(fileSet, binary.X),
		op:    binary.Op.String(),
		right: goSubKeyPreviewOperandName(fileSet, binary.Y),
	})

	return ""
}

// goSubKeyPreviewVerdict reads the comparisons the rule returns and the
// operator joining them.
func goSubKeyPreviewVerdict(
	fileSet *token.FileSet,
	statement *ast.ReturnStmt,
	rule *subKeyPreviewRule,
) string {
	if len(statement.Results) != 1 {
		return subKeyPreviewGoDeclaration + " returns something other than one expression"
	}

	joined, isBinary := statement.Results[0].(*ast.BinaryExpr)
	if !isBinary {
		return subKeyPreviewGoDeclaration + " returns an expression this pin does not read"
	}

	if joined.Op != token.LAND && joined.Op != token.LOR {
		comparison, problem := goSubKeyPreviewComparison(fileSet, joined)
		if problem != "" {
			return problem
		}

		rule.verdict = []nativeRuleComparison{comparison}
		rule.verdictJoin = "&&"

		return ""
	}

	rule.verdictJoin = joined.Op.String()

	return goSubKeyPreviewJoined(fileSet, joined, joined.Op, rule)
}

// goSubKeyPreviewJoined flattens a chain of comparisons joined by one operator,
// refusing a chain that mixes two.
func goSubKeyPreviewJoined(
	fileSet *token.FileSet,
	expr ast.Expr,
	join token.Token,
	rule *subKeyPreviewRule,
) string {
	binary, isBinary := expr.(*ast.BinaryExpr)
	if isBinary && (binary.Op == token.LAND || binary.Op == token.LOR) {
		if binary.Op != join {
			return subKeyPreviewGoDeclaration + " mixes && and ||, which this pin does not read"
		}

		if problem := goSubKeyPreviewJoined(fileSet, binary.X, join, rule); problem != "" {
			return problem
		}

		return goSubKeyPreviewJoined(fileSet, binary.Y, join, rule)
	}

	comparison, problem := goSubKeyPreviewComparison(fileSet, expr)
	if problem != "" {
		return problem
	}

	rule.verdict = append(rule.verdict, comparison)

	return ""
}

// goSubKeyPreviewComparison reads one `a >= b` out of the syntax tree.
func goSubKeyPreviewComparison(
	fileSet *token.FileSet,
	expr ast.Expr,
) (nativeRuleComparison, string) {
	binary, isBinary := expr.(*ast.BinaryExpr)
	if !isBinary {
		return nativeRuleComparison{}, fmt.Sprintf(
			"`%s` in %s is not a comparison this pin reads",
			goSubKeyPreviewSource(fileSet, expr), subKeyPreviewGoDeclaration,
		)
	}

	return nativeRuleComparison{
		left:  goSubKeyPreviewOperandName(fileSet, binary.X),
		op:    binary.Op.String(),
		right: goSubKeyPreviewOperandName(fileSet, binary.Y),
	}, ""
}

// goSubKeyPreviewOperandName spells an operand the way the vocabulary names it,
// looking through the parentheses and float64 conversions the Go rule needs and
// the Objective-C one does not.
func goSubKeyPreviewOperandName(fileSet *token.FileSet, expr ast.Expr) string {
	for {
		switch typed := expr.(type) {
		case *ast.ParenExpr:
			expr = typed.X
		case *ast.CallExpr:
			conversion, isIdent := typed.Fun.(*ast.Ident)
			if !isIdent || conversion.Name != "float64" || len(typed.Args) != 1 {
				return goSubKeyPreviewSource(fileSet, expr)
			}

			expr = typed.Args[0]
		default:
			return goSubKeyPreviewSource(fileSet, expr)
		}
	}
}

// goSubKeyPreviewSource renders an expression back as the source wrote it.
func goSubKeyPreviewSource(fileSet *token.FileSet, expr ast.Expr) string {
	var rendered bytes.Buffer

	err := printer.Fprint(&rendered, fileSet, expr)
	if err != nil {
		return fmt.Sprintf("an expression this pin cannot render: %v", err)
	}

	return rendered.String()
}

// nativeSubKeyPreviewMethodSource wraps a body in the method definition the
// Objective-C parser looks for, so a test can hand it a rule shaped differently
// from the one in the tree.
func nativeSubKeyPreviewMethodSource(body string) string {
	return "- (void)drawSubKeyPreviewInCellRect:(NSRect)cellRect {\n" + body + "\n}\n"
}

// goSubKeyPreviewFuncSource wraps a body in the function declaration the Go
// parser looks for, for the same reason.
func goSubKeyPreviewFuncSource(name, body string) string {
	return "package linux\n\nfunc " + name + "(\n" +
		"\tcell image.Rectangle,\n" +
		"\tstyle recursivegrid.Style,\n" +
		"\tsubDims domain.GridDimensions,\n" +
		") bool {\n" + body + "}\n"
}
