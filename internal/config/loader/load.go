package loader

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// appConfigsKey names the per-app override table, which appears inside
// [hotkeys] as a nested table rather than a binding, so the merge skips it.
const appConfigsKey = "app_configs"

// LoadWithValidation turns a config file into the Config the daemon runs on.
// It reports a rejection in the result rather than returning an error, so the
// caller can keep running on the defaults and still tell the user what was
// wrong. Any phase below can end the load.
func (s *Service) LoadWithValidation(path string) *config.LoadResult {
	result := &config.LoadResult{
		Config:     s.baseConfig(),
		ConfigPath: path,
	}

	if s.locateConfigFile(result, path) {
		return result
	}

	raw, decodeErr := s.decodeConfigFile(result.Config, result.ConfigPath)
	if decodeErr != nil {
		return refuse(result, decodeErr)
	}

	globalErr := s.applyGlobalHotkeys(result.Config, raw)
	if globalErr != nil {
		return refuse(result, globalErr)
	}

	modeErr := s.applyModeHotkeys(result.Config, raw)
	if modeErr != nil {
		return refuse(result, modeErr)
	}

	if rejected := s.validateNestedHotkeys(raw); rejected != nil {
		return rejected
	}

	overridePath := overrideFileToLayer(result.ConfigPath)
	if overridePath != "" {
		overrideErr := s.applyOverrideFile(result.Config, overridePath)
		if overrideErr != nil {
			return refuse(result, overrideErr)
		}
	}

	// After the last layer, because the values a derivation reads can come from
	// either file, and exactly once, because a derived value is
	// indistinguishable from one the user wrote. What the user wrote is kept so
	// that `neru config set` can derive again rather than derive from this.
	written, deriveErr := derive(result.Config)
	if deriveErr != nil {
		return refuse(result, deriveErr)
	}

	result.Written = written

	// One reading, of the configuration the daemon will actually run: an
	// override sets a scalar, and a scalar can make a setting elsewhere in the
	// file inert — `neru config set held_repeat.enabled false` is enough to
	// answer for an accel_enabled the user wrote months ago — or answer a
	// warning the file on its own would have raised. Judging the layers
	// separately would leave both standing.
	// Read alongside what the user wrote, because a warning about a value the
	// derivation settled has to name a line in a file: past the derivation the
	// two are the same string, and only the snapshot above still tells them
	// apart (config.WrittenConfig).
	warnings := &config.Warnings{}

	// Judged against the files rather than the configuration they produced: a
	// word that is inert here is only worth reporting when somebody wrote it,
	// and past the merge a default nobody chose reads the same as a line
	// somebody typed. A build for a platform no declaration has a column for
	// claims nothing, so it warns about nothing.
	writtenWords := writtenConfiguration(raw, overridePath)

	if platform, known := parity.Current(); known {
		result.Inert = warnInertWords(warnings, writtenWords, platform)
	}

	// The backend's limit rides on the same list, one warning of its own: a
	// column says where an option does anything, and X11 is not a column.
	if s.backendInert != nil {
		result.Inert = append(result.Inert,
			warnBackendInert(warnings, s.backendInert(result.Config, writtenWords))...)
	}

	validateErr := result.Config.ValidateWithWarnings(warnings, config.AsWritten(written))
	if validateErr != nil {
		wrapped := derrors.WrapConfigFailed(validateErr, "validate configuration")

		s.logger.Warn("Configuration validation failed", zap.Error(wrapped))

		return refuse(result, wrapped)
	}

	result.Warnings = warnings.Messages()

	// Logged as well as reported, because a hot reload has no one to print to:
	// the CLI shows these to whoever runs `neru config validate`, and the log
	// is the only place a daemon that reloaded on its own can say them.
	for _, warning := range result.Warnings {
		s.logger.Warn("Configuration warning", zap.String("warning", warning))
	}

	s.logger.Info("Configuration loaded successfully")

	return result
}

// refuse hands back the defaults with the reason. A half-applied config would
// leave the user on bindings they never wrote, so a bad file is dropped whole.
func refuse(result *config.LoadResult, err error) *config.LoadResult {
	result.ValidationError = err
	result.Config = config.DefaultConfig()

	// DefaultConfigForDecoding is DefaultConfig with nothing derived, which is
	// what the defaults now running were written as.
	result.Written = config.DefaultConfigForDecoding()

	// The warnings were about the file that was refused, not about the defaults
	// now running, so they go with it — and so do the inert words, which were
	// found in that same file.
	result.Warnings = nil
	result.Inert = nil

	return result
}

