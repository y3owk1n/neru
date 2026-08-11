package architecture_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/y3owk1n/neru/configs"
	"github.com/y3owk1n/neru/internal/config"
)

// defaultExample is the one shipped config meant to show every option. The
// other three demonstrate a single mode and are deliberately partial, so only
// this one is held to the completeness rule.
const defaultExample = "default-config.toml"

// Why an option can be missing from the example, in the order the rules apply.
const (
	// A config.Color is written as one value; its light/dark leaves are what
	// ResolveThemeDefaults() fills in from the theme palette.
	exemptColorLeaf = "leaf beneath a config.Color field"
	// A collection that ships empty has nothing to show. Writing it out would
	// put a bare [] in the example and assert nothing.
	exemptEmptyCollection = "collection option that is empty by default"
	// The same rule reaching the keys of a repeated table nobody ships entries
	// for. This is what exempts the app_configs and layers sub-fields — the
	// bulk of the legitimate absences — so it is named rather than folded into
	// the reason above, which is untrue of a leaf.
	exemptInsideEmptyCollection = "sub-field of a collection option that is empty by default"
)

// exemptedOptions is the named allowlist: an option with no example line and
// no structural reason to lack one. It ships empty on purpose, so that the
// first entry is a decision someone writes a reason for rather than a line
// appended to a list that was never empty.
//
// TestConfigExampleExemptionsStayHonest fails on an entry that stopped being
// needed, so this list can only shrink.
var exemptedOptions = map[string]string{}

// minSchemaOptions guards against a vacuous pass. The walk below finds roughly
// 350 leaf options today; a reflection bug that returned none would satisfy
// every assertion in this file without anyone noticing.
const minSchemaOptions = 200

// colorType is the named struct the by-type exemption keys on. Color has
// exactly Light and Dark (internal/config/color.go), so matching the type
// reaches those two leaves and nothing else — ThemeConfig.Light and .Dark are
// ThemePalette, and stay under the normal rule. TestConfigExampleExemptions-
// StayHonest pins that shape, because a third leaf would widen the exemption
// without anyone deciding it should.
var colorType = reflect.TypeFor[config.Color]()

// colorLeaves are the two names the by-type exemption skips.
var colorLeaves = []string{"light", "dark"}

// stringOrArrayType is a slice in Go and one value in TOML: it unmarshals from
// a bare string as readily as from an array (internal/config/keys.go). The
// by-kind exemption is about collections, and this is not one — a nil default
// means the option has no default, not that it ships empty.
var stringOrArrayType = reflect.TypeFor[config.StringOrStringArray]()

// bareKey matches an unquoted TOML key, the form every schema option takes.
// Quoted keys belong to the hotkey and macro tables, whose vocabulary is the
// user's rather than the schema's.
var bareKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// configOption is one leaf of the schema, addressed the way a user writes it.
type configOption struct {
	path string
	// exemption is why the option needs no example line, or "" when it needs one.
	exemption string
}

// schemaField is one struct field the walk passes through, intermediate tables
// included, addressed the way Go declares it.
//
// The example checks work in TOML terms because an example file does; the
// explicit-default check in config_chain_test.go has to speak Go, because what
// it reads is a composite literal. Both come off this one walk rather than two,
// so a schema the reflection cannot reach is invisible to neither or to both.
type schemaField struct {
	// owner is the name of the struct type that declares the field.
	owner string
	// name is the Go field name, which is what a default assigns by.
	name string
	// path is the TOML path, for a failure a reader can act on.
	path string
	// exemption is why the field needs no explicit default, or "" when it does.
	exemption string
}

// configSchema is config.Config reflected into TOML terms.
type configSchema struct {
	t       *testing.T
	options []configOption
	fields  []schemaField
	// known holds every path a shipped example may legitimately write,
	// intermediate tables included.
	known map[string]bool
	// colorNodes holds the path of every config.Color field, which is one
	// option to whoever writes it and two leaves to the walk. The example
	// checks want the leaves; the platform-support declaration wants the node,
	// because a Color is written as one value as readily as a table.
	colorNodes []string
	// opaque holds the prefixes whose children the user names rather than the
	// schema: the hotkey tables and the macro table.
	opaque []string
}

