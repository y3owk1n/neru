package loader

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/BurntSushi/toml"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// validateAppConfigsHotkeys validates hotkeys in app_configs sections from raw config.
func validateAppConfigsHotkeys(
	logger *zap.Logger,
	modeName string,
	raw map[string]any,
) *config.LoadResult {
	var appConfigsRaw []any
	switch ac := raw["app_configs"].(type) {
	case []any:
		appConfigsRaw = ac
	case []map[string]any:
		for i := range ac {
			appConfigsRaw = append(appConfigsRaw, ac[i])
		}
	default:
		return nil
	}

	for idx, entry := range appConfigsRaw {
		appMap, isMap := entry.(map[string]any)
		if !isMap {
			continue
		}

		hotkeysRaw, hasHotkeys := appMap["hotkeys"]
		if !hasHotkeys {
			continue
		}

		err := validateRawHotkeyTable(
			fmt.Sprintf("%s.app_configs[%d].hotkeys", modeName, idx),
			hotkeysRaw,
		)
		if err != nil {
			result := &config.LoadResult{
				ValidationError: err,
				Config:          config.DefaultConfig(),
				Written:         config.DefaultConfigForDecoding(),
			}
			logger.Warn("Duplicate normalized app hotkey in config",
				zap.String("mode", modeName),
				zap.Int("app_config_index", idx),
				zap.Error(err))

			return result
		}
	}

	return nil
}

// isBuiltInGlobalModeAction matches single built-in global mode commands.
func isBuiltInGlobalModeAction(actions []string) (string, bool) {
	if len(actions) != 1 {
		return "", false
	}

	parts := strings.Fields(strings.TrimSpace(actions[0]))
	if len(parts) == 0 {
		return "", false
	}

	switch parts[0] {
	case config.ModeNameHints,
		config.ModeNameGrid,
		config.ModeNameRecursiveGrid,
		config.ModeNameScroll:
		return parts[0], true
	default:
		return "", false
	}
}

// removeBindingsForSingleAction removes bindings for one built-in mode action.
func removeBindingsForSingleAction(bindings map[string][]string, action string) {
	for key, existingActions := range bindings {
		existingAction, ok := isBuiltInGlobalModeAction(existingActions)
		if ok && existingAction == action {
			delete(bindings, key)
		}
	}
}

// removeLauncherBindingsForDisabledModes removes launcher keybindings for modes
// that are explicitly disabled in the config.
func removeLauncherBindingsForDisabledModes(cfg *config.Config) {
	modeActions := map[string]bool{
		config.ModeNameHints:         cfg.Hints.Enabled,
		config.ModeNameGrid:          cfg.Grid.Enabled,
		config.ModeNameRecursiveGrid: cfg.RecursiveGrid.Enabled,
	}

	for key, actions := range cfg.Hotkeys.Bindings {
		if len(actions) == 1 {
			if action, ok := isBuiltInGlobalModeAction(actions); ok {
				if modeEnabled, found := modeActions[action]; found && !modeEnabled {
					delete(cfg.Hotkeys.Bindings, key)
				}
			}
		}
	}
}

