package ipcschema

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/buildinfo"
	"github.com/y3owk1n/neru/internal/derrors"
)

const (
	draft = "http://json-schema.org/draft-07/schema#"
	// Path is the schema's place in the repository.
	Path = "schemas/ipc.schema.json"
)

// The JSON Schema words the rendering repeats.
const (
	keyType    = "type"
	typeObject = "object"
)

// definition is one shape a client can $ref.
type definition struct {
	name        string
	goType      reflect.Type
	description string
	// nullable is a payload that is null rather than an object in one case.
	nullable bool
	// strict refuses keys the type does not declare.
	strict bool
}

func definitions() []definition {
	return []definition{
		{
			name:        "request",
			goType:      reflect.TypeFor[ipc.Command](),
			description: "One request, as a client sends it.",
			strict:      true,
		},
		{
			name:        "response",
			goType:      reflect.TypeFor[ipc.Response](),
			description: "One reply. Its data is one of the payloads below, by command.",
		},
		{
			name:        "status",
			goType:      reflect.TypeFor[ipc.StatusData](),
			description: "The data of status, and the status a watch snapshot carries.",
		},
		{
			name:        "query_displays",
			goType:      reflect.TypeFor[ipc.DisplaysData](),
			description: "The data of query-displays.",
		},
		{
			name:        "query_cursor",
			goType:      reflect.TypeFor[ipc.CursorData](),
			description: "The data of query-cursor.",
		},
		{
			name:        "query_window",
			goType:      reflect.TypeFor[ipc.WindowData](),
			description: "The data of query-window, null when no window has focus.",
			nullable:    true,
		},
		{
			name:        "query_app",
			goType:      reflect.TypeFor[ipc.AppData](),
			description: "The data of query-app.",
		},
		{
			name:        "query_hints",
			goType:      reflect.TypeFor[ipc.HintsData](),
			description: "The data of query-hints.",
		},
		{
			name:        "toggle_scroll_invert",
			goType:      reflect.TypeFor[ipc.ScrollInvertData](),
			description: "The data of toggle-scroll-invert.",
		},
		{
			name:        "toggle_screen_share",
			goType:      reflect.TypeFor[ipc.ScreenShareData](),
			description: "The data of toggle-screen-share.",
		},
		{
			name:        "toggle_cursor_follow_selection",
			goType:      reflect.TypeFor[ipc.FollowSelectionData](),
			description: "The data of toggle-cursor-follow-selection.",
		},
		{
			name:        "watch_line",
			goType:      reflect.TypeFor[ipc.WatchLine](),
			description: "One line of a watch stream after its reply: a snapshot or an event.",
		},
	}
}

// URL is where the schema for a build of the given version is published: its
// release tag's copy, or main's for any other build.
func URL(version string) string {
	return buildinfo.RawURL(Path, version)
}

// Render returns the schema as indented JSON, ending in a newline.
func Render() ([]byte, error) {
	shapes := map[string]any{}

	for _, shape := range definitions() {
		node, err := schema(shape.goType, shape.strict)
		if err != nil {
			return nil, derrors.Wrapf(err, derrors.CodeInternal, "definition %s", shape.name)
		}

		if shape.nullable {
			node = nullable(node)
		}

		node["description"] = shape.description
		shapes[shape.name] = node
	}

	root := map[string]any{
		"$schema":     draft,
		"$id":         URL(""),
		"title":       "Neru IPC",
		"description": "The shapes docs/reference/ipc.md describes. Each is a definition to $ref.",
		"definitions": shapes,
	}

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, derrors.Wrap(err, derrors.CodeInternal, "marshal the IPC schema")
	}

	return append(out, '\n'), nil
}

// schema returns the schema for one Go type as the JSON encoder writes it.
func schema(goType reflect.Type, strict bool) (map[string]any, error) {
	switch goType.Kind() { //nolint:exhaustive // every other kind fails below
	case reflect.Pointer:
		return schema(goType.Elem(), strict)
	case reflect.Bool:
		return map[string]any{keyType: "boolean"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{keyType: "integer"}, nil
	case reflect.Float32, reflect.Float64:
		return map[string]any{keyType: "number"}, nil
	case reflect.String:
		return map[string]any{keyType: "string"}, nil
	case reflect.Interface:
		return map[string]any{}, nil
	case reflect.Slice:
		items, err := schema(goType.Elem(), strict)
		if err != nil {
			return nil, err
		}

		return map[string]any{keyType: "array", "items": items}, nil
	case reflect.Map:
		values, err := schema(goType.Elem(), strict)
		if err != nil {
			return nil, err
		}

		return map[string]any{keyType: typeObject, "additionalProperties": values}, nil
	case reflect.Struct:
		return object(goType, strict)
	default:
		return nil, derrors.Newf(
			derrors.CodeInternal,
			"wire type %s has kind %s, which the schema cannot describe",
			goType,
			goType.Kind(),
		)
	}
}

// object returns the schema for a struct. The encoder always writes a field
// without omitempty, so the field is required, and a pointer field without it
// may be null.
func object(goType reflect.Type, strict bool) (map[string]any, error) {
	properties := map[string]any{}
	required := []string{}

	for field := range goType.Fields() {
		name, options, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}

		property, err := schema(field.Type, strict)
		if err != nil {
			return nil, derrors.Wrapf(err, derrors.CodeInternal, "%s.%s", goType.Name(), field.Name)
		}

		omitted := strings.Contains(options, "omitempty")
		if !omitted {
			required = append(required, name)

			if field.Type.Kind() == reflect.Pointer {
				property = nullable(property)
			}
		}

		properties[name] = property
	}

	node := map[string]any{keyType: typeObject, "properties": properties}
	if len(required) > 0 {
		node["required"] = required
	}

	if strict {
		node["additionalProperties"] = false
	}

	return node, nil
}

// nullable lets node be null as well.
func nullable(node map[string]any) map[string]any {
	return map[string]any{"anyOf": []any{node, map[string]any{keyType: "null"}}}
}
