package architecture_test

import (
	"fmt"
	"go/ast"
	"testing"
)

// transitionExitMethod is the exit that hands the keyboard to the mode coming
// up instead of giving it back, and transitionReleaseMethod is the release it
// returns.
const (
	transitionExitMethod    = "exitModeForTransition"
	transitionReleaseMethod = "releaseKeyboardIfNoModeEntered"
)

// transitionExitFloor is the fewest transition exits expected in the mode
// handler package. Four are there today — grid, recursive grid, hints and
// monitor select — so three catches a walk that has lost the package or stopped
// recognizing the call, and never fires on one mode's activation being
// restructured away.
//
// The floor is what matters most here: a reader that found no call sites would
// report the package compliant while reading none of them.
const transitionExitFloor = 3

// TestModeTransitionExitsDeferTheirRelease fails when internal/app/modes calls
// exitModeForTransition as anything but `defer h.exitModeForTransition()()`.
//
// The exit leaves the keyboard captured on purpose: releasing it on a
// mode-to-mode transition opens a window, spanning the whole of the next mode's
// activation, in which nobody holds the keyboard and everything typed reaches
// the focused application instead of Neru. What it costs is that the capture is
// then live while the app is momentarily idle, so the activation owes a release
// on every path that fails to enter a mode — and hints alone has five of those,
// counting the permission dialog that suspends the activation and returns.
//
// Forgetting it leaves the daemon idle holding the keyboard: every key the user
// presses goes nowhere at all, which is worse than the bug the exit exists to
// fix. Nothing else catches it. The bare call compiles, discarding a return
// value is legal Go, `just lint` is clean on it, and both journey tests in
// internal/app only walk the four activations that exist today — so a fifth
// mode written next year ships the breach green (ADR 0011).
//
// The pinned form is the one the package uses, and the strictness is the point:
// `defer h.exitModeForTransition()()` runs the exit at the defer statement and
// the release at return, in one expression that cannot be half-written. Two
// statements that split the call from its defer are as correct at runtime and
// still fail here, because the shape that can be got half-right is exactly the
// shape this is meant to keep out — the failure names the form to use rather
// than trying to decide which half-written versions happen to work.
//
// Test files are outside this, and that is a scope rather than an exemption:
// a test calling the exit directly is exercising the exit, not performing a
// transition, and owes nothing to a mode that is never entered.
func TestModeTransitionExitsDeferTheirRelease(t *testing.T) {
	exits := modeTransitionExits(t)

	for _, exit := range exits {
		if exit.deferredWithRelease {
			continue
		}

		t.Errorf(
			"%s: %s calls %s without deferring the release it returns; write it as "+
				"`defer h.%s()()` so the exit runs now and the release runs at return "+
				"— an activation that gives up after this must not leave idle holding "+
				"the keyboard (internal/app/modes/AGENTS.md, the mode-transition "+
				"keyboard contract)",
			exit.position, exit.scope, transitionExitMethod, transitionExitMethod,
		)
	}
}

// TestModeTransitionReleaseIsReachedThroughTheExit fails when internal/app/modes
// calls releaseKeyboardIfNoModeEntered directly.
//
// The release is the exit's return value, so the two cannot drift apart: a call
// site that reaches for the release on its own is either pairing it by hand —
// the shape the test above exists to keep out — or releasing a capture no
// transition took, which is a mode losing the keyboard mid-activation. Reaching
// it only through the exit is what makes "the exit owes a release" a property of
// the exit rather than of everyone who calls it.
func TestModeTransitionReleaseIsReachedThroughTheExit(t *testing.T) {
	for _, file := range modeHandlerSourceFiles(t) {
		forEachLockScope(file.syntax, func(scope lockScope) {
			// The exit itself returns the release by name, which is the one
			// reference that is meant to exist.
			if scope.name == "handlerState."+transitionExitMethod {
				return
			}

			inspectScope(scope.body, func(node ast.Node) {
				selector, isSelector := node.(*ast.SelectorExpr)
				if !isSelector || selector.Sel.Name != transitionReleaseMethod {
					return
				}

				t.Errorf(
					"%s:%d: %s names %s directly; it is what %s returns and is reached "+
						"only through it, so the exit and its release cannot be paired by "+
						"hand or run without one (internal/app/modes/AGENTS.md, the "+
						"mode-transition keyboard contract)",
					file.path,
					file.fileSet.Position(selector.Pos()).Line,
					scope.name,
					transitionReleaseMethod,
					transitionExitMethod,
				)
			})
		})
	}
}

// transitionExit is one call to exitModeForTransition in the mode handler
// package, with what its call site did about the release.
type transitionExit struct {
	// position is the slash-relative path with the line number, so a failure is
	// clickable.
	position string
	// scope names the function the call sits in.
	scope string
	// deferredWithRelease reports whether the call is written as the pinned
	// `defer h.exitModeForTransition()()` — the inner call of a deferred outer
	// call, so the returned release runs at the scope's return.
	deferredWithRelease bool
}

// modeTransitionExits returns every call to exitModeForTransition in the mode
// handler package's source, tests excluded.
func modeTransitionExits(t *testing.T) []transitionExit {
	t.Helper()

	var exits []transitionExit

	for _, file := range modeHandlerSourceFiles(t) {
		forEachLockScope(file.syntax, func(scope lockScope) {
			deferredInnerCalls := deferredInnerCallsIn(scope.body)

			inspectScope(scope.body, func(node ast.Node) {
				call, isCall := node.(*ast.CallExpr)
				if !isCall {
					return
				}

				selector, isSelector := call.Fun.(*ast.SelectorExpr)
				if !isSelector || selector.Sel.Name != transitionExitMethod {
					return
				}

				exits = append(exits, transitionExit{
					position: fmt.Sprintf(
						"%s:%d",
						file.path,
						file.fileSet.Position(call.Pos()).Line,
					),
					scope:               scope.name,
					deferredWithRelease: deferredInnerCalls[call],
				})
			})
		})
	}

	assertWalkedAtLeast(
		t,
		transitionExitMethod+" calls in "+modeHandlerPackageDir,
		len(exits),
		transitionExitFloor,
	)

	return exits
}

// deferredInnerCallsIn returns the inner calls of a scope's deferred double
// calls: for `defer f()()` it holds the `f()`, which is the call that runs at
// the defer statement and whose result is what runs at return.
func deferredInnerCallsIn(body *ast.BlockStmt) map[*ast.CallExpr]bool {
	inner := make(map[*ast.CallExpr]bool)

	inspectScope(body, func(node ast.Node) {
		deferStmt, isDefer := node.(*ast.DeferStmt)
		if !isDefer {
			return
		}

		if call, isCall := deferStmt.Call.Fun.(*ast.CallExpr); isCall {
			inner[call] = true
		}
	})

	return inner
}