// baseConfig is the platform defaults with any WithDefaults hotkeys over them.
//
// Built fresh each load: Hotkeys.Bindings is tagged toml:"-" so it survives the
// decode, and a shared base would accumulate every previous load's bindings.
func (s *Service) baseConfig() *config.Config {
	base := config.PlatformDefaultConfig()

	if s.defaults == nil {
		return base
	}

	for key, actions := range s.defaults.Hotkeys.Bindings {
		if base.Hotkeys.Bindings == nil {
			base.Hotkeys.Bindings = make(map[string][]string, len(s.defaults.Hotkeys.Bindings))
		}

		base.Hotkeys.Bindings[key] = actions
	}

	return base
}

// locateConfigFile settles which file to read and reports whether the load is
// already over. A missing file the user named is an error; a missing file we
// only went looking for just means the defaults.
func (s *Service) locateConfigFile(result *config.LoadResult, path string) bool {
	explicit := path != ""
	if !explicit {
		result.ConfigPath = s.FindConfigFile()
	}

	if result.ConfigPath == "" {
		s.logger.Info("No config file specified or found, using default configuration")

		result.Config = config.DefaultConfig()
		result.Written = config.DefaultConfigForDecoding()

		return true
	}

	s.logger.Info("Loading config from", zap.String("path", result.ConfigPath))

	_, statErr := os.Stat(result.ConfigPath)
	if !os.IsNotExist(statErr) {
		return false
	}

	result.Config = config.DefaultConfig()
	result.Written = config.DefaultConfigForDecoding()

	if explicit {
		result.ValidationError = derrors.WrapConfigFailed(statErr, "config file not found")

		return true
	}

	s.logger.Info("Config file not found, using default configuration")

	// A discovered path that turned out not to exist is not the path the daemon
	// is running on, so it is not reported as one.
	result.ConfigPath = ""

	return true
}

// decodeConfigFile reads the file twice, into a raw map and into the Config.
//
// The hotkey tables need the raw map: their merge rules (the disable sentinel,
// folding a user's casing onto a default key) are not expressible as struct
// tags, and the fields carry toml:"-" so the typed pass skips them. The typed
// pass validates everything else. The TOML library cannot do both at once.
func (s *Service) decodeConfigFile(cfg *config.Config, path string) (map[string]any, error) {
	var raw map[string]any

	_, rawErr := toml.DecodeFile(path, &raw)
	if rawErr != nil {
		return nil, derrors.WrapConfigFailed(rawErr, "parse config file")
	}

	_, typedErr := toml.DecodeFile(path, cfg)
	if typedErr != nil {
		return nil, derrors.WrapConfigFailed(typedErr, "parse config file")
	}

	return raw, nil
}

// applyGlobalHotkeys merges the [hotkeys] table over the default bindings.
//
// The disable sentinel removes the default an entry matches. An empty [hotkeys]
// section removes every binding, which is how skhd and friends take over the
// shortcuts; the modes stay reachable from the CLI.
func (s *Service) applyGlobalHotkeys(cfg *config.Config, raw map[string]any) error {
	hot, present := raw["hotkeys"]
	if !present {
		return nil
	}

	hotMap, isTable := hot.(map[string]any)
	if !isTable {
		err := derrors.Newf(
			derrors.CodeInvalidConfig,
			"[hotkeys] must be a TOML table, got %T",
			hot,
		)

		s.logger.Warn("Invalid hotkeys section type",
			zap.String("value_type", fmt.Sprintf("%T", hot)),
			zap.Error(err))

		return err
	}

	if len(hotMap) == 0 {
		cfg.Hotkeys.Bindings = map[string][]string{}

		return nil
	}

	duplicateErr := validateRawHotkeyTable("hotkeys", hotMap)
	if duplicateErr != nil {
		s.logger.Warn("Duplicate normalized hotkey in config", zap.Error(duplicateErr))

		return duplicateErr
	}

	parsed := parseGlobalHotkeyTable(hotMap)

	replaceReboundLaunchers(cfg.Hotkeys.Bindings, parsed)

	return s.mergeGlobalHotkeys(cfg.Hotkeys.Bindings, hotMap, parsed)
}

