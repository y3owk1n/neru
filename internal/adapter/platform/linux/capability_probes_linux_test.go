//go:build linux

//nolint:testpackage
package linux

import (
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"
)

// This is an internal test: reaching the probe registry needs its unexported
// type.
const testProbeTimeout = 20 * time.Millisecond

// errProbeFailed is a static error so probe results can be compared with
// errors.Is.
var errProbeFailed = errors.New("probe failed")

// Capability probing runs each native call on its own goroutine so a wedged
// display server cannot hang `neru doctor` or an IPC info request. Those native
// calls cannot be canceled, so a probe that never returns keeps its goroutine
// for the life of the process — and Capabilities is reached from two IPC
// handlers in the long-lived daemon.
//
// Without a cap, every status or health request against a stuck backend would
// start three more probes, growing goroutines and native handles without bound
// for as long as the backend stayed wedged. These tests pin the cap at the
// mechanism itself rather than through the adapter: on any real backend the
// probes return immediately, so an adapter-level test would pass whether the
// cap existed or not.
//
// TestCapabilityProbes_WedgedProbeIsNotRestarted is the leak regression: while
// one probe is stuck, further requests must not start another.
func TestCapabilityProbes_WedgedProbeIsNotRestarted(t *testing.T) {
	probes := newCapabilityProbes()

	release := make(chan struct{})
	defer close(release)

	var starts atomic64

	wedged := func() error {
		starts.add(1)
		<-release

		return nil
	}

	// First request starts the probe and times out waiting for it.
	if completed, _ := probes.run("wedged", testProbeTimeout, wedged); completed {
		t.Fatal("a probe that never returns reported completion")
	}

	before := goroutineFloor()

	const requests = 200

	for range requests {
		_, _ = probes.run("wedged", testProbeTimeout, wedged)
	}

	if got := starts.load(); got != 1 {
		t.Errorf("probe was started %d times across %d requests, want exactly 1", got, requests+1)
	}

	after := goroutineFloor()

	// One stuck goroutine is expected and unavoidable; hundreds are the leak.
	const tolerance = 10

	if after > before+tolerance {
		t.Errorf(
			"goroutines grew from %d to %d across %d requests; stuck probes are accumulating",
			before, after, requests,
		)
	}
}

// TestCapabilityProbes_ReleasedProbeFreesTheSlot checks the cap is not a
// one-way latch: once a stuck probe finally returns, probing resumes.
func TestCapabilityProbes_ReleasedProbeFreesTheSlot(t *testing.T) {
	probes := newCapabilityProbes()

	release := make(chan struct{})

	var starts atomic64

	slow := func() error {
		starts.add(1)
		<-release

		return nil
	}

	if completed, _ := probes.run("slow", testProbeTimeout, slow); completed {
		t.Fatal("a blocked probe reported completion")
	}

	close(release)

	// Wait for the slot to be handed back.
	deadline := time.Now().Add(2 * time.Second)
	for {
		completed, err := probes.run("slow", testProbeTimeout, func() error {
			starts.add(1)

			return nil
		})

		if completed && err == nil {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("probe slot was never released after the blocked probe returned")
		}

		time.Sleep(5 * time.Millisecond)
	}

	if got := starts.load(); got < 2 {
		t.Errorf("probe ran %d times, want the slot to be reusable after release", got)
	}
}

// TestCapabilityProbes_ReportsLastResultWhileBusy checks a request that arrives
// while a probe is stuck reuses the previous answer rather than degrading the
// capability to "timed out" on every subsequent call.
func TestCapabilityProbes_ReportsLastResultWhileBusy(t *testing.T) {
	probes := newCapabilityProbes()

	// A completed run records its result.
	completed, err := probes.run("feature", time.Second, func() error { return errProbeFailed })
	if !completed {
		t.Fatal("a probe that returns immediately reported no completion")
	}

	if !errors.Is(err, errProbeFailed) {
		t.Fatalf("run returned %v, want %v", err, errProbeFailed)
	}

	// A later probe wedges; requests during it fall back to the recorded result.
	release := make(chan struct{})
	defer close(release)

	if wedgedCompleted, _ := probes.run("feature", testProbeTimeout, func() error {
		<-release

		return nil
	}); wedgedCompleted {
		t.Fatal("a probe that never returns reported completion")
	}

	completed, err = probes.run("feature", testProbeTimeout, func() error { return nil })
	if !completed {
		t.Error("a request during a wedged probe reported no result despite an earlier one")
	}

	if !errors.Is(err, errProbeFailed) {
		t.Errorf("request during a wedged probe returned %v, want the last known %v",
			err, errProbeFailed)
	}
}

// TestCapabilityProbes_SeparateFeaturesDoNotBlockEachOther checks the cap is
// per feature: a wedged screen probe must not stop cursor or process reporting.
func TestCapabilityProbes_SeparateFeaturesDoNotBlockEachOther(t *testing.T) {
	probes := newCapabilityProbes()

	release := make(chan struct{})
	defer close(release)

	if completed, _ := probes.run("screen", testProbeTimeout, func() error {
		<-release

		return nil
	}); completed {
		t.Fatal("a probe that never returns reported completion")
	}

	completed, err := probes.run("cursor", time.Second, func() error { return nil })
	if !completed || err != nil {
		t.Errorf("cursor probe returned (completed=%t, err=%v) while screen was wedged; "+
			"features must not share a slot", completed, err)
	}
}

// goroutineFloor returns the lowest goroutine count observed over a short
// window, so a straggler still winding down is not mistaken for a leak.
func goroutineFloor() int {
	lowest := runtime.NumGoroutine()

	for range 20 {
		time.Sleep(5 * time.Millisecond)

		if current := runtime.NumGoroutine(); current < lowest {
			lowest = current
		}
	}

	return lowest
}

// atomic64 is a minimal mutex-guarded counter, avoiding an import solely for a
// test helper.
type atomic64 struct {
	mu sync.Mutex
	n  int
}

func (a *atomic64) add(delta int) {
	a.mu.Lock()
	a.n += delta
	a.mu.Unlock()
}

func (a *atomic64) load() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.n
}
