package app_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/event"
)

// simEvents is a subscription to the app's event bus, opened before a journey
// starts so it sees every event the journey causes.
type simEvents struct {
	t      *testing.T
	events <-chan event.Event
	seq    uint64
}

func subscribeEvents(t *testing.T, sim *simHarness) *simEvents {
	t.Helper()

	events, stop := sim.app.Events().Subscribe(64)
	t.Cleanup(stop)

	return &simEvents{t: t, events: events}
}

// expect waits for the next events in order and compares everything but Seq,
// which it checks only rises, with nothing dropped.
func (s *simEvents) expect(want ...event.Event) {
	s.t.Helper()

	for _, wanted := range want {
		select {
		case got := <-s.events:
			if got.Seq <= s.seq || got.Dropped != 0 {
				s.t.Fatalf(
					"event %+v after seq %d, want a later seq with nothing dropped",
					got,
					s.seq,
				)
			}

			s.seq = got.Seq
			got.Seq = 0

			if got != wanted {
				s.t.Fatalf("event = %+v, want %+v", got, wanted)
			}
		case <-time.After(simWaitHeadroom):
			s.t.Fatalf("no event within %v, want %+v", simWaitHeadroom, wanted)
		}
	}
}

// expectNone fails on any event published within window.
func (s *simEvents) expectNone(window time.Duration) {
	s.t.Helper()

	select {
	case got := <-s.events:
		s.t.Fatalf("unexpected event %+v", got)
	case <-time.After(window):
	}
}

func modeEnter(mode string) event.Event {
	return event.Event{Name: event.ModeEnter, Mode: mode}
}

func modeExit(mode string, reason event.ExitReason) event.Event {
	return event.Event{Name: event.ModeExit, Mode: mode, Reason: reason}
}

// TestSimulation_EventsReportASelectionAsCompleted pins that typing a hint
// label closes hints with the reason a script would act on.
func TestSimulation_EventsReportASelectionAsCompleted(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{"hints --action left_click"}

	sim := newSimHarness(t, cfg, threeButtons(t))
	events := subscribeEvents(t, sim)

	sim.pressHotkey(hintsHotkey)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	sim.typeLabel(sim.overlay.lastHintLabels()[0])
	sim.waitMode(domain.ModeIdle)

	events.expect(modeEnter("hints"), modeExit("hints", event.ExitCompleted))
}

// TestSimulation_EventsFollowEveryModeTransition walks the three ways a mode
// closes: Escape cancels, a hotkey into another mode switches through idle,
// and entering scroll switches without passing through idle at all.
func TestSimulation_EventsFollowEveryModeTransition(t *testing.T) {
	sim := newSimHarness(t, simConfig(), threeButtons(t))
	events := subscribeEvents(t, sim)

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)
	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)
	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	events.expect(
		modeEnter("hints"),
		modeExit("hints", event.ExitCancelled),
		modeEnter("hints"),
		modeExit("hints", event.ExitSwitched),
		modeEnter("grid"),
		modeExit("grid", event.ExitSwitched),
		modeEnter("scroll"),
		modeExit("scroll", event.ExitCancelled),
	)
}

// TestSimulation_EventsNameADeclaredMode pins that a declared mode is reported
// by the name the user gave it, not as a custom mode.
func TestSimulation_EventsNameADeclaredMode(t *testing.T) {
	sim := newSimHarness(t, simConfigDeclaringAMode(), nil)
	events := subscribeEvents(t, sim)

	sim.pressHotkey(customModeHotkey)
	sim.waitMode(domain.ModeCustom)
	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)

	events.expect(modeEnter(customModeName), modeExit(customModeName, event.ExitCancelled))
}

// TestSimulation_EventsReportWhetherAReloadTook pins that a reload closes the
// open mode first and then says whether it took, including when it did not.
func TestSimulation_EventsReportWhetherAReloadTook(t *testing.T) {
	sim := newSimHarness(t, simConfig(), threeButtons(t))
	events := subscribeEvents(t, sim)

	dir := t.TempDir()
	valid := filepath.Join(dir, "valid.toml")
	broken := filepath.Join(dir, "broken.toml")

	writeErr := os.WriteFile(valid, []byte("[hotkeys]\n\""+hintsHotkey+"\" = \"hints\"\n"), 0o600)
	if writeErr != nil {
		t.Fatalf("failed to write the valid config: %v", writeErr)
	}

	writeErr = os.WriteFile(broken, []byte("[hotkeys\n"), 0o600)
	if writeErr != nil {
		t.Fatalf("failed to write the broken config: %v", writeErr)
	}

	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)

	reloadErr := sim.app.ReloadConfig(context.Background(), valid)
	if reloadErr != nil {
		t.Fatalf("ReloadConfig(valid) error = %v", reloadErr)
	}

	reloadErr = sim.app.ReloadConfig(context.Background(), broken)
	if reloadErr == nil {
		t.Fatal("ReloadConfig(broken) succeeded, want an error")
	}

	events.expect(
		modeEnter("hints"),
		modeExit("hints", event.ExitCancelled),
		event.Event{Name: event.ConfigReload, OK: true},
		event.Event{Name: event.ConfigReload, OK: false},
	)
}

// TestSimulation_EventsReportAPauseOnlyWhenItChangesSomething pins that a
// pause closes the open mode before reporting itself, and that a stop or start
// that changes nothing reports nothing.
func TestSimulation_EventsReportAPauseOnlyWhenItChangesSomething(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)
	events := subscribeEvents(t, sim)

	sim.pressHotkey(scrollHotkey)
	sim.waitMode(domain.ModeScroll)

	sim.app.SetEnabled(false)
	sim.app.SetEnabled(false)

	events.expect(
		modeEnter("scroll"),
		modeExit("scroll", event.ExitCancelled),
		event.Event{Name: event.Disable},
	)
	events.expectNone(100 * time.Millisecond)

	sim.app.SetEnabled(true)
	sim.app.SetEnabled(true)

	events.expect(event.Event{Name: event.Enable})
	events.expectNone(100 * time.Millisecond)
}

// TestSimulation_EventsReportTheFocusedApp pins that a focus change names the
// application by bundle ID and nothing else.
func TestSimulation_EventsReportTheFocusedApp(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)
	events := subscribeEvents(t, sim)

	sim.focusApp("Some Editor", "com.example.editor")

	events.expect(event.Event{Name: event.AppFocus, BundleID: "com.example.editor"})
}
