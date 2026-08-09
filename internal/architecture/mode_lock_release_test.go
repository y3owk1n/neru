package architecture_test

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"testing"
)

// modeReleaseFloor is the fewest lock releases expected in the mode handler
// package. It holds seventy-five today across its source and its tests, so
// twenty catches a check that has stopped recognizing an unlock — a package no
// longer found, a scope walk that returns nothing — and never fires on the
// package losing a handful of call sites.
const modeReleaseFloor = 20

// lockAcquireMethods are the calls that take a lock and always get it: the
// caller returns owning it and owes the release.
var lockAcquireMethods = []string{"Lock", "RLock"}

// lockTryAcquireMethods are the calls that may fail to take the lock. The
// caller owns it only when the call answered true, so one of these is an
// acquisition only where the scope reads that answer — see
// checkedTryAcquisitionsIn.
var lockTryAcquireMethods = []string{"TryLock", "TryRLock"}

// lockReleaseMethods are the calls that release a lock.
var lockReleaseMethods = []string{"Unlock", "RUnlock"}

// knownModeLockReleaseExceptions are release sites in internal/app/modes that
// break the contract and are not fixed yet, keyed by "<file>:<scope>:<lock>"
// as the failure messages below spell a site.
//
// Each entry carries the reason it is still here, so the list can only shrink:
// TestKnownModeLockReleaseExceptionsAreStillReal deletes nothing on its own but
// fails on an entry that has stopped violating anything, and a new violation
// fails the guardrail rather than joining the list. It is currently empty,
// which is the goal state — the package satisfies the contract as written. Add
// an entry only when a violation genuinely cannot be fixed in the same change,
// never to silence one.
var knownModeLockReleaseExceptions = map[string]string{}

// TestModeHandlerUnlocksAreDeferred fails when a source file in the mode
// handler package releases a lock outside a defer.
//
// The rule internal/app/modes/AGENTS.md states is "every lock is released, via
// defer, by the method that took it; never unlock mid-method to make a blocking
// call safe". The tempting breach is precisely the one it names: a method
// holding h.mu needs to make a call that can block or re-enter, so it unlocks,
// calls, and locks again. What that buys is a window in the middle of a method
// whose invariants are written as if the lock were held — the active mode can
// change, the grid manager can be reassigned by a reload (#1277), and the
// second acquisition can be the one that inverts the order against a caller
// that was above it. The alternative the guide gives is to compute a plan under
// the lock and run it after release (planIndicatorTick/drawIndicators in
// indicator_polling.go), or to hand the blocking call to a goroutine that
// re-enters through the outer locked surface (requestScreenCapturePermissionAndResume
// in hints.go).
//
// Nothing catches this today. gocritic's badLock flags a mismatched pair such
// as Lock with RUnlock, not an unlock that simply is not deferred; the lock is
// genuinely held, so there is no runtime panic; and the inversion it opens
// surfaces only as a hang under a race no test reliably hits (ADR 0011).
//
// Test files are outside this half of the contract, and that is a scope rather
// than an exemption — it is not in the list below and nothing shrinks it. The
// rule's subject is a *method of the handler*, and a test body is not one: it
// takes h.mu and releases it inline because it is standing in for the locked
// entry point a production caller would have gone through, and there is no
// blocking call in the middle for a defer to protect. Twenty-eight releases in
// the package's tests are that shape. The other half below — that a scope
// releases only what it took — is asked of the tests too, and so is the
// hand-off check, because neither is about a lock being held too long.
func TestModeHandlerUnlocksAreDeferred(t *testing.T) {
	for _, release := range modeHandlerLockReleases(t) {
		// A release handed off as a method value is not deferred either, but the
		// test below has the message for it.
		if release.isTest || release.deferred || release.handedOff {
			continue
		}

		if _, known := knownModeLockReleaseExceptions[release.site()]; known {
			continue
		}

		t.Errorf(
			"%s: %s releases %s without a defer; release every lock via defer in "+
				"the method that took it, and never unlock mid-method to make a "+
				"blocking call safe — compute a plan under the lock and run it after "+
				"release, or hand the call to a goroutine that re-enters through the "+
				"outer locked surface (internal/app/modes/AGENTS.md, the mode-handler "+
				"locking contract)",
			release.position, release.scope, release.lock,
		)
	}
}

