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

// mapFixtureKey is the key the sweep invents when a float sits under a map,
// which is the user's own vocabulary there (a custom mode name, say).
const mapFixtureKey = "sweep"

// TestConfig_ValidateFinite_RejectsEveryNonFiniteFloat sets each float field
// in the schema to NaN, +Inf and -Inf in turn and expects validation to name
// the field. The walk is over types rather than the default values, so a
// float behind a nil pointer or inside a collection that ships empty is
// reached by allocating one, and a float added tomorrow in any of those
// shapes is covered on the day it lands.
func TestConfig_ValidateFinite_RejectsEveryNonFiniteFloat(t *testing.T) {
	routes := floatFieldRoutes(t)
	if len(routes) < minFloatFields {
		t.Fatalf(
			"found %d float fields, expected at least %d; the walk is broken",
			len(routes),
			minFloatFields,
		)
	}

	for _, route := range routes {
		for name, value := range map[string]float64{
			"nan":  math.NaN(),
			"inf":  math.Inf(1),
			"-inf": math.Inf(-1),
		} {
			t.Run(route.String()+"="+name, func(t *testing.T) {
				cfg := config.DefaultConfig()
				route.set(t, reflect.ValueOf(cfg).Elem(), value)

				err := cfg.Validate()
				if err == nil {
					t.Fatalf("%s = %s was accepted, want rejected", route, name)
				}

				if !strings.Contains(err.Error(), route.String()) {
					t.Errorf("%s = %s rejected without naming the field: %v", route, name, err)
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

// floatRoute is the way from the root of the schema to one float: a field
// name per struct hop, with the collection kinds that need an element made
// on the way down.
type floatRoute []routeHop

type routeHop struct {
	// name is the toml name of the field; empty for the final hop when the
	// float sits directly in a collection rather than in a struct.
	name string
	kind reflect.Kind // the container met before the field: Pointer, Slice, Array, Map or Invalid
}

// String is the path the validator reports: toml names joined by dots, with
// the fixture key where a map sits in the way.
func (r floatRoute) String() string {
	parts := make([]string, 0, len(r))

	for _, hop := range r {
		if hop.kind == reflect.Map {
			parts = append(parts, mapFixtureKey)
		}

		if hop.name != "" {
			parts = append(parts, hop.name)
		}
	}

	return strings.Join(parts, ".")
}

// set walks the route on a live config, allocating a pointer, a one-element
// slice or a one-entry map wherever the route crosses one, then writes value.
func (r floatRoute) set(t *testing.T, val reflect.Value, value float64) {
	t.Helper()

	for hop := range r {
		val = enterContainer(val)

		if val.Kind() == reflect.Map {
			// Map elements are not addressable, so the rest of the route
			// runs on a fresh element that is stored once it is written.
			if val.IsNil() {
				val.Set(reflect.MakeMap(val.Type()))
			}

			elem := reflect.New(val.Type().Elem()).Elem()
			if r[hop].name == "" {
				enterContainer(elem).SetFloat(value)
			} else {
				r[hop:].set(t, elem, value)
			}

			val.SetMapIndex(reflect.ValueOf(mapFixtureKey), elem)

			return
		}

		if r[hop].name == "" {
			break
		}

		typ := val.Type()
		found := false

		for index := range typ.NumField() {
			name, _, _ := strings.Cut(typ.Field(index).Tag.Get("toml"), ",")
			if name == r[hop].name {
				val = val.Field(index)
				found = true

				break
			}
		}

		if !found {
			t.Fatalf("no field %q on %s while resolving %s", r[hop].name, typ.Name(), r)
		}
	}

	enterContainer(val).SetFloat(value)
}

// enterContainer returns the settable value behind pointers, slices and
// arrays, making the element it has to pass through. A map is returned as is.
func enterContainer(val reflect.Value) reflect.Value {
	for {
		switch val.Kind() { //nolint:exhaustive // only the kinds the schema nests through
		case reflect.Pointer:
			if val.IsNil() {
				val.Set(reflect.New(val.Type().Elem()))
			}

			val = val.Elem()
		case reflect.Slice:
			if val.Len() == 0 {
				val.Set(reflect.Append(val, reflect.Zero(val.Type().Elem())))
			}

			val = val.Index(0)
		case reflect.Array:
			val = val.Index(0)
		default:
			return val
		}
	}
}

// floatFieldRoutes lists every float in the schema by walking types, so a
// nil pointer or an empty collection in the defaults hides nothing.
func floatFieldRoutes(t *testing.T) []floatRoute {
	t.Helper()

	return floatRoutesOf(reflect.TypeFor[config.Config]())
}

// TestFloatRoutes_ReachEveryShape pins the sweep's own machinery on a fixture
// carrying every shape the walker supports, so a future option in one of them
// is written rather than skipped or panicked on.
func TestFloatRoutes_ReachEveryShape(t *testing.T) {
	type leaf struct {
		Ratio float64 `toml:"ratio"`
	}

	type fixture struct {
		Direct float64            `toml:"direct"`
		Ptr    *float64           `toml:"ptr"`
		List   []leaf             `toml:"list"`
		Table  map[string]leaf    `toml:"table"`
		Rates  map[string]float64 `toml:"rates"`
		Pair   [2]float64         `toml:"pair"`
		Deep   *struct {
			Items []*leaf `toml:"items"`
		} `toml:"deep"`
	}

	want := []string{
		"direct",
		"ptr",
		"list.ratio",
		"table.sweep.ratio",
		"rates.sweep",
		"pair",
		"deep.items.ratio",
	}

	routes := floatRoutesOf(reflect.TypeFor[fixture]())

	got := make([]string, 0, len(routes))
	for _, route := range routes {
		got = append(got, route.String())
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("routes = %v, want %v", got, want)
	}

	for _, route := range routes {
		var cfg fixture

		route.set(t, reflect.ValueOf(&cfg).Elem(), math.NaN())

		err := config.FiniteFloats(reflect.ValueOf(cfg))
		if err == nil || !strings.Contains(err.Error(), route.String()) {
			t.Errorf("%s: set wrote somewhere the validator did not name: %v", route, err)
		}
	}
}

func floatRoutesOf(root reflect.Type) []floatRoute {
	var routes []floatRoute

	var walk func(typ reflect.Type, route floatRoute, via reflect.Kind)

	walk = func(typ reflect.Type, route floatRoute, via reflect.Kind) {
		switch typ.Kind() { //nolint:exhaustive // only the kinds that can hold or contain a float matter
		case reflect.Float32, reflect.Float64:
			if via != reflect.Invalid {
				// The float sits directly in a collection; the hop records
				// the collection and no field name.
				route = append(append(floatRoute{}, route...), routeHop{kind: via})
			}

			routes = append(routes, route)
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
			walk(typ.Elem(), route, typ.Kind())
		case reflect.Struct:
			for field := range typ.Fields() {
				name, _, _ := strings.Cut(field.Tag.Get("toml"), ",")
				if !field.IsExported() || name == "" || name == "-" {
					continue
				}

				hop := routeHop{name: name, kind: via}
				next := append(append(floatRoute{}, route...), hop)
				walk(field.Type, next, reflect.Invalid)
			}
		default:
		}
	}

	walk(root, nil, reflect.Invalid)

	return routes
}
