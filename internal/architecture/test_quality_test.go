package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// failCalls are the testing.TB methods that can actually fail (or visibly skip)
// a test. Anything else leaves the test reporting success.
var failCalls = map[string]bool{
	"Error": true, "Errorf": true,
	"Fatal": true, "Fatalf": true,
	"Fail": true, "FailNow": true,
	"Skip": true, "Skipf": true, "SkipNow": true,
}

// TestNoTestSwallowsAnErrorWithoutFailing forbids the
//
//	if err != nil { t.Logf("...", err) }
//
// shape: the code under test can break on every run and the suite stays
// green. Use t.Error for real failures or t.Skip for environments where the
// case cannot run — a skip is visible, a swallowed error is not.
func TestNoTestSwallowsAnErrorWithoutFailing(t *testing.T) {
	var offenders []string

	forEachTestFile(t, func(_ repoFile, fset *token.FileSet, file *ast.File) {
		ast.Inspect(file, func(node ast.Node) bool {
			ifStmt, ok := node.(*ast.IfStmt)
			if !ok {
				return true
			}

			if !isErrNotNilCond(ifStmt.Cond) {
				return true
			}

			logged, failed := scanForCalls(ifStmt.Body)
			if logged && !failed {
				offenders = append(offenders, fset.Position(ifStmt.Pos()).String())
			}

			return true
		})
	})

	reportOffenders(t, offenders,
		"error branch logs the error but never fails or skips the test")
}

// TestEveryTestBodyCanFail forbids test functions and subtests that contain no
// failure-producing call at all — they exercise code without checking anything,
// so they only ever catch an outright panic.
//
// A function whose body only delegates to t.Run is fine: the assertions live in
// the subtests, which are checked individually.
func TestEveryTestBodyCanFail(t *testing.T) {
	var offenders []string

	forEachTestFile(t, func(_ repoFile, fset *token.FileSet, file *ast.File) {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Body == nil {
				continue
			}

			if !strings.HasPrefix(funcDecl.Name.Name, "Test") {
				continue
			}

			// TestMain is a harness entry point, not an assertion site.
			if funcDecl.Name.Name == "TestMain" {
				continue
			}

			checkBodyCanFail(fset, funcDecl.Name.Name, funcDecl.Body, &offenders)
		}
	})

	reportOffenders(t, offenders,
		"test body contains no assertion and no subtests, so it can never fail")
}

// checkBodyCanFail reports a body that neither asserts nor delegates to
// subtests, recursing into each t.Run closure so subtests are judged on their
// own contents.
func checkBodyCanFail(
	fset *token.FileSet,
	name string,
	body *ast.BlockStmt,
	offenders *[]string,
) {
	subtests := 0

	ast.Inspect(body, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}

		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector || selector.Sel.Name != "Run" || len(call.Args) != 2 {
			return true
		}

		closure, isClosure := call.Args[1].(*ast.FuncLit)
		if !isClosure {
			return true
		}

		subtests++

		subName := name
		if lit, isLiteral := call.Args[0].(*ast.BasicLit); isLiteral {
			subName += "/" + strings.Trim(lit.Value, `"`)
		}

		checkBodyCanFail(fset, subName, closure.Body, offenders)

		// Do not descend further here; the recursive call covers this closure.
		return false
	})

	if subtests > 0 {
		return
	}

	if _, failed := scanForCalls(body); failed {
		return
	}

	*offenders = append(*offenders,
		fset.Position(body.Pos()).String()+"\t"+name)
}

// scanForCalls reports whether the node contains a t.Log-style call and whether
// it contains anything that can fail or skip the test. Nested t.Run closures
// are not descended into: they are separate scopes with their own assertions.
func scanForCalls(node ast.Node) (bool, bool) {
	var logged, failed bool

	ast.Inspect(node, func(inner ast.Node) bool {
		call, ok := inner.(*ast.CallExpr)
		if !ok {
			return true
		}

		if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
			if selector.Sel.Name == "Run" {
				return false
			}

			switch selector.Sel.Name {
			case "Log", "Logf":
				logged = true
			default:
				if failCalls[selector.Sel.Name] {
					failed = true
				}
			}
		}

		// Any call taking the testing handle as its first argument is treated
		// as an assertion helper: testify's assert.Equal(t, ...), a local
		// requireX(t, ...), and so on.
		if len(call.Args) > 0 {
			if ident, ok := call.Args[0].(*ast.Ident); ok &&
				(ident.Name == "t" || ident.Name == "tb") {
				failed = true
			}
		}

		return true
	})

	return logged, failed
}

// isErrNotNilCond reports whether cond is an `err != nil`-shaped check, for any
// identifier whose name mentions "err".
func isErrNotNilCond(cond ast.Expr) bool {
	binary, ok := cond.(*ast.BinaryExpr)
	if !ok || binary.Op != token.NEQ {
		return false
	}

	left, isIdent := binary.X.(*ast.Ident)
	if !isIdent || !strings.Contains(strings.ToLower(left.Name), "err") {
		return false
	}

	right, isIdent := binary.Y.(*ast.Ident)

	return isIdent && right.Name == "nil"
}

// forEachTestFile parses every _test.go file in the repository, including
// build-tagged ones, and hands it to fn. Tagged files are parsed rather than
// built, so integration tests are covered on every platform.
func forEachTestFile(
	t *testing.T,
	visit func(source repoFile, fset *token.FileSet, file *ast.File),
) {
	t.Helper()

	repoRoot := findRepoRoot(t)
	fset := token.NewFileSet()
	visited := 0

	walkRepoFiles(t, repoRoot, func(file repoFile) {
		if !strings.HasSuffix(file.name, "_test.go") {
			return
		}

		parsed, parseErr := parser.ParseFile(fset, file.abs, nil, 0)
		if parseErr != nil {
			t.Fatalf("ParseFile(%s) error = %v", file.rel, parseErr)
		}

		visited++

		visit(file, fset, parsed)
	})

	assertWalkedAtLeast(t, "test files", visited, bulkWalkFloor)
}

// reportOffenders fails the test with one line per offending site, relative to
// the repo root so the output is clickable.
func reportOffenders(t *testing.T, offenders []string, problem string) {
	t.Helper()

	if len(offenders) == 0 {
		return
	}

	repoRoot := findRepoRoot(t) + string(filepath.Separator)

	cleaned := make([]string, 0, len(offenders))
	for _, offender := range offenders {
		cleaned = append(cleaned, strings.ReplaceAll(offender, repoRoot, ""))
	}

	sort.Strings(cleaned)

	t.Errorf("%d site(s) where the %s:\n\t%s",
		len(cleaned), problem, strings.Join(cleaned, "\n\t"))
}