// TestConfigOptionsAppearInTheDefaultExample pins the example TOML to the
// schema. That file is what a user copies, so an option missing from it is one
// most users never learn exists — and forgetting the line failed nothing,
// because TOML has no opinion about the keys a file leaves out.
//
// The exemption rule is the whole design here. 161 of the 165 absences on the
// day this landed were correct, so a naive assertion would be 97% false
// positives; see docs/adr/0006-config-options-get-guardrails-not-generation.md.
func TestConfigOptionsAppearInTheDefaultExample(t *testing.T) {
	schema := reflectConfigSchema(t)
	present := defaultExampleKeyPaths(t)

	for _, option := range schema.options {
		// Rules 1 and 2, structural, applied during the walk.
		if option.exemption != "" {
			continue
		}

		// Rule 3, named.
		if _, ok := exemptedOptions[option.path]; ok {
			continue
		}

		if present[option.path] {
			continue
		}

		t.Errorf(
			"config option %q has no line in configs/%s; add one — commented out "+
				"if the option has no default, since an uncommented empty value "+
				"asserts a default that does not exist",
			option.path, defaultExample,
		)
	}
}

// TestShippedExamplesWriteOnlySchemaKeys catches the drift running the other
// way. TOML decoding ignores a key the schema does not have, silently, so a
// misspelled key in an example is a dead line that looks like it works — to
// the reviewer, and then to every user who copies the file.
func TestShippedExamplesWriteOnlySchemaKeys(t *testing.T) {
	schema := reflectConfigSchema(t)
	root := findRepoRoot(t)

	for _, name := range configs.ShippedExamples {
		t.Run(name, func(t *testing.T) {
			written := decodedKeyPaths(t, filepath.Join(root, "configs", name), schema)

			for _, path := range written {
				if schema.known[path] {
					continue
				}

				t.Errorf(
					"configs/%s writes %q, which the config schema has no key for; "+
						"the decoder drops it silently, so the line does nothing",
					name, path,
				)
			}
		})
	}
}

// TestConfigExampleExemptionsStayHonest keeps the exemptions from outliving
// what they describe. The by-kind rule needs no companion — it is recomputed
// from the defaults on every run, so a collection that gains a non-empty
// default stops being exempt by itself. The other two are claims about the
// code, and claims rot.
func TestConfigExampleExemptionsStayHonest(t *testing.T) {
	schema := reflectConfigSchema(t)
	present := defaultExampleKeyPaths(t)

	// The by-type rule skips every leaf beneath a config.Color. That is only
	// precise while Color is exactly these two leaves.
	var leaves []string

	for field := range colorType.Fields() {
		name, _ := tomlFieldName(t, colorType, field)
		leaves = append(leaves, name)
	}

	if !slices.Equal(leaves, colorLeaves) {
		t.Errorf(
			"config.Color now writes %v, not %v; the by-type exemption skips "+
				"every leaf beneath a Color field, so a new one would be exempted "+
				"without anyone deciding it should be",
			leaves, colorLeaves,
		)
	}

	// The named allowlist can only shrink.
	isOption := make(map[string]bool, len(schema.options))
	for _, option := range schema.options {
		isOption[option.path] = true
	}

	for path, reason := range exemptedOptions {
		switch {
		case !isOption[path]:
			t.Errorf(
				"exemptedOptions names %q, which is not a config option; "+
					"drop the entry (%s)",
				path, reason,
			)
		case present[path]:
			t.Errorf(
				"exemptedOptions names %q, which configs/%s now writes; "+
					"drop the entry (%s)",
				path, defaultExample, reason,
			)
		}
	}
}

// reflectConfigSchema walks config.Config against its defaults. The defaults
// are needed because the second exemption asks whether a collection ships
// empty, which the type alone cannot answer.
func reflectConfigSchema(t *testing.T) *configSchema {
	t.Helper()

	defaults := config.DefaultConfig()
	schema := &configSchema{t: t, known: make(map[string]bool)}

	value := reflect.ValueOf(*defaults)
	schema.walkStruct(value.Type(), value, "", "")

	if len(schema.options) < minSchemaOptions {
		t.Fatalf(
			"reflected only %d config options, expected at least %d; "+
				"the walk is broken and every check here would pass vacuously",
			len(schema.options), minSchemaOptions,
		)
	}

	return schema
}

