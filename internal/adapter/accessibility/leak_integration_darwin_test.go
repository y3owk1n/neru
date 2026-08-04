//go:build integration && darwin

package accessibility_test

import (
	"context"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/accessibility"
	"github.com/y3owk1n/neru/internal/adapter/accessibility/native"
	nativedarwin "github.com/y3owk1n/neru/internal/adapter/accessibility/native/darwin"
	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/ports"
)

// leakWalkCount is how many full clickable-element scans the regression test
// runs. The leak class this test pins (an element enqueued by a traversal and
// never released) leaks per walk, so a handful of walks separates a real leak
// from noise without slowing the suite.
const leakWalkCount = 5

// TestTreeWalk_ReleasesEveryElement pins the AX element ownership discipline:
// every Element a tree walk constructs must be balanced by exactly one
// Release, including elements a traversal enqueues but abandons. The live
// wrapper count must return to its baseline after every walk — the leaks
// fixed in #1150/#1152 (elements retained during priming and never released)
// would fail this immediately.
//
// Requires Accessibility permission and a real focused application; skipped
// in short mode per the CI test contract.
func TestTreeWalk_ReleasesEveryElement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping input/permission-dependent integration test in short mode")
	}

	if !nativedarwin.CheckAccessibilityPermissions() {
		t.Skip("Accessibility permission not granted; cannot walk a real tree")
	}

	log := logger.Get()
	client := native.New(log, nil)
	adapter := accessibility.NewAdapter(log, nil, nil, client, false)

	// Each walk gets its own context matching the per-call budget: a shared
	// deadline would expire cumulatively across six healthy scans and fail a
	// later walk that never hung.
	walkOnce := func(what string) error {
		ctx, cancel := context.WithTimeout(context.Background(), integrationScanBudget)
		defer cancel()

		var err error

		runWithinBudget(t, what, func() {
			_, err = adapter.ClickableElements(ctx, ports.DefaultElementFilter())
		})

		return err
	}

	// One warmup walk: lazily-initialized singletons (system-wide element,
	// cached application handles) may legitimately outlive the first walk.
	// runWithinBudget (inside walkOnce) is the hang guard — the AX client
	// discards its context, so only a watchdog can catch a wedged native query.
	warmupErr := walkOnce("warmup ClickableElements walk")
	if warmupErr != nil {
		t.Skipf("warmup walk failed (no scannable frontmost app?): %v", warmupErr)
	}

	baseline := nativedarwin.LiveElementCount()

	for walk := range leakWalkCount {
		// The adapter returns domain elements, not live AX wrappers; the walk
		// owns and must have already released every wrapper it made.
		walkErr := walkOnce("measured ClickableElements walk")
		if walkErr != nil {
			t.Fatalf("walk %d: ClickableElements() error = %v", walk, walkErr)
		}

		if live := nativedarwin.LiveElementCount(); live != baseline {
			t.Fatalf(
				"walk %d leaked AX element wrappers: live count %d, baseline %d — "+
					"some traversal constructed Elements it never released "+
					"(see the ownership rule in internal/adapter/platform/darwin/accessibility.h)",
				walk, live, baseline,
			)
		}
	}
}
