//go:build linux

package linux_test

import (
	"testing"

	nativelinux "github.com/y3owk1n/neru/internal/adapter/accessibility/native/linux"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// TestScrollAtCursor_RefusesModifiersWithoutABackend pins the loudness half of
// the scroll modifier contract: with no display server to inject through there
// is no way to present a held key, and answering nil would scroll unmodified
// while reporting success. A user who asked for ctrl + scroll_up would see a
// pan instead of a zoom and blame their binding.
//
// An unmodified scroll keeps its long-standing silent no-op — nothing about
// this change gave it somewhere to go.
func TestScrollAtCursor_RefusesModifiersWithoutABackend(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")

	// The backend is detected once per process and cached, so a session that
	// has a display server makes this branch unreachable rather than wrong.
	if platform.DetectLinuxBackend() != platform.BackendUnknown {
		t.Skip("a display server backend was detected; the no-backend branch is unreachable here")
	}

	err := nativelinux.ScrollAtCursor(0, -50, action.ModCtrl)
	if err == nil {
		t.Fatal("ScrollAtCursor with ctrl returned nil; a dropped modifier must be reported")
	}

	if !derrors.IsNotSupported(err) {
		t.Fatalf("ScrollAtCursor with ctrl returned %v, want a CodeNotSupported error", err)
	}

	unmodified := nativelinux.ScrollAtCursor(0, -50, 0)
	if unmodified != nil {
		t.Fatalf("unmodified ScrollAtCursor returned %v, want nil", unmodified)
	}
}

// smoothScrollConfig is a config.Provider with the animation switched on, which
// is all the scroll path reads.
type smoothScrollConfig struct{}

func (smoothScrollConfig) Get() *config.Config {
	cfg := config.DefaultConfig()
	cfg.SmoothScroll.Enabled = true

	return cfg
}

// TestScrollAtCursor_RefusesWithSmoothScrollAndNoBackend is the same contract
// for the animated path, which is the one that could quietly lose it: the
// animation runs on a worker goroutine, so a refusal discovered there has
// nobody to go back to. The refusal therefore has to be made before the handoff
// — and turning an option on must never turn a `CodeNotSupported` into a nil.
func TestScrollAtCursor_RefusesWithSmoothScrollAndNoBackend(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")

	if platform.DetectLinuxBackend() != platform.BackendUnknown {
		t.Skip("a display server backend was detected; the no-backend branch is unreachable here")
	}

	nativelinux.SetConfigProvider(smoothScrollConfig{})
	t.Cleanup(func() { nativelinux.SetConfigProvider(nil) })

	for _, modifiers := range []action.Modifiers{0, action.ModCtrl} {
		err := nativelinux.ScrollAtCursor(0, -50, modifiers)
		if err == nil {
			t.Fatalf("smooth ScrollAtCursor (%v) returned nil with no backend to animate through",
				modifiers)
		}

		if !derrors.IsNotSupported(err) {
			t.Fatalf("smooth ScrollAtCursor (%v) returned %v, want a CodeNotSupported error",
				modifiers, err)
		}
	}
}