func (s *configSchema) walkStruct(typ reflect.Type, val reflect.Value, prefix, exemption string) {
	for index := range typ.NumField() {
		field := typ.Field(index)
		if !field.IsExported() {
			continue
		}

		name, freeForm := tomlFieldName(s.t, typ, field)
		path := joinPath(prefix, name)

		s.known[path] = true

		fieldVal := reflect.Value{}
		if val.IsValid() {
			fieldVal = val.Field(index)
		}

		// A collection that ships empty is exempt from needing an explicit
		// default for the same reason it is exempt from needing an example
		// line: nil and empty are the same collection to every reader of it,
		// so there is no forgotten-versus-deliberate to tell apart. That is
		// also the rule's limit — a collection meant to ship non-empty, whose
		// default was forgotten, reads as empty and gets forgiven. A scalar
		// gets no such pass, which is the point: grid.row_labels defaulting to
		// "" by omission is exactly what this must not forgive.
		fieldExemption := exemption
		if fieldExemption == "" && isEmptyCollection(field.Type, fieldVal) {
			fieldExemption = exemptEmptyCollection
		}

		s.fields = append(s.fields, schemaField{
			owner:     typ.Name(),
			name:      field.Name,
			path:      path,
			exemption: fieldExemption,
		})

		if freeForm {
			// The struct decoder never sees these — internal/config/loader/
			// load.go reads them off the raw map, because their keys are the
			// key combinations the user invents. Nothing below is a schema path.
			s.opaque = append(s.opaque, path)

			continue
		}

		s.walkValue(field.Type, fieldVal, path, exemption)
	}
}

func (s *configSchema) walkValue(typ reflect.Type, val reflect.Value, path, exemption string) {
	switch {
	case typ.Kind() == reflect.Pointer:
		// An optional override: its shape is the pointed-to type, and a nil
		// default says nothing about whether the option needs a line.
		s.walkValue(typ.Elem(), reflect.Value{}, path, exemption)
	case typ == colorType:
		// Exemption 1, structural by type.
		s.colorNodes = append(s.colorNodes, path)

		for _, leaf := range colorLeaves {
			s.record(joinPath(path, leaf), exemptColorLeaf)
		}
	case typ == stringOrArrayType:
		s.record(path, exemption)
	case typ.Kind() == reflect.Struct:
		s.walkStruct(typ, val, path, exemption)
	case typ.Kind() == reflect.Slice, typ.Kind() == reflect.Map:
		s.walkCollection(typ, val, path, exemption)
	default:
		s.record(path, exemption)
	}
}

func (s *configSchema) walkCollection(
	typ reflect.Type,
	val reflect.Value,
	path, exemption string,
) {
	// Exemption 2, structural by kind.
	if exemption == "" && isEmptyCollection(typ, val) {
		exemption = exemptEmptyCollection
	}

	s.record(path, exemption)

	if typ.Kind() == reflect.Map {
		// Map keys are the user's, not the schema's.
		s.opaque = append(s.opaque, path)

		return
	}

	// A repeated table ([[app_configs]], [[layers]]) still has a declared
	// shape, and the keys an example writes under it are checked against it.
	elem := typ.Elem()
	for elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}

	if elem.Kind() != reflect.Struct || elem == colorType {
		return
	}

	inherited := exemption
	if inherited == exemptEmptyCollection {
		inherited = exemptInsideEmptyCollection
	}

	s.walkStruct(elem, reflect.Value{}, path, inherited)
}

func (s *configSchema) record(path, exemption string) {
	s.known[path] = true
	s.options = append(s.options, configOption{path: path, exemption: exemption})
}

func (s *configSchema) isOpaque(path string) bool {
	return slices.ContainsFunc(s.opaque, func(prefix string) bool {
		return path == prefix || strings.HasPrefix(path, prefix+".")
	})
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}

	return prefix + "." + name
}

