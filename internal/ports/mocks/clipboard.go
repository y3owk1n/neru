package mocks

import (
	"context"
	"sync"

	"github.com/y3owk1n/neru/internal/ports"
)

// MockClipboardPort is a mock implementation of ports.ClipboardPort.
type MockClipboardPort struct {
	WriteTextFunc func(context.Context, string) error

	mu      sync.Mutex
	written []string
}

// WriteText implements ports.ClipboardPort.
func (m *MockClipboardPort) WriteText(ctx context.Context, text string) error {
	if m.WriteTextFunc != nil {
		err := m.WriteTextFunc(ctx, text)
		if err != nil {
			return err
		}
	}

	m.mu.Lock()
	m.written = append(m.written, text)
	m.mu.Unlock()

	return nil
}

// Written returns the texts written to the clipboard, in order.
func (m *MockClipboardPort) Written() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.written...)
}

// Ensure MockClipboardPort implements ports.ClipboardPort.
var _ ports.ClipboardPort = (*MockClipboardPort)(nil)
