package architecture_test

import (
	"go/ast"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/parity"
	"github.com/y3owk1n/neru/internal/supportref"
)

// supportADR is the decision every failure in this file cites. The rule it
// states is the one being broken, so the message that reports a breach names it
// rather than leaving a reader to find out why any of this is demanded.
const supportADR = "docs/adr/0013-parity-is-measured-in-words-not-subsystems.md"

// The declarations under test, named where a failure has to send someone.
const (
	optionDeclaration = "config.PlatformSupport() in internal/config/platform_support.go"
	actionDeclaration = "action.PlatformSupport() in internal/domain/action/platform_support.go"
	flagDeclaration   = "modecmd.PlatformSupport() in internal/domain/modecmd/platform_support.go"
)

// The floors that keep a walk which matched nothing from passing every
// assertion here. The schema has a few hundred options and the other two
// vocabularies a few dozen words each; a reflection or AST bug that returned
// none would report a perfectly declared vocabulary made of nothing at all.
const (
	minDeclarableOptions = 200
	minActionNames       = 25
	minModeFlags         = 10
	// minColorNodes guards the one place this reading departs from the example
	// checks'. A Color is declared at the field and not at its two leaves, so a
	// by-type match that stopped matching would drop the field from the schema
	// side of the walk and leave every Color looking undeclared — while the
	// count above stayed comfortably over its floor.
	minColorNodes = 25
)

// actionNameType is the Go type whose constants are the action vocabulary. The
// constants are read from the source rather than from a list in the package,
// because a list is a second declaration and would drift from the first — which
// is the failure this whole file is about.
const actionNameType = "Name"

// TestEveryConfigOptionDeclaresItsPlatformSupport pins the rule ADR 0013
// settled: every option in the schema is either supported on every platform or
// declared as narrower, and both answers are written down.
//
// Nothing else in the tree can tell the two apart. smooth_scroll is a
// cross-platform block that only the darwin scroll animator reads: it compiled,
// linted, passed every test and shipped as a silent no-op, while the capability
// matrix reported scroll injection as supported on all three Linux backends,
// because scroll injection genuinely works. This is the check that would have
// stopped it.
func TestEveryConfigOptionDeclaresItsPlatformSupport(t *testing.T) {
	declared := declaredOptionPaths(t)

	for _, path := range declarableOptionPaths(t) {
		if declared[path] {
			continue
		}

		t.Errorf(
			"config option %q declares no platform support; add it to %s — as "+
				"supported everywhere if it is, which is written down rather than "+
				"assumed, because an option that is neither shared nor declared "+
				"ships as a silent no-op (%s)",
			path, optionDeclaration, supportADR,
		)
	}
}

// TestEveryActionDeclaresItsPlatformSupport is the same rule for the action
// vocabulary. An action is a name a person writes in a binding, so an action
// that runs on macOS and does nothing on Linux is the same broken promise as an
// option that does.
func TestEveryActionDeclaresItsPlatformSupport(t *testing.T) {
	declared := declaredWords(t, action.PlatformSupport(), parity.KindAction)

	for _, name := range actionNames(t) {
		if declared[name] {
			continue
		}

		t.Errorf(
			"action %q declares no platform support; add it to %s — as supported "+
				"everywhere if it is, because an action that is neither shared nor "+
				"declared ships as a silent no-op (%s)",
			name, actionDeclaration, supportADR,
		)
	}
}

// TestEveryModeFlagDeclaresItsPlatformSupport is the same rule for the mode
// flags. The grammar's descriptor table already owns which modes accept a flag;
// this is the other column a flag needs before it can be believed.
func TestEveryModeFlagDeclaresItsPlatformSupport(t *testing.T) {
	declared := declaredWords(t, modecmd.PlatformSupport(), parity.KindModeFlag)

	flags := modecmd.All()
	if len(flags) < minModeFlags {
		t.Fatalf(
			"found only %d mode flags, expected at least %d; the vocabulary is "+
				"unreadable and this check would pass vacuously",
			len(flags), minModeFlags,
		)
	}

	for _, descriptor := range flags {
		name := descriptor.Name().String()
		if declared[name] {
			continue
		}

		t.Errorf(
			"mode flag %q declares no platform support; add it to %s — as "+
				"supported everywhere if it is, because a flag that is neither "+
				"shared nor declared ships as a silent no-op (%s)",
			name, flagDeclaration, supportADR,
		)
	}
}