// TestModeHandlerReleasesOnlyLocksItTook fails when a scope in the mode handler
// package releases a lock it never takes.
//
// "No method releases a lock it did not take" is the sentence that makes the
// lock order readable at all: the order in modeLockOrder describes who may hold
// what while taking what, and that reasoning is local only while every hold
// begins and ends inside one scope. A method that unlocks on its caller's
// behalf moves the release somewhere the reader of either method can see it,
// and the compiler's split of Handler from handlerState — which stops a
// lock-held method from calling a locking entry point — says nothing about a
// stray release.
//
// A function literal is its own scope. It runs on its own goroutine or at its
// own time, so a deferred unlock inside a timer callback must not stand in for
// the missing one in the method that scheduled it. The exception is the literal
// a function defers into itself — defer func() { ... }() runs in that
// function's frame at its return, and both halves of this contract read it as
// part of the scope around it.
// A release named but not called — h.mu.Unlock passed as a method value — is
// the same breach with the frame left blank: whoever runs it did not take the
// lock, and no scope here can say who that is.
//
// A try-acquire is not a hold. h.mu.TryLock() answers false under contention
// and the scope keeps running, so a release below it releases a lock this
// goroutine never took — a fatal "unlock of unlocked mutex", or worse, the
// holder's lock taken out from under it. Both production uses read the answer
// and give up on false (planIndicatorTick in indicator_polling.go,
// rehideSystemCursor in cursor_visibility.go), each using TryLock precisely to
// avoid deadlocking against the polling goroutine, which is what makes a third
// site that drops the answer a plausible regression rather than a hypothetical
// one. So a try-acquire counts as an acquisition only where the scope reads the
// answer: written into a branch condition, or assigned to a name a branch
// condition reads. Any other shape fails here, which is the direction to fail
// in — the message names the shape the package already uses.
//
// What this cannot see is whether the failing branch actually gives up. A scope
// that reads the answer, logs it and falls through to the release is still
// unlocking what it may not hold, but telling that from the correct shape means
// following a negation into the arm the failure lands in and on to the return —
// flow analysis, not a syntactic walk. Reading the answer is the line this test
// draws.
//
// One hold genuinely escapes this, and it escapes by leaving the package:
// hints.go hands &h.outer.mu to domainHint.NewManager, which takes and releases
// it inside internal/domain/hint. A walk over this directory cannot follow it,
// and a lock passed to another package is the shape to look at twice.
func TestModeHandlerReleasesOnlyLocksItTook(t *testing.T) {
	for _, release := range modeHandlerLockReleases(t) {
		if _, known := knownModeLockReleaseExceptions[release.site()]; known {
			continue
		}

		if release.handedOff {
			t.Errorf(
				"%s: %s hands the release of %s to another frame as a method value; "+
					"no method releases a lock it did not take — take and release each "+
					"lock in one scope (internal/app/modes/AGENTS.md, the mode-handler "+
					"locking contract)",
				release.position, release.scope, release.lock,
			)

			continue
		}

		if release.acquired {
			continue
		}

		if release.triedUnchecked {
			t.Errorf(
				"%s: %s releases %s but only tries for it, without reading whether it "+
					"got it; a contended TryLock answers false and the scope runs on "+
					"holding nothing, so this releases a lock the scope may never have "+
					"taken — branch on the answer and give up when it is false, the way "+
					"planIndicatorTick does (internal/app/modes/AGENTS.md, the "+
					"mode-handler locking contract)",
				release.position, release.scope, release.lock,
			)

			continue
		}

		t.Errorf(
			"%s: %s releases %s but never takes it; no method releases a lock it "+
				"did not take — take and release each lock in one scope "+
				"(internal/app/modes/AGENTS.md, the mode-handler locking contract)",
			release.position, release.scope, release.lock,
		)
	}
}

// TestKnownModeLockReleaseExceptionsAreStillReal keeps the exception list
// honest: an entry that no longer names a violating release site is stale and
// must be deleted, or the list becomes a place where the locking contract goes
// to die.
func TestKnownModeLockReleaseExceptionsAreStillReal(t *testing.T) {
	violating := make(map[string]bool)

	for _, release := range modeHandlerLockReleases(t) {
		if release.breaksTheContract() {
			violating[release.site()] = true
		}
	}

	for site, reason := range knownModeLockReleaseExceptions {
		if violating[site] {
			continue
		}

		t.Errorf(
			"known exception %s no longer breaks the locking contract (%q); delete "+
				"its entry",
			site, reason,
		)
	}
}

