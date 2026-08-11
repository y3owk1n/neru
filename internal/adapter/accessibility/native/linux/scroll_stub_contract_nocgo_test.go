//go:build linux && !cgo

package linux

import (
	"testing"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
)

// TestScrollBackendAvailable_RefusesOnACgoLessBuild pins the one configuration
// where "smooth_scroll is inert" and "the backend is missing" could hide each
// other.
//
// Backend detection reads the environment and nothing else, so a CGO_ENABLED=0
// build in a real X11 or Wayland session reports that backend exactly as a CGO
// build does — while every injection path in it is a CodeNotSupported stub. The
// animation runs on a worker goroutine, so a refusal discovered there reaches
// nobody: it has to be available synchronously, before the handoff. A nil from
// either function here would be the silent no-op ADR 0013 exists to end,
// reintroduced by turning an option on.
//
// The two checks are called directly rather than through ScrollAtCursor because
// the detected backend is cached process-wide, and a test cannot make this
// process believe it has a display server after another test has looked.
func TestScrollBackendAvailable_RefusesOnACgoLessBuild(t *testing.T) {
	checks := map[string]func() error{
		"x11":     x11ScrollBackendAvailable,
		"wayland": waylandScrollBackendAvailable,
	}

	for backend, available := range checks {
		err := available()
		if err == nil {
			t.Errorf("the %s availability check passed on a CGO-less build; "+
				"the animation would inject nothing and report success", backend)

			continue
		}

		if !derrors.IsNotSupported(err) {
			t.Errorf("the %s availability check returned %v, want CodeNotSupported",
				backend, err)
		}
	}
}

// TestBeginScrollSession_RefusesOnACgoLessBuild is the other half: the session
// the worker would open refuses too, so a backend that somehow reached the
// animator still injects nothing rather than half of something.
func TestBeginScrollSession_RefusesOnACgoLessBuild(t *testing.T) {
	sessions := map[string]func(action.Modifiers) (scrollSession, error){
		"x11":     newX11ScrollSession,
		"wayland": newWaylandScrollSession,
	}

	for backend, open := range sessions {
		session, err := open(0)
		if err == nil {
			t.Errorf("the %s session opened on a CGO-less build", backend)

			continue
		}

		if session != nil {
			t.Errorf("the %s session refused and handed back a session anyway", backend)
		}

		if !derrors.IsNotSupported(err) {
			t.Errorf("the %s session returned %v, want CodeNotSupported", backend, err)
		}
	}
}
