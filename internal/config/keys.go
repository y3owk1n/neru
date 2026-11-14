package config

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

// StringOrStringArray is a type that can unmarshal from either a TOML string
// or a TOML array of strings. Used for backward compatibility.
type StringOrStringArray []string

// UnmarshalTOML implements custom unmarshaling for TOML compatibility.
// It accepts both single string values and arrays of strings.
func (s *StringOrStringArray) UnmarshalTOML(value any) error {
	switch val := value.(type) {
	case string:
		*s = []string{val}

	case []any:
		*s = make([]string, 0, len(val))
		for _, a := range val {
			actionStr, ok := a.(string)
			if !ok {
				return derrors.Newf(derrors.CodeInvalidConfig, "expected string, got %T", a)
			}

			*s = append(*s, actionStr)
		}

	case []string:
		*s = val

	default:
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"cannot unmarshal %T into StringOrStringArray",
			value,
		)
	}

	return nil
}

// IsValidNamedKey checks whether a key name is a recognized named key
// (case-insensitive). The set it asks is declared in internal/domain/keyvocab.
func IsValidNamedKey(key string) bool {
	return keyvocab.IsNamedKey(key)
}

// CanonicalNamedKeyForm returns the canonical display form of a named key
// (e.g. "pagedown" becomes "PageDown", "UP" becomes "Up", "f1" becomes "F1").
// If the key is not a recognized named key, it returns the input unchanged
// and false as the second return value.
func CanonicalNamedKeyForm(key string) (string, bool) {
	return keyvocab.NamedKeyDisplay(key)
}

// NormalizeKey is keyvocab.NormalizeKeyForComparison with Primary resolved by
// this table rather than the published one, for comparing the keys of a
// configuration while it is being loaded or validated.
func (p PrimaryModifierConfig) NormalizeKey(key string) string {
	return keyvocab.NormalizeKeyWithPrimary(key, p.token(runtime.GOOS))
}

// UsePrimaryModifier makes table what Primary means in this process, for
// hotkeys and action modifiers alike. The daemon calls it whenever it adopts a
// configuration. The value lives in internal/domain/action, the lowest layer
// that reads it, so there is one copy.
func UsePrimaryModifier(table PrimaryModifierConfig) {
	switch table.token(runtime.GOOS) {
	case keyvocab.ModifierCmd:
		action.UsePrimaryModifier(action.ModCmd)
	case keyvocab.ModifierAlt:
		action.UsePrimaryModifier(action.ModAlt)
	default:
		action.UsePrimaryModifier(action.ModCtrl)
	}
}

// ForOS returns the modifier written for goos.
func (p PrimaryModifierConfig) ForOS(goos string) string {
	return PerPlatform[string](p).ForOS(goos)
}

// PlatformValueType is the type of one platform's modifier name.
func (p PrimaryModifierConfig) PlatformValueType() reflect.Type {
	return PerPlatform[string](p).PlatformValueType()
}

// UnmarshalTOML reads the table as PerPlatform does.
func (p *PrimaryModifierConfig) UnmarshalTOML(data any) error {
	return (*PerPlatform[string])(p).UnmarshalTOML(data)
}

// token is the comparison token of the modifier written for goos. An empty
// entry, which validation refuses in a file, reads as the default, so a Config
// built in code without the table resolves Primary as it always has.
func (p PrimaryModifierConfig) token(goos string) string {
	name := p.ForOS(goos)
	if strings.TrimSpace(name) == "" {
		name = defaultPrimaryModifier().ForOS(goos)
	}

	return keyvocab.ComparisonModifier(name, "")
}
