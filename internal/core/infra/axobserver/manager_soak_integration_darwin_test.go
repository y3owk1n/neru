//go:build darwin

package axobserver

import (
	"os"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// TestNativeObserverSoakBalance drives the real native AXObserver layer through
// many arm/disarm cycles and asserts there is no CFObject or thread leak: the
// created-minus-released counts return to zero and the run-loop thread is idle.
//
// It uses the test process's own pid as the observation target. Whether native
// notification registration succeeds depends on whether the test process is
// accessibility-trusted (it usually is not under `go test`), but every
// AXObserver / application element that is created is released on both the
// success and the failure path, so the CFRelease balance must hold either way —
// that balance is the leak assertion. The trust state is logged so it is clear
// which path this run exercised.
func TestNativeObserverSoakBalance(t *testing.T) {
	pid := os.Getpid()

	if darwin.CheckAccessibilityPermissions() {
		t.Log("process is accessibility-trusted: soak exercises live observer arm/disarm")
	} else {
		t.Log("process is NOT accessibility-trusted: soak exercises the create/release " +
			"failure-path balance (arm returns nil after releasing); still a valid leak check")
	}

	// SelfPID -1 never matches, so our own pid is not excluded and is used purely
	// as a lifecycle target.
	m := NewManager(nil, Config{SelfPID: -1})
	defer m.Close()

	targets := []ports.ObservationTarget{
		{PID: pid, BundleID: "test.self", Source: ports.ObservationFrontWindow},
	}

	const iterations = 500

	for range iterations {
		// Blocking sends so no cycle is dropped under load.
		m.cmds <- reconcileCmd{targets: targets}
		m.cmds <- reconcileCmd{targets: nil}
	}

	m.flush()

	if m.ThreadRunning() {
		t.Fatal("observer thread must be idle after the soak (no observers armed)")
	}

	if got := m.LiveObservers(); got != 0 {
		t.Fatalf("live observers after soak = %d, want 0", got)
	}

	obs, appEls := darwin.ObserverLiveCounts()
	if obs != 0 || appEls != 0 {
		t.Fatalf("CFRelease imbalance after soak: observers=%d appElements=%d", obs, appEls)
	}
}
