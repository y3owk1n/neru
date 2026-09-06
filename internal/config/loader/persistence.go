package loader

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// OverridePath returns the path to the override file derived from the given
// config path. For "config.toml" the override is "config.override.toml" in the
// same directory; for "my-neru.toml" it is "my-neru.override.toml", etc.
func OverridePath(path string) string {
	if path == "" {
		return ""
	}

	return strings.TrimSuffix(path, filepath.Ext(path)) + config.OverrideSuffix
}

// writeStringOrStringArrayMap writes a map[string]StringOrStringArray (or
// map[string][]string) as a TOML table to the given file.  Single-action
// entries are emitted as plain strings for backward compatibility; multi-action
// entries use TOML array syntax.  The section header (e.g. "[scroll.hotkeys]")
// is always written so that an empty map round-trips correctly.
//
// When defaults is non-nil, any default key not present in _map (after
// normalization) is emitted as "__disabled__" so that Save+LoadWithValidation
// round-trips correctly under merge-on-top-of-defaults semantics.
func writeStringOrStringArrayMap(
	file *os.File,
	sectionHeader string,
	_map map[string]config.StringOrStringArray,
	defaults map[string]config.StringOrStringArray,
) error {
	_, err := fmt.Fprintf(file, "\n[%s]\n", sectionHeader)
	if err != nil {
		return derrors.Wrap(
			err, derrors.CodeConfigIOFailed, "failed to write section header",
		)
	}

	if len(_map) == 0 {
		return nil
	}

	keys := make([]string, 0, len(_map))
	for k := range _map {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, key := range keys {
		actions := _map[key]

		if len(actions) == 0 {
			continue
		}

		var line string
		if len(actions) == 1 {
			line = fmt.Sprintf("%q = %q", key, actions[0])
		} else {
			quoted := make([]string, 0, len(actions))
			for _, a := range actions {
				quoted = append(quoted, fmt.Sprintf("%q", a))
			}

			line = fmt.Sprintf("%q = [%s]", key, strings.Join(quoted, ", "))
		}

		_, err := fmt.Fprintln(file, line)
		if err != nil {
			return derrors.Wrap(
				err, derrors.CodeConfigIOFailed, "failed to write binding",
			)
		}
	}

	// Emit __disabled__ markers for default bindings that were removed.
	if defaults != nil {
		disabledKeys := make([]string, 0)
		for defaultKey := range defaults {
			found := config.FindNormalizedMapKey(_map, defaultKey)
			if _, exists := _map[found]; !exists {
				disabledKeys = append(disabledKeys, defaultKey)
			}
		}

		sort.Strings(disabledKeys)

		for _, key := range disabledKeys {
			line := fmt.Sprintf("%q = %q", key, config.DisabledSentinel)

			_, disabledErr := fmt.Fprintln(file, line)
			if disabledErr != nil {
				return derrors.Wrap(
					disabledErr,
					derrors.CodeConfigIOFailed,
					"failed to write disabled binding marker",
				)
			}
		}
	}

	return nil
}

// Save writes the configuration to the specified path.
func Save(cfg *config.Config, path string) error {
	dir := filepath.Dir(path)

	mkdirErr := os.MkdirAll(dir, config.DefaultDirPerms)
	if mkdirErr != nil {
		return derrors.Wrap(
			mkdirErr,
			derrors.CodeConfigIOFailed,
			"failed to create config directory",
		)
	}

	var closeErr error
	// #nosec G304 -- Path is validated and controlled by the application
	file, fileErr := os.Create(path)
	if fileErr != nil {
		return derrors.Wrap(fileErr, derrors.CodeConfigIOFailed, "failed to create config file")
	}

	defer func() {
		cerr := file.Close()
		if cerr != nil && closeErr == nil {
			closeErr = derrors.Wrap(cerr, derrors.CodeConfigIOFailed, "failed to close config file")
		}
	}()

	// Encode the main config struct to TOML.
	// The Hotkeys field is tagged toml:"-" so the encoder skips it entirely;
	// we append the flat [hotkeys] section manually afterwards.
	encoder := toml.NewEncoder(file)

	encodeErr := encoder.Encode(cfg)
	if encodeErr != nil {
		return derrors.Wrap(encodeErr, derrors.CodeSerializationFailed, "failed to encode config")
	}

	// Write the [hotkeys] section so that LoadWithValidation sees
	// raw["hotkeys"] and merges user entries on top of defaults.  An empty
	// section (no keys) is the documented way to disable all hotkeys.
	//
	// Convert map[string][]string → map[string]StringOrStringArray so we can
	// reuse writeStringOrStringArrayMap (StringOrStringArray is []string).
	defaults := config.DefaultConfig()

	hotkeysSOSA := make(map[string]config.StringOrStringArray, len(cfg.Hotkeys.Bindings))
	for k, v := range cfg.Hotkeys.Bindings {
		hotkeysSOSA[k] = config.StringOrStringArray(v)
	}

	defaultHotkeysSOSA := make(
		map[string]config.StringOrStringArray,
		len(defaults.Hotkeys.Bindings),
	)
	for k, v := range defaults.Hotkeys.Bindings {
		defaultHotkeysSOSA[k] = config.StringOrStringArray(v)
	}

	err := writeStringOrStringArrayMap(file, "hotkeys", hotkeysSOSA, defaultHotkeysSOSA)
	if err != nil {
		return err
	}

	// Write per-mode [<mode>.hotkeys] sections.
	// These fields are tagged toml:"-" so the encoder skips them; we write
	// them manually to preserve the single-string format for single-action
	// entries (backward compatibility).
	hotkeysSections := []struct {
		header   string
		hotkeys  map[string]config.StringOrStringArray
		defaults map[string]config.StringOrStringArray
	}{
		{"scroll.hotkeys", cfg.Scroll.Hotkeys, defaults.Scroll.Hotkeys},
		{"hints.hotkeys", cfg.Hints.Hotkeys, defaults.Hints.Hotkeys},
		{"grid.hotkeys", cfg.Grid.Hotkeys, defaults.Grid.Hotkeys},
		{
			"recursive_grid.hotkeys",
			cfg.RecursiveGrid.Hotkeys,
			defaults.RecursiveGrid.Hotkeys,
		},
		{
			"monitor_select.hotkeys",
			cfg.MonitorSelect.Hotkeys,
			defaults.MonitorSelect.Hotkeys,
		},
	}
	for _, section := range hotkeysSections {
		err = writeStringOrStringArrayMap(file, section.header, section.hotkeys, section.defaults)
		if err != nil {
			return err
		}
	}

	// A declared mode's table is written the same way, after the encoder has
	// written the declaration it belongs under. Sorted, so two saves of the
	// same configuration produce the same file.
	for _, name := range slices.Sorted(maps.Keys(cfg.Modes)) {
		err = writeStringOrStringArrayMap(
			file,
			modesKey+"."+name+".hotkeys",
			cfg.Modes[name].Hotkeys,
			config.DefaultCustomModeHotkeys(),
		)
		if err != nil {
			return err
		}
	}

	return closeErr
}

// SaveOverride persists a nested map of overrides to a TOML file at the given
// path. The map should mirror the TOML section structure (e.g. {"hints": {"hint_characters": "abc"}}).
func SaveOverride(path string, overrides map[string]any) error {
	if len(overrides) == 0 {
		_, statErr := os.Stat(path)
		if statErr == nil {
			return os.Remove(path)
		}

		return nil
	}

	dir := filepath.Dir(path)

	mkdirErr := os.MkdirAll(dir, config.DefaultDirPerms)
	if mkdirErr != nil {
		return derrors.Wrap(
			mkdirErr,
			derrors.CodeConfigIOFailed,
			"failed to create override directory",
		)
	}

	var closeErr error

	file, fileErr := os.Create(path)
	if fileErr != nil {
		return derrors.Wrap(fileErr, derrors.CodeConfigIOFailed, "failed to create override file")
	}
	defer func() {
		cerr := file.Close()
		if cerr != nil && closeErr == nil {
			closeErr = derrors.Wrap(
				cerr,
				derrors.CodeConfigIOFailed,
				"failed to close override file",
			)
		}
	}()

	_, headerErr := fmt.Fprintln(file, "# Neru runtime config overrides")
	if headerErr != nil {
		return derrors.Wrap(
			headerErr,
			derrors.CodeConfigIOFailed,
			"failed to write override header",
		)
	}

	_, commentErr := fmt.Fprintln(
		file,
		"# Generated by `neru config set`. Edit or remove this file to revert changes.",
	)
	if commentErr != nil {
		return derrors.Wrap(
			commentErr,
			derrors.CodeConfigIOFailed,
			"failed to write override comment",
		)
	}

	_, blankErr := fmt.Fprintln(file)
	if blankErr != nil {
		return derrors.Wrap(
			blankErr,
			derrors.CodeConfigIOFailed,
			"failed to write override blank line",
		)
	}

	encodeErr := toml.NewEncoder(file).Encode(overrides)
	if encodeErr != nil {
		return derrors.Wrap(
			encodeErr,
			derrors.CodeSerializationFailed,
			"failed to encode overrides",
		)
	}

	return closeErr
}

// setNestedMapValue sets a value at a dotted path in a nested map.
func setNestedMapValue(data map[string]any, path string, value any) {
	parts := strings.Split(path, ".")

	current := data
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value

			return
		}

		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}

		current = next
	}
}

