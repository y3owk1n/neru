package electron

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// fakeSetter stands in for the cross-process AX attribute setter. It records
// which attributes it was asked to set, and with which value, and returns a
// programmed result per attribute (a missing entry means the set fails).
type fakeSetter struct {
	calls   []string
	values  []bool
	results map[string]bool
}

func (f *fakeSetter) set(_ int, attribute string, value bool) bool {
	f.calls = append(f.calls, attribute)
	f.values = append(f.values, value)

	return f.results[attribute]
}

func (f *fakeSetter) countOf(attribute string) int {
	count := 0

	for _, called := range f.calls {
		if called == attribute {
			count++
		}
	}

	return count
}

// lastValueOf returns the value of the most recent set of the given attribute,
// and whether the attribute was set at all.
func (f *fakeSetter) lastValueOf(attribute string) (value, found bool) {
	for i := len(f.calls) - 1; i >= 0; i-- {
		if f.calls[i] == attribute {
			return f.values[i], true
		}
	}

	return false, false
}

// newEnableTest swaps in a fake setter, clears the pid cache, and returns the
// fake plus a log observer. Everything is restored when the test ends.
func newEnableTest(t *testing.T, results map[string]bool) (*fakeSetter, *observer.ObservedLogs, *zap.Logger) {
	t.Helper()

	fake := &fakeSetter{results: results}

	prev := setAttribute
	setAttribute = fake.set

	t.Cleanup(func() {
		setAttribute = prev

		enabledPIDsMu.Lock()
		enabledPIDs = make(map[int]axState)
		enabledPIDsMu.Unlock()
	})

	enabledPIDsMu.Lock()
	enabledPIDs = make(map[int]axState)
	enabledPIDsMu.Unlock()

	core, logs := observer.New(zap.DebugLevel)

	return fake, logs, zap.New(core)
}

func stateForPID(pid int) axState {
	enabledPIDsMu.Lock()
	defer enabledPIDsMu.Unlock()

	return enabledPIDs[pid]
}

func TestEnsurePIDManualSetOnceAndCached(t *testing.T) {
	fake, logs, logger := newEnableTest(t, map[string]bool{manualAttributeName: true})

	ensurePIDAccessibility(100, "com.example.app", false, logger)
	ensurePIDAccessibility(100, "com.example.app", false, logger)

	if got := fake.countOf(manualAttributeName); got != 1 {
		t.Fatalf("manual set attempts = %d, want 1 (cached after success)", got)
	}

	if got := fake.countOf(enhancedAttributeName); got != 0 {
		t.Fatalf("enhanced set attempts = %d, want 0 (not requested)", got)
	}

	state := stateForPID(100)
	if !state.manual || state.enhanced {
		t.Fatalf("state = %+v, want manual only", state)
	}

	if got := logs.FilterMessage("manual accessibility set").Len(); got != 1 {
		t.Fatalf("manual success logs = %d, want 1", got)
	}
}

func TestEnsurePIDManualFailureRetriesAndLogsOnce(t *testing.T) {
	fake, logs, logger := newEnableTest(t, map[string]bool{manualAttributeName: false})

	ensurePIDAccessibility(100, "com.example.app", false, logger)
	ensurePIDAccessibility(100, "com.example.app", false, logger)

	if got := fake.countOf(manualAttributeName); got != 2 {
		t.Fatalf("manual set attempts = %d, want 2 (a failed set is retried)", got)
	}

	state := stateForPID(100)
	if state.manual {
		t.Fatalf("state.manual = true, want false after a failed set")
	}

	if !state.manualFailed {
		t.Fatalf("state.manualFailed = false, want true")
	}

	if got := logs.FilterMessage("manual accessibility set failed").Len(); got != 1 {
		t.Fatalf("manual failure logs = %d, want 1 (logged once, not per focus)", got)
	}
}

