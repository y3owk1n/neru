package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/core"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// findNormalizedMapKey returns the existing map key in m whose normalized form
// matches the normalized form of rawKey. If no match is found it returns rawKey
// itself so callers can use the result directly.
func findNormalizedMapKey[V any](m map[string]V, rawKey string) string {
	norm := NormalizeKeyForComparison(rawKey)
	for k := range m {
		if NormalizeKeyForComparison(k) == norm {
			return k
		}
	}

	return rawKey
}

func validateRawHotkeyTable(fieldName string, rawTable any) error {
	hotkeyMap, ok := rawTable.(map[string]any)
	if !ok {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s must be a TOML table, got %T",
			fieldName,
			rawTable,
		)
	}

	seenRaw := make(map[string]string, len(hotkeyMap))
	for key := range hotkeyMap {
		norm := NormalizeKeyForComparison(key)
		if prev, dup := seenRaw[norm]; dup {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s has duplicate bindings (%q and %q normalize to the same key)",
				fieldName,
				prev,
				key,
			)
		}

		seenRaw[norm] = key
	}

	return nil
}

// AlertProvider defines the interface for displaying native system alerts.
// This is used to break the import cycle between config and ports.
type AlertProvider interface {
	ShowAlert(ctx context.Context, title, message string) error
}

// Service manages application configuration with thread-safe access and change notifications.
// This replaces the global configuration pattern with dependency injection.

// safeSendConfig attempts to send a config without blocking.
// Returns true if sent successfully, false if channel is full or closed.
func safeSendConfig(_channel chan<- *Config, config *Config) bool {
	sent := true
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Channel was closed between copying watchers and sending.
				// This is expected when a watcher's context is canceled
				// concurrently with a Reload or Update.
				sent = false
			}
		}()

		select {
		case _channel <- config:
		default:
			// Channel is full
			sent = false
		}
	}()

	return sent
}

// Service manages application configuration with thread-safe access and change notifications.
// This replaces the global configuration pattern with dependency injection.
type Service struct {
	config        *Config
	path          string
	mu            sync.RWMutex
	watchers      []chan<- *Config
	logger        *zap.Logger
	alertProvider AlertProvider
}

// NewService creates a new configuration service.
func NewService(
	cfg *Config,
	path string,
	logger *zap.Logger,
	alertProvider AlertProvider,
) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &Service{
		config:        cfg,
		path:          path,
		logger:        logger,
		alertProvider: alertProvider,
	}
}

