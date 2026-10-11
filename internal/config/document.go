package config

import (
	"reflect"
	"strings"
)

// hotkeysKey names every hotkey table.
const hotkeysKey = "hotkeys"

var (
	colorType      = reflect.TypeFor[Color]()
	hotkeysCfgType = reflect.TypeFor[HotkeysConfig]()
)

// TOMLKey reports how a field of a config struct appears in config.toml: its
// key, whether it is a hotkey table the loader reads from the raw map rather
// than through the field's own tag, and false when the field is not a key.
// The published schema and the dump both use it, so the two cannot disagree
// about what a config file contains.
func TOMLKey(field reflect.StructField) (string, bool, bool) {
	name, _, _ := strings.Cut(field.Tag.Get("toml"), ",")

	switch name {
	case "":
		return "", false, false
	case "-":
		jsonName, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if jsonName != hotkeysKey {
			return "", false, false
		}

		return hotkeysKey, true, true
	default:
		return name, false, true
	}
}

// Document returns cfg as config.toml writes it: keyed by the file's names, a
// color as one string when both themes share it, and a hotkey table as the
// file's table of bindings. It leaves out an unset optional value, as a file
// would. `neru config dump` prints it.
func Document(cfg *Config) map[string]any {
	return documentTable(reflect.ValueOf(cfg).Elem())
}

// documentValue returns value as the file writes it, and false for an unset
// optional value, which the file has no way to write.
func documentValue(value reflect.Value) (any, bool) {
	if value.Type() == colorType {
		color, _ := reflect.TypeAssert[Color](value)
		if color.IsSameForBothThemes() {
			return color.Light, true
		}

		return map[string]any{"light": color.Light, "dark": color.Dark}, true
	}

	switch value.Kind() { //nolint:exhaustive // scalars are written as they are
	case reflect.Pointer:
		if value.IsNil() {
			return nil, false
		}

		return documentValue(value.Elem())
	case reflect.Struct:
		return documentTable(value), true
	case reflect.Map:
		table := make(map[string]any, value.Len())

		for _, key := range value.MapKeys() {
			if entry, set := documentValue(value.MapIndex(key)); set {
				table[key.String()] = entry
			}
		}

		return table, true
	case reflect.Slice:
		items := make([]any, 0, value.Len())

		for index := range value.Len() {
			if item, set := documentValue(value.Index(index)); set {
				items = append(items, item)
			}
		}

		return items, true
	default:
		return value.Interface(), true
	}
}

// documentTable returns a config struct as its table: one entry per key.
func documentTable(value reflect.Value) map[string]any {
	table := map[string]any{}

	for index := range value.NumField() {
		name, hotkeyTable, isKey := TOMLKey(value.Type().Field(index))
		if !isKey {
			continue
		}

		fieldValue := value.Field(index)

		// The global table keeps its bindings one level down, on HotkeysConfig.
		if hotkeyTable && fieldValue.Type() == hotkeysCfgType {
			fieldValue = fieldValue.FieldByName("Bindings")
		}

		if entry, set := documentValue(fieldValue); set {
			table[name] = entry
		}
	}

	return table
}
