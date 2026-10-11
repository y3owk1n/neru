package app_test

import (
	"context"
	"encoding/json"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/ipcschema"
	"github.com/y3owk1n/neru/internal/schematest"
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

// TestSimulation_ModeRepliesMatchThePublishedSchema pins the replies only a
// running mode handler can give against schemas/ipc.schema.json: the hints
// query, with elements and without, and the follow-selection toggle, which
// answers only while a mode runs.
func TestSimulation_ModeRepliesMatchThePublishedSchema(t *testing.T) {
	save := simElement(t, "save", image.Rect(100, 100, 220, 140), "Save")

	for _, elements := range [][]*element.Element{{save}, nil} {
		sim := newSimHarness(t, simConfig(), elements)

		hints := sim.app.HandleCommand(
			context.Background(),
			ipc.Command{Action: domain.CommandQueryHints},
		)
		assertMatchesIPCSchema(t, "response", hints)
		assertMatchesIPCSchema(t, "query_hints", hints.Data)
	}

	sim := newSimHarness(t, simConfig(), []*element.Element{save})
	sim.pressHotkey(hintsHotkey)
	sim.waitMode(domain.ModeHints)

	follow := sim.app.HandleCommand(
		context.Background(),
		ipc.Command{Action: domain.CommandToggleCursorFollowSelection},
	)
	if !follow.Success {
		t.Fatalf("toggle-cursor-follow-selection failed in hints mode: %s", follow.Message)
	}

	assertMatchesIPCSchema(t, "toggle_cursor_follow_selection", follow.Data)
}

// assertMatchesIPCSchema checks value, as a client decodes it off the wire,
// against one definition of the published IPC schema.
func assertMatchesIPCSchema(t *testing.T, definition string, value any) {
	t.Helper()

	rendered, err := ipcschema.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var schema map[string]any

	err = json.Unmarshal(rendered, &schema)
	if err != nil {
		t.Fatalf("the IPC schema is not JSON: %v", err)
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%T) error = %v", value, err)
	}

	var document any

	err = json.Unmarshal(encoded, &document)
	if err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", encoded, err)
	}

	definitions, _ := schema["definitions"].(map[string]any)
	node, _ := definitions[definition].(map[string]any)

	err = schematest.Check(schema, node, document, definition)
	if err != nil {
		t.Errorf("%s: %v\n%s", definition, err, encoded)
	}
}
