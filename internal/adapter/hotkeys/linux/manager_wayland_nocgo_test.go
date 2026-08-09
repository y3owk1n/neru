//go:build linux && !cgo

package linux

import (
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	eventtaplinux "github.com/y3owk1n/neru/internal/adapter/eventtap/linux"
)

// This is the wiring the no-cgo stub's refusal is for, driven end to end:
// `eventtap/linux/global_hotkey_stub_contract_nocgo_test.go` pins that Start
// returns CodeNotSupported, and this pins what the manager then does with it.
//
// Both halves used to be wrong on a CGO_ENABLED=0 Wayland build. Start returned
// nil, so the manager logged "config keybindings are active" at info while
// nothing read the keyboard; making it loud without this fix swapped that for
// the /dev/input warning, which sends the user after an `input` group
// membership that changes nothing in a build with no evdev in it.
//
// Note where this runs: the CI Linux leg builds with cgo on, so nothing on the
// gate compiles this file. `just test-linux-nocgo` does.
func TestManager_EnsureWaylandStarted_ReportsTheBuildWithoutCgo(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	mgr := NewManager(zap.New(core))
	// Set directly rather than relying on backend detection: the listener is
	// only constructed on a detected Wayland session, and the test host is not
	// required to be one.
	mgr.waylandHotkeys = eventtaplinux.NewGlobalHotkeyListener(nil)

	// Three attempts stands in for the reload and sleep recovery loops, which
	// re-register on every retry.
	for range 3 {
		mgr.ensureWaylandStarted()
	}

	if mgr.waylandStarted {
		t.Error(
			"waylandStarted is true without cgo; HealthCheck would stop trying to " +
				"recover a listener that was never started",
		)
	}

	var warns []string

	for _, entry := range logs.All() {
		if entry.Level == zapcore.InfoLevel &&
			strings.Contains(entry.Message, "config keybindings are active") {
			t.Error("the manager announced active keybindings while nothing reads the keyboard")
		}

		if entry.Level == zapcore.WarnLevel {
			warns = append(warns, entry.Message)
		}
	}

	if len(warns) != 1 {
		t.Fatalf("got %d warnings across 3 attempts, want 1: %v", len(warns), warns)
	}

	if strings.Contains(warns[0], "/dev/input") {
		t.Errorf("the no-cgo warning points the user at /dev/input permissions: %q", warns[0])
	}
}
