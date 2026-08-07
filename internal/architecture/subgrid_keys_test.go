package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

// subgridKeyNames are the two spellings the subgrid key set travels under: the
// config option, and the field an overlay backend keeps its copy in.
var subgridKeyNames = map[string]bool{
	"SublayerKeys": true,
	"sublayerKeys": true,
}

// subgridKeyDeciders are the two packages allowed to decide what the key set is
// when it is blank, rather than to pass on the one they were handed.
// internal/config resolves the option (ResolveSublayerKeys) and validates it;
// internal/domain/grid caps and cases it for everyone (SubgridKeys).
//
// They are matched exactly rather than as path prefixes: internal/config/loader
// is a different package that decides what a field change means, and a chain
// reintroduced there would be as invisible as the five this replaced.
var subgridKeyDeciders = []string{
	"internal/config",
	"internal/domain/grid",
}

// TestSubgridKeySetIsDecidedOnce keeps the drawn keys and the accepted keys the
// same keys. Four consumers used to answer "what if sublayer_keys is blank?"
// for themselves — the mode layer, the component factory and two overlay
// backends, the last two with an alphabet hardcoded in them — and nothing made
// their answers agree, so a configuration could produce a subgrid that drew one
// key set and acted on another (#1269).
//
// The shape it catches is a blank test in a function that reads the keys: that
// is a fallback chain, wherever it is written and whatever it falls back to.
// Passing the resolved value on is what a consumer is for.
func TestSubgridKeySetIsDecidedOnce(t *testing.T) {
	fileSet := token.NewFileSet()

	for _, file := range goFiles(t) {
		if decidesSubgridKeys(file.dir) {
			continue
		}

		parsed, parseErr := parser.ParseFile(
			fileSet,
			file.absPath,
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			t.Fatalf("ParseFile(%s) error = %v", file.relPath, parseErr)
		}

		for _, decl := range parsed.Decls {
			funcDecl, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || funcDecl.Body == nil {
				continue
			}

			if !testsSubgridKeysForBlank(funcDecl.Body) {
				continue
			}

			t.Errorf(
				"%s: %s tests the subgrid key set for blank; "+
					"the blank case is answered once, by config.ResolveSublayerKeys, "+
					"and the keys are capped and cased once, by grid.SubgridKeys — "+
					"a second answer here is a subgrid that draws one key set and "+
					"accepts another",
				file.relPath, funcDecl.Name.Name,
			)
		}
	}
}

// subgridKeyCarriers are the managers that keep their own copy of the key set
// in the surface they draw through, rather than reading the configuration at
// draw time the way the macOS overlay does. A copy can go stale — the surface
// is rebuilt underneath them — so on these two the keys have to be handed over
// again by every draw that can put a subgrid on screen.
var subgridKeyCarriers = []string{
	"internal/adapter/overlay/linux",
	"internal/adapter/overlay/windows",
}

// subgridKeySync is the method that does that handing over, and subgridDraws
// the backend calls that can put a subgrid on screen: ShowSubgrid draws one
// outright, and DrawGrid is what the surface is holding when the next
// ShowSubgrid lands on it.
const subgridKeySync = "syncSublayerKeysLocked"

var subgridDraws = []string{"ShowSubgrid", "DrawGrid"}

// TestSubgridDrawHandsOverTheKeys pins the half of #1269 that a resolved option
// cannot reach on its own. Deleting the hardcoded per-platform alphabets left
// the Windows manager syncing its surface in DrawGrid and nowhere else, so a
// surface rebuilt between the two drew a subgrid with no keys on it at all —
// invisible, and still accepting the keys the mode layer had. The alphabet that
// used to mask that is gone, which is exactly why it needs pinning here.
func TestSubgridDrawHandsOverTheKeys(t *testing.T) {
	fileSet := token.NewFileSet()
	checked := 0

	for _, file := range goFiles(t) {
		if !slices.Contains(subgridKeyCarriers, file.dir) {
			continue
		}

		parsed, parseErr := parser.ParseFile(
			fileSet,
			file.absPath,
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			t.Fatalf("ParseFile(%s) error = %v", file.relPath, parseErr)
		}

		for _, decl := range parsed.Decls {
			funcDecl, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || funcDecl.Body == nil || !callsAnyMethod(funcDecl.Body, subgridDraws) {
				continue
			}

			checked++

			if callsAnyMethod(funcDecl.Body, []string{subgridKeySync}) {
				continue
			}

			t.Errorf(
				"%s: %s draws through the surface without calling %s first, so it can "+
					"draw a subgrid the user cannot see but can still act on",
				file.relPath, funcDecl.Name.Name, subgridKeySync,
			)
		}
	}

	// A rename of either backend call would otherwise leave this passing over
	// nothing at all, which is the failure mode a guardrail can least afford.
	if checked == 0 {
		t.Errorf(
			"found no method calling any of %v in %v; the walk is broken and this "+
				"check would pass vacuously",
			subgridDraws, subgridKeyCarriers,
		)
	}
}

