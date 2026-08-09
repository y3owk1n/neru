package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// modeHandlerPackageDir holds modes.Handler, whose locking contract is the
// strictest in the repo: internal/app/modes/AGENTS.md states a lock order and
// the rules that keep it followable.
const modeHandlerPackageDir = "internal/app/modes"

// modeHandlerSourceFileFloor is the fewest non-test Go files the mode handler
// package is expected to hold. It carries thirty-eight today, so twenty catches
// a filter that has lost the package — a rename, a move, a directory spelled
// differently — and never fires on the package shrinking by a file or two.
const modeHandlerSourceFileFloor = 20

// modeLockOrder mirrors, one entry per mutex, the lock order
// internal/app/modes/AGENTS.md states. From the outermost hold inward the guide
// gives:
//
//	moveMonitorMu -> h.mu -> StyleResolver.mu
//
// The guide is where a position is stated and argued; this map is the
// mechanical half, because no test here reads that prose (ADR 0011). A mutex
// declared in the package and missing from both is a lock nobody has placed:
// the next contributor cannot tell whether it goes above h.mu or below it, and
// the answer they guess is only wrong the day two goroutines take the pair in
// opposite orders. That failure is silent to every check that runs before it
// ships — the code compiles, the linters pass, the tests pass, and what the
// user gets is a frozen keyboard.
//
// Keys are "<Type>.<field>" for a struct field, with an embedded mutex named by
// its type, and "var <name>" for a variable. Values state the position, not the
// mutex's purpose.
//
// Two kinds of lock are deliberately out of range. Locks the guide names but
// this package does not declare — StyleResolver.mu and the resolver's applyMu,
// the Linux overlay manager's renderMu, the event tap adapter's mu — live in
// other packages, and the subject here is what a change to this one can add.
// And a lock reached through a name that is not written sync.Mutex or
// sync.RWMutex — a named wrapper type, a sync.Locker field, a mutex embedded in
// a struct type held by a field — is beyond what a syntactic match can see;
// introducing one means stating its position in the guide by hand.
var modeLockOrder = map[string]string{
	"Handler.mu": "the handler lock, and the middle of the stated order: taken " +
		"below moveMonitorMu and held across the overlay adapter, which is where " +
		"StyleResolver.mu is reached",
	"Handler.moveMonitorMu": "outermost: MoveMonitor holds it across two separate " +
		"h.mu holds — the frame clear and the refresh — with the unlocked cursor " +
		"warp between them, so it sits above h.mu and is never taken beneath it",
}

// TestModeHandlerMutexesArePlacedInTheLockOrder fails when internal/app/modes
// declares a mutex that modeLockOrder does not place.
//
// This is the registry the rest of the locking contract rests on. "No method
// releases a lock it did not take" is reasoning anyone can do only while the
// set of locks is known; a lock nobody placed is one the reader of the guide
// does not know exists, so no reasoning about order covers it.
//
// What it checks is that a position was stated, not that the position is
// obeyed. No test here can watch two goroutines take a pair in opposite orders
// — that is what the guide's prose and a reviewer are for. This is the tripwire
// that the prose was written at all.
func TestModeHandlerMutexesArePlacedInTheLockOrder(t *testing.T) {
	for site, file := range modeHandlerMutexDeclarations(t) {
		if _, registered := modeLockOrder[site]; registered {
			continue
		}

		t.Errorf(
			"%s declares the mutex %s with no position in the lock order; state "+
				"where it sits relative to the locks already there in "+
				"internal/app/modes/AGENTS.md (\"Any new mutex needs a stated "+
				"position in that order\") and mirror that position in modeLockOrder",
			file, site,
		)
	}
}

