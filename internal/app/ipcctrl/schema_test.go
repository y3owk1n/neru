package ipcctrl_test

import (
	"context"
	"encoding/json"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/event"
	"github.com/y3owk1n/neru/internal/ipcschema"
	"github.com/y3owk1n/neru/internal/schematest"
)

// statusDefinition names the status shape in the IPC schema.
const statusDefinition = "status"

// TestReplies_MatchThePublishedSchema pins that what the daemon sends is what
// schemas/ipc.schema.json tells a client to expect, for each kind of reply.
func TestReplies_MatchThePublishedSchema(t *testing.T) {
	schema := ipcSchema(t)
	controller := newTestController()

	status := controller.HandleCommand(
		context.Background(),
		ipc.Command{Action: domain.CommandStatus},
	)
	assertMatches(t, schema, "response", status)
	assertMatches(t, schema, statusDefinition, status.Data)

	toggle := controller.HandleCommand(
		context.Background(),
		ipc.Command{Action: domain.CommandToggleScrollInvert},
	)
	assertMatches(t, schema, "response", toggle)
	assertMatches(t, schema, "toggle_scroll_invert", toggle.Data)

	refused := controller.HandleCommand(
		context.Background(),
		ipc.Command{Action: "no-such-command"},
	)
	assertMatches(t, schema, "response", refused)

	assertMatches(t, schema, "query_cursor", ipc.CursorData{X: 1, Y: 2})
	assertMatches(t, schema, "query_window", (*ipc.WindowData)(nil))
	assertMatches(
		t,
		schema,
		"request",
		ipc.Command{Action: actionHints, Args: []string{"--action=left_click"}},
	)
}

// TestWatchLines_MatchThePublishedSchema pins the two kinds of line a watch
// sends after its reply: the snapshot it opens with, and an event.
func TestWatchLines_MatchThePublishedSchema(t *testing.T) {
	schema := ipcSchema(t)
	bus := event.NewBus(nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	lines, _ := watchLines(ctx, t, watchController(bus), nil)

	assertMatches(t, schema, "watch_line", nextLine(t, lines))

	bus.Publish(
		event.Event{
			Name:   event.Select,
			Mode:   actionHints,
			Action: "left_click",
			Point:  image.Pt(3, 4),
		},
	)
	assertMatches(t, schema, "watch_line", nextLine(t, lines))
}

// TestPublishedSchema_RefusesWhatIsNotTheWireFormat pins that the schema is
// strict enough to catch a client or a daemon that drifted from it.
func TestPublishedSchema_RefusesWhatIsNotTheWireFormat(t *testing.T) {
	schema := ipcSchema(t)

	tests := []struct {
		name       string
		definition string
		document   string
	}{
		{"status without its mode", statusDefinition, `{"enabled":true}`},
		{"request with an unknown field", "request", `{"action":"status","params":{}}`},
		{
			"cursor x that is a string",
			"query_cursor",
			`{"x":"1","y":2,"display":null,"display_index":null}`,
		},
		{"window that is not an object", "query_window", `[1]`},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var document any

			err := json.Unmarshal([]byte(testCase.document), &document)
			if err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if check(schema, testCase.definition, document) == nil {
				t.Errorf("the %s schema accepted %s", testCase.definition, testCase.document)
			}
		})
	}
}

func ipcSchema(t *testing.T) map[string]any {
	t.Helper()

	out, err := ipcschema.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var schema map[string]any

	err = json.Unmarshal(out, &schema)
	if err != nil {
		t.Fatalf("the IPC schema is not JSON: %v", err)
	}

	return schema
}

// assertMatches checks value, as a client decodes it off the wire, against one
// definition.
func assertMatches(t *testing.T, schema map[string]any, definition string, value any) {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%T) error = %v", value, err)
	}

	var document any

	err = json.Unmarshal(encoded, &document)
	if err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", encoded, err)
	}

	err = check(schema, definition, document)
	if err != nil {
		t.Errorf("%s: %v\n%s", definition, err, encoded)
	}
}

func check(schema map[string]any, definition string, document any) error {
	definitions, _ := schema["definitions"].(map[string]any)
	node, _ := definitions[definition].(map[string]any)

	return schematest.Check(schema, node, document, definition)
}