// TestPlatformSupportDeclaresNothingItCannotName catches the drift running the
// other way. A declaration entry naming an option, action or flag that no
// longer exists is worse than a missing one: it publishes a documentation row
// and a `neru doctor` line about a word nobody can write.
func TestPlatformSupportDeclaresNothingItCannotName(t *testing.T) {
	options := slices.Collect(maps.Keys(declarableOptionSet(t)))
	actions := actionNames(t)

	flags := make([]string, 0, len(modecmd.All()))
	for _, descriptor := range modecmd.All() {
		flags = append(flags, descriptor.Name().String())
	}

	tests := []struct {
		kind        parity.Kind
		declaration parity.Declaration
		home        string
		vocabulary  []string
	}{
		{parity.KindOption, config.PlatformSupport(), optionDeclaration, options},
		{parity.KindAction, action.PlatformSupport(), actionDeclaration, actions},
		{parity.KindModeFlag, modecmd.PlatformSupport(), flagDeclaration, flags},
	}

	for _, test := range tests {
		t.Run(string(test.kind), func(t *testing.T) {
			for _, word := range test.declaration {
				if word.Kind != test.kind {
					t.Errorf(
						"%s declares %q under kind %q; a table declares one kind, or "+
							"the projections that read it file a word under the wrong "+
							"vocabulary (%s)",
						test.home, word.Name, word.Kind, supportADR,
					)

					continue
				}

				if slices.Contains(test.vocabulary, word.Name) {
					continue
				}

				t.Errorf(
					"%s declares platform support for %q, which the %s vocabulary no "+
						"longer holds; drop the entry, or the published table and "+
						"`neru doctor` describe a word nobody can write (%s)",
					test.home, word.Name, test.kind, supportADR,
				)
			}
		})
	}
}

// TestPlatformSupportDeclaresEachWordOnce keeps the declaration a declaration.
// Two entries for one word are two answers to one question, and the lookup
// takes the first — so the second is invisible, which is the shape of failure
// this whole file exists to end.
func TestPlatformSupportDeclaresEachWordOnce(t *testing.T) {
	declarations := map[string]parity.Declaration{
		optionDeclaration: config.PlatformSupport(),
		actionDeclaration: action.PlatformSupport(),
		flagDeclaration:   modecmd.PlatformSupport(),
	}

	for _, home := range slices.Sorted(maps.Keys(declarations)) {
		seen := make(map[string]bool)

		for _, word := range declarations[home] {
			key := string(word.Kind) + "\x00" + word.Name + "\x00" + word.Value
			if seen[key] {
				t.Errorf(
					"%s declares %q twice; one word carries one platform column, and a "+
						"lookup can only report the first (%s)",
					home, word.Written(), supportADR,
				)
			}

			seen[key] = true
		}
	}
}

// TestPlatformSupportDeclaresTheWordBehindEveryValue pins the relationship
// between the two shapes of entry. A declaration about `hints.strategy =
// vision` is a claim about one value of an option that has to exist and be
// declared in its own right; without the bare entry, the option itself would
// have no column and the guardrails above would have let it through.
func TestPlatformSupportDeclaresTheWordBehindEveryValue(t *testing.T) {
	for home, declaration := range map[string]parity.Declaration{
		optionDeclaration: config.PlatformSupport(),
		actionDeclaration: action.PlatformSupport(),
		flagDeclaration:   modecmd.PlatformSupport(),
	} {
		for _, word := range declaration {
			if word.Value == "" {
				continue
			}

			if _, found := declaration.Lookup(word.Kind, word.Name, ""); !found {
				t.Errorf(
					"%s declares %q but never declares %q itself; a value's column "+
						"narrows a word that has one, so declare the word too (%s)",
					home, word.Written(), word.Name, supportADR,
				)
			}
		}
	}
}