// parseGlobalHotkeyTable reads each entry's actions. An entry that does not
// parse is left out, and the merge below reports it.
func parseGlobalHotkeyTable(hotMap map[string]any) map[string][]string {
	parsed := make(map[string][]string, len(hotMap))

	for key, value := range hotMap {
		if key == appConfigsKey {
			continue
		}

		actions, err := parseRawHotkeyActions("hotkeys."+key, value)
		if err != nil {
			continue
		}

		parsed[key] = actions
	}

	return parsed
}

// replaceReboundLaunchers drops the default binding of any built-in mode the
// user rebound. Otherwise a new chord adds a second way in rather than moving
// it, and the old default still works.
func replaceReboundLaunchers(bindings map[string][]string, parsed map[string][]string) {
	rebound := make(map[string]struct{})

	for _, actions := range parsed {
		if len(actions) == 1 && actions[0] == config.DisabledSentinel {
			continue
		}

		if action, isBuiltIn := isBuiltInGlobalModeAction(actions); isBuiltIn {
			rebound[action] = struct{}{}
		}
	}

	for action := range rebound {
		removeBindingsForSingleAction(bindings, action)
	}
}

// mergeGlobalHotkeys lays the user's entries over the defaults.
func (s *Service) mergeGlobalHotkeys(
	bindings map[string][]string,
	hotMap map[string]any,
	parsed map[string][]string,
) error {
	for key, value := range hotMap {
		if key == appConfigsKey {
			continue
		}

		// The default that normalizes to the same chord, so "Primary+Shift+g"
		// replaces "Primary+Shift+G" rather than doubling up on it.
		canonicalKey := config.FindNormalizedMapKey(bindings, key)

		actions, parsedOk := parsed[key]
		if !parsedOk {
			return s.rejectHotkey(key, value, "must be a string or array of strings")
		}

		if len(actions) == 0 {
			return s.rejectHotkey(key, value, "must not be empty")
		}

		if len(actions) == 1 && actions[0] == config.DisabledSentinel {
			if _, exists := bindings[canonicalKey]; !exists {
				s.logger.Warn("__disabled__ used for key that is not a default binding",
					zap.String("key", key))
			}

			delete(bindings, canonicalKey)

			continue
		}

		// Remove the default's casing before inserting under the user's.
		delete(bindings, canonicalKey)

		bindings[key] = actions
	}

	return nil
}

// rejectHotkey refuses a malformed [hotkeys] entry, logging the type found.
func (s *Service) rejectHotkey(key string, value any, reason string) error {
	err := derrors.New(derrors.CodeInvalidConfig, "hotkeys."+key+" "+reason)

	s.logger.Warn("Invalid hotkey configuration",
		zap.String("key", key),
		zap.String("value_type", fmt.Sprintf("%T", value)),
		zap.Error(err))

	return err
}

// modeHotkeyTarget pairs a mode's config name with the bindings it merges into.
type modeHotkeyTarget struct {
	modeKey string
	dest    *map[string]config.StringOrStringArray
}

// modeHotkeyTargets lists every mode that has its own hotkey table.
func modeHotkeyTargets(cfg *config.Config) []modeHotkeyTarget {
	return []modeHotkeyTarget{
		{config.ModeNameScroll, &cfg.Scroll.Hotkeys},
		{config.ModeNameHints, &cfg.Hints.Hotkeys},
		{config.ModeNameGrid, &cfg.Grid.Hotkeys},
		{config.ModeNameRecursiveGrid, &cfg.RecursiveGrid.Hotkeys},
		{config.ModeNameMonitorSelect, &cfg.MonitorSelect.Hotkeys},
	}
}

