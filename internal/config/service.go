package config

import (
	"context"
	"fmt"
	"sync"

	"github.com/y3owk1n/neru/internal/errors"
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

// Reload reloads the configuration from the specified path.
func (s *Service) Reload(ctx context.Context, path string) error {
	// Load and validate new config
	result := LoadWithValidation(path)

	if result.ValidationError != nil {
		return errors.Wrap(result.ValidationError, errors.CodeInvalidConfig,
			"configuration validation failed")
	}

	// Update configuration atomically
	s.mu.Lock()
	s.config = result.Config
	s.path = result.ConfigPath
	watchers := make([]chan<- *Config, len(s.watchers))
	copy(watchers, s.watchers)
	s.mu.Unlock()

	// Notify watchers (outside the lock to avoid deadlock)
	for _, watcher := range watchers {
		select {
		case watcher <- result.Config:
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Skip if watcher is not ready
		}
	}

	return nil
}

// Watch returns a channel that receives configuration updates.
// The channel is closed when the context is canceled.
func (s *Service) Watch(ctx context.Context) <-chan *Config {
	ch := make(chan *Config, 1)

	s.mu.Lock()
	s.watchers = append(s.watchers, ch)
	s.mu.Unlock()

	// Send current config immediately
	ch <- s.Get()

	// Clean up when context is done
	go func() {
		<-ctx.Done()
		s.mu.Lock()
		defer s.mu.Unlock()

		// Remove watcher from list
		for i, w := range s.watchers {
			if w == ch {
				s.watchers = append(s.watchers[:i], s.watchers[i+1:]...)
				break
			}
		}
		close(ch)
	}()

	return ch
}

// Validate validates the given configuration.
func (s *Service) Validate(cfg *Config) error {
	if cfg == nil {
		return errors.New(errors.CodeInvalidConfig, "configuration cannot be nil")
	}

	// Validate hints configuration
	if cfg.Hints.Enabled {
		if len(cfg.Hints.HintCharacters) < 2 {
			return errors.Newf(errors.CodeInvalidConfig,
				"hints.hint_characters must have at least 2 characters, got %d",
				len(cfg.Hints.HintCharacters))
		}

		if len(cfg.Hints.ClickableRoles) == 0 {
			return errors.New(errors.CodeInvalidConfig,
				"hints.clickable_roles cannot be empty when hints are enabled")
		}
	}

	// Validate grid configuration
	if cfg.Grid.Enabled {
		if len(cfg.Grid.Characters) < 2 {
			return errors.Newf(errors.CodeInvalidConfig,
				"grid.characters must have at least 2 characters, got %d",
				len(cfg.Grid.Characters))
		}
	}

	return nil
}

// Update updates the configuration (for testing/internal use).
func (s *Service) Update(cfg *Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}

	s.mu.Lock()
	s.config = cfg
	watchers := make([]chan<- *Config, len(s.watchers))
	copy(watchers, s.watchers)
	s.mu.Unlock()

	// Notify watchers
	for _, watcher := range watchers {
		select {
		case watcher <- cfg:
		default:
			// Skip if watcher is not ready
		}
	}

	return nil
}

// LoadOrDefault loads configuration from the given path, or returns default config if it fails.
func LoadOrDefault(path string) (*Service, error) {
	result := LoadWithValidation(path)

	if result.ValidationError != nil {
		// Return default config with error
		defaultCfg := DefaultConfig()
		return NewService(
				defaultCfg,
				"",
			), fmt.Errorf(
				"failed to load config: %w",
				result.ValidationError,
			)
	}

	return NewService(result.Config, result.ConfigPath), nil
}
