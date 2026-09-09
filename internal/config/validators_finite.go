package config

import (
	"math"
	"reflect"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
)

// ValidateFinite rejects NaN and infinite floats anywhere in the schema. Both
// reach a field untouched: TOML spells them nan and inf, and neru config set
// parses them the same way. Every range check downstream compares, and a
// comparison with NaN is false, so a NaN would pass every one of them and
// then disable whatever heuristic the field drives. Walking the whole struct
// keeps the rule exhaustive: a float added tomorrow is covered on the day it
// lands. It runs first in the ladder because no other check is meaningful
// on a non-finite value.
func (c *Config) ValidateFinite() error {
	return finiteFloats(reflect.ValueOf(c).Elem(), "")
}

func finiteFloats(val reflect.Value, path string) error {
	switch val.Kind() { //nolint:exhaustive // only the kinds that can hold or contain a float matter
	case reflect.Float32, reflect.Float64:
		f := val.Float()
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s must be a finite number",
				path,
			)
		}
	case reflect.Pointer:
		if !val.IsNil() {
			return finiteFloats(val.Elem(), path)
		}
	case reflect.Struct:
		typ := val.Type()
		for index := range typ.NumField() {
			field := typ.Field(index)
			if !field.IsExported() {
				continue
			}

			name, _, _ := strings.Cut(field.Tag.Get("toml"), ",")
			if name == "" || name == "-" {
				// Untagged fields and the free-form hotkey tables hold no
				// floats, and nothing below a toml:"-" is a schema path.
				continue
			}

			err := finiteFloats(val.Field(index), joinPath(path, name))
			if err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		for index := range val.Len() {
			err := finiteFloats(val.Index(index), path)
			if err != nil {
				return err
			}
		}
	case reflect.Map:
		iter := val.MapRange()
		for iter.Next() {
			err := finiteFloats(iter.Value(), joinPath(path, iter.Key().String()))
			if err != nil {
				return err
			}
		}
	default:
	}

	return nil
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}

	return prefix + "." + name
}
