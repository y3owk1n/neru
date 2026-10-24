package configschema

import (
	"encoding/json"
	"reflect"

	"github.com/y3owk1n/neru/internal/buildinfo"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

const (
	draft = "http://json-schema.org/draft-07/schema#"
	// Path is the schema's place in the repository.
	Path = "schemas/config.schema.json"
	// referenceURL is the page each top-level table's description links to.
	referenceURL = "https://github.com/y3owk1n/neru/blob/main/docs/reference/configuration.md"
	// The definitions name the shapes many fields share.
	colorDefinition      = "color"
	solidColorDefinition = "solid_color"
	stepsDefinition      = "steps"
)

// The JSON Schema words the rendering repeats.
const (
	keyType       = "type"
	keyProperties = "properties"
	keyExtra      = "additionalProperties"
	typeObject    = "object"
	typeString    = "string"
)

// sectionAnchors names the reference heading for a top-level key whose
// heading is not the key itself.
var sectionAnchors = map[string]string{
	"app_configs": "per-app-global-hotkey-overrides",
}

var (
	colorType = reflect.TypeFor[config.Color]()
	stepsType = reflect.TypeFor[config.StringOrStringArray]()
	// paletteType holds plain strings that ValidateTheme reads as solid colors.
	paletteType = reflect.TypeFor[config.ThemePalette]()
	// perPlatformType is what every config.PerPlatform instantiation, and each
	// type defined from one, implements.
	perPlatformType = reflect.TypeFor[interface{ PlatformValueType() reflect.Type }]()
)

// URL is where the schema for a build of the given version is published: its
// release tag's copy, or main's for any other build. It is what a config's
// #:schema line names.
func URL(version string) string {
	return buildinfo.RawURL(Path, version)
}

// Render returns the schema as indented JSON, ending in a newline.
func Render() ([]byte, error) {
	root, err := object(reflect.TypeFor[config.Config]())
	if err != nil {
		return nil, err
	}

	properties, _ := root[keyProperties].(map[string]any)
	for key, property := range properties {
		anchor, named := sectionAnchors[key]
		if !named {
			anchor = key
		}

		node, _ := property.(map[string]any)
		node["description"] = "See " + referenceURL + "#" + anchor
	}

	root["$schema"] = draft
	root["$id"] = URL("")
	root["title"] = "Neru configuration"
	root["definitions"] = map[string]any{
		colorDefinition:      colorSchema(),
		solidColorDefinition: solidColorSchema(),
		stepsDefinition:      stepsSchema(),
	}

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeInternal, "marshal the config schema")
	}

	return append(out, '\n'), nil
}

// schema returns the schema for one Go type.
func schema(goType reflect.Type) (map[string]any, error) {
	switch goType {
	case colorType:
		return ref(colorDefinition), nil
	case stepsType:
		return ref(stepsDefinition), nil
	}

	if goType.Implements(perPlatformType) {
		return perPlatformSchema(goType)
	}

	switch goType.Kind() { //nolint:exhaustive // every other kind fails below
	case reflect.Pointer:
		return schema(goType.Elem())
	case reflect.Bool:
		return map[string]any{keyType: "boolean"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{keyType: "integer"}, nil
	case reflect.Float32, reflect.Float64:
		return map[string]any{keyType: "number"}, nil
	case reflect.String:
		return map[string]any{keyType: typeString}, nil
	case reflect.Slice:
		items, err := schema(goType.Elem())
		if err != nil {
			return nil, err
		}

		return map[string]any{keyType: "array", "items": items}, nil
	case reflect.Map:
		values, err := schema(goType.Elem())
		if err != nil {
			return nil, err
		}

		return map[string]any{keyType: typeObject, keyExtra: values}, nil
	case reflect.Struct:
		return object(goType)
	default:
		return nil, derrors.Newf(
			derrors.CodeInternal,
			"config type %s has kind %s, which the schema cannot describe",
			goType,
			goType.Kind(),
		)
	}
}

// object returns the schema for a struct: one property per toml-tagged field,
// and no others, so an editor flags a misspelled key.
func object(goType reflect.Type) (map[string]any, error) {
	properties := map[string]any{}

	for field := range goType.Fields() {
		name, hotkeyTable, isKey := config.TOMLKey(field)

		var (
			property map[string]any
			err      error
		)

		switch {
		case !isKey:
			continue
		case hotkeyTable:
			property = map[string]any{
				keyType:  typeObject,
				keyExtra: ref(stepsDefinition),
			}
		case goType == paletteType:
			property = ref(solidColorDefinition)
		default:
			property, err = schema(field.Type)
			if err != nil {
				return nil, derrors.Wrapf(
					err,
					derrors.CodeInternal,
					"%s.%s",
					goType.Name(),
					field.Name,
				)
			}
		}

		properties[name] = property
	}

	return map[string]any{
		keyType:       typeObject,
		keyProperties: properties,
		keyExtra:      false,
	}, nil
}

// ref points at one of the shared definitions.
func ref(definition string) map[string]any {
	return map[string]any{"$ref": "#/definitions/" + definition}
}

// colorSchema accepts what config.Color decodes: one color for both themes,
// or a table with a light and a dark one. Empty means unset.
func colorSchema() map[string]any {
	value := map[string]any{keyType: typeString, "pattern": "^$|" + config.ColorPattern}

	return map[string]any{
		"oneOf": []any{
			value,
			map[string]any{
				keyType:       typeObject,
				keyProperties: map[string]any{"light": value, "dark": value},
				keyExtra:      false,
			},
		},
	}
}

// perPlatformSchema accepts what config.PerPlatform decodes: one value for
// every platform, or a table that names some of them.
func perPlatformSchema(goType reflect.Type) (map[string]any, error) {
	marker, _ := reflect.TypeAssert[interface{ PlatformValueType() reflect.Type }](
		reflect.Zero(goType),
	)

	value, err := schema(marker.PlatformValueType())
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"oneOf": []any{
			value,
			map[string]any{
				keyType: typeObject,
				keyProperties: map[string]any{
					"darwin":  value,
					"linux":   value,
					"windows": value,
				},
				keyExtra: false,
			},
		},
	}, nil
}

// solidColorSchema accepts what ValidateTheme takes for a palette color: a
// color without alpha, or empty for unset.
func solidColorSchema() map[string]any {
	return map[string]any{keyType: typeString, "pattern": "^$|" + config.SolidColorPattern}
}

// stepsSchema accepts what config.StringOrStringArray decodes: one step, or a
// list of them.
func stepsSchema() map[string]any {
	return map[string]any{
		"anyOf": []any{
			map[string]any{keyType: typeString},
			map[string]any{keyType: "array", "items": map[string]any{keyType: typeString}},
		},
	}
}
