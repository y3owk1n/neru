//go:build integration && darwin

package darwin

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// focusedAppForPriming returns the focused application element, skipping when
// the test cannot read a real tree.
func focusedAppForPriming(t *testing.T) *Element {
	t.Helper()

	if testing.Short() {
		t.Skip("Skipping input/permission-dependent integration test in short mode")
	}

	if !CheckAccessibilityPermissions() {
		t.Skip("Accessibility permission not granted; cannot walk a real tree")
	}

	app := FocusedApplication()
	if app == nil {
		t.Skip("no focused application to prime")
	}

	t.Cleanup(app.Release)

	return app
}

// TestHasUsableAccessibilityTree_BudgetReleasesEveryElement forces the walk
// to its element budget on a real tree. Stopping there must count as ready and
// release every element the walk took.
func TestHasUsableAccessibilityTree_BudgetReleasesEveryElement(t *testing.T) {
	app := focusedAppForPriming(t)

	core, logs := observer.New(zap.DebugLevel)
	baseline := LiveElementCount()

	ready := hasUsableAccessibilityTree(context.Background(), app, 1, zap.New(core))

	if logs.FilterMessage("Accessibility tree reached the priming node budget").Len() == 0 {
		t.Skip("the focused application exposes too few elements to reach a budget of 1")
	}

	if !ready {
		t.Error("a walk stopped by its budget reported not ready, so priming would retry it")
	}

	if live := LiveElementCount(); live != baseline {
		t.Errorf("a walk stopped by its budget leaked elements: live count %d, baseline %d",
			live, baseline)
	}
}

// TestHasUsableAccessibilityTree_CanceledReleasesEveryElement pins that a
// canceled walk reports not ready and leaves nothing retained.
func TestHasUsableAccessibilityTree_CanceledReleasesEveryElement(t *testing.T) {
	app := focusedAppForPriming(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	baseline := LiveElementCount()

	if hasUsableAccessibilityTree(ctx, app, maxPrimingNodes, zap.NewNop()) {
		t.Error("a canceled walk reported ready")
	}

	if live := LiveElementCount(); live != baseline {
		t.Errorf("a canceled walk leaked elements: live count %d, baseline %d", live, baseline)
	}
}
