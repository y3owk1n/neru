package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// subgridCellDecider is the one package allowed to work out where the cells of
// a subgrid fall: it is where SubgridCells lives, and the manager there is one
// of the callers whose answer has to be that one.
const subgridCellDecider = "internal/domain/grid"

// subgridKeySetCall and subgridCellSetCall are the two answers a subgrid is
// drawn from — which keys it has, and which rectangles they sit on.
const (
	subgridKeySetCall  = "SubgridKeys"
	subgridCellSetCall = "SubgridCells"
)

// TestSubgridCellsAreComputedOnce keeps the cell a person sees and the point
// the cursor lands in the same rectangle. The breakpoint arithmetic that
// divides a grid cell into a subgrid was written four times — once in the
// manager, deciding where the cursor goes, and once in each overlay backend,
// deciding where the cell is drawn — with the 0.5 it rounds by declared under
// three names. They agreed by coincidence, and a copy that drifted would draw
// a cell in one place and click in another with no test to notice (#1287).
//
// The shape it catches is a backend that asks which keys a subgrid has and
// then works out its own rectangles for them: outside the package that owns
// both answers, the two calls travel together. That is a tripwire on the way
// back to a second implementation rather than proof there is only one — a
// backend that called SubgridCells and then ignored it would pass — but it is
// the check that reaches the Linux and Windows drawing code from a macOS host,
// which no unit test on this machine compiles.
func TestSubgridCellsAreComputedOnce(t *testing.T) {
	fileSet := token.NewFileSet()
	checked := 0

	for _, file := range goFiles(t) {
		if file.dir == subgridCellDecider {
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

			if !callsAnyMethod(funcDecl.Body, []string{subgridKeySetCall}) {
				continue
			}

			checked++

			if callsAnyMethod(funcDecl.Body, []string{subgridCellSetCall}) {
				continue
			}

			t.Errorf(
				"%s: %s draws a subgrid's keys but not its cells; the rectangles "+
					"come from grid.%s, because the manager moves the cursor into "+
					"the same ones and a second answer here is a cell drawn "+
					"somewhere other than where it clicks",
				file.relPath, funcDecl.Name.Name, subgridCellSetCall,
			)
		}
	}

	// Every backend renaming its draw would otherwise leave this passing over
	// nothing, which is the failure mode a guardrail can least afford.
	if checked == 0 {
		t.Errorf(
			"found no function outside %s calling %s; the walk is broken and this "+
				"check would pass vacuously",
			subgridCellDecider, subgridKeySetCall,
		)
	}
}