func TestEnsurePIDEnhancedOnlyWhenRequested(t *testing.T) {
	results := map[string]bool{manualAttributeName: true, enhancedAttributeName: true}

	fake, logs, logger := newEnableTest(t, results)

	ensurePIDAccessibility(100, "com.brave.Browser", false, logger)

	if got := fake.countOf(enhancedAttributeName); got != 0 {
		t.Fatalf("enhanced set attempts = %d, want 0 when useEnhanced is false", got)
	}

	if state := stateForPID(100); state.enhanced {
		t.Fatalf("state.enhanced = true, want false when useEnhanced is false")
	}

	if got := logs.FilterMessage("enhanced accessibility set for web content").Len(); got != 0 {
		t.Fatalf("enhanced logs = %d, want 0", got)
	}
}

func TestEnsurePIDEnhancedSetWhenRequested(t *testing.T) {
	results := map[string]bool{manualAttributeName: true, enhancedAttributeName: true}

	fake, logs, logger := newEnableTest(t, results)

	ensurePIDAccessibility(100, "com.brave.Browser", true, logger)
	ensurePIDAccessibility(100, "com.brave.Browser", true, logger)

	if got := fake.countOf(enhancedAttributeName); got != 1 {
		t.Fatalf("enhanced set attempts = %d, want 1 (cached after success)", got)
	}

	state := stateForPID(100)
	if !state.manual || !state.enhanced {
		t.Fatalf("state = %+v, want manual and enhanced", state)
	}

	if got := logs.FilterMessage("enhanced accessibility set for web content").Len(); got != 1 {
		t.Fatalf("enhanced success logs = %d, want 1", got)
	}
}

func TestEnsurePIDEnhancedFailureRetriesAndLogsOnce(t *testing.T) {
	results := map[string]bool{manualAttributeName: true, enhancedAttributeName: false}

	fake, logs, logger := newEnableTest(t, results)

	ensurePIDAccessibility(100, "com.brave.Browser", true, logger)
	ensurePIDAccessibility(100, "com.brave.Browser", true, logger)

	if got := fake.countOf(enhancedAttributeName); got != 2 {
		t.Fatalf("enhanced set attempts = %d, want 2 (a failed set is retried)", got)
	}

	state := stateForPID(100)
	if state.enhanced || !state.enhancedFailed {
		t.Fatalf("state = %+v, want enhanced false and enhancedFailed true", state)
	}

	if got := logs.FilterMessage("enhanced accessibility set failed").Len(); got != 1 {
		t.Fatalf("enhanced failure logs = %d, want 1", got)
	}
}

func TestEnsurePIDReuseResetsState(t *testing.T) {
	fake, _, logger := newEnableTest(t, map[string]bool{manualAttributeName: true})

	ensurePIDAccessibility(100, "com.first.app", false, logger)
	ensurePIDAccessibility(100, "com.second.app", false, logger)

	if got := fake.countOf(manualAttributeName); got != 2 {
		t.Fatalf("manual set attempts = %d, want 2 (pid reused by a different bundle)", got)
	}

	if state := stateForPID(100); state.bundle != "com.second.app" {
		t.Fatalf("state.bundle = %q, want com.second.app", state.bundle)
	}
}

func TestEnsurePIDSameBundleCaseInsensitiveKeepsCache(t *testing.T) {
	fake, _, logger := newEnableTest(t, map[string]bool{manualAttributeName: true})

	ensurePIDAccessibility(100, "com.example.App", false, logger)
	ensurePIDAccessibility(100, "COM.EXAMPLE.APP", false, logger)

	if got := fake.countOf(manualAttributeName); got != 1 {
		t.Fatalf("manual set attempts = %d, want 1 (same bundle, different case)", got)
	}
}

