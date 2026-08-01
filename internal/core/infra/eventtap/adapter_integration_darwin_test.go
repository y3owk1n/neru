//go:build integration && darwin

package eventtap_test

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/eventtap"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin" // Link CGO implementations
)

// Pin the main thread during package init so TestMain still runs on it.
func init() {
	runtime.LockOSThread()
}

// TestMain services the macOS main run loop while the tests run. Resolving
// hotkey strings to key codes happens on the main queue, which nothing drains
// in a plain `go test` binary.
func TestMain(m *testing.M) {
	os.Exit(darwin.RunMainLoopForTesting(m.Run))
}

// TestEventTapAdapterIntegration tests the event tap adapter.
// Note: This test requires accessibility permissions and might fail in headless CI.
func TestEventTapAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	// Create infra event tap
	tap := eventtap.NewEventTap(func(_ string) {}, logger)
	if tap == nil {
		t.Skip("Skipping EventTap test: failed to create event tap (missing permissions?)")
	}

	adapter := eventtap.NewAdapter(tap, logger)

	ctx := context.Background()

	t.Run("Enable and Disable", func(t *testing.T) {
		// Enable
		enableErr := adapter.Enable(ctx)
		if enableErr != nil {
			t.Errorf("Enable() error = %v, want nil", enableErr)
		}

		// Verify enabled
		if !adapter.IsEnabled() {
			t.Error("IsEnabled() = false, want true")
		}

		// Disable
		disableErr := adapter.Disable(ctx)
		if disableErr != nil {
			t.Errorf("Disable() error = %v, want nil", disableErr)
		}

		// Verify disabled
		if adapter.IsEnabled() {
			t.Error("IsEnabled() = true, want false")
		}
	})

	t.Run("SetHandler is inert and leaves the tap usable", func(t *testing.T) {
		// On darwin the key handler is fixed when NewEventTap is called;
		// Adapter.SetHandler is a deliberate no-op that only warns. Pin both
		// halves of that contract: the replacement handler never runs, and
		// calling it does not disturb the tap's enable state.
		err := adapter.Enable(ctx)
		if err != nil {
			t.Fatalf("Enable() error = %v, want nil", err)
		}

		t.Cleanup(func() { _ = adapter.Disable(ctx) })

		replaced := false

		adapter.SetHandler(func(_ string) { replaced = true })

		if replaced {
			t.Error("handler passed to SetHandler was invoked; darwin SetHandler must stay inert")
		}

		if !adapter.IsEnabled() {
			t.Error("IsEnabled() = false after SetHandler, want the tap to stay enabled")
		}
	})

	t.Run("SetHotkeys accepts populated and empty sets", func(t *testing.T) {
		err := adapter.Enable(ctx)
		if err != nil {
			t.Fatalf("Enable() error = %v, want nil", err)
		}

		t.Cleanup(func() { _ = adapter.Disable(ctx) })

		// An empty slice is documented as valid (it clears monitoring), so
		// cycling through populated -> empty -> populated must leave the tap
		// enabled and usable rather than tearing it down.
		for _, hotkeys := range [][]string{
			{"cmd+shift+k", "cmd+shift+l"},
			{},
			nil,
			{"cmd+shift+k"},
		} {
			adapter.SetHotkeys(hotkeys)

			if !adapter.IsEnabled() {
				t.Fatalf("IsEnabled() = false after SetHotkeys(%v), want true", hotkeys)
			}
		}
	})

	t.Run("SetKeyboardLayout reports whether the layout resolved", func(t *testing.T) {
		// SetKeyboardLayout returns a bool precisely so callers can fall back
		// when a configured layout does not exist. A backend that always
		// returned true would silently accept typos in user config.
		if adapter.SetKeyboardLayout("com.apple.keylayout.NotARealLayout") {
			t.Error("SetKeyboardLayout(bogus layout) = true, want false")
		}

		if !adapter.SetKeyboardLayout("com.apple.keylayout.US") {
			t.Error("SetKeyboardLayout(com.apple.keylayout.US) = false, want true")
		}
	})

	t.Run("Destroy", func(t *testing.T) {
		// Create a new adapter for this test to avoid interfering with other tests
		testTap := eventtap.NewEventTap(func(_ string) {}, logger)
		if testTap == nil {
			t.Skip("Skipping Destroy test: failed to create event tap")
		}

		testAdapter := eventtap.NewAdapter(testTap, logger)

		// Enable first
		_ = testAdapter.Enable(ctx)

		// Then destroy - should not panic
		testAdapter.Destroy()
	})
}
