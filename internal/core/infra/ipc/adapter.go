package ipc

import (
	"context"
	"sync"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// Adapter implements ports.IPCPort by wrapping the existing IPC server.
type Adapter struct {
	server  *Server
	logger  *zap.Logger
	running bool
	mu      sync.Mutex
}

// NewAdapter creates a new IPC adapter.
func NewAdapter(server *Server, logger *zap.Logger) *Adapter {
	return &Adapter{
		server: server,
		logger: logger,
	}
}

// Start starts the IPC server.
func (a *Adapter) Start(_ context.Context) error {
	// The existing IPC server Start() is non-blocking and runs in a goroutine
	// but doesn't take a context.
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return nil
	}

	a.server.Start()
	a.running = true

	return nil
}

// Stop stops the IPC server.
func (a *Adapter) Stop(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	stopServerErr := a.server.Stop()
	if stopServerErr == nil {
		a.running = false
	}

	return stopServerErr
}

// IsRunning returns true if the IPC server is running.
func (a *Adapter) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.running
}

// Send sends a command to the IPC server.
// Send is not supported by the server adapter.
func (a *Adapter) Send(_ context.Context, _ any) (any, error) {
	// This method doesn't make sense for the server adapter.
	// It should be implemented by a client adapter if needed.
	return nil, derrors.New(derrors.CodeInternal, "Send not implemented for server adapter")
}

// Ensure Adapter implements ports.IPCPort.
var _ ports.IPCPort = (*Adapter)(nil)
