//go:build integration && darwin

package axobserver

import (
	"os"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

// TestObserverSoakWatchUnwatch drives the real observer bridge through many
// watch/unwatch cycles and asserts that every cycle returns to the idle
// invariant: no Core Foundation object is retained and the run-loop thread is
// stopped.
//
// Registration only succeeds when the test process is accessibility-trusted; a
// plain `go test` process is not, so AXObserverAddNotification is refused and
// the watch tears itself down inside the bridge. The idle invariant must hold
// on that failure path too, which is what this soak pins. When the process is
// trusted, it additionally checks the live-object counts while a process is
// watched.
func TestObserverSoakWatchUnwatch(t *testing.T) {
	pid := os.Getpid()
	watchedAtLeastOnce := false

	const iterations = 1000
	for i := range iterations {
		if darwin.WatchObserver(pid, 0.25) {
			watchedAtLeastOnce = true

			if obs, appEl := darwin.ObserverLiveCounts(); obs < 1 || appEl < 1 {
				t.Fatalf("iteration %d: watching but live counts obs=%d appEl=%d, want >= 1 each",
					i, obs, appEl)
			}

			if !darwin.ObserverThreadRunning() {
				t.Fatalf("iteration %d: watching but run-loop thread not running", i)
			}

			// Re-watching the watched pid is a success no-op: no new objects.
			obsBefore, appElBefore := darwin.ObserverLiveCounts()

			if !darwin.WatchObserver(pid, 0.25) {
				t.Fatalf("iteration %d: re-watching the watched pid should succeed", i)
			}

			if obs, appEl := darwin.ObserverLiveCounts(); obs != obsBefore || appEl != appElBefore {
				t.Fatalf("iteration %d: re-watch created objects obs=%d->%d appEl=%d->%d",
					i, obsBefore, obs, appElBefore, appEl)
			}
		}

		darwin.UnwatchObserver()

		if obs, appEl := darwin.ObserverLiveCounts(); obs != 0 || appEl != 0 {
			t.Fatalf(
				"iteration %d: live counts after unwatch obs=%d appEl=%d, want 0",
				i,
				obs,
				appEl,
			)
		}

		if darwin.ObserverThreadRunning() {
			t.Fatalf("iteration %d: run-loop thread still running at idle", i)
		}
	}

	if watchedAtLeastOnce {
		t.Logf(
			"soak done: %d cycles, including the watched live-object balance assertion",
			iterations,
		)
	} else {
		t.Logf("soak done: %d cycles of teardown and thread lifecycle only; the watched "+
			"live-object balance was NOT checked because this process is not "+
			"accessibility-trusted, so run under a trusted binary to cover it", iterations)
	}
}

// TestWatchRejectsInvalidPID confirms a watch for a pid that cannot name a
// process fails and leaks nothing.
func TestWatchRejectsInvalidPID(t *testing.T) {
	if darwin.WatchObserver(0, 0.25) {
		t.Error("watch with pid 0 should fail")
	}

	if darwin.WatchObserver(-1, 0.25) {
		t.Error("watch with a negative pid should fail")
	}

	if obs, appEl := darwin.ObserverLiveCounts(); obs != 0 || appEl != 0 {
		t.Errorf("rejected watch live counts obs=%d appEl=%d, want 0", obs, appEl)
	}

	if darwin.ObserverThreadRunning() {
		t.Error("run-loop thread should not be running after rejected watches")
	}
}

// TestObserverTeardownWhenIdle confirms unwatch is safe to call with nothing
// watched and no thread running.
func TestObserverTeardownWhenIdle(t *testing.T) {
	darwin.UnwatchObserver()

	if obs, appEl := darwin.ObserverLiveCounts(); obs != 0 || appEl != 0 {
		t.Fatalf("live counts at idle obs=%d appEl=%d, want 0", obs, appEl)
	}

	if darwin.ObserverThreadRunning() {
		t.Fatal("run-loop thread should not be running at idle")
	}
}