// LoadWithValidation loads configuration from the specified path and returns both
// the config and any validation error separately. This allows callers to decide
// how to handle validation failures (e.g., show alert and use default config).
func (s *Service) LoadWithValidation(path string) *LoadResult {
	configResult := &LoadResult{
		Config:     defaultConfigForDecoding(),
		ConfigPath: path,
	}

	explicitPath := path != ""
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
		if explicitPath {
			configResult.ValidationError = core.WrapConfigFailed(statErr, "config file not found")
		} else {
			s.logger.Info("Config file not found, using default configuration")
			// Clear ConfigPath for auto-discovered missing files
			configResult.ConfigPath = ""
		}

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

	configResult.Config.ResolveThemeDefaults()

	// Process hotkeys from raw map.
	// User entries are merged on top of the defaults from DefaultConfig().
	// To remove a default binding, set it to "__disabled__".
	// An empty [hotkeys] section (no keys) disables all hotkeys — this is
	// the documented way for external hotkey daemons (e.g. skhd) to manage
	// shortcuts without conflicts. Modes remain accessible via CLI commands.
	if hot, ok := raw["hotkeys"]; ok {
		hotMap, isTable := hot.(map[string]any)
		if !isTable {
			configResult.ValidationError = derrors.Newf(
				derrors.CodeInvalidConfig,
				"[hotkeys] must be a TOML table, got %T",
				hot,
			)
			configResult.Config = DefaultConfig()

			s.logger.Warn("Invalid hotkeys section type",
				zap.Any("value", hot),
				zap.Error(configResult.ValidationError))

			return configResult
		}

		if len(hotMap) == 0 {
			// Empty [hotkeys] section: disable all hotkeys.
			configResult.Config.Hotkeys.Bindings = map[string][]string{}
		} else {
			err = validateRawHotkeyTable("hotkeys", hotMap)
			if err != nil {
				configResult.ValidationError = err
				configResult.Config = DefaultConfig()

				s.logger.Warn("Duplicate normalized hotkey in config",
					zap.Error(configResult.ValidationError))

				return configResult
			}

			// Merge user entries on top of defaults (already populated by DefaultConfig).
			for key, value := range hotMap {
				// Find the existing default key that normalizes to the same value
				// so that e.g. "cmd+shift+s" correctly overrides "Cmd+Shift+S".
				canonicalKey := findNormalizedMapKey(
					configResult.Config.Hotkeys.Bindings, key,
				)

				switch _val := value.(type) {
				case string:
					if _val == DisabledSentinel {
						if _, exists := configResult.Config.Hotkeys.Bindings[canonicalKey]; !exists {
							s.logger.Warn("__disabled__ used for key that is not a default binding",
								zap.String("key", key))
						}

						delete(configResult.Config.Hotkeys.Bindings, canonicalKey)
					} else {
						// Remove old casing before inserting with user's casing.
						delete(configResult.Config.Hotkeys.Bindings, canonicalKey)
						configResult.Config.Hotkeys.Bindings[key] = []string{_val}
					}
				case []any:
					actions := make([]string, 0, len(_val))
					for _, a := range _val {
						actionStr, ok := a.(string)
						if !ok {
							configResult.ValidationError = derrors.Newf(
								derrors.CodeInvalidConfig,
								"hotkeys.%s must be a string or array of strings",
								key,
							)
							configResult.Config = DefaultConfig()
							s.logger.Warn("Invalid hotkey configuration",
								zap.String("key", key),
								zap.Any("value", value),
								zap.Error(configResult.ValidationError))

							return configResult
						}

						actions = append(actions, actionStr)
					}

					// Handle __disabled__ sentinel in array form for consistency
					// with per-mode hotkeys.
					if len(actions) == 1 && actions[0] == DisabledSentinel {
						if _, exists := configResult.Config.Hotkeys.Bindings[canonicalKey]; !exists {
							s.logger.Warn("__disabled__ used for key that is not a default binding",
								zap.String("key", key))
						}

						delete(configResult.Config.Hotkeys.Bindings, canonicalKey)
					} else {
						delete(configResult.Config.Hotkeys.Bindings, canonicalKey)
						configResult.Config.Hotkeys.Bindings[key] = actions
					}
				default:
					configResult.ValidationError = derrors.Newf(
						derrors.CodeInvalidConfig,
						"hotkeys.%s must be a string or array of strings",
						key,
					)
					configResult.Config = DefaultConfig()
					s.logger.Warn("Invalid hotkey configuration",
						zap.String("key", key),
						zap.Any("value", value),
						zap.Error(configResult.ValidationError))

					return configResult
				}
			}
		}
	}

	// Process per-mode hotkeys from raw map.
	// These fields are tagged toml:"-" (to prevent the encoder from emitting
	// arrays for single-action entries), so the struct decoder skips them.
	// User entries are merged on top of the defaults from DefaultConfig().
	// To remove a default binding, set it to "__disabled__".
	// An empty [<mode>.hotkeys] section clears all bindings for that mode.
	type modeHotkeys struct {
		modeKey string
		dest    *map[string]StringOrStringArray
	}

	modeHotkeyList := []modeHotkeys{
		{"scroll", &configResult.Config.Scroll.Hotkeys},
		{"hints", &configResult.Config.Hints.Hotkeys},
		{"grid", &configResult.Config.Grid.Hotkeys},
		{"recursive_grid", &configResult.Config.RecursiveGrid.Hotkeys},
	}

	for _, modeHotkey := range modeHotkeyList {
		modeRaw, modeRawOk := raw[modeHotkey.modeKey]
		if !modeRawOk {
			continue
		}

		modeMap, modeRawOk := modeRaw.(map[string]any)
		if !modeRawOk {
			continue
		}

		chRaw, modeRawOk := modeMap["hotkeys"]
		if !modeRawOk {
			continue
		}

		chMap, modeRawOk := chRaw.(map[string]any)
		if !modeRawOk {
			continue
		}

		if len(chMap) == 0 {
			// Empty section: clear all bindings for this mode.
			*modeHotkey.dest = make(map[string]StringOrStringArray)

			continue
		}

		err = validateRawHotkeyTable(modeHotkey.modeKey+".hotkeys", chMap)
		if err != nil {
			configResult.ValidationError = err
			configResult.Config = DefaultConfig()

			s.logger.Warn("Duplicate normalized custom hotkey in config",
				zap.String("mode", modeHotkey.modeKey),
				zap.Error(configResult.ValidationError))

			return configResult
		}

		// Merge user entries on top of defaults (already populated by DefaultConfig).
		for key, value := range chMap {
			var _sosa StringOrStringArray

			err := _sosa.UnmarshalTOML(value)
			if err != nil {
				configResult.ValidationError = derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s.hotkeys.%s: %v",
					modeHotkey.modeKey,
					key,
					err,
				)
				configResult.Config = DefaultConfig()

				return configResult
			}

			// Find the existing default key that normalizes to the same value
			// so that e.g. "escape" correctly overrides "Escape".
			canonicalKey := findNormalizedMapKey(*modeHotkey.dest, key)

			// Sentinel value removes the default binding for this key.
			if len(_sosa) == 1 && _sosa[0] == DisabledSentinel {
				if _, exists := (*modeHotkey.dest)[canonicalKey]; !exists {
					s.logger.Warn("__disabled__ used for key that is not a default binding",
						zap.String("mode", modeHotkey.modeKey),
						zap.String("key", key))
				}

				delete(*modeHotkey.dest, canonicalKey)
			} else {
				// Remove old casing before inserting with user's casing.
				delete(*modeHotkey.dest, canonicalKey)
				(*modeHotkey.dest)[key] = _sosa
			}
		}
	}

	if hintsRaw, ok := raw["hints"].(map[string]any); ok {
		switch appConfigsRaw := hintsRaw["app_configs"].(type) {
		case []any:
			for idx, entry := range appConfigsRaw {
				appMap, isMap := entry.(map[string]any)
				if !isMap {
					continue
				}

				hotkeysRaw, hasHotkeys := appMap["hotkeys"]
				if !hasHotkeys {
					continue
				}

				err = validateRawHotkeyTable(
					fmt.Sprintf("hints.app_configs[%d].hotkeys", idx),
					hotkeysRaw,
				)
				if err != nil {
					configResult.ValidationError = err
					configResult.Config = DefaultConfig()

					s.logger.Warn("Duplicate normalized app hotkey in config",
						zap.Int("app_config_index", idx),
						zap.Error(configResult.ValidationError))

					return configResult
				}
			}
		case []map[string]any:
			for idx, appMap := range appConfigsRaw {
				hotkeysRaw, ok := appMap["hotkeys"]
				if !ok {
					continue
				}

				err = validateRawHotkeyTable(
					fmt.Sprintf("hints.app_configs[%d].hotkeys", idx),
					hotkeysRaw,
				)
				if err != nil {
					configResult.ValidationError = err
					configResult.Config = DefaultConfig()

					s.logger.Warn("Duplicate normalized app hotkey in config",
						zap.Int("app_config_index", idx),
						zap.Error(configResult.ValidationError))

					return configResult
				}
			}
		}
	}

	validateErr := configResult.Config.Validate()
	if validateErr != nil {
		configResult.ValidationError = core.WrapConfigFailed(validateErr, "validate configuration")
		configResult.Config = DefaultConfig()

		s.logger.Warn("Configuration validation failed",
			zap.Error(configResult.ValidationError))

		return configResult
	}

	s.logger.Info("Configuration loaded successfully")

	return configResult
}

