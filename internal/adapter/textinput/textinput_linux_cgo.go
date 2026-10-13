//go:build linux && cgo

package textinput

/*
#cgo linux pkg-config: wayland-client xkbcommon
#include "../platform/linux/textfield_wayland.h"
*/
import "C"

import (
	"context"
	"errors"
	"sync"

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

// TextInput is the Wayland hint-search field: a transparent layer-shell
// surface over the drawn search box that takes the keyboard while search is
// open, so input methods and dead keys reach the query as they do on macOS.
// X11 and compositors without a layer shell, GNOME among them, get
// started == false and keep reading the event tap's key stream.
//
// The handler calls Start and Stop holding its lock, so h.mu comes before
// TextInput.mu. Opening the field waits on the compositor for at most
// NERU_TEXT_FIELD_OPEN_TIMEOUT_MS, and Stop waits only for run to notice the
// stop pipe, since run never blocks inside Wayland.
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
	if !backend.IsWayland() || backend == platform.BackendWaylandGNOME {
		return false, nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopLocked()

	// The frame is relative to the active screen, and the field is placed in
	// global coordinates.
	handle := C.neru_text_field_open(
		C.int(frame.Screen.Min.X+frame.X), C.int(frame.Screen.Min.Y+frame.Y),
		C.int(frame.Width), C.int(frame.Height))
	if handle == nil {
		t.logger.Debug("Hint search field unavailable, reading the key stream")

		return false, nil
	}

	session, err := startFieldSession(&waylandField{handle: handle}, callbacks, t.logger)
	if err != nil {
		t.logger.Debug("Hint search field could not start, reading the key stream", zap.Error(err))

		return false, nil
	}

	t.session = session

	return true, nil
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
	// dispatch handles the compositor's events and returns the field flags,
	// or -1 when the connection failed.
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

// fieldSession serves one open field. run owns the field and is the only
// goroutine that touches it. Callbacks run in order on a second goroutine,
// because the handler stops the session from inside them and stop waits for
// run to return.
type fieldSession struct {
	callbacks ports.TextInputCallbacks
	logger    *zap.Logger

	stopRead, stopWrite int
	stopping            chan struct{}
	done                chan struct{}

	mailbox fieldMailbox
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
		callbacks: callbacks,
		logger:    logger,
		stopRead:  pipe[0],
		stopWrite: pipe[1],
		stopping:  make(chan struct{}),
		done:      make(chan struct{}),
		mailbox:   fieldMailbox{wake: make(chan struct{}, 1)},
	}

	go session.deliver()
	go session.run(field)

	return session, nil
}

// stop closes the field. Callbacks not yet run are dropped from here on, and
// the handler's session token covers one already waiting on its lock.
func (s *fieldSession) stop() {
	close(s.stopping)

	_, _ = unix.Write(s.stopWrite, []byte{0})
	<-s.done
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
			dispatched := field.dispatch()
			if dispatched < 0 {
				s.lost(errLostConnection)

				return
			}

			flags |= dispatched
		}

		if fds[1].Revents != 0 {
			flags |= field.repeat()
		}

		if flags&fieldChanged != 0 {
			s.mailbox.post(fieldEvent{query: field.query(), changed: true})
		}

		if flags&fieldConfirm != 0 {
			s.mailbox.post(fieldEvent{confirm: true})
		}

		if flags&fieldCancel != 0 {
			s.mailbox.post(fieldEvent{cancel: true})
		}
	}
}

var errLostConnection = errors.New("wayland connection failed")

// lost ends a session nobody asked to stop. Cancel closes the search, which
// turns the event tap back on, so the keyboard is never left with no reader.
func (s *fieldSession) lost(err error) {
	s.logger.Warn("Hint search field stopped, closing the search", zap.Error(err))
	s.mailbox.post(fieldEvent{cancel: true})
}

func (s *fieldSession) deliver() {
	for {
		select {
		case <-s.stopping:
			return
		case <-s.mailbox.wake:
		}

		for _, event := range s.mailbox.take() {
			select {
			case <-s.stopping:
				return
			default:
			}

			s.call(event)
		}
	}
}

func (s *fieldSession) call(event fieldEvent) {
	switch {
	case event.changed && s.callbacks.OnQueryChanged != nil:
		s.callbacks.OnQueryChanged(event.query)
	case event.confirm && s.callbacks.OnConfirm != nil:
		s.callbacks.OnConfirm()
	case event.cancel && s.callbacks.OnCancel != nil:
		s.callbacks.OnCancel()
	}
}

// fieldEvent is one callback to run.
type fieldEvent struct {
	query   string
	changed bool
	confirm bool
	cancel  bool
}

// fieldMailbox holds events in order without bound or loss, except that a
// query replaces a query right before it, since only the latest text matters.
type fieldMailbox struct {
	mu     sync.Mutex
	events []fieldEvent
	wake   chan struct{}
}

func (m *fieldMailbox) post(event fieldEvent) {
	m.mu.Lock()

	if last := len(m.events) - 1; event.changed && last >= 0 && m.events[last].changed {
		m.events[last] = event
	} else {
		m.events = append(m.events, event)
	}

	m.mu.Unlock()

	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *fieldMailbox) take() []fieldEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	events := m.events
	m.events = nil

	return events
}
