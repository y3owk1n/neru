//go:build integration && windows

package windows

import (
	"syscall"
	"testing"
)

// Real Win32 tests for the process-wide callback registrations.
//
// Go's runtime keys registered callbacks on the function value, never frees a
// slot, and has a fixed cb_max = 2000 of them (runtime/zcallback_windows.go).
// A path that registers one per call therefore ends the process on "too many
// callback functions" — a runtime throw, unrecoverable — after enough mode
// activations. These tests count the slots a workload consumes and require the
// answer to be zero.
//
// They are in-package because the enumeration path they drive is unexported,
// and integration-tagged because they call the live Win32 APIs.

// callbackWorkloadRuns is how many times a workload is repeated inside a
// measurement. The count only has to be large enough that "one per run" is
// unmistakable next to "one for the process"; the measurement is exact, so
// there is no need to approach the table's size and no reason to install two
// thousand real keyboard hooks to prove it.
const callbackWorkloadRuns = 20

// newCallbackProbe registers one throwaway callback and returns its address.
//
// The closure captures tag so that each probe is a distinct function value:
// the runtime returns the existing slot for a function value it has already
// seen, and two identical non-capturing literals may share one static value.
func newCallbackProbe(tag int) uintptr {
	return syscall.NewCallback(func() uintptr {
		return uintptr(tag)
	})
}

// countCallbackRegistrations reports how many callback slots work consumed.
//
// It reads the one property the runtime exposes without an API: a registered
// callback's address is its slot index into a single contiguous table, so
// consecutive registrations sit a fixed stride apart. Two probes before the
// workload measure that stride, a third after it measures the gap, and the
// slots work took are the ones in between.
//
// A runtime that lays callbacks out some other way is skipped rather than
// guessed at — a check that cannot measure must say so instead of passing.
func countCallbackRegistrations(t *testing.T, work func()) int {
	t.Helper()

	first := newCallbackProbe(1)
	second := newCallbackProbe(2)

	if second <= first {
		t.Skipf(
			"skipping: callback addresses are not ascending (%#x then %#x), so this "+
				"runtime's callback slots cannot be counted this way",
			first, second,
		)
	}

	stride := second - first

	work()

	third := newCallbackProbe(3)

	if third <= second {
		t.Fatalf(
			"callback addresses stopped ascending (%#x then %#x); the slot count "+
				"below would be meaningless",
			second, third,
		)
	}

	gap := third - second
	if gap%stride != 0 {
		t.Fatalf(
			"callback addresses are %d apart after a stride of %d; the table is not "+
				"the uniform layout this count assumes",
			gap, stride,
		)
	}

	return int(gap/stride) - 1
}

// TestEnumerateMonitors_RegistersOneCallbackForTheProcess holds the monitor
// enumeration path to a single callback registration.
//
// enumerateMonitors is on the mode-activation path — activeScreenBounds,
// screenBoundsByName, screenNames and NewOverlayWindow all reach it — so a
// registration per pass was the shortest countdown of the two.
//
// The first pass outside the measurement is what registers that one callback.
// The enumeration results are deliberately dropped: a headless or session-0
// runner legitimately reports no monitors, and what is being counted is the
// same either way.
func TestEnumerateMonitors_RegistersOneCallbackForTheProcess(t *testing.T) {
	_, _ = enumerateMonitors()

	registered := countCallbackRegistrations(t, func() {
		for range callbackWorkloadRuns {
			_, _ = enumerateMonitors()
		}
	})

	if registered != 0 {
		t.Fatalf(
			"%d enumeration passes registered %d callbacks; each one is a slot the "+
				"process never gets back",
			callbackWorkloadRuns, registered,
		)
	}
}

// TestEnumerateMonitors_DoesNotCarryStateBetweenPasses pins the cost of the
// single callback: what it appends to is a package variable rather than a
// per-call capture, so a pass that failed to install or clear it would hand the
// caller the previous pass's monitors as well as its own.
//
// Two passes over an unchanged display therefore have to report the same count.
func TestEnumerateMonitors_DoesNotCarryStateBetweenPasses(t *testing.T) {
	first, err := enumerateMonitors()
	if err != nil {
		t.Skipf("skipping: no monitors to enumerate (%v)", err)
	}

	second, err := enumerateMonitors()
	if err != nil {
		t.Fatalf("second pass after a first that found %d monitors: %v", len(first), err)
	}

	if len(second) != len(first) {
		t.Fatalf(
			"consecutive passes reported %d then %d monitors; the shared collector is "+
				"carrying state across passes",
			len(first), len(second),
		)
	}
}

// TestStartKeyboardHook_RegistersOneCallbackForTheProcess holds the keyboard
// hook to a single callback registration across install cycles.
//
// EventTap.Enable calls StartKeyboardHook on every enable cycle, so this is the
// same countdown on the input path. Installing real hooks is what makes this
// test worth having: it counts what the code path actually registers rather
// than what the accessor returns.
//
// The hook's key callback returns false throughout, so every key seen while the
// test runs is passed straight on to the rest of the system.
func TestStartKeyboardHook_RegistersOneCallbackForTheProcess(t *testing.T) {
	passThrough := func(string, bool) bool { return false }

	warmUp, err := StartKeyboardHook(passThrough)
	if err != nil {
		t.Skipf("skipping: cannot install a keyboard hook here (%v)", err)
	}

	warmUp.Stop()

	var installErr error

	registered := countCallbackRegistrations(t, func() {
		for range callbackWorkloadRuns {
			hook, err := StartKeyboardHook(passThrough)
			if err != nil {
				installErr = err

				return
			}

			hook.Stop()
		}
	})

	if installErr != nil {
		t.Fatalf("installing a keyboard hook after a successful first install: %v", installErr)
	}

	if registered != 0 {
		t.Fatalf(
			"%d install cycles registered %d callbacks; each one is a slot the process "+
				"never gets back",
			callbackWorkloadRuns, registered,
		)
	}
}