// lockRelease is one Unlock or RUnlock call site in the mode handler package.
type lockRelease struct {
	// file is the slash-relative path of the file holding the call.
	file string
	// position is that path with the line number, so a failure is clickable.
	position string
	// scope names the function or function literal the call sits in.
	scope string
	// lock is the expression the release was called on, "h.mu" or "h.outer.mu".
	lock string
	// deferred reports whether the release is the call of a defer statement in
	// its own scope.
	deferred bool
	// acquired reports whether the same scope also takes the same lock.
	acquired bool
	// triedUnchecked marks a release whose scope only ever try-acquired the
	// lock and dropped the answer, so it may hold nothing to release. It picks
	// the failure message; the release is not acquired either way.
	triedUnchecked bool
	// handedOff marks a release named but not called — h.mu.Unlock as a method
	// value, passed to something that will run it in a frame that never held
	// the lock.
	handedOff bool
	// isTest marks a release in a _test.go file.
	isTest bool
}

// site is the key the exception list uses, stable across line moves — a
// violation that survives an edit above it keeps its entry.
func (r lockRelease) site() string {
	return fmt.Sprintf("%s:%s:%s", r.file, r.scope, r.lock)
}

// breaksTheContract reports whether this release fails either half of the
// contract, which is what an entry in the exception list has to be excusing.
func (r lockRelease) breaksTheContract() bool {
	if r.handedOff || !r.acquired {
		return true
	}

	return !r.deferred && !r.isTest
}

// modeHandlerLockReleases returns every lock release in the mode handler
// package, tests included, with what its own scope did about it.
func modeHandlerLockReleases(t *testing.T) []lockRelease {
	t.Helper()

	var releases []lockRelease

	files := modeHandlerSourceFiles(t)
	files = append(files, modeHandlerTestFiles(t)...)

	for _, file := range files {
		forEachLockScope(file.syntax, func(scope lockScope) {
			acquired := locksTakenIn(scope.body)
			deferred := deferredCallsIn(scope.body)
			called := calledSelectorsIn(scope.body)

			inspectScope(scope.body, func(node ast.Node) {
				selector, isSelector := node.(*ast.SelectorExpr)
				if !isSelector || !slices.Contains(lockReleaseMethods, selector.Sel.Name) {
					return
				}

				call, isCalled := called[selector]
				lock := types.ExprString(selector.X)

				release := lockRelease{
					file: file.path,
					position: fmt.Sprintf(
						"%s:%d",
						file.path,
						file.fileSet.Position(selector.Pos()).Line,
					),
					scope:     scope.name,
					lock:      lock,
					deferred:  isCalled && (scope.runsDeferred || deferred[call]),
					acquired:  acquired.held[lock] || scope.outer.held[lock],
					handedOff: !isCalled,
					isTest:    file.isTest,
					triedUnchecked: acquired.triedUnchecked[lock] ||
						scope.outer.triedUnchecked[lock],
				}

				releases = append(releases, release)
			})
		})
	}

	assertWalkedAtLeast(
		t, "lock releases in "+modeHandlerPackageDir, len(releases), modeReleaseFloor,
	)

	return releases
}

// calledSelectorsIn maps each method selector a scope calls to the call it is
// the callee of. A selector missing from it is a method *value*: named, not
// called, and therefore about to run somewhere this scope cannot see.
func calledSelectorsIn(body *ast.BlockStmt) map[*ast.SelectorExpr]*ast.CallExpr {
	called := make(map[*ast.SelectorExpr]*ast.CallExpr)

	inspectScope(body, func(node ast.Node) {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return
		}

		if selector, isSelector := call.Fun.(*ast.SelectorExpr); isSelector {
			called[selector] = call
		}
	})

	return called
}

// lockAcquisitions is what one scope did about taking locks.
type lockAcquisitions struct {
	// held are the locks the scope owns by the time it can release anything:
	// taken with Lock or RLock, or with a try-acquire whose answer it read.
	held map[string]bool
	// triedUnchecked are the locks the scope try-acquires without reading
	// whether it got them. It owns those only by luck.
	triedUnchecked map[string]bool
}

