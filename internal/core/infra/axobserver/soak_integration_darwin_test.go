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
	armedAtLeastOnce := false

	const iterations = 1000
	for i := range iterations {
		darwin.StartObserverThread()

		if !darwin.ObserverThreadRunning() {
			t.Fatalf("iteration %d: run-loop thread did not start", i)
		}

		if handle := darwin.ArmObserver(pid, 0.25); handle != nil {
			armedAtLeastOnce = true

			if obs, appEl := darwin.ObserverLiveCounts(); obs < 1 || appEl < 1 {
				t.Fatalf("iteration %d: armed but live counts obs=%d appEl=%d, want >= 1 each",
					i, obs, appEl)
			}

			darwin.DisarmObserver(handle, true)
		}

		darwin.StopObserverThread()

		if obs, appEl := darwin.ObserverLiveCounts(); obs != 0 || appEl != 0 {
			t.Fatalf(
				"iteration %d: live counts after teardown obs=%d appEl=%d, want 0",
				i,
				obs,
				appEl,
			)
		}

		if darwin.ObserverThreadRunning() {
			t.Fatalf("iteration %d: run-loop thread still running at idle", i)
		}
	}

	if armedAtLeastOnce {
		t.Logf(
			"soak done: %d cycles, including the armed live-object balance assertion",
			iterations,
		)
	} else {
		t.Logf("soak done: %d cycles of teardown and thread lifecycle only; the armed "+
			"live-object balance was NOT checked because this process is not "+
			"accessibility-trusted, so run under a trusted binary to cover it", iterations)
	}
}

// TestArmRejectsInvalidPID confirms an arm for a pid that cannot name a process
// is rejected and leaks nothing.
func TestArmRejectsInvalidPID(t *testing.T) {
	darwin.StartObserverThread()
	t.Cleanup(darwin.StopObserverThread)

	if darwin.ArmObserver(0, 0.25) != nil {
		t.Error("arm with pid 0 should fail")
	}

	if darwin.ArmObserver(-1, 0.25) != nil {
		t.Error("arm with a negative pid should fail")
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