// applyModeHotkeys merges each [<mode>.hotkeys] table over that mode's
// defaults, by the same rules as the global table.
//
// These fields are tagged toml:"-" so the encoder does not turn a single-action
// entry into an array, which means they must be read from the raw map here.
func (s *Service) applyModeHotkeys(cfg *config.Config, raw map[string]any) error {
	for _, target := range modeHotkeyTargets(cfg) {
		table, present := modeHotkeyTable(raw, target.modeKey)
		if !present {
			continue
		}

		if len(table) == 0 {
			*target.dest = make(map[string]config.StringOrStringArray)

			continue
		}

		duplicateErr := validateRawHotkeyTable(target.modeKey+".hotkeys", table)
		if duplicateErr != nil {
			s.logger.Warn("Duplicate normalized custom hotkey in config",
				zap.String("mode", target.modeKey),
				zap.Error(duplicateErr))

			return duplicateErr
		}

		mergeErr := s.mergeModeHotkeys(target, table)
		if mergeErr != nil {
			return mergeErr
		}
	}

	return nil
}

// modeHotkeyTable reaches [<mode>.hotkeys] in the raw decode. A non-table at
// either level means no section to merge; the typed decode reports bad shapes.
func modeHotkeyTable(raw map[string]any, modeKey string) (map[string]any, bool) {
	modeRaw, isTable := raw[modeKey].(map[string]any)
	if !isTable {
		return nil, false
	}

	table, isTable := modeRaw["hotkeys"].(map[string]any)
	if !isTable {
		return nil, false
	}

	return table, true
}

// mergeModeHotkeys lays one mode's entries over its defaults.
func (s *Service) mergeModeHotkeys(target modeHotkeyTarget, table map[string]any) error {
	for key, value := range table {
		var actions config.StringOrStringArray

		unmarshalErr := actions.UnmarshalTOML(value)
		if unmarshalErr != nil {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s.hotkeys.%s: %v",
				target.modeKey,
				key,
				unmarshalErr,
			)
		}

		// The default that normalizes the same, so "escape" replaces "Escape".
		canonicalKey := config.FindNormalizedMapKey(*target.dest, key)

		if len(actions) == 1 && actions[0] == config.DisabledSentinel {
			if _, exists := (*target.dest)[canonicalKey]; !exists {
				s.logger.Warn("__disabled__ used for key that is not a default binding",
					zap.String("mode", target.modeKey),
					zap.String("key", key))
			}

			delete(*target.dest, canonicalKey)

			continue
		}

		// Remove the default's casing before inserting under the user's.
		delete(*target.dest, canonicalKey)

		(*target.dest)[key] = actions
	}

	return nil
}

// validateNestedHotkeys checks the [[app_configs]] hotkey tables, top-level and
// per-mode. The typed decode already loaded them; what is left to catch is two
// entries normalizing to the same chord, where only one would ever fire.
func (s *Service) validateNestedHotkeys(raw map[string]any) *config.LoadResult {
	if result := validateAppConfigsHotkeys(s.logger, appConfigsKey, raw); result != nil {
		return result
	}

	for _, modeKey := range []string{
		config.ModeNameHints,
		config.ModeNameGrid,
		config.ModeNameRecursiveGrid,
		config.ModeNameScroll,
	} {
		modeRaw, isTable := raw[modeKey].(map[string]any)
		if !isTable {
			continue
		}

		if result := validateAppConfigsHotkeys(s.logger, modeKey, modeRaw); result != nil {
			return result
		}
	}

	return nil
}

// overrideFileToLayer names the file `neru config set` writes, or "" when there
// is none to layer: it exists only once a runtime change has been persisted.
func overrideFileToLayer(configPath string) string {
	overridePath := OverridePath(configPath)
	if overridePath == "" {
		return ""
	}

	overrideStat, statErr := os.Stat(overridePath)

	layerable := statErr == nil && !overrideStat.IsDir()
	if !layerable {
		return ""
	}

	return overridePath
}

// applyOverrideFile layers the file `neru config set` writes over the config,
// so a runtime change outlives a restart. It is the last layer, which is why it
// only decodes: deriving or validating here would be doing it to a
// configuration that is finally complete, and the caller does both once, there.
func (s *Service) applyOverrideFile(cfg *config.Config, overridePath string) error {
	s.logger.Info("Loading config overrides from", zap.String("path", overridePath))

	_, decodeErr := toml.DecodeFile(overridePath, cfg)
	if decodeErr != nil {
		wrapped := derrors.WrapConfigFailed(decodeErr, "parse config override file")

		s.logger.Warn("Config override file parse failed",
			zap.String("path", overridePath),
			zap.Error(wrapped))

		return wrapped
	}

	return nil
}
