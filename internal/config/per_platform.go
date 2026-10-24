package config

import (
	"reflect"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// PerPlatform is an option that takes its own value on each platform, so one
// config file shared across machines says what each of them runs. Every
// platform that is neither darwin nor windows reads the linux entry.
//
// It is written as a table keyed by platform, or as one value for all three:
//
//	shell = ["/bin/zsh", "-lc"]
//
//	[general.shell]
//	darwin = ["/bin/zsh", "-lc"]
//	windows = ['C:\Windows\System32\cmd.exe', "/c"]
//
// A table leaves every platform it does not name at the value it already has.
type PerPlatform[T any] struct {
	Darwin  T `json:"darwin"  toml:"darwin"`
	Linux   T `json:"linux"   toml:"linux"`
	Windows T `json:"windows" toml:"windows"`
}

// ForOS returns the value for goos.
func (p PerPlatform[T]) ForOS(goos string) T {
	if parity.Platform(goos) == parity.Darwin {
		return p.Darwin
	}

	if parity.Platform(goos) == parity.Windows {
		return p.Windows
	}

	return p.Linux
}

// platformEntry is one platform's value, named by the key the file uses.
type platformEntry[T any] struct {
	platform parity.Platform
	value    T
}

// entries lists each platform with its value, in a fixed order, for a check
// that has to name the entry it refuses.
func (p PerPlatform[T]) entries() []platformEntry[T] {
	return []platformEntry[T]{
		{parity.Darwin, p.Darwin},
		{parity.Linux, p.Linux},
		{parity.Windows, p.Windows},
	}
}

// PlatformValueType is the type of one platform's value. The config schema
// asks it to describe the single-value form beside the table.
func (p PerPlatform[T]) PlatformValueType() reflect.Type {
	return reflect.TypeFor[T]()
}

// UnmarshalTOML reads one value for every platform, or a table that sets only
// the platforms it names. A key that names no platform is refused, because the
// decoder hands this table over whole and cannot report it as unknown.
func (p *PerPlatform[T]) UnmarshalTOML(data any) error {
	table, isTable := data.(map[string]any)
	if !isTable {
		value, err := decodePlatformValue[T](data)
		if err != nil {
			return err
		}

		p.Darwin, p.Linux, p.Windows = value, value, value

		return nil
	}

	for key, raw := range table {
		value, err := decodePlatformValue[T](raw)
		if err != nil {
			return derrors.Newf(derrors.CodeInvalidConfig, "%s: %v", key, err)
		}

		switch parity.Platform(key) {
		case parity.Darwin:
			p.Darwin = value
		case parity.Linux:
			p.Linux = value
		case parity.Windows:
			p.Windows = value
		default:
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%q is not a platform. Use darwin, linux or windows",
				key,
			)
		}
	}

	return nil
}

// decodePlatformValue converts what the TOML decoder produced into T, for the
// value types a PerPlatform option holds.
func decodePlatformValue[T any](data any) (T, error) {
	var value T

	switch target := any(&value).(type) {
	case *string:
		text, isString := data.(string)
		if !isString {
			return value, derrors.Newf(derrors.CodeInvalidConfig, "must be a string, got %T", data)
		}

		*target = text
	case *[]string:
		items, isArray := data.([]any)
		if !isArray {
			return value, derrors.Newf(
				derrors.CodeInvalidConfig,
				"must be an array of strings, got %T",
				data,
			)
		}

		*target = make([]string, 0, len(items))

		for _, item := range items {
			text, isString := item.(string)
			if !isString {
				return value, derrors.Newf(
					derrors.CodeInvalidConfig,
					"must be an array of strings, got an element of %T",
					item,
				)
			}

			*target = append(*target, text)
		}
	default:
		return value, derrors.Newf(
			derrors.CodeInternal,
			"PerPlatform cannot decode a %T",
			value,
		)
	}

	return value, nil
}
