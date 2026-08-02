//go:build integration && darwin

package accessibility_test

import (
	"sync"
	"testing"
	"time"

	darwinplatform "github.com/y3owk1n/neru/internal/adapter/platform/darwin"
)

// integrationScanBudget bounds a live-AX test call. It is a hang guard, not a
// performance assertion: the whole suite takes a couple of seconds when the AX
// API answers, so anything approaching this means a call is wedged.
const integrationScanBudget = 30 * time.Second

// runWithinBudget runs work on its own goroutine and fails the test if it has
// not returned within integrationScanBudget.
//
// A context deadline cannot do this job. The AX client takes a context and
// discards it — see InfraAXClient.FrontmostWindow in infra_client.go, whose
// parameter is `_ context.Context` — so once a query is inside the Objective-C
// bridge nothing observes cancellation. The context the suite passes is still
// worth having, because the scan's per-source goroutines check it before
// starting further work, but it cannot interrupt a call already in the bridge.
// Only a watchdog outside that call can.
//
// Without this the failure mode is Go's own -timeout (10 minutes by default)
// firing with a panic and a full goroutine dump, ten minutes after the run
// stopped making progress and with no statement of which call wedged. This
// fails in 30 seconds and names it.
//
// The goroutine is deliberately abandoned on timeout: it is blocked in a native
// call and cannot be reclaimed. The test binary is exiting anyway.
//
// work must not call t.Fatal — it runs off the test goroutine. Have it assign
// results and assert on them after this returns.
func runWithinBudget(t *testing.T, what string, work func()) {
	t.Helper()

	done := make(chan struct{})

	go func() {
		defer close(done)

		work()
	}()

	select {
	case <-done:
	case <-time.After(integrationScanBudget):
		t.Fatalf(
			"%s did not return within %s; the accessibility server of the frontmost "+
				"application is not answering, so the call is stuck in the native AX bridge",
			what,
			integrationScanBudget,
		)
	}
}

// hasInputPermission reports whether this process has been granted macOS
// Accessibility permission. It is resolved once: the answer cannot change
// during a run.
var hasInputPermission = sync.OnceValue(darwinplatform.CheckAccessibilityPermissions)

// requireInputPermission skips a test that cannot work without macOS
// Accessibility permission.
//
// Reading the AX tree and posting synthetic input are behind the same TCC gate,
// and a `go test` binary is not normally granted it. Without the grant the
// window server drops the posted moves, so the cursor never leaves its starting
// point and the AX query cannot find a frontmost window — and the tests report
// that as a failure, which is a claim about the code that is not true. What
// failed is the environment.
//
// A suite that is reliably red for a reason the developer cannot act on is a
// suite people learn to ignore, and it also stops `just test` from being usable
// as evidence that a change is sound. Skipping says what is actually happening.
//
// This does not lose coverage where it counts: the macOS CI runner does have the
// permission, so these tests run there for real on every PR. To run them locally,
// grant Accessibility to the terminal (or to the test binary) under System
// Settings > Privacy & Security > Accessibility.
func requireInputPermission(t *testing.T) {
	t.Helper()

	if !hasInputPermission() {
		t.Skip(
			"macOS Accessibility permission is not granted to this process, so posted " +
				"input is dropped and the AX tree is unreadable; grant it under System " +
				"Settings > Privacy & Security > Accessibility to run this",
		)
	}
}
