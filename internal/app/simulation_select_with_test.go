package app_test

// Journeys for [hints.select_with]: a label whose completing key is typed with
// modifiers held runs the entry for those modifiers in place of the session's
// --action.

import (
	"context"
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// postSelectionQuiet is how long a journey waits for work that must not happen.
const postSelectionQuiet = 150 * time.Millisecond

const (
	selectWithCtrl = "ctrl"
	stepRightClick = "action right_click"
	stepHintsClick = "hints left_click"
)

// typeLabelWith presses a label one character at a time, every character held
// with modifiers, as the event tap spells such a chord.
func (h *simHarness) typeLabelWith(modifiers, label string) {
	for _, r := range strings.ToUpper(label) {
		h.press(modifiers + "+" + string(r))
	}
}

// openHints activates hints with the default hotkey and returns the labels
// drawn.
func (h *simHarness) openHints() []string {
	h.t.Helper()

	h.pressHotkey(hintsHotkey)
	h.waitMode(domain.ModeHints)
	h.waitFor("hints drawn", func() bool { return h.overlay.hintDrawCount() > 0 })

	return h.overlay.lastHintLabels()
}

// TestSimulation_SelectWith_FinishesARepeatSessionInOneKey is the request the
// option exists for. Under --repeat a plain label clicks and reopens hints, and
// the same label typed with the entry's modifier clicks once more and exits.
func TestSimulation_SelectWith_FinishesARepeatSessionInOneKey(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{stepHintsRepeatClicks}
	cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
		selectWithCtrl: {"action session_action", stepIdle},
	}

	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, cfg, []*element.Element{save})

	labels := sim.openHints()
	drawsBefore := sim.overlay.hintDrawCount()

	sim.typeLabel(labels[0])
	sim.waitFor("the plain label clicked", func() bool { return len(sim.ax.recordedClicks()) == 1 })
	sim.waitFor("hints reopened for the next selection", func() bool {
		return sim.overlay.hintDrawCount() > drawsBefore
	})

	if sim.app.CurrentMode() != domain.ModeHints {
		t.Fatalf("mode after a plain --repeat selection = %v, want hints", sim.app.CurrentMode())
	}

	sim.typeLabelWith("Ctrl", sim.overlay.lastHintLabels()[0])
	sim.waitFor(
		"the modified label clicked",
		func() bool { return len(sim.ax.recordedClicks()) == 2 },
	)
	sim.waitMode(domain.ModeIdle)

	last := sim.ax.recordedClicks()[1]
	if last.action != action.TypeLeftClick || last.point != save.Center() {
		t.Fatalf("session_action produced %v at %v, want the session's left click at %v",
			last.action, last.point, save.Center())
	}
}

// TestSimulation_SelectWith_EntryRunsInPlaceOfTheActionThenOnExit pins that an
// entry replaces the session's action rather than adding to it, that a session
// without --repeat still exits after it, and that the binding's --on-exit still
// runs once the entry is done.
func TestSimulation_SelectWith_EntryRunsInPlaceOfTheActionThenOnExit(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{
		"hints --action=left_click --on-exit='action middle_click'",
	}
	cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
		"alt": {stepRightClick},
	}

	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, cfg, []*element.Element{save})

	sim.typeLabelWith("Alt", sim.openHints()[0])

	sim.waitFor("the entry and the exit step both ran", func() bool {
		return len(sim.ax.recordedClicks()) == 2
	})
	sim.waitMode(domain.ModeIdle)

	clicks := sim.ax.recordedClicks()

	if clicks[0].action != action.TypeRightClick || clicks[0].point != save.Center() {
		t.Errorf("the entry produced %v at %v, want a right click at %v",
			clicks[0].action, clicks[0].point, save.Center())
	}

	if clicks[1].action != action.TypeMiddleClick {
		t.Errorf("--on-exit produced %v, want a middle click", clicks[1].action)
	}
}

