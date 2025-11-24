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

	config *config.Config
}

// Get implements ports.ConfigPort.
func (m *MockConfigPort) Get() *config.Config {
	if m.GetFunc != nil {
		return m.GetFunc()
	}

	if m.config != nil {
		return m.config
	}

	return config.DefaultConfig()
}

// Reload implements ports.ConfigPort.
func (m *MockConfigPort) Reload(context context.Context, path string) error {
	if m.ReloadFunc != nil {
		return m.ReloadFunc(context, path)
	}

	return nil
}

// Watch implements ports.ConfigPort.
func (m *MockConfigPort) Watch(context context.Context) <-chan *config.Config {
	if m.WatchFunc != nil {
		return m.WatchFunc(context)
	}

	ch := make(chan *config.Config, 1)
	ch <- m.Get()

	close(ch)

	return ch
}

// Validate implements ports.ConfigPort.
func (m *MockConfigPort) Validate(config *config.Config) error {
	if m.ValidateFunc != nil {
		return m.ValidateFunc(config)
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
func (m *MockConfigPort) SetConfig(config *config.Config) {
	m.config = config
}