// DefaultConfigDir returns the preferred directory for the Neru config file.
// It checks $XDG_CONFIG_HOME first, falling back to ~/.config/neru.
// This is the single source of truth for the primary config location,
// used by both FindConfigFile and config init.
func DefaultConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "neru"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".config", "neru"), nil
}

// FindConfigFile searches for a configuration file in standard locations.
// Returns the path to the config file, or an empty string if not found.
func (s *Service) FindConfigFile() string {
	// Try preferred config directory first (XDG_CONFIG_HOME or ~/.config)
	configDir, dirErr := DefaultConfigDir()
	if dirErr == nil {
		path := filepath.Join(configDir, "config.toml")
		if found := s.tryConfigPath(path); found != "" {
			return found
		}
	} else {
		s.logger.Warn("Failed to determine config directory", zap.Error(dirErr))
	}

	// When XDG_CONFIG_HOME is set, also check ~/.config as a fallback
	if os.Getenv("XDG_CONFIG_HOME") != "" {
		homeDir, homeErr := os.UserHomeDir()
		if homeErr == nil {
			path := filepath.Join(homeDir, ".config", "neru", "config.toml")
			if found := s.tryConfigPath(path); found != "" {
				return found
			}
		}
	}

	// Try legacy and current-directory locations
	homeDir, homeErr := os.UserHomeDir()
	if homeErr == nil {
		// Try .neru.toml
		path := filepath.Join(homeDir, ".neru.toml")
		if found := s.tryConfigPath(path); found != "" {
			return found
		}
	} else {
		s.logger.Warn("Failed to get user home directory", zap.Error(homeErr))
	}

	// Try current directory
	if found := s.tryConfigPath("neru.toml"); found != "" {
		return found
	}

	// Try config.toml
	if found := s.tryConfigPath("config.toml"); found != "" {
		return found
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
			s.logger.Debug("Watcher channel full, skipping notification")

			continue
		}

		// Check if context was canceled during send
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

		// Close channel to honor interface contract
		close(channel)
	}()

	return channel
}

// Validate validates the given configuration.
func (s *Service) Validate(config *Config) error {
	// Delegate to Config.Validate for comprehensive validation
	return config.Validate()
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
		if !safeSendConfig(watcher, config) {
			s.logger.Debug("Watcher channel full, skipping notification")
		}
		// Note: Update doesn't check context cancellation as it's a synchronous operation
	}

	return nil
}

// tryConfigPath attempts to find a config file at the given path.
// Returns the path if it exists, empty string otherwise.
func (s *Service) tryConfigPath(path string) string {
	_, err := os.Stat(path)
	if err == nil {
		return path
	}

	if !os.IsNotExist(err) {
		s.logger.Warn("Failed to check config file",
			zap.String("path", path),
			zap.Error(err))
	}

	return ""
}
