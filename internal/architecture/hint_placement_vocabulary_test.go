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

// Where macOS turns a configured placement into the HINT_PLACEMENT_* constant
// it hands the overlay.
const (
	hintPlacementTranslator  = "internal/adapter/overlay/render/hints/overlay_darwin.go"
	hintPlacementFunc        = "hintPlacementValue"
	hintPlacementConstPrefix = "HintPlacement"
	hintPlacementMacroPrefix = "HINT_PLACEMENT_"
)

// TestHintPlacementTranslationMatchesTheVocabulary checks which macro each
// configured placement becomes on macOS. The Objective-C enum takes its values
// from the HINT_PLACEMENT_* macros, so the native numbers cannot disagree.
// Swapping two arms of hintPlacementValue would still compile, and hints would
// draw above the elements they should sit below.
//
// It reads the switch rather than calling it, because the function is behind a
// darwin build tag and cgo, and this package has to reach it from any host.
func TestHintPlacementTranslationMatchesTheVocabulary(t *testing.T) {
	t.Parallel()

	translated := parseHintPlacementSwitch(t)

	for _, placement := range config.HintPlacements() {
		constant := hintPlacementConstPrefix + pascalCase(placement)

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
// default case has no config constant to key on and is skipped. It is the
// branch that refuses a placement outside the vocabulary, and the unset value
// is settled to the documented default before the switch. Both are pinned by
// the darwin-only tests in the renderer's own package.
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
