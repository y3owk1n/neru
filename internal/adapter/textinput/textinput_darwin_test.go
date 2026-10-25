package textinput

import (
	"slices"
	"sync"
	"testing"
	"unsafe"
)

// TestTextInput_Bridge_RunsControlAfterTheQueriesBeforeIt drives the exported
// bridges the way the field does, two edits and then Return or Escape, and
// pins that the control reaches the handler after the text before it. The
// first query holds the callbacks while the rest arrive, so the second one is
// queued alone and is not merged into the first. A test file cannot build a C
// string, so both queries are empty and only their position is asserted.
func TestTextInput_Bridge_RunsControlAfterTheQueriesBeforeIt(t *testing.T) {
	tests := []struct {
		name    string
		control func(unsafe.Pointer)
		want    string
	}{
		{name: "return confirms", control: textInputConfirmBridge, want: "confirm"},
		{name: "escape cancels", control: textInputCancelBridge, want: "cancel"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := newRecorder()
			inFirst := make(chan struct{})
			release := make(chan struct{})

			var first sync.Once

			callbacks := calls.callbacks()
			onQuery := callbacks.OnQueryChanged
			callbacks.OnQueryChanged = func(query string) {
				first.Do(func() {
					close(inFirst)
					<-release
				})
				onQuery(query)
			}

			textInput := NewTextInput(nil)

			textInput.queue = newCallbackQueue(callbacks)
			defer textInput.queue.stop()

			textInputQueryBridge(nil, nil)
			<-inFirst
			textInputQueryBridge(nil, nil)
			test.control(nil)
			close(release)

			got := calls.waitFor(t, 3)

			want := []string{"query ", "query ", test.want}
			if !slices.Equal(got, want) {
				t.Errorf("callbacks ran in order %q, want %q", got, want)
			}
		})
	}
}