// TestEveryDeclaredPlatformIsAPlatform keeps a column honest. A typo in a
// platform name reads as a narrower column, so an option supported everywhere
// would start warning on a platform it works on — quietly, because nothing
// downstream can tell a misspelled platform from a missing one.
func TestEveryDeclaredPlatformIsAPlatform(t *testing.T) {
	for home, declaration := range map[string]parity.Declaration{
		optionDeclaration: config.PlatformSupport(),
		actionDeclaration: action.PlatformSupport(),
		flagDeclaration:   modecmd.PlatformSupport(),
	} {
		for _, word := range declaration {
			if len(word.Platforms) == 0 {
				t.Errorf(
					"%s declares %q as supported on no platform at all; a word nothing "+
						"supports is a word to delete, not to declare (%s)",
					home, word.Written(), supportADR,
				)
			}

			for _, platform := range word.Platforms {
				if !slices.Contains(parity.AllPlatforms, platform) {
					t.Errorf(
						"%s declares %q on platform %q, which is not one of %v (%s)",
						home, word.Written(), platform, parity.AllPlatforms, supportADR,
					)
				}
			}

			if !word.Platforms.Everywhere() && word.Note == "" {
				t.Errorf(
					"%s declares %q as narrower than every platform with no note "+
						"saying why; the note is what the load-time warning and the "+
						"published table tell the user (%s)",
					home, word.Written(), supportADR,
				)
			}
		}
	}
}

// TestPlatformSupportTable_IsGeneratedFromTheDeclarations pins the published
// table to the declarations it projects.
//
// It regenerates each page's region and compares: a platform column changed
// without regenerating fails here rather than shipping a page that describes a
// binary nobody built. Every page carrying the markers is checked rather than
// the one that carries them today — a second page nothing checked would be the
// drift this exists to end, in a new place.
func TestPlatformSupportTable_IsGeneratedFromTheDeclarations(t *testing.T) {
	published := 0

	for _, doc := range markdownFiles(t, findRepoRoot(t)) {
		contents, err := os.ReadFile(doc.absPath)
		if err != nil {
			t.Fatalf("failed to read %s: %v", doc.relPath, err)
		}

		document := string(contents)
		if !strings.Contains(document, supportref.BeginMarker) {
			continue
		}

		published++

		assertSupportTableIsCurrent(t, doc.relPath, document)
	}

	if published == 0 {
		t.Errorf(
			"no page carries the platform-support table; add the %q marker to the "+
				"page that owns capability status (%s)",
			supportref.BeginMarker, supportADR,
		)
	}
}

// assertSupportTableIsCurrent checks one page: every word with a narrow column
// has a row, and the whole region is what the declarations render today.
func assertSupportTableIsCurrent(t *testing.T, name, document string) {
	t.Helper()

	region, err := supportref.Region(document)
	if err != nil {
		t.Errorf("%s: %v", name, err)

		return
	}

	limited := supportref.Declaration().Limited()
	if len(limited) == 0 {
		t.Fatalf(
			"nothing is declared as narrower than every platform, so %s would "+
				"publish an empty table; the known members of ADR 0013 went missing (%s)",
			name, supportADR,
		)
	}

	for _, word := range limited {
		if !strings.Contains(region, "`"+word.Written()+"`") {
			t.Errorf(
				"%s is missing from the platform-support table in %s; run "+
					"`just gensupportref` (%s)",
				word.Written(), name, supportADR,
			)
		}
	}

	regenerated, err := supportref.Rewrite(document)
	if err != nil {
		t.Errorf("%s: %v", name, err)

		return
	}

	if regenerated != document {
		t.Errorf(
			"the platform-support table in %s is out of date with the declarations; "+
				"run `just gensupportref` (the region is generated — do not edit it "+
				"by hand; %s)",
			name, supportADR,
		)
	}
}

