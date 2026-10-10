package app_test

import (
	"context"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// TestSimulation_QueryHintsListsWhatHintsWouldLabel covers `neru query hints`:
// it answers with the elements hints mode would label, each with the text
// --text matches and its bounds, and draws nothing and enters no mode.
func TestSimulation_QueryHintsListsWhatHintsWouldLabel(t *testing.T) {
	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")
	sim := newSimHarness(t, simConfig(), []*element.Element{save})

	resp := sim.app.HandleCommand(
		context.Background(),
		ipc.Command{Action: domain.CommandQueryHints},
	)
	if !resp.Success {
		t.Fatalf("query hints failed: %s (%s)", resp.Message, resp.Code)
	}

	data, ok := resp.Data.(ipc.HintsData)
	if !ok {
		t.Fatalf("data is %T, want ipc.HintsData", resp.Data)
	}

	wantRole := string(save.Role())
	if vocab, known := element.CurrentVocabulary(); known {
		wantRole = string(vocab) + ":" + wantRole
	}

	want := ipc.HintData{Role: wantRole, Title: "Save", X: 100, Y: 100, Width: 120, Height: 40}
	if len(data.Hints) != 1 || data.Hints[0] != want {
		t.Fatalf("hints = %+v, want [%+v]", data.Hints, want)
	}

	if mode := sim.app.CurrentMode(); mode != domain.ModeIdle {
		t.Errorf("mode = %v after the query, want idle", mode)
	}

	if draws := sim.overlay.hintDrawCount(); draws != 0 {
		t.Errorf("the query drew hints %d times, want none", draws)
	}
}
