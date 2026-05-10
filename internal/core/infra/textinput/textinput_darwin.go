//go:build darwin

package textinput

/*
#cgo CFLAGS: -x objective-c
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
	"sync/atomic"
	"unsafe"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core/ports"
)

// TextInput manages the macOS native text input session.
type TextInput struct {
	logger    *zap.Logger
	callbacks ports.TextInputCallbacks
	mu        sync.RWMutex

	querySeq        uint64
	lastExecutedSeq uint64
	callbackMu      sync.Mutex
}

var (
	globalTextInput   *TextInput
	globalTextInputMu sync.RWMutex
)

// NewTextInput creates a new TextInput instance.
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
	t.mu.Lock()
	t.callbacks = callbacks
	t.mu.Unlock()

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
		return false, nil
	}

	return true, nil
}

// StopHintSearchSession stops the native hint search session.
func (t *TextInput) StopHintSearchSession(_ context.Context) error {
	C.NeruStopHintSearchTextInput()

	t.mu.Lock()
	t.callbacks = ports.TextInputCallbacks{}
	t.mu.Unlock()

	return nil
}

//export textInputQueryBridge
func textInputQueryBridge(query *C.char, _ unsafe.Pointer) {
	globalTextInputMu.RLock()
	textInput := globalTextInput
	globalTextInputMu.RUnlock()

	if textInput == nil {
		return
	}

	seq := atomic.AddUint64(&textInput.querySeq, 1)

	queryStr := ""
	if query != nil {
		queryStr = C.GoString(query)
	}

	textInput.mu.RLock()
	callback := textInput.callbacks.OnQueryChanged
	textInput.mu.RUnlock()

	if callback == nil {
		return
	}

	go func(seq uint64, query string) {
		textInput.callbackMu.Lock()
		defer textInput.callbackMu.Unlock()

		if seq < textInput.lastExecutedSeq {
			return
		}
		textInput.lastExecutedSeq = seq

		callback(query)
	}(seq, queryStr)
}

//export textInputConfirmBridge
func textInputConfirmBridge(_ unsafe.Pointer) {
	globalTextInputMu.RLock()
	textInput := globalTextInput
	globalTextInputMu.RUnlock()

	if textInput == nil {
		return
	}

	textInput.mu.RLock()
	callback := textInput.callbacks.OnConfirm
	textInput.mu.RUnlock()

	if callback == nil {
		return
	}

	go func() {
		textInput.callbackMu.Lock()
		defer textInput.callbackMu.Unlock()

		callback()
	}()
}

//export textInputCancelBridge
func textInputCancelBridge(_ unsafe.Pointer) {
	globalTextInputMu.RLock()
	textInput := globalTextInput
	globalTextInputMu.RUnlock()

	if textInput == nil {
		return
	}

	textInput.mu.RLock()
	callback := textInput.callbacks.OnCancel
	textInput.mu.RUnlock()

	if callback == nil {
		return
	}

	go func() {
		textInput.callbackMu.Lock()
		defer textInput.callbackMu.Unlock()

		callback()
	}()
}
