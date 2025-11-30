package ports

import (
	"context"

	"github.com/y3owk1n/neru/internal/config"
)

// ConfigRetrieval defines the interface for retrieving configuration data.
type ConfigRetrieval interface {
	// Get returns the current configuration.
	Get() *config.Config

	// Path returns the current configuration file path.
	Path() string
}

// ConfigManagement defines the interface for managing configuration lifecycle.
type ConfigManagement interface {
	// Reload reloads the configuration from the specified path.
	Reload(ctx context.Context, path string) error

	// Watch returns a channel that receives config updates.
	// The channel is closed when the context is canceled.
	Watch(ctx context.Context) <-chan *config.Config
}

// ConfigValidation defines the interface for configuration validation.
type ConfigValidation interface {
	// Validate validates the given configuration.
	Validate(cfg *config.Config) error
}

// ConfigPort defines the full interface for configuration management.
// It composes retrieval, lifecycle management, and validation capabilities.
// Implementations handle loading, validation, and watching for config changes.
type ConfigPort interface {
	ConfigRetrieval
	ConfigManagement
	ConfigValidation
}
