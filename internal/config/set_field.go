package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

// SetField sets a field on a Config by dotted path (e.g. "hints.hint_characters").
// Path elements use TOML tag names. Returns an error if the path is unknown or
// the value cannot be converted.
func SetField(cfg *Config, path, value string) error {
	parts := strings.Split(path, ".")
	if len(parts) == 0 || parts[0] == "" {
		return derrors.New(derrors.CodeInvalidConfig, "config path cannot be empty")
	}

	target := reflect.ValueOf(cfg).Elem()
	for partIdx, part := range parts {
		field := findFieldByTomlTag(target, part)
		if !field.IsValid() {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"unknown config field: %s (path element %q not found)",
				path, part,
			)
		}

		if partIdx == len(parts)-1 {
			return setLeafValue(field, value)
		}

		target = derefStruct(field)
		if target.Kind() != reflect.Struct {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%q in path %q is not a struct",
				part, path,
			)
		}
	}

	return nil
}

// DeepCopyConfig returns a deep copy of cfg via JSON round-trip.
func DeepCopyConfig(cfg *Config) (*Config, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeSerializationFailed, "deep copy config")
	}

	var dst Config

	unmarshalErr := json.Unmarshal(data, &dst)
	if unmarshalErr != nil {
		return nil, derrors.Wrap(unmarshalErr, derrors.CodeSerializationFailed, "deep copy config")
	}

	return &dst, nil
}

func findFieldByTomlTag(target reflect.Value, tagName string) reflect.Value {
	typ := target.Type()
	for fieldIdx := range typ.NumField() {
		field := typ.Field(fieldIdx)

		tag := field.Tag.Get("toml")
		if tag == "" {
			tag = field.Name
		}

		if idx := strings.Index(tag, ","); idx >= 0 {
			tag = tag[:idx]
		}

		if tag == tagName {
			return target.Field(fieldIdx)
		}
	}

	return reflect.Value{}
}

func derefStruct(field reflect.Value) reflect.Value {
	for field.Kind() == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}

		field = field.Elem()
	}

	return field
}

// many variants (Chan, Map, Func, etc.) that are never valid config field types.
//
//nolint:exhaustive // Only config-relevant types are handled; reflect.Kind has
func setLeafValue(field reflect.Value, value string) error {
	field = derefStruct(field)

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return derrors.Newf(derrors.CodeInvalidConfig, "cannot parse %q as integer", value)
		}

		field.SetInt(n)

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return derrors.Newf(derrors.CodeInvalidConfig, "cannot parse %q as float", value)
		}

		field.SetFloat(f)

	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return derrors.Newf(derrors.CodeInvalidConfig, "cannot parse %q as boolean", value)
		}

		field.SetBool(b)

	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			items := parseStringSlice(value)

			sl := reflect.MakeSlice(field.Type(), len(items), len(items))
			for i, s := range items {
				sl.Index(i).SetString(s)
			}

			field.Set(sl)
		} else {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"unsupported slice element type for %s",
				field.Type(),
			)
		}

	case reflect.Struct:
		if field.Type().Name() == "Color" {
			c := parseColorValue(value)

			field.Set(reflect.ValueOf(c))
		} else {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"cannot set struct field %q directly; use a dotted path to a leaf field or provide a JSON object",
				field.Type().Name(),
			)
		}

	default:
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"unsupported config field type: %s",
			field.Type(),
		)
	}

	return nil
}

func parseStringSlice(value string) []string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		inner := strings.TrimSpace(value[1 : len(value)-1])
		if inner == "" {
			return nil
		}

		var out []string
		for item := range strings.SplitSeq(inner, ",") {
			item = strings.TrimSpace(item)

			item = strings.Trim(item, `"'`)
			if item != "" {
				out = append(out, item)
			}
		}

		return out
	}

	if value == "" {
		return nil
	}

	var out []string
	for item := range strings.SplitSeq(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}

	return out
}

func parseColorValue(value string) Color {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		var m map[string]string

		err := json.Unmarshal([]byte(value), &m)
		if err == nil {
			return Color{Light: m["light"], Dark: m["dark"]}
		}
	}

	return Color{Light: value, Dark: value}
}

// ValidateConfigSetField validates a config path is settable and value is valid
// by performing the mutation on a throwaway copy. Used by the CLI to validate
// before sending to the daemon.
func ValidateConfigSetField(path, value string) error {
	cfg := DefaultConfig()

	err := SetField(cfg, path, value)
	if err != nil {
		return err
	}

	err = cfg.Validate()
	if err != nil {
		return err
	}

	return nil
}

// unknownField is returned by ConfigFieldType when the path cannot be resolved.
const unknownField = "unknown"

// ConfigFieldType returns a human-readable type hint for a config path.
//
//nolint:revive,exhaustive
func ConfigFieldType(path string) string {
	cfg := DefaultConfig()
	target := reflect.ValueOf(cfg).Elem()

	parts := strings.Split(path, ".")
	for partIdx, part := range parts {
		field := findFieldByTomlTag(target, part)
		if !field.IsValid() {
			return unknownField
		}

		// Intermediate path elements must resolve to structs so the next
		// iteration can call findFieldByTomlTag without panicking on
		// NumField of a non-struct type.
		if partIdx < len(parts)-1 {
			target = derefStruct(field)
			if target.Kind() != reflect.Struct {
				return unknownField
			}

			continue
		}

		target = field
	}

	if !target.IsValid() {
		return unknownField
	}

	target = derefStruct(target)
	switch target.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "float"
	case reflect.Bool:
		return "boolean"
	case reflect.Slice:
		return "array"
	case reflect.Struct:
		if target.Type().Name() == "Color" {
			return "color (#RGB/#RRGGBB/#AARRGGBB or JSON object)"
		}

		return "object"
	default:
		return fmt.Sprintf("unknown (%s)", target.Kind())
	}
}
