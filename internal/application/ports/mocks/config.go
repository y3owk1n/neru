package mocks

import (
	"context"

	"github.com/y3owk1n/neru/internal/config"
)

// MockConfigPort is a mock implementation of ports.ConfigPort.
type MockConfigPort struct {
	GetFunc      func() *config.Config
	ReloadFunc   func(context.Context, string) error
	WatchFunc    func(context.Context) <-chan *config.Config
	ValidateFunc func(*config.Config) error
	PathFunc     func() string

	cfg *config.Config
}

// Get implements ports.ConfigPort.
func (m *MockConfigPort) Get() *config.Config {
	if m.GetFunc != nil {
		return m.GetFunc()
	}
	if m.cfg != nil {
		return m.cfg
	}
	return config.DefaultConfig()
}

// Reload implements ports.ConfigPort.
func (m *MockConfigPort) Reload(ctx context.Context, path string) error {
	if m.ReloadFunc != nil {
		return m.ReloadFunc(ctx, path)
	}
	return nil
}

// Watch implements ports.ConfigPort.
func (m *MockConfigPort) Watch(ctx context.Context) <-chan *config.Config {
	if m.WatchFunc != nil {
		return m.WatchFunc(ctx)
	}
	ch := make(chan *config.Config, 1)
	ch <- m.Get()
	close(ch)
	return ch
}

// Validate implements ports.ConfigPort.
func (m *MockConfigPort) Validate(cfg *config.Config) error {
	if m.ValidateFunc != nil {
		return m.ValidateFunc(cfg)
	}
	return nil
}

// Path implements ports.ConfigPort.
func (m *MockConfigPort) Path() string {
	if m.PathFunc != nil {
		return m.PathFunc()
	}
	return "/mock/config.toml"
}

// SetConfig sets the config for the mock.
func (m *MockConfigPort) SetConfig(cfg *config.Config) {
	m.cfg = cfg
}
