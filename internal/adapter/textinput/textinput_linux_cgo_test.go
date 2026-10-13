//go:build linux && cgo

package textinput

import (
	"slices"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/y3owk1n/neru/internal/ports"
)

// TestTextInput_StartHintSearchSession_FallsBackWithoutACompositor pins the
// degrade the handler depends on. A field that cannot open has to report
// started == false and no error, so hint search keeps reading the key stream
// instead of switching the event tap off with nothing to type into.
func TestTextInput_StartHintSearchSession_FallsBackWithoutACompositor(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "neru-test-no-such-compositor")

	input := NewTextInput(zap.NewNop())

	started, err := input.StartHintSearchSession(
		t.Context(),
		ports.TextInputCallbacks{},
		ports.TextInputFrame{X: 0, Y: 0, Width: 200, Height: 30},
	)
	if err != nil {
		t.Fatalf("StartHintSearchSession() error = %v", err)
	}

	if started {
		t.Fatal("StartHintSearchSession() started a field with no compositor to show it")
	}

	err = input.StopHintSearchSession(t.Context())
	if err != nil {
		t.Fatalf("StopHintSearchSession() error = %v", err)
	}
}

// fakeField stands in for the native field. Each byte written to its display
// pipe is one batch of compositor events, answered with the next flags sent on
// flags.
type fakeField struct {
	display, repeatPipe [2]int
	flags               chan int
	dispatched          chan struct{}
}

func newFakeField(t *testing.T) *fakeField {
	t.Helper()

	field := &fakeField{flags: make(chan int, 8), dispatched: make(chan struct{}, 8)}

	for _, pipe := range []*[2]int{&field.display, &field.repeatPipe} {
		err := unix.Pipe2(pipe[:], unix.O_CLOEXEC)
		if err != nil {
			t.Fatalf("Pipe2() error = %v", err)
		}
	}

	t.Cleanup(func() {
		_ = unix.Close(field.display[1])
		_ = unix.Close(field.repeatPipe[1])
	})

	return field
}

// send makes the field report flags on its next dispatch.
func (f *fakeField) send(t *testing.T, flags int) {
	t.Helper()

	f.flags <- flags

	_, err := unix.Write(f.display[1], []byte{0})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
}

func (f *fakeField) displayFD() int { return f.display[0] }
func (f *fakeField) repeatFD() int  { return f.repeatPipe[0] }
func (f *fakeField) repeat() int    { return 0 }
func (f *fakeField) query() string  { return "query" }

func (f *fakeField) dispatch() int {
	var buf [1]byte

	_, _ = unix.Read(f.display[0], buf[:])

	defer func() { f.dispatched <- struct{}{} }()

	return <-f.flags
}

func (f *fakeField) close() {
	_ = unix.Close(f.display[0])
	_ = unix.Close(f.repeatPipe[0])
}

func waitFor(t *testing.T, what string, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// TestFieldSession_StopFromInsideACallbackReturns pins the handler's path: it
// stops the session from inside OnConfirm, and stop waits for the session to
// end, so the callback must not run on the goroutine stop waits for.
func TestFieldSession_StopFromInsideACallbackReturns(t *testing.T) {
	field := newFakeField(t)
	sessionReady := make(chan *fieldSession, 1)
	confirmed := make(chan struct{})

	session, err := startFieldSession(field, ports.TextInputCallbacks{
		OnConfirm: func() {
			(<-sessionReady).stop()
			close(confirmed)
		},
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("startFieldSession() error = %v", err)
	}

	sessionReady <- session

	field.send(t, fieldConfirm)
	waitFor(t, "OnConfirm to stop the session", confirmed)
}

// TestFieldSession_RunsNothingAfterStop pins that a stop is final. A callback
// can be waiting on the handler lock while the handler stops the session, and
// events the field reported behind it must not run once it gets through.
func TestFieldSession_RunsNothingAfterStop(t *testing.T) {
	field := newFakeField(t)
	inFirst := make(chan struct{})
	release := make(chan struct{})

	var (
		callsMu sync.Mutex
		calls   []string
	)

	record := func(name string) {
		callsMu.Lock()
		defer callsMu.Unlock()

		calls = append(calls, name)
	}

	session, err := startFieldSession(field, ports.TextInputCallbacks{
		OnQueryChanged: func(string) {
			record("query")
			close(inFirst)
			<-release
		},
		OnCancel: func() { record("cancel") },
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("startFieldSession() error = %v", err)
	}

	field.send(t, fieldChanged)
	waitFor(t, "the first callback", inFirst)

	field.send(t, fieldCancel)
	waitFor(t, "the field to report the cancel", field.dispatched)
	waitFor(t, "the field to report the cancel", field.dispatched)

	session.stop()
	close(release)

	time.Sleep(50 * time.Millisecond)

	callsMu.Lock()
	defer callsMu.Unlock()

	if !slices.Equal(calls, []string{"query"}) {
		t.Errorf("callbacks run = %v, want only the one already running at stop", calls)
	}
}

// TestFieldSession_LostConnectionCancelsTheSearch pins the exit nobody asked
// for. The event tap is off while the field is open, so a field that died
// quietly would leave the keyboard with no reader.
func TestFieldSession_LostConnectionCancelsTheSearch(t *testing.T) {
	field := newFakeField(t)
	canceled := make(chan struct{})

	session, err := startFieldSession(field, ports.TextInputCallbacks{
		OnCancel: func() { close(canceled) },
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("startFieldSession() error = %v", err)
	}

	field.send(t, -1)
	waitFor(t, "the cancel", canceled)

	session.stop()
}