// declaredOptionPaths is the set of option paths the config declaration names,
// values included under their bare path.
func declaredOptionPaths(t *testing.T) map[string]bool {
	t.Helper()

	return declaredWords(t, config.PlatformSupport(), parity.KindOption)
}

// declaredWords is the set of names one declaration covers for one kind.
func declaredWords(t *testing.T, declaration parity.Declaration, kind parity.Kind) map[string]bool {
	t.Helper()

	declared := make(map[string]bool, len(declaration))

	for _, word := range declaration {
		if word.Kind == kind {
			declared[word.Name] = true
		}
	}

	if len(declared) == 0 {
		t.Fatalf("the %s declaration is empty; nothing below would check anything", kind)
	}

	return declared
}

// declarableOptionPaths is every option path a person can write, in schema
// order: the schema's leaves, with a config.Color declared at the field rather
// than at its light and dark leaves.
//
// The Color rule is the one place this reading differs from the example
// checks', and it differs because the question differs: an example file writes
// the leaves, and a person deciding whether an option works on their platform
// is deciding about the color, not about one of its two halves.
func declarableOptionPaths(t *testing.T) []string {
	t.Helper()

	schema := reflectConfigSchema(t)

	var paths []string

	for _, option := range schema.options {
		if option.exemption == exemptColorLeaf {
			continue
		}

		paths = append(paths, option.path)
	}

	if len(schema.colorNodes) < minColorNodes {
		t.Fatalf(
			"reflected only %d config.Color fields, expected at least %d; the "+
				"by-type match is broken, so the colors would report as undeclared",
			len(schema.colorNodes), minColorNodes,
		)
	}

	paths = append(paths, schema.colorNodes...)

	if len(paths) < minDeclarableOptions {
		t.Fatalf(
			"reflected only %d declarable options, expected at least %d; the walk "+
				"is broken and this check would pass vacuously",
			len(paths), minDeclarableOptions,
		)
	}

	return paths
}

// declarableOptionSet is the same reading as a set, for the check that runs the
// other way.
func declarableOptionSet(t *testing.T) map[string]bool {
	t.Helper()

	paths := declarableOptionPaths(t)
	set := make(map[string]bool, len(paths))

	for _, path := range paths {
		set[path] = true
	}

	return set
}

// actionNames reads the action vocabulary out of its own source: every constant
// of type Name in internal/domain/action.
//
// Read rather than listed, because a list here would be one more copy of the
// vocabulary to drift — the failure ADR 0008 settled for the keys and the roles.
func actionNames(t *testing.T) []string {
	t.Helper()

	directory := filepath.Join(findRepoRoot(t), "internal", "domain", "action")

	var names []string

	for _, file := range parsedGoFiles(t, directory) {
		for _, decl := range file.Decls {
			genDecl, isGen := decl.(*ast.GenDecl)
			if !isGen || genDecl.Tok != token.CONST {
				continue
			}

			names = append(names, namedStringConstants(genDecl, actionNameType)...)
		}
	}

	if len(names) < minActionNames {
		t.Fatalf(
			"found only %d %s constants in internal/domain/action, expected at "+
				"least %d; the AST walk is broken and this check would pass vacuously",
			len(names), actionNameType, minActionNames,
		)
	}

	return names
}

// namedStringConstants reads the string values of one const block's entries
// that are declared with the given type. A block whose first entry names the
// type carries it to the rest, which is how the action vocabulary is written.
func namedStringConstants(decl *ast.GenDecl, typeName string) []string {
	var values []string

	for _, spec := range decl.Specs {
		valueSpec, isValue := spec.(*ast.ValueSpec)
		if !isValue {
			continue
		}

		ident, isIdent := valueSpec.Type.(*ast.Ident)
		if !isIdent || ident.Name != typeName {
			continue
		}

		for _, value := range valueSpec.Values {
			literal, isLiteral := value.(*ast.BasicLit)
			if !isLiteral || literal.Kind != token.STRING {
				continue
			}

			unquoted, err := strconv.Unquote(literal.Value)
			if err != nil || strings.TrimSpace(unquoted) == "" {
				continue
			}

			values = append(values, unquoted)
		}
	}

	return values
}