// callsAnyMethod reports whether the body calls a method with one of the given
// names on anything.
func callsAnyMethod(body *ast.BlockStmt, names []string) bool {
	found := false

	ast.Inspect(body, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return !found
		}

		switch called := call.Fun.(type) {
		case *ast.SelectorExpr:
			if slices.Contains(names, called.Sel.Name) {
				found = true
			}
		case *ast.Ident:
			if slices.Contains(names, called.Name) {
				found = true
			}
		}

		return !found
	})

	return found
}

// decidesSubgridKeys reports whether the file's package is allowed to answer
// the blank case rather than to pass on the answer.
func decidesSubgridKeys(dir string) bool {
	return slices.Contains(subgridKeyDeciders, dir)
}

// testsSubgridKeysForBlank reports whether the body tests the subgrid key set
// for blank — the shape every fallback chain this replaced had, whether it read
// the field directly (`o.sublayerKeys != ""`), through a local it had just been
// copied into (`keys := ...; if keys == ""`), or by length (`len(keys) == 0`).
//
// Testing something else for blank is nobody's business here: a function may
// well ask whether the grid characters are blank next door to reading the keys,
// and flagging that would make the check unusable. The cost of drawing the line
// there is that this recognizes shapes rather than meaning — a blank test
// spelled some third way would pass it. It is a tripwire on the way back to a
// second answer, not a proof that there is only one.
func testsSubgridKeysForBlank(body *ast.BlockStmt) bool {
	names := subgridKeyLocals(body)
	found := false

	ast.Inspect(body, func(node ast.Node) bool {
		binary, isBinary := node.(*ast.BinaryExpr)
		if !isBinary || (binary.Op != token.EQL && binary.Op != token.NEQ) {
			return !found
		}

		if isBlank(binary.X) && namesAnyOf(binary.Y, names) ||
			isBlank(binary.Y) && namesAnyOf(binary.X, names) {
			found = true
		}

		return !found
	})

	return found
}

// subgridKeyLocals names the key set as this body spells it: the two field
// spellings, plus every local assigned from one of them. One ordered pass is
// enough — a local has to be assigned before it is copied again.
func subgridKeyLocals(body *ast.BlockStmt) map[string]bool {
	names := map[string]bool{}
	for name := range subgridKeyNames {
		names[name] = true
	}

	ast.Inspect(body, func(node ast.Node) bool {
		assign, isAssign := node.(*ast.AssignStmt)
		if !isAssign {
			return true
		}

		for index, rhs := range assign.Rhs {
			if index >= len(assign.Lhs) || !namesAnyOf(rhs, names) {
				continue
			}

			target, isIdent := assign.Lhs[index].(*ast.Ident)
			if isIdent {
				names[target.Name] = true
			}
		}

		return true
	})

	return names
}

// namesAnyOf reports whether the expression mentions any of the given names,
// so that a call wrapped around one (strings.TrimSpace(keys)) still counts.
func namesAnyOf(expr ast.Expr, names map[string]bool) bool {
	found := false

	ast.Inspect(expr, func(node ast.Node) bool {
		switch named := node.(type) {
		case *ast.SelectorExpr:
			if names[named.Sel.Name] {
				found = true
			}
		case *ast.Ident:
			if names[named.Name] {
				found = true
			}
		}

		return !found
	})

	return found
}

// isBlank reports whether the expression is one of the two ways a blank string
// is written on the far side of a comparison: the literal "", or the 0 a
// len() is compared against.
func isBlank(expr ast.Expr) bool {
	literal, isLiteral := expr.(*ast.BasicLit)
	if !isLiteral {
		return false
	}

	return literal.Kind == token.STRING && literal.Value == `""` ||
		literal.Kind == token.INT && literal.Value == "0"
}
