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

// callbackQueueSize bounds the callbacks waiting to run. A user types far
// slower than they drain, so a full queue only means the handler is stuck.
const callbackQueueSize = 64

// TextInput is the Wayland hint-search field: a transparent layer-shell
// surface over the drawn search box that takes the keyboard while search is
// open, so input methods and dead keys reach the query as they do on macOS.
// X11 and compositors without a layer shell, GNOME among them, get
// started == false and keep reading the event tap's key stream.
//
// The query is keystream. Nothing here logs it.
type TextInput struct {
	logger  *zap.Logger
	mu      sync.Mutex
	session *fieldSession
}

// fieldSession is one open field. run owns the native field and is the only
// goroutine that touches it.
type fieldSession struct {
	stopWrite int
	done      chan struct{}
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
	if !platform.DetectLinuxBackend().IsWayland() {
		return false, nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopLocked()

	field := C.neru_text_field_open(
		C.int(frame.X), C.int(frame.Y), C.int(frame.Width), C.int(frame.Height))
	if field == nil {
		t.logger.Debug("Hint search field unavailable, reading the key stream")

		return false, nil
	}

	var stop [2]int

	err := unix.Pipe2(stop[:], unix.O_CLOEXEC)
	if err != nil {
		C.neru_text_field_close(field)
		t.logger.Debug("Hint search field could not start, reading the key stream", zap.Error(err))

		return false, nil
	}

	session := &fieldSession{stopWrite: stop[1], done: make(chan struct{})}
	t.session = session

	go session.run(field, stop[0], callbacks, t.logger)

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

	_, _ = unix.Write(t.session.stopWrite, []byte{0})
	<-t.session.done
	_ = unix.Close(t.session.stopWrite)
	t.session = nil
}

// run serves the field until stopped. Callbacks run in order on their own
// goroutine, because the handler stops the session from inside them and
// StopHintSearchSession waits for run to return.
func (s *fieldSession) run(
	field *C.NeruTextField,
	stopRead int,
	callbacks ports.TextInputCallbacks,
	logger *zap.Logger,
) {
	queue := make(chan func(), callbackQueueSize)
	defer close(s.done)
	defer close(queue)
	defer unix.Close(stopRead) //nolint:errcheck // nothing to do on a failed close
	defer C.neru_text_field_close(field)

	go func() {
		for callback := range queue {
			callback()
		}
	}()

	deliver := func(callback func()) {
		if callback == nil {
			return
		}

		select {
		case queue <- callback:
		default:
			logger.Warn("Hint search callbacks are not draining, dropping one")
		}
	}

	fds := []unix.PollFd{
		{Fd: int32(C.neru_text_field_display_fd(field)), Events: unix.POLLIN},
		{Fd: int32(C.neru_text_field_repeat_fd(field)), Events: unix.POLLIN},
		{Fd: int32(stopRead), Events: unix.POLLIN},
	}

	for {
		_, err := unix.Poll(fds, -1)
		if errors.Is(err, unix.EINTR) {
			continue
		}

		if err != nil || fds[2].Revents != 0 {
			return
		}

		flags := C.int(0)

		if fds[0].Revents != 0 {
			dispatched := C.neru_text_field_dispatch(field)
			if dispatched < 0 {
				logger.Warn("Hint search field lost its Wayland connection")
				deliver(callbacks.OnCancel)

				return
			}

			flags |= dispatched
		}

		if fds[1].Revents != 0 {
			flags |= C.neru_text_field_repeat(field)
		}

		if flags&C.NERU_TEXT_FIELD_CHANGED != 0 && callbacks.OnQueryChanged != nil {
			query := C.GoString(C.neru_text_field_query(field))
			onQueryChanged := callbacks.OnQueryChanged
			deliver(func() { onQueryChanged(query) })
		}

		if flags&C.NERU_TEXT_FIELD_CONFIRM != 0 {
			deliver(callbacks.OnConfirm)
		}

		if flags&C.NERU_TEXT_FIELD_CANCEL != 0 {
			deliver(callbacks.OnCancel)
		}
	}
}
