package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/y3owk1n/neru/internal/core"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"go.uber.org/zap"
)

// Service manages application configuration with thread-safe access and change notifications.
// This replaces the global configuration pattern with dependency injection.

// safeSendConfig safely sends a config to a watcher channel that might be closed.
func safeSendConfig(ch chan<- *Config, config *Config) (sent bool) {
	defer func() {
		if recover() != nil {
			// Channel was closed, mark as not sent
			sent = false
		}
	}()

	select {
	case ch <- config:
		return true
	default:
		return false
	}
}

type Service struct {
	config   *Config
	path     string
	mu       sync.RWMutex
	watchers []chan<- *Config
	logger   *zap.Logger
}

// NewService creates a new configuration service.
func NewService(cfg *Config, path string, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Service{
		config: cfg,
		path:   path,
		logger: logger,
	}
}

// LoadWithValidation loads configuration from the specified path and returns both
// the config and any validation error separately. This allows callers to decide
// how to handle validation failures (e.g., show alert and use default config).
func (s *Service) LoadWithValidation(path string) *LoadResult {
	configResult := &LoadResult{
		Config:     DefaultConfig(),
		ConfigPath: path,
	}

	if path == "" {
		configResult.ConfigPath = s.FindConfigFile()
	}

	if configResult.ConfigPath == "" {
		s.logger.Info("No config file specified or found, using default configuration")

		return configResult
	}

	s.logger.Info("Loading config from", zap.String("path", configResult.ConfigPath))

	_, statErr := os.Stat(configResult.ConfigPath)
	if os.IsNotExist(statErr) {
		s.logger.Info("Config file not found, using default configuration")

		return configResult
	}

	// Double decode is necessary because:
	// 1. Raw map is needed for hotkey processing (which may have complex validation)
	// 2. Typed struct decode ensures proper validation of other config fields
	// TOML library doesn't support mixed struct/map decoding in single pass

	var raw map[string]any

	_, decodeErr := toml.DecodeFile(configResult.ConfigPath, &raw)
	if decodeErr != nil {
		configResult.ValidationError = core.WrapConfigFailed(decodeErr, "parse config file")
		configResult.Config = DefaultConfig()

		return configResult
	}

	// Decode into typed config struct (separate pass for validation)
	_, err := toml.DecodeFile(configResult.ConfigPath, configResult.Config)
	if err != nil {
		configResult.ValidationError = core.WrapConfigFailed(err, "parse config file")
		configResult.Config = DefaultConfig()

		return configResult
	}

	// Process hotkeys from raw map
	if hot, ok := raw["hotkeys"]; ok {
		if hotMap, ok := hot.(map[string]any); ok && len(hotMap) > 0 {
			// Clear default bindings when user provides hotkeys config
			configResult.Config.Hotkeys.Bindings = map[string]string{}

			for key, value := range hotMap {
				str, ok := value.(string)
				if !ok {
					configResult.ValidationError = derrors.Newf(
						derrors.CodeInvalidConfig,
						"hotkeys.%s must be a string action",
						key,
					)
					configResult.Config = DefaultConfig()

					return configResult
				}

				configResult.Config.Hotkeys.Bindings[key] = str
			}
		}
	}

	validateErr := configResult.Config.Validate()
	if validateErr != nil {
		configResult.ValidationError = core.WrapConfigFailed(validateErr, "validate configuration")
		configResult.Config = DefaultConfig()

		return configResult
	}

	s.logger.Info("Configuration loaded successfully")

	return configResult
}

// FindConfigFile searches for a configuration file in standard locations.
// Returns the path to the config file, or an empty string if not found.
func (s *Service) FindConfigFile() string {
	// Try XDG config directory first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		path := filepath.Join(xdgConfig, "neru", "config.toml")

		_, err := os.Stat(path)
		if err == nil {
			return path
		}
	}

	// Try standard config directory
	homeDir, homeErr := os.UserHomeDir()
	if homeErr == nil {
		// Try .config/neru/config.toml
		path := filepath.Join(homeDir, ".config", "neru", "config.toml")

		_, err := os.Stat(path)
		if err == nil {
			return path
		}

		// Try .neru.toml
		path = filepath.Join(homeDir, ".neru.toml")

		_, err = os.Stat(path)
		if err == nil {
			return path
		}
	}

	// Try current directory
	_, err := os.Stat("neru.toml")
	if err == nil {
		return "neru.toml"
	}

	// Try config.toml
	_, err = os.Stat("config.toml")
	if err == nil {
		return "config.toml"
	}

	return ""
}

// LoadAndApply loads configuration and applies it to the service.
func (s *Service) LoadAndApply(path string) error {
	loadResult := s.LoadWithValidation(path)

	if loadResult.ValidationError != nil {
		return loadResult.ValidationError
	}

	s.mu.Lock()
	s.config = loadResult.Config
	s.path = loadResult.ConfigPath
	s.mu.Unlock()

	return nil
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
	loadResult := s.LoadWithValidation(path)

	if loadResult.ValidationError != nil {
		return loadResult.ValidationError
	}

	// Update configuration atomically
	s.mu.Lock()
	s.config = loadResult.Config
	s.path = loadResult.ConfigPath
	watchers := make([]chan<- *Config, len(s.watchers))
	copy(watchers, s.watchers)
	s.mu.Unlock()

	// Notify watchers (outside the lock to avoid deadlock)
	for _, watcher := range watchers {
		if !safeSendConfig(watcher, loadResult.Config) {
			// Channel was closed or full, skip this watcher
			continue
		}

		// Check if context was cancelled during send
		select {
		case <-ctx.Done():
			return core.WrapContextCanceled(ctx, "notify config watchers")
		default:
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

	// Clean up when context is done (use sync.Once to prevent double-close)
	var once sync.Once
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

		// Ensure channel is only closed once
		once.Do(func() {
			close(channel)
		})
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
		if len(config.Hints.HintCharacters) < MinCharactersLength {
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
		if len(config.Grid.Characters) < MinCharactersLength {
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
		safeSendConfig(watcher, config)
		// Note: Update doesn't check context cancellation as it's a synchronous operation
	}

	return nil
}
