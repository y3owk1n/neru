//go:build linux && cgo

package textinput

/*
#cgo linux pkg-config: wayland-client xkbcommon x11
#include "../platform/linux/textfield_wayland.h"
#include "../platform/linux/textfield_x11.h"
*/
import "C"

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/y3owk1n/neru/internal/adapter/platform"
	// The bridge's C sources and the generated protocol code compile in these
	// two packages, so they are linked in here.
	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux"
	_ "github.com/y3owk1n/neru/internal/adapter/platform/linux/wlr_protocol"
	"github.com/y3owk1n/neru/internal/ports"
)

// Flags a field reports after handling its events.
const (
	fieldChanged = int(C.NERU_TEXT_FIELD_CHANGED)
	fieldConfirm = int(C.NERU_TEXT_FIELD_CONFIRM)
	fieldCancel  = int(C.NERU_TEXT_FIELD_CANCEL)
)

// TextInput is the Linux hint-search field, which takes the keyboard while
// search is open so input methods and dead keys reach the query as they do on
// macOS. On Wayland it is a transparent layer-shell surface over the drawn
// search box, and on X11 a one-pixel window that holds the input focus and
// reads keys through XIM. GNOME, which has no layer shell, and a field that
// cannot open get started == false and keep reading the event tap's key
// stream.
//
// The handler calls Start and Stop holding its lock, so h.mu comes before
// TextInput.mu. Start waits at most 250 ms for a field to open: the Wayland
// field bounds its compositor round trips (NERU_TEXT_FIELD_OPEN_TIMEOUT_MS),
// and the X11 field opens on a goroutine of its own, since its input method
// answers with no timeout. Stop waits at most fieldStopWait, because closing
// that input method waits on it too.
//
// The query is keystream. Nothing here logs it.
type TextInput struct {
	logger  *zap.Logger
	mu      sync.Mutex
	session *fieldSession
}

// NewTextInput creates a TextInput.
func NewTextInput(logger *zap.Logger) *TextInput {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &TextInput{logger: logger}
}