func TestEnsurePIDReportsReadiness(t *testing.T) {
	fake, _, logger := newEnableTest(t, map[string]bool{
		manualAttributeName:   true,
		enhancedAttributeName: false,
	})

	// Manual only: ready as soon as the manual set succeeds.
	if ready, _ := ensurePIDAccessibility(100, "com.example.app", false, logger); !ready {
		t.Fatal("manual-only ready = false, want true after the manual set succeeds")
	}

	// Enhanced wanted but its set fails: not ready, so the caller retries.
	if ready, _ := ensurePIDAccessibility(200, "com.brave.Browser", true, logger); ready {
		t.Fatal("ready = true, want false while the enhanced set keeps failing")
	}

	fake.results[enhancedAttributeName] = true

	if ready, _ := ensurePIDAccessibility(200, "com.brave.Browser", true, logger); !ready {
		t.Fatal("ready = false, want true once every wanted attribute is set")
	}
}

func TestEnsurePIDFirstEncounterGuidesRetry(t *testing.T) {
	_, _, logger := newEnableTest(t, map[string]bool{manualAttributeName: false})

	// First time this pid is seen: worth a backoff retry in case the app is
	// still bringing its accessibility tree up.
	if _, retry := ensurePIDAccessibility(100, "com.example.app", false, logger); !retry {
		t.Fatal("retry = false on first encounter, want true so a slow launch is retried")
	}

	// Seen before and still failing: a known app, so no retry burst on a later
	// focus.
	if _, retry := ensurePIDAccessibility(100, "com.example.app", false, logger); retry {
		t.Fatal("retry = true on a later focus, want false so native apps are not re-probed")
	}

	// A different process reusing the pid is a fresh encounter again.
	if _, retry := ensurePIDAccessibility(100, "com.other.app", false, logger); !retry {
		t.Fatal("retry = false after pid reuse, want true for the new process")
	}
}

func TestEnsurePIDEnhancedClearedWhenDisabled(t *testing.T) {
	results := map[string]bool{manualAttributeName: true, enhancedAttributeName: true}

	fake, logs, logger := newEnableTest(t, results)

	// Enhanced is on for a browser, then web-content hints are turned off.
	ensurePIDAccessibility(100, "com.brave.Browser", true, logger)
	ensurePIDAccessibility(100, "com.brave.Browser", false, logger)

	if value, found := fake.lastValueOf(enhancedAttributeName); !found || value {
		t.Fatalf("last enhanced set = (value=%v, found=%v), want (false, true) to clear it", value, found)
	}

	if state := stateForPID(100); state.enhanced {
		t.Fatal("state.enhanced = true, want false after the attribute is cleared")
	}

	if got := logs.FilterMessage("enhanced accessibility cleared").Len(); got != 1 {
		t.Fatalf("enhanced cleared logs = %d, want 1", got)
	}

	// Re-focusing with hints still off does not clear again.
	ensurePIDAccessibility(100, "com.brave.Browser", false, logger)

	if got := fake.countOf(enhancedAttributeName); got != 2 {
		t.Fatalf("enhanced set attempts = %d, want 2 (one to set, one to clear)", got)
	}
}

func TestForgetAppAccessibilityResetsState(t *testing.T) {
	fake, _, logger := newEnableTest(t, map[string]bool{manualAttributeName: true})

	ensurePIDAccessibility(100, "com.example.App", false, logger)

	// An empty bundle id must not wipe unrelated entries.
	ForgetAppAccessibility("")

	if state := stateForPID(100); state.bundle == "" {
		t.Fatal("state was cleared by an empty-bundle forget, want it kept")
	}

	// The app terminates; its cached state is dropped (bundle match is
	// case-insensitive, like the pid-reuse guard).
	ForgetAppAccessibility("COM.EXAMPLE.APP")

	if state := stateForPID(100); state.bundle != "" {
		t.Fatalf("state = %+v, want empty after the app is forgotten", state)
	}

	// A new process reuses the retired pid: the attribute is set again.
	ensurePIDAccessibility(100, "com.example.App", false, logger)

	if got := fake.countOf(manualAttributeName); got != 2 {
		t.Fatalf("manual set attempts = %d, want 2 (cache cleared on terminate, so the reused pid is set again)", got)
	}
}
