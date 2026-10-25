package textinput

import (
	"slices"
	"testing"
	"time"
)

// TestTextInput_Bridge_RunsConfirmAfterTheQueriesBeforeIt drives the exported
// bridges the way the field does, two edits and then Return, and pins that
// the confirm reaches the handler after the text it confirms. A test file
// cannot build a C string, so both queries are empty and only their position
// is asserted.
func TestTextInput_Bridge_RunsConfirmAfterTheQueriesBeforeIt(t *testing.T) {
	calls := newRecorder()
	textInput := NewTextInput(nil)

	textInput.queue = newCallbackQueue(calls.callbacks())
	defer textInput.queue.stop()

	textInputQueryBridge(nil, nil)
	textInputQueryBridge(nil, nil)
	textInputConfirmBridge(nil)

	var got []string
	for !slices.Contains(got, "confirm") || !slices.Contains(got, "query ") {
		select {
		case <-calls.ran:
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for a query and a confirm, got %q", got)
		}

		calls.mu.Lock()
		got = slices.Clone(calls.calls)
		calls.mu.Unlock()
	}

	if got[len(got)-1] != "confirm" {
		t.Errorf("callbacks ran in order %q, want every query before confirm", got)
	}
}