// TestSimulation_SelectWith_ModifierHeldThroughALongerLabel pins that holding
// the modifier for the whole label works, since every modified hint character
// types into the label and only the completing one decides.
func TestSimulation_SelectWith_ModifierHeldThroughALongerLabel(t *testing.T) {
	cfg := simConfig()
	cfg.Hints.HintCharacters = "sd"
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{stepHintsClick}
	cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
		selectWithCtrl: {stepRightClick},
	}

	elements := threeButtons(t)
	sim := newSimHarness(t, cfg, elements)

	labels := sim.openHints()

	longest := ""
	for _, label := range labels {
		if len(label) > len(longest) {
			longest = label
		}
	}

	if len(longest) < 2 {
		t.Fatalf("labels %v are single characters, want a longer one", labels)
	}

	sim.typeLabelWith("Ctrl", longest)
	sim.waitFor("the entry clicked", func() bool { return len(sim.ax.recordedClicks()) == 1 })

	click := sim.ax.recordedClicks()[0]
	if click.action != action.TypeRightClick {
		t.Fatalf("the entry produced %v, want a right click", click.action)
	}

	if !slices.ContainsFunc(
		elements,
		func(e *element.Element) bool { return e.Center() == click.point },
	) {
		t.Fatalf("click landed at %v, want an element center", click.point)
	}
}

// TestSimulation_SelectWith_ModifiersWithoutAnEntryLeaveTheLabelAlone pins
// that only a modifier set with an entry types into the label. Any other
// chord goes to the bindings, so the label typed after it still selects.
func TestSimulation_SelectWith_ModifiersWithoutAnEntryLeaveTheLabelAlone(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{stepHintsClick}
	cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
		selectWithCtrl: {stepRightClick},
	}

	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, cfg, []*element.Element{save})

	label := sim.openHints()[0]

	sim.typeLabelWith("Alt", label)
	sim.typeLabel(label)

	sim.waitFor("the plain label clicked", func() bool { return len(sim.ax.recordedClicks()) == 1 })

	if got := sim.ax.recordedClicks()[0].action; got != action.TypeLeftClick {
		t.Fatalf(
			"the label typed after an unmapped chord produced %v, want the session's left click",
			got,
		)
	}
}

// TestSimulation_SelectWith_AppEntryReplacesTheGlobalOne pins the per-app
// table. The focused app's entry for a modifier set wins over the global one.
func TestSimulation_SelectWith_AppEntryReplacesTheGlobalOne(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{stepHintsClick}
	cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
		selectWithCtrl: {stepRightClick},
	}
	cfg.Hints.AppConfigs = []config.AppConfig{{
		BundleID:   simFixtureBundleID,
		SelectWith: map[string]config.StringOrStringArray{selectWithCtrl: {"action middle_click"}},
	}}

	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, cfg, []*element.Element{save})

	sim.typeLabelWith("Ctrl", sim.openHints()[0])
	sim.waitFor("the entry clicked", func() bool { return len(sim.ax.recordedClicks()) == 1 })

	if got := sim.ax.recordedClicks()[0].action; got != action.TypeMiddleClick {
		t.Fatalf("with the app focused the entry produced %v, want its middle click", got)
	}
}

