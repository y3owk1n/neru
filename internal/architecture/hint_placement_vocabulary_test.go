package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// The three native spellings of one hint placement. The C header is what Go
// reads through cgo, so the compiler keeps the macro *names* honest; nothing
// keeps the Objective-C enum the drawing code switches on, and nothing keeps
// the macro each Go placement is translated to.
const (
	hintPlacementHeader      = "internal/adapter/platform/darwin/overlay.h"
	hintPlacementSource      = "internal/adapter/platform/darwin/overlay_darwin.m"
	hintPlacementTranslator  = "internal/adapter/overlay/render/hints/overlay_darwin.go"
	hintPlacementEnum        = "HintPlacement"
	hintPlacementMacroPrefix = "HINT_PLACEMENT_"

	// hintPlacementFunc is the function in hintPlacementTranslator that turns a
	// configured placement into the constant handed to the overlay.
	hintPlacementFunc = "hintPlacementValue"

	// hintPlacementDeclaration names the Go side in failure messages, so a
	// reader is sent to the declaration rather than to this test.
	hintPlacementDeclaration = "config.HintPlacements()"
)

// TestHintPlacementVocabularyIsPinnedAcrossTheLanguageBoundary keeps a
// placement that validates a placement that draws.
//
// `hints.ui.placement` is declared once in Go (config.HintPlacements), and the
// default, the validator, the macOS renderer and the Linux and Windows
// overlays all read that declaration. The values then cross into Objective-C,
// where the header macros Go reads and the enum overlay_darwin.m switches on
// were held together by a comment saying they must match. This is ADR 0007's
// deliberate exception to the one-implementation rule: Go cannot be the
// implementation of an Objective-C enum, so the copies get a pin instead of a
// deletion (docs/adr/0007-a-shared-derivation-has-one-implementation.md).
//
// It fails on three shapes: a Go placement with no native constant, a header
// macro and an enum member that disagree on a number, and a native constant
// nothing in Go names. The last one is what catches a placement deleted from
// Go and left behind in the native code.
func TestHintPlacementVocabularyIsPinnedAcrossTheLanguageBoundary(t *testing.T) {
	t.Parallel()

	macros := cHeaderIntConstants(t, hintPlacementHeader)
	members := objcEnumIntConstants(t, hintPlacementSource, hintPlacementEnum)

	placements := config.HintPlacements()
	if len(placements) == 0 {
		t.Fatal("config.HintPlacements() is empty; there is no vocabulary to pin")
	}

	for _, placement := range placements {
		macro := hintPlacementMacroPrefix + strings.ToUpper(placement)

		macroValue, hasMacro := macros[macro]
		if !hasMacro {
			t.Errorf(
				"%s: no `#define %s`, but config.HintPlacements() accepts %q",
				hintPlacementHeader, macro, placement,
			)

			continue
		}

		member := hintPlacementEnum + nativeEnumMemberName(placement)

		memberValue, hasMember := members[member]
		if !hasMember {
			t.Errorf(
				"%s: enum %s has no member %s, but %s defines %s",
				hintPlacementSource, hintPlacementEnum, member, hintPlacementHeader, macro,
			)

			continue
		}

		if macroValue != memberValue {
			t.Errorf(
				"placement %q: %s is %d in %s but %s is %d in %s; "+
					"Go passes the header value and the drawing code compares the enum, "+
					"so a mismatch draws the wrong placement",
				placement,
				macro, macroValue, hintPlacementHeader,
				member, memberValue, hintPlacementSource,
			)
		}
	}

	assertNativeConstantCount(
		t, hintPlacementHeader, hintPlacementMacroPrefix, macros,
		len(placements), hintPlacementDeclaration,
	)
	assertNativeConstantCount(
		t, hintPlacementSource, hintPlacementEnum, members,
		len(placements), hintPlacementDeclaration,
	)
}

// TestHintPlacementTranslationMatchesTheVocabulary closes the last link in the
// chain: Go string, C macro, Objective-C enum. The test above pins the macro to
// the enum, but the macOS renderer is what decides *which* macro a placement
// becomes, and swapping two arms of that switch would leave every other check
// green while hints drew above the elements they should sit below.
//
// It reads the switch rather than calling it, because the function is behind a
// darwin build tag and cgo, and this package has to reach it from any host.
func TestHintPlacementTranslationMatchesTheVocabulary(t *testing.T) {
	t.Parallel()

	translated := parseHintPlacementSwitch(t)

	for _, placement := range config.HintPlacements() {
		constant := hintPlacementEnum + nativeEnumMemberName(placement)

		macro, translatedHere := translated[constant]
		if !translatedHere {
			t.Errorf(
				"%s: %s has no `case config.%s:`, so %q is refused by the overlay "+
					"instead of drawing at its own placement",
				hintPlacementTranslator, hintPlacementFunc, constant, placement,
			)

			continue
		}

		want := hintPlacementMacroPrefix + strings.ToUpper(placement)
		if macro != want {
			t.Errorf(
				"%s: %s translates config.%s to C.%s, want C.%s",
				hintPlacementTranslator, hintPlacementFunc, constant, macro, want,
			)
		}
	}
}

// parseHintPlacementSwitch returns the placement constant each case of
// hintPlacementValue returns a macro for, keyed by the config constant's name
// ("HintPlacementTop") and valued by the macro's ("HINT_PLACEMENT_TOP"). The
// default case has no config constant to key on and is skipped: it is the
// branch that refuses a placement outside the vocabulary, and the unset value
// is settled to the documented default before the switch — both pinned by the
// darwin-only tests in the renderer's own package.
func parseHintPlacementSwitch(t *testing.T) map[string]string {
	t.Helper()

	fileSet := token.NewFileSet()

	parsed, err := parser.ParseFile(
		fileSet,
		filepath.Join(findRepoRoot(t), filepath.FromSlash(hintPlacementTranslator)),
		nil,
		parser.SkipObjectResolution,
	)
	if err != nil {
		t.Fatalf("%s: cannot parse (renamed?): %v", hintPlacementTranslator, err)
	}

	translated := make(map[string]string)
	found := false

	ast.Inspect(parsed, func(node ast.Node) bool {
		funcDecl, isFunc := node.(*ast.FuncDecl)
		if !isFunc || funcDecl.Name.Name != hintPlacementFunc {
			return true
		}

		found = true

		ast.Inspect(funcDecl, func(inner ast.Node) bool {
			clause, isClause := inner.(*ast.CaseClause)
			if !isClause {
				return true
			}

			collectHintPlacementCase(clause, translated)

			return true
		})

		return false
	})

	if !found {
		t.Fatalf(
			"%s: no func %s (renamed?); the placement translation is unpinned",
			hintPlacementTranslator, hintPlacementFunc,
		)
	}

	return translated
}

// collectHintPlacementCase records one `case config.HintPlacementX: return
// int(C.HINT_PLACEMENT_Y), nil` clause.
func collectHintPlacementCase(clause *ast.CaseClause, into map[string]string) {
	macro := ""

	ast.Inspect(clause, func(node ast.Node) bool {
		selector, isSelector := node.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}

		if ident, isIdent := selector.X.(*ast.Ident); isIdent && ident.Name == "C" {
			macro = selector.Sel.Name
		}

		return true
	})

	if macro == "" {
		return
	}

	for _, expr := range clause.List {
		selector, isSelector := expr.(*ast.SelectorExpr)
		if !isSelector {
			continue
		}

		if ident, isIdent := selector.X.(*ast.Ident); isIdent && ident.Name == "config" {
			into[selector.Sel.Name] = macro
		}
	}
}