// locksTakenIn returns the locks a single scope acquires, apart from the ones
// it only tries for without reading the answer.
//
// A try-acquire is not an acquisition on its own: TryLock answers false under
// contention and the scope runs on holding nothing, so it counts only where the
// scope reads that answer. TestModeHandlerReleasesOnlyLocksItTook has the
// hazard and the limits of reading it this way.
func locksTakenIn(body *ast.BlockStmt) lockAcquisitions {
	acquisitions := lockAcquisitions{
		held:           make(map[string]bool),
		triedUnchecked: make(map[string]bool),
	}

	checked := checkedTryAcquisitionsIn(body)

	inspectScope(body, func(node ast.Node) {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return
		}

		lock, method := lockCallTarget(call)

		switch {
		case slices.Contains(lockAcquireMethods, method), checked[call]:
			acquisitions.held[lock] = true
		case slices.Contains(lockTryAcquireMethods, method):
			acquisitions.triedUnchecked[lock] = true
		}
	})

	return acquisitions
}

// checkedTryAcquisitionsIn returns the try-acquires whose answer the scope
// reads: written straight into a branch condition — if !h.mu.TryLock() — or
// assigned to a name a branch condition reads.
//
// A shape it does not recognize is reported as unchecked, so the guardrail
// fails loudly naming the shape the package uses rather than passing something
// it cannot read.
func checkedTryAcquisitionsIn(body *ast.BlockStmt) map[*ast.CallExpr]bool {
	checked := make(map[*ast.CallExpr]bool)
	conditions := branchConditionsIn(body)

	for _, condition := range conditions {
		inspectNode(condition, func(node ast.Node) {
			if call, isTry := tryAcquireCall(node); isTry {
				checked[call] = true
			}
		})
	}

	branchedOn := identsIn(conditions)

	inspectScope(body, func(node ast.Node) {
		assign, isAssign := node.(*ast.AssignStmt)
		if !isAssign || len(assign.Lhs) != len(assign.Rhs) {
			return
		}

		for index, rhs := range assign.Rhs {
			call, isTry := tryAcquireCall(rhs)
			if !isTry {
				continue
			}

			if name, isName := assign.Lhs[index].(*ast.Ident); isName && branchedOn[name.Name] {
				checked[call] = true
			}
		}
	})

	return checked
}

// tryAcquireCall reports whether a node is a call to TryLock or TryRLock.
func tryAcquireCall(node ast.Node) (*ast.CallExpr, bool) {
	call, isCall := node.(*ast.CallExpr)
	if !isCall {
		return nil, false
	}

	if _, method := lockCallTarget(call); !slices.Contains(lockTryAcquireMethods, method) {
		return nil, false
	}

	return call, true
}

// branchConditionsIn returns the expressions a scope branches on: the
// conditions of its if and for statements, the tag of each switch, and the
// expressions of each case, which are the conditions of a switch with no tag.
func branchConditionsIn(body *ast.BlockStmt) []ast.Expr {
	var conditions []ast.Expr

	inspectScope(body, func(node ast.Node) {
		switch stmt := node.(type) {
		case *ast.IfStmt:
			conditions = append(conditions, stmt.Cond)
		case *ast.ForStmt:
			if stmt.Cond != nil {
				conditions = append(conditions, stmt.Cond)
			}
		case *ast.SwitchStmt:
			if stmt.Tag != nil {
				conditions = append(conditions, stmt.Tag)
			}
		case *ast.CaseClause:
			conditions = append(conditions, stmt.List...)
		}
	})

	return conditions
}

// identsIn returns the names the given expressions read.
func identsIn(exprs []ast.Expr) map[string]bool {
	names := make(map[string]bool)

	for _, expr := range exprs {
		inspectNode(expr, func(node ast.Node) {
			if ident, isIdent := node.(*ast.Ident); isIdent {
				names[ident.Name] = true
			}
		})
	}

	return names
}

// deferredCallsIn returns the calls a single scope defers.
func deferredCallsIn(body *ast.BlockStmt) map[*ast.CallExpr]bool {
	deferred := make(map[*ast.CallExpr]bool)

	inspectScope(body, func(node ast.Node) {
		if deferStmt, isDefer := node.(*ast.DeferStmt); isDefer {
			deferred[deferStmt.Call] = true
		}
	})

	return deferred
}

// lockCallTarget splits a method call into the expression it was called on and
// the method name — "h.mu" and "Unlock" for h.mu.Unlock(). It answers empty
// strings for anything that is not a method call.
func lockCallTarget(call *ast.CallExpr) (string, string) {
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return "", ""
	}

	return types.ExprString(selector.X), selector.Sel.Name
}

