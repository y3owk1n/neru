//go:build darwin

package textinput

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#include "../platform/darwin/textinput.h"
#include <stdlib.h>

extern void textInputQueryBridge(char* query, void* userData);
extern void textInputConfirmBridge(void* userData);
extern void textInputCancelBridge(void* userData);
*/
import "C"

import (
	"context"
	"sync"
	"unsafe"

	"go.uber.org/zap"

	// Blank import links the Objective-C bridge that defines
	// NeruStartHintSearchTextInput / NeruStopHintSearchTextInput. Without it the
	// package compiles but fails to link on its own, which is why this package
	// had no tests. `go test ./internal/adapter/textinput/` could not build a
	// test binary. keyfeed_darwin.go carries the same import for the same reason.
	_ "github.com/y3owk1n/neru/internal/adapter/platform/darwin"
	"github.com/y3owk1n/neru/internal/ports"
)

// TextInput manages the macOS native text input session.
//
// The field calls the bridges on the main thread, sometimes inside a round
// trip that the handler waits on while holding its lock. So the bridges only
// post to the session's queue, and the queue runs the callbacks in order on a
// goroutine of its own. Start and Stop release mu before entering C, since the
// bridges take it on the main thread.
type TextInput struct {
	logger *zap.Logger
	queue  *callbackQueue
	mu     sync.RWMutex
}

var (
	globalTextInput   *TextInput
	globalTextInputMu sync.RWMutex
)

// NewTextInput creates a new TextInput.
func NewTextInput(logger *zap.Logger) *TextInput {
	textInput := &TextInput{logger: logger}

	globalTextInputMu.Lock()
	globalTextInput = textInput
	globalTextInputMu.Unlock()

	return textInput
}

// StartHintSearchSession starts the native hint search session.
func (t *TextInput) StartHintSearchSession(
	_ context.Context,
	callbacks ports.TextInputCallbacks,
	frame ports.TextInputFrame,
) (bool, error) {
	queue := newCallbackQueue(callbacks)

	t.mu.Lock()
	previous := t.queue
	t.queue = queue
	t.mu.Unlock()

	if previous != nil {
		previous.stop()
	}

	started := C.NeruStartHintSearchTextInput(
		C.TextInputQueryCallback(C.textInputQueryBridge),
		C.TextInputControlCallback(C.textInputConfirmBridge),
		C.TextInputControlCallback(C.textInputCancelBridge),
		C.int(frame.X),
		C.int(frame.Y),
		C.int(frame.Width),
		C.int(frame.Height),
		nil,
	)

	if started == 0 {
		// The handler does not stop a session that never started.
		t.mu.Lock()
		if t.queue == queue {
			t.queue = nil
		}
		t.mu.Unlock()

		queue.stop()

		return false, nil
	}

	return true, nil
}

// StopHintSearchSession stops the native hint search session.
func (t *TextInput) StopHintSearchSession(_ context.Context) error {
	t.mu.Lock()
	queue := t.queue
	t.queue = nil
	t.mu.Unlock()

	if queue != nil {
		queue.stop()
	}

	C.NeruStopHintSearchTextInput()

	return nil
}

// activeQueue returns the running session's queue, or nil when there is none.
func activeQueue() *callbackQueue {
	globalTextInputMu.RLock()
	textInput := globalTextInput
	globalTextInputMu.RUnlock()

	if textInput == nil {
		return nil
	}

	textInput.mu.RLock()
	defer textInput.mu.RUnlock()

	return textInput.queue
}

//export textInputQueryBridge
func textInputQueryBridge(query *C.char, _ unsafe.Pointer) {
	queue := activeQueue()
	if queue == nil {
		return
	}

	queryStr := ""
	if query != nil {
		queryStr = C.GoString(query)
	}

	queue.queryChanged(queryStr)
}

//export textInputConfirmBridge
func textInputConfirmBridge(_ unsafe.Pointer) {
	if queue := activeQueue(); queue != nil {
		queue.confirm()
	}
}

//export textInputCancelBridge
func textInputCancelBridge(_ unsafe.Pointer) {
	if queue := activeQueue(); queue != nil {
		queue.cancel()
	}
}
