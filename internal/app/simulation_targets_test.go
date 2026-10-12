package app_test

import (
	"context"
	"encoding/json"
	"image"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// suppliedTargets are two places a tool hands hints mode, away from the one
// element the scan would find.
const suppliedTargets = `{"hints":[` +
	`{"x":400,"y":300,"width":60,"height":30,"title":"Play"},` +
	`{"x":600,"y":300,"width":60,"height":30,"title":"Stop"}]}`

// activateWithTargets opens hints mode on suppliedTargets with a left click.
func activateWithTargets(t *testing.T, sim *simHarness) {
	t.Helper()

	resp := sim.app.HandleCommand(context.Background(), ipc.Command{
		Action: domain.ModeNameHints,
		Args:   []string{"--action=left_click", "--targets=" + suppliedTargets},
	})
	if !resp.Success {
		t.Fatalf("hints --targets failed: %s (%s)", resp.Message, resp.Code)
	}

	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })
}

// TestSimulation_HintsTargets_LabelsAndClicksTheSuppliedTargets covers
// `neru hints --targets`: hints mode labels exactly the targets given, not
// what a scan finds, and selecting one acts at its center.
func TestSimulation_HintsTargets_LabelsAndClicksTheSuppliedTargets(t *testing.T) {
	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, simConfig(), []*element.Element{save})

	activateWithTargets(t, sim)

	labels := sim.overlay.lastHintLabels()
	if len(labels) != 2 {
		t.Fatalf("hints mode drew %v, want one label per supplied target", labels)
	}

	sim.typeLabel(labels[1])
	sim.waitFor("click recorded", func() bool { return len(sim.ax.recordedClicks()) > 0 })

	want := image.Pt(630, 315)
	if clicks := sim.ax.recordedClicks(); len(clicks) != 1 || clicks[0].point != want {
		t.Fatalf("clicks = %+v, want one at the second target's center %v", clicks, want)
	}

	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_HintsTargets_SelectionWithoutAnActionKeepsTheTargets pins
// the re-activation a selection with no --action makes: hints mode comes back
// on the supplied targets, not on what a scan finds.
func TestSimulation_HintsTargets_SelectionWithoutAnActionKeepsTheTargets(t *testing.T) {
	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, simConfig(), []*element.Element{save})

	resp := sim.app.HandleCommand(context.Background(), ipc.Command{
		Action: domain.ModeNameHints,
		Args:   []string{"--targets=" + suppliedTargets},
	})
	if !resp.Success {
		t.Fatalf("hints --targets failed: %s (%s)", resp.Message, resp.Code)
	}

	sim.waitMode(domain.ModeHints)
	sim.waitFor("hints drawn", func() bool { return sim.overlay.hintDrawCount() > 0 })

	before := sim.overlay.hintDrawCount()
	labels := sim.overlay.lastHintLabels()

	sim.typeLabel(labels[0])
	sim.waitFor("hints drawn again", func() bool { return sim.overlay.hintDrawCount() > before })

	if got := sim.cursor.position(); got != image.Pt(430, 315) {
		t.Errorf("cursor at %v, want the first target's center", got)
	}

	if relabelled := sim.overlay.lastHintLabels(); len(relabelled) != 2 {
		t.Errorf("hints mode came back with %v, want the two supplied targets again", relabelled)
	}
}

// TestSimulation_HintsTargets_ExitOnAScreenChange pins that a screen change
// ends a session on supplied targets rather than scanning in their place.
func TestSimulation_HintsTargets_ExitOnAScreenChange(t *testing.T) {
	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, simConfig(), []*element.Element{save})

	activateWithTargets(t, sim)

	sim.changeScreen(simDisplayResized())
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_HintsTargets_ExitOnAMonitorMove pins the same for a move to
// another display, where nothing could scan the targets either.
func TestSimulation_HintsTargets_ExitOnAMonitorMove(t *testing.T) {
	cfg := simConfig()
	cfg.Hotkeys.Bindings[moveMonitorHotkey] = []string{moveMonitorBinding}

	// An element on the display moved to, so a scan there would find
	// something and only the exit can explain returning to idle.
	other := simElement(t, "other", image.Rect(-800, 1000, -700, 1040), "Other")

	sim := newSimHarnessWithDisplays(t, cfg, []*element.Element{other}, []simDisplay{
		{name: mainDisplayName, bounds: simScreen},
		{name: secondDisplayName, bounds: image.Rect(-1600, 0, 0, 2560)},
	})

	activateWithTargets(t, sim)

	sim.pressHotkey(moveMonitorHotkey)
	sim.waitMode(domain.ModeIdle)
}

// TestSimulation_HintsTargets_ReadsBackAQueryAnswer pins the round trip the
// shape was chosen for: what `neru query hints --json` prints, handed back
// as --targets, is labeled the way the query said.
func TestSimulation_HintsTargets_ReadsBackAQueryAnswer(t *testing.T) {
	elements := []*element.Element{
		simElement(t, "save", image.Rect(100, 100, 220, 140), "Save"),
		simElement(t, "open", image.Rect(300, 100, 420, 140), "Open"),
	}
	sim := newSimHarness(t, simConfig(), elements)

	query := sim.app.HandleCommand(
		context.Background(),
		ipc.Command{Action: domain.CommandQueryHints},
	)
	if !query.Success {
		t.Fatalf("query hints failed: %s", query.Message)
	}

	answer, err := json.Marshal(query.Data)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	data, _ := query.Data.(ipc.HintsData)

	queried := make([]string, len(data.Hints))
	for idx, hint := range data.Hints {
		queried[idx] = hint.Label
	}

	resp := sim.app.HandleCommand(context.Background(), ipc.Command{
		Action: domain.ModeNameHints,
		Args:   []string{"--targets", string(answer)},
	})
	if !resp.Success {
		t.Fatalf("hints --targets with a query answer failed: %s", resp.Message)
	}

	sim.waitMode(domain.ModeHints)

	if drawn := sim.overlay.lastHintLabels(); !slices.Equal(drawn, queried) {
		t.Errorf("hints mode drew %v from the query answer, the query reported %v", drawn, queried)
	}
}
