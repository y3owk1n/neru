package config

import (
	"context"
	"sync"

	derrors "github.com/y3owk1n/neru/internal/errors"
)

// Service manages application configuration with thread-safe access and change notifications.
// This replaces the global configuration pattern with dependency injection.
type Service struct {
	config   *Config
	path     string
	mu       sync.RWMutex
	watchers []chan<- *Config
}

// NewService creates a new configuration service.
func NewService(cfg *Config, path string) *Service {
	return &Service{
		config: cfg,
		path:   path,
	}
}

// Get returns the current configuration (thread-safe).
func (s *Service) Get() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config
}

// Path returns the configuration file path.
func (s *Service) Path() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.path
}

// GetConfigPath is an alias for Path for compatibility.
func (s *Service) GetConfigPath() string {
	return s.Path()
}

// Reload reloads the configuration from the specified path.
func (s *Service) Reload(ctx context.Context, path string) error {
	// Load and validate new config
	configResult := LoadWithValidation(path)

	if configResult.ValidationError != nil {
		return derrors.Wrap(configResult.ValidationError, derrors.CodeInvalidConfig,
			"configuration validation failed")
	}

	// Update configuration atomically
	s.mu.Lock()
	s.config = configResult.Config
	s.path = configResult.ConfigPath
	watchers := make([]chan<- *Config, len(s.watchers))
	copy(watchers, s.watchers)
	s.mu.Unlock()

	// Notify watchers (outside the lock to avoid deadlock)
	for _, watcher := range watchers {
		select {
		case watcher <- configResult.Config:
		case <-ctx.Done():
			return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
		default:
			// Skip if watcher is not ready
		}
	}

	return nil
}

// ReloadConfig reloads the configuration from the specified path (compatibility wrapper).
func (s *Service) ReloadConfig(path string) error {
	return s.Reload(context.Background(), path)
}

// Watch returns a channel that receives configuration updates.
// The channel is closed when the context is canceled.
func (s *Service) Watch(ctx context.Context) <-chan *Config {
	channel := make(chan *Config, 1)

	s.mu.Lock()
	s.watchers = append(s.watchers, channel)
	s.mu.Unlock()

	// Send current config immediately
	channel <- s.Get()

	// Clean up when context is done
	go func() {
		<-ctx.Done()

		s.mu.Lock()
		defer s.mu.Unlock()

		// Remove watcher from list
		for i, w := range s.watchers {
			if w == channel {
				s.watchers = append(s.watchers[:i], s.watchers[i+1:]...)

				break
			}
		}

		close(channel)
	}()

	return channel
}

// Validate validates the given configuration.
func (s *Service) Validate(config *Config) error {
	if config == nil {
		return derrors.New(derrors.CodeInvalidConfig, "configuration cannot be nil")
	}

	// Validate hints configuration
	if config.Hints.Enabled {
		if len(config.Hints.HintCharacters) < 2 {
			return derrors.Newf(derrors.CodeInvalidConfig,
				"hints.hint_characters must have at least 2 characters, got %d",
				len(config.Hints.HintCharacters))
		}

		if len(config.Hints.ClickableRoles) == 0 {
			return derrors.New(derrors.CodeInvalidConfig,
				"hints.clickable_roles cannot be empty when hints are enabled")
		}
	}

	// Validate grid configuration
	if config.Grid.Enabled {
		if len(config.Grid.Characters) < 2 {
			return derrors.Newf(derrors.CodeInvalidConfig,
				"grid.characters must have at least 2 characters, got %d",
				len(config.Grid.Characters))
		}
	}

	return nil
}

// Update updates the configuration (for testing/internal use).
func (s *Service) Update(config *Config) error {
	validateErr := s.Validate(config)
	if validateErr != nil {
		return validateErr
	}

	s.mu.Lock()
	s.config = config
	watchers := make([]chan<- *Config, len(s.watchers))
	copy(watchers, s.watchers)
	s.mu.Unlock()

	// Notify watchers
	for _, watcher := range watchers {
		select {
		case watcher <- config:
		default:
			// Skip if watcher is not ready
		}
	}

	return nil
}

// LoadOrDefault loads configuration from the given path, or returns default config if it fails.
func LoadOrDefault(path string) (*Service, error) {
	configResult := LoadWithValidation(path)

	if configResult.ValidationError != nil {
		// Return default config with error
		defaultConfig := DefaultConfig()

		return NewService(
				defaultConfig,
				"",
			), derrors.Wrap(
				configResult.ValidationError,
				derrors.CodeInvalidConfig,
				"failed to load config",
			)
	}

	return NewService(configResult.Config, configResult.ConfigPath), nil
}
