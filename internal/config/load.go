package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
)

// appConfigsKey is the name of the per-application override table. It appears
// inside [hotkeys] as a nested table rather than as a binding, so the hotkey
// merge skips it.
const appConfigsKey = "app_configs"

// LoadWithValidation turns a config file into the Config the daemon runs on,
// reporting a rejection instead of returning an error so the caller can keep
// running on the defaults while telling the user what was wrong with their file.
//
// The phases run in order and any one of them can end the load: locate the file,
// decode it, merge the global hotkeys, merge the per-mode hotkeys, check the
// hotkeys nested under [[app_configs]], validate the whole config, then layer
// the override file on top.
func (s *Service) LoadWithValidation(path string) *LoadResult {
	result := &LoadResult{
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

	result.Config.ResolveThemeDefaults()

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

	validateErr := result.Config.Validate()
	if validateErr != nil {
		wrapped := derrors.WrapConfigFailed(validateErr, "validate configuration")

		s.logger.Warn("Configuration validation failed", zap.Error(wrapped))

		return refuse(result, wrapped)
	}

	overrideErr := s.applyOverrideFile(result.Config, result.ConfigPath)
	if overrideErr != nil {
		return refuse(result, overrideErr)
	}

	s.logger.Info("Configuration loaded successfully")

	removeLauncherBindingsForDisabledModes(result.Config)

	return result
}

// refuse hands back the default config along with the reason the file was
// rejected. A half-applied config would leave the user running on bindings they
// never wrote, so a file the load cannot finish is discarded whole.
func refuse(result *LoadResult, err error) *LoadResult {
	result.ValidationError = err
	result.Config = DefaultConfig()

	return result
}

// baseConfig is what a load starts from: the platform defaults, with any hotkey
// bindings injected through WithDefaults laid over them.
//
// It is built fresh for every load rather than shared, because Hotkeys.Bindings
// is tagged toml:"-" and so survives the decode untouched — a shared base would
// accumulate the bindings of every load before it.
func (s *Service) baseConfig() *Config {
	base := newDefaultConfig()
	applyPlatformDefaults(base)

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

// locateConfigFile settles which file the load reads, and reports whether the
// load is already over — either because there is no file to read or because a
// file named explicitly is not there.
//
// A file the user named and a file that was merely discovered are treated
// differently when missing: the first is an error worth showing, the second just
// means the daemon runs on the defaults.
func (s *Service) locateConfigFile(result *LoadResult, path string) bool {
	explicit := path != ""
	if !explicit {
		result.ConfigPath = s.FindConfigFile()
	}

	if result.ConfigPath == "" {
		s.logger.Info("No config file specified or found, using default configuration")

		result.Config = DefaultConfig()

		return true
	}

	s.logger.Info("Loading config from", zap.String("path", result.ConfigPath))

	_, statErr := os.Stat(result.ConfigPath)
	if !os.IsNotExist(statErr) {
		return false
	}

	result.Config = DefaultConfig()

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

// decodeConfigFile reads the file twice: once into a raw map and once into the
// typed Config.
//
// The hotkey tables need the raw map. Their merge rules — the disable sentinel,
// folding a user's casing onto the matching default key — are not expressible as
// struct tags, and the fields carry toml:"-" so the typed pass skips them. The
// typed pass is what validates everything else. The TOML library cannot mix
// struct and map decoding in a single pass.
func (s *Service) decodeConfigFile(cfg *Config, path string) (map[string]any, error) {
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
// An entry set to the disable sentinel removes the default it matches. An empty
// [hotkeys] section removes every binding, which is how an external hotkey
// daemon such as skhd takes over the shortcuts without conflicting; the modes
// stay reachable through the CLI.
func (s *Service) applyGlobalHotkeys(cfg *Config, raw map[string]any) error {
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
// parse is left out; the merge that follows reports it, with the key and the
// type that was found.
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
// user has bound to a key of their own.
//
// Without this, binding a mode to a new chord would add a second way to reach it
// rather than move it, and the mode would still answer to the default the user
// meant to replace.
func replaceReboundLaunchers(bindings map[string][]string, parsed map[string][]string) {
	rebound := make(map[string]struct{})

	for _, actions := range parsed {
		if len(actions) == 1 && actions[0] == DisabledSentinel {
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

		// The default key that normalizes to the same chord, so that
		// "Primary+Shift+g" replaces "Primary+Shift+G" instead of sitting
		// beside it and leaving two bindings on one chord.
		canonicalKey := findNormalizedMapKey(bindings, key)

		actions, parsedOk := parsed[key]
		if !parsedOk {
			return s.rejectHotkey(key, value, "must be a string or array of strings")
		}

		if len(actions) == 0 {
			return s.rejectHotkey(key, value, "must not be empty")
		}

		if len(actions) == 1 && actions[0] == DisabledSentinel {
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

// rejectHotkey builds the refusal for a malformed [hotkeys] entry and logs the
// type that was found, which is the part the message cannot carry.
func (s *Service) rejectHotkey(key string, value any, reason string) error {
	err := derrors.New(derrors.CodeInvalidConfig, "hotkeys."+key+" "+reason)

	s.logger.Warn("Invalid hotkey configuration",
		zap.String("key", key),
		zap.String("value_type", fmt.Sprintf("%T", value)),
		zap.Error(err))

	return err
}

// modeHotkeyTarget pairs a mode's name in the config file with the bindings map
// its [<mode>.hotkeys] table merges into.
type modeHotkeyTarget struct {
	modeKey string
	dest    *map[string]StringOrStringArray
}

// modeHotkeyTargets lists every mode that has its own hotkey table.
func modeHotkeyTargets(cfg *Config) []modeHotkeyTarget {
	return []modeHotkeyTarget{
		{ModeNameScroll, &cfg.Scroll.Hotkeys},
		{ModeNameHints, &cfg.Hints.Hotkeys},
		{ModeNameGrid, &cfg.Grid.Hotkeys},
		{ModeNameRecursiveGrid, &cfg.RecursiveGrid.Hotkeys},
		{ModeNameMonitorSelect, &cfg.MonitorSelect.Hotkeys},
	}
}

// applyModeHotkeys merges each [<mode>.hotkeys] table over that mode's default
// bindings, following the same rules as the global table: the disable sentinel
// removes a default, and an empty section clears the mode's bindings entirely.
//
// These fields are tagged toml:"-" so that the encoder does not turn a
// single-action entry into an array, which means the typed decode skips them and
// they have to be read from the raw map here.
func (s *Service) applyModeHotkeys(cfg *Config, raw map[string]any) error {
	for _, target := range modeHotkeyTargets(cfg) {
		table, present := modeHotkeyTable(raw, target.modeKey)
		if !present {
			continue
		}

		if len(table) == 0 {
			*target.dest = make(map[string]StringOrStringArray)

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

// modeHotkeyTable reaches [<mode>.hotkeys] in the raw decode. Anything that is
// not a table at either level means the mode has no hotkey section to merge; the
// typed decode is what reports a section of the wrong shape.
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
		var actions StringOrStringArray

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

		// The default key that normalizes to the same key, so that "escape"
		// replaces "Escape" instead of sitting beside it.
		canonicalKey := findNormalizedMapKey(*target.dest, key)

		if len(actions) == 1 && actions[0] == DisabledSentinel {
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

// validateNestedHotkeys checks the hotkey tables carried by [[app_configs]],
// both the top-level ones and the per-mode ones.
//
// These are checked rather than merged: the typed decode has already loaded
// them, and what is left to catch is two entries that normalize to the same
// chord, where only one of them would ever fire.
func (s *Service) validateNestedHotkeys(raw map[string]any) *LoadResult {
	if result := validateAppConfigsHotkeys(s.logger, appConfigsKey, raw); result != nil {
		return result
	}

	for _, modeKey := range []string{
		ModeNameHints,
		ModeNameGrid,
		ModeNameRecursiveGrid,
		ModeNameScroll,
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

// applyOverrideFile layers the file written by `neru config set` on top of what
// the config file produced, so a setting changed at runtime outlives a restart
// and wins over both the config file and the defaults.
//
// The result is validated again, because a field that is valid on its own can
// still contradict one the config file set.
func (s *Service) applyOverrideFile(cfg *Config, configPath string) error {
	overridePath := OverridePath(configPath)
	if overridePath == "" {
		return nil
	}

	// The override file is optional — it exists only once `neru config set` has
	// written one — so anything that is not a readable file simply means there
	// is nothing to layer.
	overrideStat, statErr := os.Stat(overridePath)

	layerable := statErr == nil && !overrideStat.IsDir()
	if !layerable {
		return nil
	}

	s.logger.Info("Loading config overrides from", zap.String("path", overridePath))

	_, decodeErr := toml.DecodeFile(overridePath, cfg)
	if decodeErr != nil {
		wrapped := derrors.WrapConfigFailed(decodeErr, "parse config override file")

		s.logger.Warn("Config override file parse failed",
			zap.String("path", overridePath),
			zap.Error(wrapped))

		return wrapped
	}

	cfg.ResolveThemeDefaults()

	validateErr := cfg.Validate()
	if validateErr != nil {
		wrapped := derrors.WrapConfigFailed(validateErr, "validate configuration with overrides")

		s.logger.Warn("Configuration with overrides validation failed", zap.Error(wrapped))

		return wrapped
	}

	return nil
}