// deleteNestedMapValue removes a dotted-path key from a nested map. Empty
// ancestor maps are pruned so the serialized override file stays clean.
func deleteNestedMapValue(data map[string]any, path string) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 || parts[0] == "" {
		return
	}

	// Walk down, collecting each level so we can prune on the way back up.
	stack := make([]map[string]any, 0, len(parts))
	stack = append(stack, data)

	current := data
	for i := range len(parts) - 1 {
		next, ok := current[parts[i]].(map[string]any)
		if !ok {
			return
		}

		stack = append(stack, next)
		current = next
	}

	// Delete the leaf key.
	delete(current, parts[len(parts)-1])

	// Prune empty ancestors from leaf upward.
	for i := len(stack) - 1; i > 0; i-- {
		if len(stack[i]) > 0 {
			break
		}

		delete(stack[i-1], parts[i-1])
	}
}

// getFieldValue reads a typed value from a Config by dotted path.
func getFieldValue(cfg *config.Config, path string) (any, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 || parts[0] == "" {
		return nil, derrors.New(derrors.CodeInvalidConfig, "config path cannot be empty")
	}

	target := reflect.ValueOf(cfg).Elem()
	for partIdx, part := range parts {
		field := findFieldByTomlTag(target, part)
		if !field.IsValid() {
			return nil, derrors.Newf(
				derrors.CodeInvalidConfig,
				"unknown config field: %s (path element %q not found)",
				path,
				part,
			)
		}

		if partIdx == len(parts)-1 {
			return field.Interface(), nil
		}

		target = derefStruct(field)
		if target.Kind() != reflect.Struct {
			return nil, derrors.Newf(
				derrors.CodeInvalidConfig,
				"%q in path %q is not a struct",
				part,
				path,
			)
		}
	}

	return nil, derrors.Newf(derrors.CodeInvalidConfig, "path %q resolved to nil", path)
}