// TestSimulation_SelectWith_LabelsWaitForTheEntrysSteps pins the window while
// an entry's steps run. The hints on screen still belong to that selection, so
// a label typed meanwhile is dropped rather than moving the cursor under the
// steps, the entry's click lands on the hint it was typed on, and --on-exit
// runs once.
func TestSimulation_SelectWith_LabelsWaitForTheEntrysSteps(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[hintsHotkey] = []string{
		"hints --action=left_click --on-exit='action middle_click'",
	}
	cfg.Hints.SelectWith = map[string]config.StringOrStringArray{
		selectWithCtrl: {"action sleep 300ms", "action session_action", stepIdle},
	}

	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	cancel := simElement(t, "cancel", image.Rect(300, 100, 420, 140), "Cancel")
	sim := newSimHarness(t, cfg, []*element.Element{save, cancel})

	labels := sim.openHints()
	first := sim.overlay.lastHintCenter(t, labels[0])

	// The first key after a finished label only resets the input, so the
	// second label is typed twice to be sure it would select.
	sim.typeLabelWith("Ctrl", labels[0])
	sim.typeLabel(labels[1])
	sim.typeLabel(labels[1])

	sim.waitMode(domain.ModeIdle)
	sim.waitFor("the entry's click and --on-exit", func() bool {
		return len(sim.ax.recordedClicks()) >= 2
	})

	// Long enough for a doubled --on-exit or a second selection to land.
	time.Sleep(postSelectionQuiet)

	clicks := sim.ax.recordedClicks()
	if len(clicks) != 2 {
		t.Fatalf("clicks = %v, want the entry's left click then one --on-exit middle click", clicks)
	}

	if clicks[0].action != action.TypeLeftClick || clicks[0].point != first {
		t.Errorf("the entry clicked %v at %v, want a left click on the hint it was typed on, %v",
			clicks[0].action, clicks[0].point, first)
	}

	if clicks[1].action != action.TypeMiddleClick {
		t.Errorf("--on-exit produced %v, want a middle click", clicks[1].action)
	}
}

// TestSimulation_SessionActionRunsTheSessionsActionAtTheCursor covers
// session_action outside an entry. It refuses outside hints, does nothing in a
// session opened without --action, and otherwise clicks at the cursor holding
// the session's --modifier, leaving the mode open.
func TestSimulation_SessionActionRunsTheSessionsActionAtTheCursor(t *testing.T) {
	runSessionAction := func(sim *simHarness) ipc.Response {
		return sim.app.HandleCommand(context.Background(), ipc.Command{
			Action: domain.CommandAction,
			Args:   []string{"session_action"},
		})
	}

	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")

	t.Run("outside hints", func(t *testing.T) {
		sim := newSimHarness(t, simConfig(), []*element.Element{save})

		if resp := runSessionAction(sim); resp.Success {
			t.Fatalf("session_action outside hints succeeded: %s", resp.Message)
		}
	})

	t.Run("a session without --action", func(t *testing.T) {
		sim := newSimHarness(t, simConfig(), []*element.Element{save})
		sim.openHints()

		if resp := runSessionAction(sim); !resp.Success {
			t.Fatalf("session_action failed: %s", resp.Message)
		}

		if clicks := sim.ax.recordedClicks(); len(clicks) != 0 {
			t.Fatalf("session_action without --action clicked %v", clicks)
		}
	})

	t.Run("a session with --action and --modifier", func(t *testing.T) {
		cfg := simConfig()
		cfg.Hotkeys.Bindings[hintsHotkey] = []string{"hints --action=right_click --modifier=shift"}
		sim := newSimHarness(t, cfg, []*element.Element{save})
		sim.openHints()

		if resp := runSessionAction(sim); !resp.Success {
			t.Fatalf("session_action failed: %s", resp.Message)
		}

		clicks := sim.ax.recordedClicks()
		if len(clicks) != 1 || clicks[0].action != action.TypeRightClick ||
			clicks[0].modifiers != action.ModShift || clicks[0].point != sim.cursor.position() {
			t.Fatalf("session_action produced %v, want one shift right click at the cursor %v",
				clicks, sim.cursor.position())
		}

		if sim.app.CurrentMode() != domain.ModeHints {
			t.Fatalf("mode after session_action = %v, want hints left open", sim.app.CurrentMode())
		}
	})
}

// lastHintCenter returns the center of the element the most recent hints frame
// labeled label.
func (m *simOverlayPort) lastHintCenter(t *testing.T, label string) image.Point {
	t.Helper()

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.hintFrames) > 0 {
		for _, h := range m.hintFrames[len(m.hintFrames)-1].Hints {
			if h.Label() == label {
				return h.Element().Center()
			}
		}
	}

	t.Fatalf("no hint labeled %q in the last frame", label)

	return image.Point{}
}