// parseRawHotkeyActions parses a raw TOML hotkey value into actions.
func parseRawHotkeyActions(fieldName string, value any) ([]string, error) {
	switch val := value.(type) {
	case string:
		return []string{val}, nil
	case []any:
		actions := make([]string, 0, len(val))
		for _, actionValue := range val {
			actionStr, ok := actionValue.(string)
			if !ok {
				return nil, derrors.Newf(
					derrors.CodeInvalidConfig,
					"%s must be a string or array of strings",
					fieldName,
				)
			}

			actions = append(actions, actionStr)
		}

		return actions, nil
	default:
		return nil, derrors.Newf(
			derrors.CodeInvalidConfig,
			"%s must be a string or array of strings",
			fieldName,
		)
	}
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

	// Sorted, so the pair named in the message below is the same on every run
	// rather than whichever two the map happened to yield first.
	keys := slices.Sorted(maps.Keys(hotkeyMap))

	seenRaw := make(map[string]string, len(hotkeyMap))
	for _, key := range keys {
		norm := config.NormalizeKeyForComparison(key)
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

// AlertProvider is the interface for displaying native system alerts.
// This is used to break the import cycle between config and ports.
type AlertProvider interface {
	ShowAlert(ctx context.Context, title, message string) error
}

// safeSendConfig attempts to send a config without blocking.
// Returns true if sent successfully, false if channel is full or closed.
func safeSendConfig(channel chan<- *config.Config, cfg *config.Config) bool {
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
		case channel <- cfg:
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
	config *config.Config
	// written is config before any derived value was settled — see
	// [config.LoadResult.Written]. It moves under the same lock and in the same
	// call as config, because a running configuration paired with a stale
	// written one would have `neru config set` deriving from values nobody
	// wrote. Nil until a load or a WithWritten supplies one; Written() says
	// what that falls back to.
	written       *config.Config
	path          string
	mu            sync.RWMutex
	watchers      []chan<- *config.Config
	logger        *zap.Logger
	alertProvider AlertProvider
	// alertShowing keeps at most one validation alert outstanding. The dialog
	// is modal and no longer blocks the reload, so without this a run of
	// failed reloads would stack dialogs and park a goroutine behind each.
	alertShowing atomic.Bool

	// defaults is the base configuration used as the starting point by
	// LoadWithValidation. It is initialized from config.DefaultConfigForDecoding()
	// in NewService, but can be overridden by tests via withDefaults.
	defaults *config.Config

	// backendInert reports the words a configuration writes that the display
	// backend this process runs under cannot honor, a limit finer than the
	// platform column InertWords judges by. Nil, the default, means the
	// backend keeps every promise the column makes. It is injected rather than
	// detected here, because the detector is an adapter and this package sits
	// below the adapters; WithBackendInert says who supplies it.
	backendInert func(*config.Config, config.Written) parity.Declaration
}

// NewService creates a new configuration service.
func NewService(
	cfg *config.Config,
	path string,
	logger *zap.Logger,
	alertProvider AlertProvider,
) *Service {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &Service{
		config:        cfg,
		defaults:      config.DefaultConfigForDecoding(),
		path:          path,
		logger:        logger.Named("config"),
		alertProvider: alertProvider,
	}
}

// WithBackendInert names the words the display backend this process runs
// under cannot honor, for LoadWithValidation to warn about beside the
// platform-inert ones. The only one today is [config.X11InertWords], which the
// composition roots pass when the detected backend is X11; a test passes it to
// stand in for that detection.
func (s *Service) WithBackendInert(
	inert func(*config.Config, config.Written) parity.Declaration,
) *Service {
	s.backendInert = inert

	return s
}

// WithDefaults sets the base defaults used by LoadWithValidation. This is
// intended for tests that need to override platform-specific default hotkeys.
func (s *Service) WithDefaults(cfg *config.Config) *Service {
	if cfg != nil {
		s.defaults = cfg
	}

	return s
}

// WithWritten records what the configuration the service was constructed with
// was written as, for the caller that loaded it somewhere else — the daemon
// loads once at startup and hands both halves to the app, which builds this
// service from them.
func (s *Service) WithWritten(cfg *config.Config) *Service {
	if cfg != nil {
		s.written = cfg
	}

	return s
}

// Written returns the configuration the running one was derived from, which is
// what a field change has to be applied to. See [config.LoadResult.Written].
//
// It falls back to the running configuration when nothing supplied one, which
// is a service that was handed a config rather than loading one. Deriving from
// an already-derived configuration is what this whole pair exists to avoid, but
// it settles to itself rather than to something wrong: what is lost is the
// re-inference, not the value.
func (s *Service) Written() *config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.written == nil {
		return s.config
	}

	return s.written
}

// FindConfigFile searches for a configuration file in standard locations.
// Returns the path to the config file, or an empty string if not found.
func (s *Service) FindConfigFile() string {
	// Try preferred config directory first (XDG_CONFIG_HOME, %APPDATA% on
	// Windows, or ~/.config)
	preferredDir := ""

	configDir, dirErr := config.DefaultConfigDir()
	if dirErr == nil {
		preferredDir = configDir

		path := filepath.Join(configDir, "config.toml")
		if found := s.tryConfigPath(path); found != "" {
			return found
		}
	} else {
		s.logger.Warn("Failed to determine config directory", zap.Error(dirErr))
	}

	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		s.logger.Warn("Failed to get user home directory", zap.Error(homeErr))
	}

	// Also check ~/.config/neru whenever it is not already the preferred
	// directory. That covers XDG_CONFIG_HOME pointing elsewhere, and Windows,
	// where the preferred directory is %APPDATA%\neru — cross-platform
	// dotfiles keep working there without setting XDG_CONFIG_HOME.
	if homeErr == nil {
		dotConfigDir := filepath.Join(homeDir, ".config", "neru")
		if dotConfigDir != preferredDir {
			path := filepath.Join(dotConfigDir, "config.toml")
			if found := s.tryConfigPath(path); found != "" {
				return found
			}
		}
	}

	// Try legacy and current-directory locations
	if homeErr == nil {
		// Try .neru.toml
		path := filepath.Join(homeDir, ".neru.toml")
		if found := s.tryConfigPath(path); found != "" {
			return found
		}
	}

	// Try current directory
	if found := s.tryConfigPath("neru.toml"); found != "" {
		return found
	}

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
	s.written = loadResult.Written
	s.path = loadResult.ConfigPath
	s.mu.Unlock()

	return nil
}

