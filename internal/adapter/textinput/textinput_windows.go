//go:build windows

package textinput

import (
	"context"
	"image"
	"sync"

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
// The handler calls Start and Stop holding its lock, so h.mu comes before
// TextInput.mu. Neither waits on anything but the field's own thread.
//
// The query is keystream. Nothing here logs it.
type TextInput struct {
	logger *zap.Logger
	mu     sync.Mutex
	field  *platformwindows.TextField
	queue  *callbackQueue
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
	bounds := image.Rect(frame.X, frame.Y, frame.X+frame.Width, frame.Y+frame.Height)

	field, err := platformwindows.OpenTextField(bounds, platformwindows.TextFieldEvents{
		QueryChanged: queue.queryChanged,
		Confirm:      queue.confirm,
		Cancel:       queue.cancel,
	})
	if err != nil {
		queue.stop()
		t.logger.Debug("Hint search field unavailable, reading the key stream", zap.Error(err))

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
