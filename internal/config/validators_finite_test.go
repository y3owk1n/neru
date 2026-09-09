package config_test

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// minFloatFields guards the sweep against passing vacuously: the schema
// carries well over twenty float options today.
const minFloatFields = 20

// TestConfig_ValidateFinite_RejectsEveryNonFiniteFloat sets each float field
// in the schema to NaN, +Inf and -Inf in turn and expects validation to name
// the field. The walk is reflective so a float added tomorrow is covered on
// the day it lands, which is the whole point of one check over per-field ones.
func TestConfig_ValidateFinite_RejectsEveryNonFiniteFloat(t *testing.T) {
	paths := floatFieldPaths(t)
	if len(paths) < minFloatFields {
		t.Fatalf(
			"found %d float fields, expected at least %d; the walk is broken",
			len(paths),
			minFloatFields,
		)
	}

	for _, path := range paths {
		for name, value := range map[string]float64{
			"nan":  math.NaN(),
			"inf":  math.Inf(1),
			"-inf": math.Inf(-1),
		} {
			t.Run(path+"="+name, func(t *testing.T) {
				cfg := config.DefaultConfig()
				setFloatByPath(t, reflect.ValueOf(cfg).Elem(), path, value)

				err := cfg.Validate()
				if err == nil {
					t.Fatalf("%s = %s was accepted, want rejected", path, name)
				}

				if !strings.Contains(err.Error(), path) {
					t.Errorf("%s = %s rejected without naming the field: %v", path, name, err)
				}
			})
		}
	}
}

// TestConfig_ValidateFinite_AcceptsTheDefaults pins the other direction: the
// shipped configuration is finite everywhere.
func TestConfig_ValidateFinite_AcceptsTheDefaults(t *testing.T) {
	err := config.DefaultConfig().ValidateFinite()
	if err != nil {
		t.Fatalf("the default config was rejected as non-finite: %v", err)
	}
}

// floatFieldPaths lists the toml path of every float in the schema, through
// structs and non-nil pointers; collections ship empty and carry no floats.
func floatFieldPaths(t *testing.T) []string {
	t.Helper()

	var paths []string

	var walk func(val reflect.Value, path string)

	walk = func(val reflect.Value, path string) {
		switch val.Kind() { //nolint:exhaustive // only the kinds that can hold or contain a float matter
		case reflect.Float32, reflect.Float64:
			paths = append(paths, path)
		case reflect.Pointer:
			if !val.IsNil() {
				walk(val.Elem(), path)
			}
		case reflect.Struct:
			typ := val.Type()
			for index := range typ.NumField() {
				name, _, _ := strings.Cut(typ.Field(index).Tag.Get("toml"), ",")
				if !typ.Field(index).IsExported() || name == "" || name == "-" {
					continue
				}

				if path != "" {
					name = path + "." + name
				}

				walk(val.Field(index), name)
			}
		default:
		}
	}

	walk(reflect.ValueOf(config.DefaultConfig()).Elem(), "")

	return paths
}

func setFloatByPath(t *testing.T, val reflect.Value, path string, value float64) {
	t.Helper()

	for segment := range strings.SplitSeq(path, ".") {
		for val.Kind() == reflect.Pointer {
			val = val.Elem()
		}

		typ := val.Type()
		found := false

		for index := range typ.NumField() {
			name, _, _ := strings.Cut(typ.Field(index).Tag.Get("toml"), ",")
			if name == segment {
				val = val.Field(index)
				found = true

				break
			}
		}

		if !found {
			t.Fatalf("no field %q on %s while resolving %s", segment, typ.Name(), path)
		}
	}

	val.SetFloat(value)
}