// TestModeLockOrderEntriesAreStillReal keeps the registry from outliving the
// locks it describes. An entry for a mutex that is gone is a position in an
// order nothing occupies, and it reads to the next contributor as a constraint
// they have to respect. The list may only shrink by deletion, never by decay.
func TestModeLockOrderEntriesAreStillReal(t *testing.T) {
	declared := modeHandlerMutexDeclarations(t)

	for site, position := range modeLockOrder {
		if _, exists := declared[site]; exists {
			continue
		}

		t.Errorf(
			"modeLockOrder registers %s (%q), which %s no longer declares; delete "+
				"the entry so the registered order describes the locks that exist",
			site, position, modeHandlerPackageDir,
		)
	}
}

// modeHandlerMutexDeclarations returns every mutex declared in a non-test file
// of the mode handler package, as a map from site to the file declaring it.
//
// Test files are not read. A mutex in a test fake is not on the order this
// registry describes: the one the package has today (the fake accessibility
// client in accessibility_deadline_test.go) is reached from under h.mu, so its
// edge is h.mu -> fake, never the reverse, and no production hold can end up
// beneath it.
func modeHandlerMutexDeclarations(t *testing.T) map[string]string {
	t.Helper()

	declared := make(map[string]string)

	for _, file := range modeHandlerSourceFiles(t) {
		ast.Inspect(file.syntax, func(node ast.Node) bool {
			switch decl := node.(type) {
			case *ast.TypeSpec:
				for _, site := range mutexFieldSites(decl.Name.Name, decl.Type) {
					declared[site] = file.path
				}
			case *ast.ValueSpec:
				for _, name := range mutexVarNames(decl) {
					declared["var "+name] = file.path
				}
			}

			return true
		})
	}

	if len(declared) < modeMutexFloor {
		t.Fatalf(
			"found %d mutex declarations in %s, want at least %d; either a lock was "+
				"removed — delete its modeLockOrder entry and lower this floor — or "+
				"this has stopped recognizing how the package spells a mutex, and "+
				"every check below it is passing over nothing",
			len(declared), modeHandlerPackageDir, modeMutexFloor,
		)
	}

	return declared
}

// modeMutexFloor is the fewest mutex declarations expected in the mode handler
// package: it declares exactly the two modeLockOrder places, Handler.mu and
// Handler.moveMonitorMu. There is no headroom to leave, which is why the
// failure above names both readings — a floor with none cannot tell a matcher
// that broke from a lock that went away, and only the first of those is a bug.
const modeMutexFloor = 2

// mutexVarNames returns the names a var declaration binds to a mutex, whether
// the type is written out (var mu sync.Mutex) or left to the value
// (var mu = sync.Mutex{}, var mu = new(sync.Mutex)).
func mutexVarNames(spec *ast.ValueSpec) []string {
	declaresMutex := isMutexType(spec.Type)

	for _, value := range spec.Values {
		if isMutexValue(value) {
			declaresMutex = true
		}
	}

	if !declaresMutex {
		return nil
	}

	names := make([]string, 0, len(spec.Names))
	for _, name := range spec.Names {
		names = append(names, name.Name)
	}

	return names
}

// isMutexValue reports whether expr constructs a mutex: sync.Mutex{},
// &sync.Mutex{} or new(sync.Mutex).
func isMutexValue(expr ast.Expr) bool {
	switch value := expr.(type) {
	case *ast.UnaryExpr:
		return value.Op == token.AND && isMutexValue(value.X)
	case *ast.CompositeLit:
		return isMutexType(value.Type)
	case *ast.CallExpr:
		ident, isIdent := value.Fun.(*ast.Ident)
		if !isIdent || ident.Name != "new" || len(value.Args) != 1 {
			return false
		}

		return isMutexType(value.Args[0])
	default:
		return false
	}
}

