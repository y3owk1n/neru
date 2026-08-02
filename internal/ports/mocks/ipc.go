package mocks

import (
	"context"
	"sync"

	"github.com/y3owk1n/neru/internal/ports"
)

// MockIPCPort is a mock implementation of ports.IPCPort.
//
// It tracks server state and records commands sent through it, so a test can
// assert on IPC traffic without a socket or named pipe.
type MockIPCPort struct {
	StartFunc func(context.Context) error
	StopFunc  func(context.Context) error
	SendFunc  func(context.Context, any) (any, error)

	mu       sync.Mutex
	running  bool
	commands []any
}

// Start implements ports.IPCPort.
func (m *MockIPCPort) Start(ctx context.Context) error {
	if m.StartFunc != nil {
		return m.StartFunc(ctx)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.running = true

	return nil
}

// Stop implements ports.IPCPort.
func (m *MockIPCPort) Stop(ctx context.Context) error {
	if m.StopFunc != nil {
		return m.StopFunc(ctx)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.running = false

	return nil
}

// Send implements ports.IPCPort. With no SendFunc configured it records the
// command and reports no response and no error, which is what a test asserting
// only on what was sent wants.
//
//nolint:nilnil // "no response, no error" is the meaningful default here.
func (m *MockIPCPort) Send(ctx context.Context, command any) (any, error) {
	m.mu.Lock()
	m.commands = append(m.commands, command)
	m.mu.Unlock()

	if m.SendFunc != nil {
		return m.SendFunc(ctx, command)
	}

	return nil, nil
}

// IsRunning implements ports.IPCPort.
func (m *MockIPCPort) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.running
}

// SentCommands returns the commands passed to Send, in order.
func (m *MockIPCPort) SentCommands() []any {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]any(nil), m.commands...)
}

// Ensure MockIPCPort implements ports.IPCPort.
var _ ports.IPCPort = (*MockIPCPort)(nil)
