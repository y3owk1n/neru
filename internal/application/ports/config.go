package ports

import (
	"context"

	"github.com/y3owk1n/neru/internal/config"
)

// ConfigPort defines the interface for configuration management.
// Implementations handle loading, validation, and watching for config changes.
type ConfigPort interface {
	// Get returns the current configuration.
	Get() *config.Config

	// Reload reloads the configuration from the specified path.
	Reload(ctx context.Context, path string) error

	// Watch returns a channel that receives config updates.
	// The channel is closed when the context is cancelled.
	Watch(ctx context.Context) <-chan *config.Config

	// Validate validates the given configuration.
	Validate(cfg *config.Config) error

	// Path returns the current configuration file path.
	Path() string
}