// StartHintSearchSession opens the field over frame and reports whether it
// took the keyboard.
func (t *TextInput) StartHintSearchSession(
	_ context.Context,
	callbacks ports.TextInputCallbacks,
	frame ports.TextInputFrame,
) (bool, error) {
	backend := platform.DetectLinuxBackend()
	if backend == platform.BackendWaylandGNOME || backend == platform.BackendUnknown {
		return false, nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopLocked()

	// The frame is relative to the active screen, and the field is placed in
	// global coordinates.
	left, top := C.int(frame.Screen.Min.X+frame.X), C.int(frame.Screen.Min.Y+frame.Y)
	width, height := C.int(frame.Width), C.int(frame.Height)

	var session *fieldSession

	if backend.IsWayland() {
		if handle := C.neru_text_field_open(left, top, width, height); handle != nil {
			var err error

			session, err = startFieldSession(&waylandField{handle: handle}, callbacks, t.logger)
			if err != nil {
				t.logger.Debug(
					"Hint search field could not start, reading the key stream",
					zap.Error(err),
				)
			}
		}
	} else {
		session = openX11Session(left, top, width, height, callbacks, t.logger)
	}

	if session == nil {
		t.logger.Debug("Hint search field unavailable, reading the key stream")

		return false, nil
	}

	t.session = session

	return true, nil
}

// fieldOpenWait bounds how long Start waits for the X11 field, which opens on
// a goroutine of its own because its input method answers with no timeout. A
// method slower than this leaves the search on the key stream.
const fieldOpenWait = 250 * time.Millisecond

// fieldStopWait bounds how long Stop waits for a session to end. Closing the
// X11 field's input method waits on its server, and a session that takes
// longer finishes closing on its own.
const fieldStopWait = 500 * time.Millisecond

// An X11 field's open: opening until it holds the focus, open once the
// goroutine has gone on to take it, or abandoned when Start stopped waiting
// first, in which case the field never takes the focus.
const (
	x11FieldOpening int32 = iota
	x11FieldOpen
	x11FieldAbandoned
)

// openX11Session opens the X11 field off the caller's goroutine and returns its
// session, or nil when it did not open within fieldOpenWait.
func openX11Session(
	left, top, width, height C.int,
	callbacks ports.TextInputCallbacks,
	logger *zap.Logger,
) *fieldSession {
	var state atomic.Int32

	result := make(chan *fieldSession, 1)

	go func() {
		handle := C.neru_x11_text_field_prepare(left, top, width, height)
		if handle == nil {
			result <- nil

			return
		}

		field := &x11Field{handle: handle}

		if !state.CompareAndSwap(x11FieldOpening, x11FieldOpen) {
			field.close()

			return
		}

		if C.neru_x11_text_field_focus(handle) == 0 {
			field.close()

			result <- nil

			return
		}

		session, err := startFieldSession(field, callbacks, logger)
		if err != nil {
			logger.Debug(
				"Hint search field could not start, reading the key stream",
				zap.Error(err),
			)
		}

		result <- session
	}()

	select {
	case session := <-result:
		return session
	case <-time.After(fieldOpenWait):
		if state.CompareAndSwap(x11FieldOpening, x11FieldAbandoned) {
			return nil
		}

		// The field opened as the wait ran out, and taking the focus is a round
		// trip to the X server.
		return <-result
	}
}

// StopHintSearchSession closes the field and waits until it let the keyboard
// go.
func (t *TextInput) StopHintSearchSession(_ context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopLocked()

	return nil
}

func (t *TextInput) stopLocked() {
	if t.session == nil {
		return
	}

	t.session.stop()
	t.session = nil
}

// nativeField is the part of the native field a session drives.
type nativeField interface {
	displayFD() int
	repeatFD() int
	// dispatch handles the events that have arrived, without blocking, and
	// returns the field flags, or -1 when the connection failed.
	dispatch() int
	repeat() int
	query() string
	close()
}

type waylandField struct {
	handle *C.NeruTextField
}

func (f *waylandField) displayFD() int { return int(C.neru_text_field_display_fd(f.handle)) }
func (f *waylandField) repeatFD() int  { return int(C.neru_text_field_repeat_fd(f.handle)) }
func (f *waylandField) dispatch() int  { return int(C.neru_text_field_dispatch(f.handle)) }
func (f *waylandField) repeat() int    { return int(C.neru_text_field_repeat(f.handle)) }
func (f *waylandField) close()         { C.neru_text_field_close(f.handle) }

func (f *waylandField) query() string {
	return C.GoString(C.neru_text_field_query(f.handle))
}

// x11Field is the field on X11, where the server repeats held keys itself.
type x11Field struct {
	handle *C.NeruX11TextField
}

func (f *x11Field) displayFD() int { return int(C.neru_x11_text_field_fd(f.handle)) }
func (f *x11Field) repeatFD() int  { return -1 } // poll ignores a negative descriptor
func (f *x11Field) dispatch() int  { return int(C.neru_x11_text_field_dispatch(f.handle)) }
func (f *x11Field) repeat() int    { return 0 }
func (f *x11Field) close()         { C.neru_x11_text_field_close(f.handle) }

func (f *x11Field) query() string {
	return C.GoString(C.neru_x11_text_field_query(f.handle))
}

// fieldSession serves one open field. run owns the field and is the only
// goroutine that touches it. Callbacks go through the queue, which runs them
// on a goroutine of its own, because the handler stops the session from inside
// them and stop waits for run to return.
type fieldSession struct {
	logger *zap.Logger
	queue  *callbackQueue

	stopRead, stopWrite int
	done                chan struct{}
}

func startFieldSession(
	field nativeField,
	callbacks ports.TextInputCallbacks,
	logger *zap.Logger,
) (*fieldSession, error) {
	var pipe [2]int

	err := unix.Pipe2(pipe[:], unix.O_CLOEXEC)
	if err != nil {
		field.close()

		return nil, err
	}

	session := &fieldSession{
		logger:    logger,
		queue:     newCallbackQueue(callbacks),
		stopRead:  pipe[0],
		stopWrite: pipe[1],
		done:      make(chan struct{}),
	}

	go session.run(field)

	return session, nil
}

// stop closes the field, waiting for it for at most fieldStopWait. Callbacks
// not yet run are dropped from here on, and the handler's session token covers
// one already waiting on its lock.
func (s *fieldSession) stop() {
	s.queue.stop()

	_, _ = unix.Write(s.stopWrite, []byte{0})

	select {
	case <-s.done:
	case <-time.After(fieldStopWait):
		s.logger.Warn("Hint search field is slow to close, leaving it to finish")
	}

	_ = unix.Close(s.stopWrite)
}

func (s *fieldSession) run(field nativeField) {
	defer close(s.done)
	defer unix.Close(s.stopRead) //nolint:errcheck // nothing to do on a failed close
	defer field.close()

	fds := []unix.PollFd{
		{Fd: int32(field.displayFD()), Events: unix.POLLIN},
		{Fd: int32(field.repeatFD()), Events: unix.POLLIN},
		{Fd: int32(s.stopRead), Events: unix.POLLIN},
	}

	// Opening the field can leave events already read off the connection,
	// which would otherwise wait for the next one to arrive.
	if !s.report(field, field.dispatch()) {
		return
	}

	for {
		_, err := unix.Poll(fds, -1)
		if errors.Is(err, unix.EINTR) {
			continue
		}

		if fds[2].Revents != 0 {
			return
		}

		if err != nil {
			s.lost(err)

			return
		}

		flags := 0

		if fds[0].Revents != 0 {
			flags = field.dispatch()
		}

		if fds[1].Revents != 0 && flags >= 0 {
			flags |= field.repeat()
		}

		if !s.report(field, flags) {
			return
		}
	}
}

// report hands what the field reported to the callbacks, and returns false
// when the connection failed and the session has ended.
func (s *fieldSession) report(field nativeField, flags int) bool {
	if flags < 0 {
		s.lost(errLostConnection)

		return false
	}

	if flags&fieldChanged != 0 {
		s.queue.queryChanged(field.query())
	}

	if flags&fieldConfirm != 0 {
		s.queue.confirm()
	}

	if flags&fieldCancel != 0 {
		s.queue.cancel()
	}

	return true
}

var errLostConnection = errors.New("display connection failed")

// lost ends a session nobody asked to stop. Cancel closes the search, which
// turns the event tap back on, so the keyboard is never left with no reader.
func (s *fieldSession) lost(err error) {
	s.logger.Warn("Hint search field stopped, closing the search", zap.Error(err))
	s.queue.cancel()
}