// Get returns the current configuration (thread-safe).
func (s *Service) Get() *config.Config {
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
	loadResult := s.LoadWithValidation(path)

	if loadResult.ValidationError != nil {
		return loadResult.ValidationError
	}

	s.mu.Lock()
	s.config = loadResult.Config
	s.written = loadResult.Written
	s.path = loadResult.ConfigPath
	watchers := make([]chan<- *config.Config, len(s.watchers))
	copy(watchers, s.watchers)
	s.mu.Unlock()

	for _, watcher := range watchers {
		if !safeSendConfig(watcher, loadResult.Config) {
			s.logger.Debug("Watcher channel full, skipping notification")

			continue
		}

		select {
		case <-ctx.Done():
			return derrors.WrapContextCanceled(ctx, "notify config watchers")
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
func (s *Service) Watch(ctx context.Context) <-chan *config.Config {
	channel := make(chan *config.Config, 1)

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
func (s *Service) Validate(cfg *config.Config) error {
	// Delegate to Config.Validate for comprehensive validation
	return cfg.Validate()
}

// Update updates the configuration (for testing/internal use).
//
// written is the configuration cfg was derived from, kept for the next field
// change. It is not optional: a running configuration left paired with the
// previous written one would have that next change deriving from values nobody
// wrote. A caller with no separate written form passes cfg for both.
func (s *Service) Update(cfg, written *config.Config) error {
	validateErr := s.Validate(cfg)
	if validateErr != nil {
		return validateErr
	}

	s.mu.Lock()
	s.config = cfg
	s.written = written
	watchers := make([]chan<- *config.Config, len(s.watchers))
	copy(watchers, s.watchers)
	s.mu.Unlock()

	// Notify watchers
	for _, watcher := range watchers {
		if !safeSendConfig(watcher, cfg) {
			s.logger.Debug("Watcher channel full, skipping notification")
		}
		// Note: Update doesn't check context cancellation as it's a synchronous operation
	}

	return nil
}

// Replace swaps the in-memory config without validation or watcher
// notification. Use only when callers manage consistency themselves
// (e.g. --no-reload batches multiple changes before a final reload).
//
// written is the configuration cfg was derived from, kept for the next field
// change, and not optional for the same reason as in Update. A batch of
// --no-reload changes is exactly where it matters: each one derives from the
// last one's written half, not from its resolved output.
func (s *Service) Replace(cfg, written *config.Config) {
	s.mu.Lock()
	s.config = cfg
	s.written = written
	s.mu.Unlock()
}

// SaveOverrideField persists a single config field change to the override file.
// It reads any existing overrides, applies the new field, and writes the result.
// Returns nil if there is no config path (daemon was started without a file).
func (s *Service) SaveOverrideField(key, value string) error {
	s.mu.RLock()
	configPath := s.path
	s.mu.RUnlock()

	if configPath == "" {
		return nil
	}

	overridePath := OverridePath(configPath)

	// Read existing overrides
	overrides := make(map[string]any)

	_, decodeErr := toml.DecodeFile(overridePath, &overrides)
	if decodeErr != nil && !os.IsNotExist(decodeErr) {
		return derrors.Wrap(
			decodeErr,
			derrors.CodeConfigIOFailed,
			"failed to read existing overrides",
		)
	}

	// Parse the value to the correct type by setting it on a throwaway Config
	// and reading it back via reflection.
	scratch := &config.Config{}

	setErr := SetField(scratch, key, value)
	if setErr != nil {
		return setErr
	}

	typedVal, valErr := getFieldValue(scratch, key)
	if valErr != nil {
		return valErr
	}

	setNestedMapValue(overrides, key, typedVal)

	saveErr := SaveOverride(overridePath, overrides)
	if saveErr != nil {
		return saveErr
	}

	// The key names a schema field, which is not the user's content; the value
	// is, and a value can be an exec command line, so only its length is logged.
	s.logger.Info("Config override persisted",
		zap.String("key", key),
		zap.Int("value_length", len(value)),
		zap.String("override_path", overridePath))

	return nil
}

// RemoveOverrideField removes a single config field from the override file.
// After the next reload, the field will revert to its value from the base
// config file (or the code default).
func (s *Service) RemoveOverrideField(key string) error {
	s.mu.RLock()
	configPath := s.path
	s.mu.RUnlock()

	if configPath == "" {
		return nil
	}

	overridePath := OverridePath(configPath)

	overrides := make(map[string]any)

	_, decodeErr := toml.DecodeFile(overridePath, &overrides)
	if decodeErr != nil && !os.IsNotExist(decodeErr) {
		return derrors.Wrap(
			decodeErr,
			derrors.CodeConfigIOFailed,
			"failed to read existing overrides",
		)
	}

	deleteNestedMapValue(overrides, key)

	saveErr := SaveOverride(overridePath, overrides)
	if saveErr != nil {
		return saveErr
	}

	s.logger.Info("Config override removed",
		zap.String("key", key),
		zap.String("override_path", overridePath))

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
