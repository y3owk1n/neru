//go:build integration && darwin

package axobserver

import (
	"os"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

// TestObserverSoakArmDisarm drives the real observer bridge through many
// start/arm/disarm/stop cycles and asserts that every cycle returns to the idle
// invariant: no Core Foundation object is retained and the run-loop thread is
// stopped.
//
// Registration only succeeds when the test process is accessibility-trusted; a
// plain `go test` process is not, so AXObserverAddNotification is refused and the
// observer is torn down inside Arm. The idle invariant must hold on that failure
// path too, which is what this soak pins. When the process is trusted, it
// additionally checks the live-object counts while an observer is armed.
func TestObserverSoakArmDisarm(t *testing.T) {
	pid := os.Getpid()
	mask := darwin.AXNotifCreated | darwin.AXNotifWindowCreated
	armedAtLeastOnce := false

	const iterations = 1000
	for i := range iterations {
		darwin.StartObserverThread()

		if !darwin.ObserverThreadRunning() {
			t.Fatalf("iteration %d: run-loop thread did not start", i)
		}

		if handle := darwin.ArmObserver(pid, mask, 0.25); handle != nil {
			armedAtLeastOnce = true

			if obs, appEl := darwin.ObserverLiveCounts(); obs < 1 || appEl < 1 {
				t.Fatalf("iteration %d: armed but live counts obs=%d appEl=%d, want >= 1 each",
					i, obs, appEl)
			}

			darwin.DisarmObserver(handle, true)
		}

		darwin.StopObserverThread()

		if obs, appEl := darwin.ObserverLiveCounts(); obs != 0 || appEl != 0 {
			t.Fatalf("iteration %d: live counts after teardown obs=%d appEl=%d, want 0", i, obs, appEl)
		}

		if darwin.ObserverThreadRunning() {
			t.Fatalf("iteration %d: run-loop thread still running at idle", i)
		}
	}

	if armedAtLeastOnce {
		t.Logf("soak done: %d cycles, including the armed live-object balance assertion", iterations)
	} else {
		t.Logf("soak done: %d cycles of teardown and thread lifecycle only; the armed "+
			"live-object balance was NOT checked because this process is not "+
			"accessibility-trusted, so run under a trusted binary to cover it", iterations)
	}
}

// TestArmRejectsUnsupportedMask confirms an arm whose mask names no known
// notification is rejected and leaks nothing.
func TestArmRejectsUnsupportedMask(t *testing.T) {
	pid := os.Getpid()

	darwin.StartObserverThread()
	t.Cleanup(darwin.StopObserverThread)

	if darwin.ArmObserver(pid, 0, 0.25) != nil {
		t.Error("arm with a zero mask should fail")
	}

	if darwin.ArmObserver(pid, darwin.AXObserverMask(1<<20), 0.25) != nil {
		t.Error("arm with an unmappable mask should fail")
	}

	if obs, appEl := darwin.ObserverLiveCounts(); obs != 0 || appEl != 0 {
		t.Errorf("rejected arm live counts obs=%d appEl=%d, want 0", obs, appEl)
	}
}

// TestObserverTeardownWhenIdle confirms the teardown entry points are safe to
// call with nothing armed and no thread running.
func TestObserverTeardownWhenIdle(t *testing.T) {
	darwin.DisarmObserver(nil, true)
	darwin.StopObserverThread()

	if obs, appEl := darwin.ObserverLiveCounts(); obs != 0 || appEl != 0 {
		t.Fatalf("live counts at idle obs=%d appEl=%d, want 0", obs, appEl)
	}

	if darwin.ObserverThreadRunning() {
		t.Fatal("run-loop thread should not be running at idle")
	}
}
