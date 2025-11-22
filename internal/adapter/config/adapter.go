package config

import (
	"context"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/config"
)

// Adapter implements ports.ConfigPort by wrapping config.Service.
type Adapter struct {
	service *config.Service
}

// NewAdapter creates a new config adapter.
func NewAdapter(service *config.Service) *Adapter {
	return &Adapter{
		service: service,
	}
}

// Get returns the current configuration.
func (a *Adapter) Get() *config.Config {
	return a.service.Get()
}

// Reload reloads the configuration from the specified path.
func (a *Adapter) Reload(ctx context.Context, path string) error {
	return a.service.Reload(ctx, path)
}

// Watch returns a channel that receives config updates.
func (a *Adapter) Watch(ctx context.Context) <-chan *config.Config {
	return a.service.Watch(ctx)
}

// Validate validates the given configuration.
func (a *Adapter) Validate(cfg *config.Config) error {
	return a.service.Validate(cfg)
}

// Path returns the current configuration file path.
func (a *Adapter) Path() string {
	return a.service.Path()
}

// Ensure Adapter implements ports.ConfigPort
var _ ports.ConfigPort = (*Adapter)(nil)
