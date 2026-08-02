//go:build integration && darwin

package hotkeys_test

import (
	"os"
	"runtime"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/hotkeys"
	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/adapter/platform/darwin" // Link CGO implementations
)

// Pin the main thread during package init so TestMain still runs on it.
func init() {
	runtime.LockOSThread()
}

// TestMain services the macOS main run loop while the tests run. Registering a
// hotkey needs the keyboard layout maps and a CGEventTap, both of which are
// built on the main queue; nothing drains that queue in a plain `go test`
// binary.
func TestMain(m *testing.M) {
	os.Exit(darwin.RunMainLoopForTesting(m.Run))
}

// TestManagerRegistersAgainstTheRealOS exercises the CGEventTap path end to end.
func TestManagerRegistersAgainstTheRealOS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Creating a CGEventTap is behind the macOS Accessibility TCC gate, and a
	// `go test` binary is not normally granted it. Without the grant every
	// Register fails, which reads as "the hotkey manager is broken" when what
	// is missing is the permission. Skipping says what actually happened; CI
	// has the grant, so the real path is still exercised on every PR.
	if !darwin.CheckAccessibilityPermissions() {
		t.Skip(
			"macOS Accessibility permission is not granted to this process, so a " +
				"CGEventTap cannot be created; grant it under System Settings > " +
				"Privacy & Security > Accessibility to run this",
		)
	}

	log := logger.Get()
	manager := hotkeys.NewManager(log)

	hotkeys.SetGlobalManager(manager) // Required for C callbacks
	defer hotkeys.SetGlobalManager(nil)

	t.Cleanup(manager.UnregisterAll)

	t.Run("Register and Unregister", func(t *testing.T) {
		// A deliberately awkward chord, so the test does not fight a real
		// system or application shortcut.
		const key = "cmd+alt+ctrl+shift+f12"

		hotkeyID, err := manager.Register(key, func() {})
		if err != nil {
			t.Fatalf("Register(%q) error = %v, want nil", key, err)
		}

		if hotkeyID == 0 {
			t.Fatalf("Register(%q) returned zero HotkeyID", key)
		}

		manager.Unregister(hotkeyID)

		// Re-registering the same chord only succeeds if Unregister actually
		// released the underlying tap.
		secondID, err := manager.Register(key, func() {})
		if err != nil {
			t.Fatalf("Register(%q) after Unregister error = %v, want nil", key, err)
		}

		manager.Unregister(secondID)
	})

	t.Run("Register Invalid Hotkey", func(t *testing.T) {
		_, err := manager.Register("invalid-hotkey", func() {})
		if err == nil {
			t.Error("Register(\"invalid-hotkey\") error = nil, want error")
		}
	})
}