// lockScope is one span of code a lock can be taken and released in.
type lockScope struct {
	// name is what a failure message calls it.
	name string
	// body is the block to read.
	body *ast.BlockStmt
	// runsDeferred marks a function literal its enclosing function defers into
	// itself — defer func() { ... }(). That literal runs in the enclosing
	// frame, at the enclosing return, so a release inside it is a deferred
	// release by the method that took the lock. A literal handed to go, to
	// time.AfterFunc or to a callback struct is none of those things and stays a
	// scope of its own.
	runsDeferred bool
	// outer is what the enclosing scope did about taking locks, carried in only
	// for a runsDeferred literal, which releases what that scope acquired.
	outer lockAcquisitions
}

// forEachLockScope hands visit every lock scope of a file: each function body,
// and each function literal inside one, named for the function it was written
// in.
func forEachLockScope(file *ast.File, visit func(scope lockScope)) {
	for _, decl := range file.Decls {
		funcDecl, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || funcDecl.Body == nil {
			continue
		}

		visitLockScope(lockScope{name: funcName(funcDecl), body: funcDecl.Body}, visit)
	}
}

// visitLockScope hands scope to visit, then descends into the function literals
// its body holds, each as a scope of its own.
func visitLockScope(scope lockScope, visit func(lockScope)) {
	visit(scope)

	deferredLiterals := deferredLiteralsIn(scope.body)
	takenHere := locksTakenIn(scope.body)

	inspectScope(scope.body, func(node ast.Node) {
		literal, isLiteral := node.(*ast.FuncLit)
		if !isLiteral {
			return
		}

		nested := lockScope{name: scope.name + "'s function literal", body: literal.Body}

		if deferredLiterals[literal] {
			nested.name = scope.name + "'s deferred function literal"
			nested.runsDeferred = true
			nested.outer = takenHere
		}

		visitLockScope(nested, visit)
	})
}

// deferredLiteralsIn returns the function literals a scope defers into itself.
func deferredLiteralsIn(body *ast.BlockStmt) map[*ast.FuncLit]bool {
	literals := make(map[*ast.FuncLit]bool)

	inspectScope(body, func(node ast.Node) {
		deferStmt, isDefer := node.(*ast.DeferStmt)
		if !isDefer {
			return
		}

		if literal, isLiteral := deferStmt.Call.Fun.(*ast.FuncLit); isLiteral {
			literals[literal] = true
		}
	})

	return literals
}

// inspectScope walks the nodes of one lock scope: the body handed in, with
// nested function literals reported but not entered.
func inspectScope(body *ast.BlockStmt, visit func(node ast.Node)) {
	inspectNode(body, visit)
}

// inspectNode walks one node the way a lock scope is read — everything under
// it, stopping at a nested function literal, which is a scope of its own. It
// takes any node so a branch condition can be read on the same terms as a body.
func inspectNode(root ast.Node, visit func(node ast.Node)) {
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			return false
		}

		visit(node)

		_, isLiteral := node.(*ast.FuncLit)

		return !isLiteral
	})
}

// funcName names a function the way a failure message should: with its
// receiver type when it has one, so Handler.HandleKeyPress is distinguishable
// from a package-level function of the same name.
func funcName(funcDecl *ast.FuncDecl) string {
	if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
		return funcDecl.Name.Name
	}

	return receiverTypeName(funcDecl.Recv.List[0].Type) + "." + funcDecl.Name.Name
}

// modeHandlerTestFileFloor is the fewest test files the mode handler package is
// expected to hold. It carries twenty-two today, so eight catches a filter that
// has lost the package and never fires on a test file being merged away.
const modeHandlerTestFileFloor = 8

// modeHandlerTestFiles returns the parsed test files of the mode handler
// package, off the walk forEachTestFile already does over the checkout.
func modeHandlerTestFiles(t *testing.T) []modeHandlerFile {
	t.Helper()

	var files []modeHandlerFile

	forEachTestFile(t, func(source repoFile, fileSet *token.FileSet, parsed *ast.File) {
		if source.dir != modeHandlerPackageDir {
			return
		}

		files = append(files, modeHandlerFile{
			path:    source.rel,
			isTest:  true,
			syntax:  parsed,
			fileSet: fileSet,
		})
	})

	assertWalkedAtLeast(
		t, "test files in "+modeHandlerPackageDir, len(files), modeHandlerTestFileFloor,
	)

	return files
}