// mutexFieldSites returns the registry sites for the mutex fields of one type,
// prefixed by the name the type is declared under. An embedded mutex is named
// by its type, which is how a caller spells it too, and an inline struct field
// is walked into, so a lock cannot hide one level down.
func mutexFieldSites(typeName string, typeExpr ast.Expr) []string {
	structType, isStruct := typeExpr.(*ast.StructType)
	if !isStruct {
		return nil
	}

	var sites []string

	for _, field := range structType.Fields.List {
		if !isMutexType(field.Type) {
			for _, name := range fieldNames(field) {
				sites = append(sites, mutexFieldSites(typeName+"."+name, field.Type)...)
			}

			continue
		}

		if len(field.Names) == 0 {
			sites = append(sites, typeName+"."+mutexTypeName(field.Type))

			continue
		}

		for _, name := range field.Names {
			sites = append(sites, typeName+"."+name.Name)
		}
	}

	return sites
}

// fieldNames returns what a struct field is called, using the embedded type's
// own name when the field has none.
func fieldNames(field *ast.Field) []string {
	if len(field.Names) == 0 {
		return []string{receiverTypeName(field.Type)}
	}

	names := make([]string, 0, len(field.Names))
	for _, name := range field.Names {
		names = append(names, name.Name)
	}

	return names
}

// isMutexType reports whether expr names one of the sync mutexes, through a
// pointer or not. A *sync.Mutex field is an alias for a lock declared
// elsewhere, and taking it still puts this package in an order, so it counts.
func isMutexType(expr ast.Expr) bool {
	return mutexTypeName(expr) != ""
}

// mutexTypeName returns the sync mutex type expr names, or the empty string.
func mutexTypeName(expr ast.Expr) string {
	if pointer, isPointer := expr.(*ast.StarExpr); isPointer {
		expr = pointer.X
	}

	selector, isSelector := expr.(*ast.SelectorExpr)
	if !isSelector {
		return ""
	}

	pkg, isIdent := selector.X.(*ast.Ident)
	if !isIdent || pkg.Name != "sync" {
		return ""
	}

	switch selector.Sel.Name {
	case "Mutex", "RWMutex":
		return selector.Sel.Name
	default:
		return ""
	}
}

// modeHandlerFile is one parsed Go file of the mode handler package.
type modeHandlerFile struct {
	// path is the slash-relative path, as a failure message spells it.
	path string
	// isTest marks a _test.go file, which the two guardrails over this package
	// judge differently.
	isTest bool
	// syntax is the parsed file.
	syntax *ast.File
	// fileSet resolves the positions inside syntax.
	fileSet *token.FileSet
}

// modeHandlerSourceFiles parses the non-test Go files of the mode handler
// package, off the walk goFiles already does over the checkout.
//
// Files are parsed rather than built, so the build-tagged ones
// (cursor_darwin.go, cursor_other.go) are read on every platform.
func modeHandlerSourceFiles(t *testing.T) []modeHandlerFile {
	t.Helper()

	fileSet := token.NewFileSet()

	var files []modeHandlerFile

	for _, file := range goFiles(t) {
		if file.dir != modeHandlerPackageDir {
			continue
		}

		files = append(files, modeHandlerFile{
			path:    file.relPath,
			syntax:  parseRepoGoFile(t, fileSet, file.absPath, file.relPath),
			fileSet: fileSet,
		})
	}

	assertWalkedAtLeast(
		t, "source files in "+modeHandlerPackageDir, len(files), modeHandlerSourceFileFloor,
	)

	return files
}

// parseRepoGoFile parses one Go file of the checkout, failing the test rather
// than returning an error: a file that will not parse is a broken checkout, and
// skipping it would leave the guardrail passing over the one file it cannot
// read.
func parseRepoGoFile(
	t *testing.T,
	fileSet *token.FileSet,
	absPath, relPath string,
) *ast.File {
	t.Helper()

	parsed, parseErr := parser.ParseFile(fileSet, absPath, nil, parser.SkipObjectResolution)
	if parseErr != nil {
		t.Fatalf("ParseFile(%s) error = %v", relPath, parseErr)
	}

	return parsed
}
