//go:build windows

package textinput

import (
	"context"
	"image"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"

	platformwindows "github.com/y3owk1n/neru/internal/adapter/platform/windows"
	"github.com/y3owk1n/neru/internal/ports"
)

// TextInput is the Windows hint-search field: an EDIT control in a nearly
// transparent tool window over the drawn search box, so input methods and dead
// keys reach the query as they do on macOS. When Windows will not give the
// field the foreground, StartHintSearchSession reports started == false and
// hint search keeps reading the event tap's key stream.
//
// The handler calls Start and Stop holding its lock, so the order is h.mu,
// then TextInput.mu, then the queue's own lock, which is a leaf. Start and Stop
// wait on the field's thread for at most 500 ms each, since taking the
// foreground can wait on another app's input queue.
//
// The query is keystream. Nothing here logs it.
type TextInput struct {
	logger         *zap.Logger
	mu             sync.Mutex
	field          *platformwindows.TextField
	queue          *callbackQueue
	fallbackWarned atomic.Bool
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
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopLocked()

	queue := newCallbackQueue(callbacks)
	// The frame is relative to the active screen, and the field is placed in
	// global coordinates.
	bounds := image.Rect(frame.X, frame.Y, frame.X+frame.Width, frame.Y+frame.Height).
		Add(frame.Screen.Min)

	field, err := platformwindows.OpenTextField(bounds, platformwindows.TextFieldEvents{
		QueryChanged: queue.queryChanged,
		Confirm:      queue.confirm,
		Cancel:       queue.cancel,
	})
	if err != nil {
		queue.stop()

		// Without the field, input methods, dead keys and shifted symbols do
		// not reach the search. The first fallback is a warning, and later ones
		// stay at debug, since a session that refuses once usually refuses
		// every time.
		log := t.logger.Debug
		if t.fallbackWarned.CompareAndSwap(false, true) {
			log = t.logger.Warn
		}

		log(
			"Hint search field unavailable, so input methods and dead keys will not reach the search",
			zap.Error(err),
		)

		return false, nil
	}

	t.field = field
	t.queue = queue

	return true, nil
}

// StopHintSearchSession closes the field and gives the foreground back.
func (t *TextInput) StopHintSearchSession(_ context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopLocked()

	return nil
}

func (t *TextInput) stopLocked() {
	if t.field == nil {
		return
	}

	t.queue.stop()
	t.field.Close()
	t.field = nil
	t.queue = nil
}