// tomlFieldName returns the name a field is written under and whether its
// contents are free-form. A toml:"-" field is still written in a config file —
// the hotkey tables are the reason the tag is there — so its name comes from
// the json tag, and everything below it is the user's vocabulary.
func tomlFieldName(t *testing.T, owner reflect.Type, field reflect.StructField) (string, bool) {
	t.Helper()

	name, _, _ := strings.Cut(field.Tag.Get("toml"), ",")

	switch name {
	case "":
		t.Fatalf(
			"%s.%s carries no toml tag; the schema cannot name it, so no "+
				"projection of it can be checked",
			owner.Name(), field.Name,
		)
	case "-":
		jsonName, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if jsonName == "" || jsonName == "-" {
			t.Fatalf(
				"%s.%s is toml:\"-\" with no json name to fall back on",
				owner.Name(), field.Name,
			)
		}

		return jsonName, true
	}

	return name, false
}

func defaultExampleKeyPaths(t *testing.T) map[string]bool {
	t.Helper()

	return exampleKeyPaths(t, filepath.Join(findRepoRoot(t), "configs", defaultExample))
}

// exampleKeyPaths reads the option paths an example file writes, counting
// commented-out lines. Some options have no default, and an uncommented empty
// value would assert one that does not exist, so the example documents their
// shape with the line commented out.
//
// Live and commented lines carry their own table cursor. A commented block
// names its own table — the macro example does — and letting that header
// retarget the live keys below it would file a real option under a table it is
// not in, which fails open: the option reads as absent, or worse, a phantom
// path marks a genuinely missing one present.
func exampleKeyPaths(t *testing.T, path string) map[string]bool {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}

	present := make(map[string]bool)
	liveTable, commentedTable := "", ""

	for raw := range strings.SplitSeq(string(contents), "\n") {
		trimmed := strings.TrimSpace(raw)
		commented := strings.HasPrefix(trimmed, "#")
		line := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))

		if table, isHeader := tableHeader(line); isHeader {
			commentedTable = table

			if !commented {
				liveTable = table
			}

			continue
		}

		key, _, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		if !bareKey.MatchString(key) {
			continue
		}

		table := liveTable
		if commented {
			table = commentedTable
		}

		present[joinPath(table, key)] = true
	}

	return present
}

// tableHeader reads a [table] or [[array.of.tables]] header.
func tableHeader(line string) (string, bool) {
	switch {
	case strings.HasPrefix(line, "[["):
		return strings.TrimSuffix(strings.TrimPrefix(line, "[["), "]]"), true
	case strings.HasPrefix(line, "["):
		return strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"), true
	}

	return "", false
}

// decodedKeyPaths returns the paths the TOML decoder actually reads out of an
// example. Going through the decoder rather than the text is the point: these
// are exactly the keys a user's copy would carry, expanded the same way.
func decodedKeyPaths(t *testing.T, path string, schema *configSchema) []string {
	t.Helper()

	var raw map[string]any

	_, err := toml.DecodeFile(path, &raw)
	if err != nil {
		t.Fatalf("failed to decode %s: %v", path, err)
	}

	var paths []string

	collectKeyPaths(raw, "", schema, &paths)
	slices.Sort(paths)

	return paths
}

func collectKeyPaths(node map[string]any, prefix string, schema *configSchema, out *[]string) {
	for key, value := range node {
		path := joinPath(prefix, key)

		*out = append(*out, path)

		if schema.isOpaque(path) {
			continue
		}

		switch typed := value.(type) {
		case map[string]any:
			collectKeyPaths(typed, path, schema, out)
		case []map[string]any:
			for _, entry := range typed {
				collectKeyPaths(entry, path, schema, out)
			}
		case []any:
			// A repeated table decodes here when its entries are heterogeneous.
			for _, entry := range typed {
				if table, ok := entry.(map[string]any); ok {
					collectKeyPaths(table, path, schema, out)
				}
			}
		}
	}
}

// isEmptyCollection reports whether a default value is a slice or map that
// ships empty. A StringOrStringArray is excluded: it is a slice in Go and one
// value in TOML, so an empty one is an option with no default rather than a
// collection with an empty one.
func isEmptyCollection(typ reflect.Type, val reflect.Value) bool {
	if typ == stringOrArrayType {
		return false
	}

	if typ.Kind() != reflect.Slice && typ.Kind() != reflect.Map {
		return false
	}

	return !val.IsValid() || val.Len() == 0
}
