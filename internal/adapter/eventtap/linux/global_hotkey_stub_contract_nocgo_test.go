//go:build linux && !cgo

package linux

import (
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
)

// The evdev global-hotkey listener is the Wayland substitute for OS-level
// global hotkeys, and it needs cgo. In a CGO_ENABLED=0 Linux build the whole
// type is a stub, so Start has to say so.
//
// Returning nil is the failure this pins. The only caller is
// `hotkeys/linux.Manager.ensureWaylandStarted`, which reads a nil error as
// "the listener is reading the keyboard" and sets waylandStarted, logging the
// info line that tells the user their config keybindings are active. On a
// no-cgo build nothing is reading anything: the user is told their hotkeys
// work, presses one, and nothing happens — with no warning anywhere pointing
// at the build. CodeNotSupported routes the same call into the warn branch
// that already exists for an unreadable /dev/input.
//
// The rule is `internal/adapter/platform/AGENTS.md`, "Stubs are loud".
//
// Note where this runs: the CI Linux leg builds with CGO on, so nothing on the
// gate compiles this file. `just test-linux-nocgo` does, as does any
// CGO_ENABLED=0 Linux build — run it after touching the no-cgo twins.
func TestGlobalHotkeyListener_StartReportsNotSupportedWithoutCgo(t *testing.T) {
	listener := NewGlobalHotkeyListener(nil)

	err := listener.Start()
	if err == nil {
		t.Fatal(
			"Start returned nil without cgo; the hotkey manager would report the " +
				"user's config keybindings active while nothing reads the keyboard",
		)
	}

	if !derrors.IsNotSupported(err) {
		t.Errorf("Start returned %v (code %q), want CodeNotSupported",
			err, derrors.GetCode(err))
	}
}

// TestGlobalHotkeyListener_StartIsStableAcrossCalls guards against a stub that
// refuses once and then changes its answer. The manager retries Start on every
// registration once waylandStarted has been cleared, so a second call that
// returned nil would put it back in the state above.
func TestGlobalHotkeyListener_StartIsStableAcrossCalls(t *testing.T) {
	listener := NewGlobalHotkeyListener(nil)

	for i := range 3 {
		err := listener.Start()
		if !derrors.IsNotSupported(err) {
			t.Fatalf("Start call %d returned %v, want CodeNotSupported every time", i+1, err)
		}
	}
}

// TestGlobalHotkeyListener_IsRunningStaysFalseAfterStart pins the other half of
// the refusal: IsRunning and DeviceCount are what
// `hotkeys/linux.Manager.HealthCheck` polls, and a stub that claimed to be
// running would make the health loop stop trying to recover a listener that
// does not exist.
func TestGlobalHotkeyListener_IsRunningStaysFalseAfterStart(t *testing.T) {
	listener := NewGlobalHotkeyListener(nil)

	_ = listener.Start()

	if listener.IsRunning() {
		t.Error("IsRunning reported true without cgo; no evdev reader exists")
	}

	if got := listener.DeviceCount(); got != 0 {
		t.Errorf("DeviceCount() = %d without cgo, want 0", got)
	}
}
