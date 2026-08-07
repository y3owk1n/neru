package architecture_test

import (
	"go/ast"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// validatorLadder is the method that runs every config validator in order. A
// validator this one does not call never runs, whatever it checks.
const validatorLadder = "ValidateWithWarnings"

// validatorPrefix is the naming convention the ladder's steps share. It is what
// makes the set of validators discoverable at all: there is no interface, no
// table and no registration, only the name.
const validatorPrefix = "Validate"

// plainValidator is the ladder's other entry point, which delegates to it. Its
// name happens to equal validatorPrefix; the two are spelled separately because
// one names a method and the other a naming convention.
const plainValidator = "Validate"

// defaultsBuilder is the function that assembles the shared defaults. Every
// schema field has to be assigned by name somewhere it reaches.
const defaultsBuilder = "newDefaultConfig"

// ladderEntryPoints are the Validate* methods that are the ladder rather than a
// step in it, mapped to why they cannot appear inside it.
//
// TestValidatorLadderExemptionsStayHonest fails on an entry that stopped
// describing the code, so this list can only shrink.
var ladderEntryPoints = map[string]string{
	plainValidator:  "the plain entry point; it delegates to " + validatorLadder,
	validatorLadder: "the ladder itself, which cannot be a step in itself",
}

// exemptedDefaults is the named allowlist for the explicit-default rule, keyed
// Type.Field: a schema field with no assignment in the defaults and no
// structural reason to lack one. It ships empty on purpose, so that the first
// entry is a decision someone writes a reason for rather than a line appended
// to a list that was never empty.
//
// TestConfigDefaultExemptionsStayHonest fails on an entry that stopped being
// needed, so this list can only shrink.
var exemptedDefaults = map[string]string{}

// minSchemaFields guards against a vacuous pass, the same way
// minSchemaOptions does for the example checks. The walk finds a few hundred
// fields today; a reflection bug that returned none would satisfy every
// assertion here without anyone noticing.
const minSchemaFields = 150

// minLadderSteps guards the other walk. An AST pass that matched nothing —
// because the ladder was renamed, or the receiver stopped being c — would
// report a perfectly wired ladder made of no validators at all.
const minLadderSteps = 15

// bindingWalk is the helper that reads every action string the configuration
// can dispatch: the global bindings, each mode's, the per-app overrides of
// both, the Mission Control hooks and the macro bodies. A validator that goes
// through it is not checking one section of the file — it is reading all of
// them, which is why it cannot run before the validators that own them.
const bindingWalk = "eachBindingAction"

// minBindingWalks guards the ordering check against passing vacuously. Two
// validators go through the binding walk today; a name match that found none
// would leave it asserting nothing about an empty tail.
const minBindingWalks = 2

// TestEveryConfigValidatorRunsInTheLadder pins the validator link of the option
// chain (internal/config/AGENTS.md): a validator that is written but never
// called is indistinguishable, to a user, from one that was never written.
//
// Nothing is unwired today and this test finds nothing on the day it lands;
// that is the bargain, and
// docs/adr/0006-config-options-get-guardrails-not-generation.md is the answer
// to "what was this for". It is an AST pass rather than reflection because the
// ladder is a hand-ordered sequence of calls, and only the source says which
// calls it makes.
func TestEveryConfigValidatorRunsInTheLadder(t *testing.T) {
	declared := declaredConfigValidators(t)
	called := validatorsCalledByLadder(t)

	if len(called) < minLadderSteps {
		t.Fatalf(
			"found only %d validator calls in Config.%s, expected at least %d; "+
				"the AST walk is broken and this check would pass vacuously",
			len(called), validatorLadder, minLadderSteps,
		)
	}

	for _, name := range declared {
		if called[name] {
			continue
		}

		if _, isEntryPoint := ladderEntryPoints[name]; isEntryPoint {
			continue
		}

		t.Errorf(
			"Config.%s is declared in internal/config but never called by Config.%s, "+
				"so it never runs; add it to the ladder, or delete it",
			name, validatorLadder,
		)
	}
}

// TestValidatorLadderExemptionsStayHonest keeps the two exemptions from
// outliving what they describe. Both are claims about the code, and claims rot:
// an entry point that stopped delegating is a validator nobody runs, wearing an
// exemption that says it is fine.
func TestValidatorLadderExemptionsStayHonest(t *testing.T) {
	declared := declaredConfigValidators(t)
	called := validatorsCalledByLadder(t)

	for name, reason := range ladderEntryPoints {
		switch {
		case !slices.Contains(declared, name):
			t.Errorf(
				"ladderEntryPoints names Config.%s, which is not a %s* method on *Config; "+
					"drop the entry (%s)",
				name, validatorPrefix, reason,
			)
		case called[name]:
			t.Errorf(
				"ladderEntryPoints names Config.%s, which Config.%s now calls; "+
					"drop the entry (%s)",
				name, validatorLadder, reason,
			)
		}
	}

	// The plain entry point is exempt because it delegates. If it ever grew a
	// ladder of its own, the exemption would be hiding a second one.
	if !methodCalls(t, configMethodDecl(t, plainValidator), validatorLadder) {
		t.Errorf(
			"Config.%s no longer calls Config.%s; it is exempt from the wiring "+
				"check only because it delegates, so it now hides whatever it does instead",
			plainValidator, validatorLadder,
		)
	}
}

// TestTheBindingWalksCloseTheLadder pins the one ordering the ladder has. Its
// steps are otherwise independent — each reads its own section — but the two
// that read every binding in the file are not, and they close the sequence.
//
// The ladder said so in a comment for two releases while it was not true: the
// comment claimed ValidateMacros ran last and a step appended after it in
// #1201 left the claim standing (#1270). A stated contract nothing enforces is
// believed, which is the whole premise of the guardrails around it
// (docs/adr/0006-config-options-get-guardrails-not-generation.md), so the
// ordering is either declared here or it is not a contract at all.
//
// What it does not pin is the order of the walks among themselves: neither
// reads what the other establishes, and pinning that would invent a dependency
// the code does not have.
func TestTheBindingWalksCloseTheLadder(t *testing.T) {
	order := ladderCallOrder(t)
	walks := validatorsWalkingBindings(t)

	if len(order) < minLadderSteps {
		t.Fatalf(
			"found only %d validator calls in Config.%s, expected at least %d; "+
				"the AST walk is broken and this check would pass vacuously",
			len(order), validatorLadder, minLadderSteps,
		)
	}

	if len(walks) < minBindingWalks {
		t.Fatalf(
			"found only %d validators going through %s, expected at least %d; the "+
				"AST walk is broken and this check would pass vacuously",
			len(walks), bindingWalk, minBindingWalks,
		)
	}

	unwired := slices.DeleteFunc(slices.Sorted(maps.Keys(walks)), func(name string) bool {
		return slices.Contains(order, name)
	})
	if len(unwired) > 0 {
		t.Fatalf(
			"%v go through %s but Config.%s never calls them, so there is no "+
				"position for them to hold; TestEveryConfigValidatorRunsInTheLadder "+
				"is the check that answers for that",
			unwired, bindingWalk, validatorLadder,
		)
	}

	// From the first walk rather than the last len(walks) entries: a validator
	// the ladder happened to call twice would shift a window counted from the
	// end, and let the step it was hiding through.
	firstWalk := slices.IndexFunc(order, func(name string) bool { return walks[name] })

	for _, name := range order[firstWalk+1:] {
		if walks[name] {
			continue
		}

		t.Errorf(
			"Config.%s runs after a validator that reads every binding in the file "+
				"through %s, so the whole-configuration walks no longer close "+
				"Config.%s; move it above them, or the walks report faults against "+
				"tables the validator that owns them has not read yet",
			name, bindingWalk, validatorLadder,
		)
	}
}

// TestEverySchemaFieldHasAnExplicitDefault pins the second universal link of
// the option chain (internal/config/AGENTS.md): every field the schema declares
// is assigned a value by name in the shared defaults, rather than inheriting
// Go's zero value by omission.
//
// Reflection alone cannot tell a deliberate zero from a forgotten one — both
// read as "". grid.row_labels and grid.col_labels were the forgotten kind for
// as long as the guide claimed otherwise, with their real meaning living in a
// consumer, so the question this asks is whether the source names the field,
// not what value it ends up with.
func TestEverySchemaFieldHasAnExplicitDefault(t *testing.T) {
	missing := undefaultedSchemaFields(t)

	for _, key := range slices.Sorted(maps.Keys(missing)) {
		// Rule 3, named. Rules 1 and 2 are structural and already applied.
		if _, exempt := exemptedDefaults[key]; exempt {
			continue
		}

		field := missing[key]

		t.Errorf(
			"%s is assigned by name at %d of the %d places %s() builds a %s, so "+
				"config option %q takes Go's zero value by omission; assign it at "+
				"each one, even where the value assigned is that same zero — a zero "+
				"nobody wrote cannot be told from a forgotten one",
			key, field.setAt, field.sites, defaultsBuilder, field.owner, field.path,
		)
	}
}

// TestConfigDefaultExemptionsStayHonest keeps the named allowlist from
// outliving what it describes. The structural exemptions need no companion
// here: both are recomputed from the live defaults on every run, so a field
// that stops qualifying stops being exempt by itself.
func TestConfigDefaultExemptionsStayHonest(t *testing.T) {
	missing := undefaultedSchemaFields(t)

	for key, reason := range exemptedDefaults {
		if _, needed := missing[key]; !needed {
			t.Errorf(
				"exemptedDefaults names %s, which is no longer a schema field without "+
					"a default; drop the entry (%s)",
				key, reason,
			)
		}
	}
}

// undefaultedField is a schema field the shared defaults leave unstated, with
// the tally that says how badly: sites is how many times the builder writes a
// literal of the owning struct, setAt how many of those name the field.
type undefaultedField struct {
	schemaField

	setAt int
	sites int
}

// undefaultedSchemaFields returns every schema field the shared defaults do not
// assign by name at every literal of its owning struct, keyed Type.Field.
//
// It is keyed by declaration rather than by TOML path because that is the shape
// of the fix: the mode indicator's per-mode table is one struct reached five
// times, and one missing line in the defaults is one thing to correct, not
// five. The path of one place it shows up rides along for the failure message.
func undefaultedSchemaFields(t *testing.T) map[string]undefaultedField {
	t.Helper()

	schema := reflectConfigSchema(t)
	sites := defaultLiteralSites(t)

	if len(schema.fields) < minSchemaFields {
		t.Fatalf(
			"reflected only %d schema fields, expected at least %d; the walk is "+
				"broken and this check would pass vacuously",
			len(schema.fields), minSchemaFields,
		)
	}

	missing := make(map[string]undefaultedField)

	for _, field := range schema.fields {
		// Rules 1 and 2, structural, applied during the reflection walk.
		if field.exemption != "" {
			continue
		}

		literals := sites[field.owner]

		setAt := 0

		for _, fields := range literals {
			if fields[field.name] {
				setAt++
			}
		}

		if len(literals) > 0 && setAt == len(literals) {
			continue
		}

		key := field.owner + "." + field.name
		if _, seen := missing[key]; !seen {
			missing[key] = undefaultedField{
				schemaField: field,
				setAt:       setAt,
				sites:       len(literals),
			}
		}
	}

	return missing
}

// declaredConfigValidators names every Validate* method declared on *Config
// across the config package, sorted so failures read in a stable order.
func declaredConfigValidators(t *testing.T) []string {
	t.Helper()

	return configMethodsWithPrefix(t, validatorPrefix)
}

// validatorsCalledByLadder names every Validate* method the ladder calls on its
// own receiver. A call on anything else is not a step of this ladder.
func validatorsCalledByLadder(t *testing.T) map[string]bool {
	t.Helper()

	return configMethodsCalledBy(t, validatorLadder, validatorPrefix)
}

// ladderCallOrder names every validator the ladder calls on its own receiver,
// in the order the source makes the calls. The set the wiring checks read
// answers which validators run; only a sequence answers when.
func ladderCallOrder(t *testing.T) []string {
	t.Helper()

	return configMethodCallOrder(t, validatorLadder, validatorPrefix)
}

// validatorsWalkingBindings names every ladder step that goes through the
// binding walk.
//
// The call has to be a direct one, which is how the wiring checks read the
// ladder itself: a validator that reached the walk through a helper would go
// unfound here, and the ordering would stop being demanded of it. Nothing is
// written that way today — eachBindingAction takes the visitor, so a caller
// hands it one — and minBindingWalks catches the day this finds none at all.
//
// The entry points are excluded rather than found and dropped later: the ladder
// calls the validators that walk, so it walks too, and it is not a step in the
// sequence whose order this is about.
func validatorsWalkingBindings(t *testing.T) map[string]bool {
	t.Helper()

	// Fails loudly on a renamed walk, rather than leaving every validator
	// looking like it does not go through one.
	configMethodDecl(t, bindingWalk)

	walks := make(map[string]bool)

	for _, name := range declaredConfigValidators(t) {
		if _, isEntryPoint := ladderEntryPoints[name]; isEntryPoint {
			continue
		}

		if methodCalls(t, configMethodDecl(t, name), bindingWalk) {
			walks[name] = true
		}
	}

	return walks
}

// configMethodsWithPrefix names every method on *Config whose name starts with
// the prefix, sorted so failures read in a stable order. The prefix is the only
// thing that makes such a set discoverable: there is no interface, no table and
// no registration behind either the validator ladder or the derivation chain.
func configMethodsWithPrefix(t *testing.T, prefix string) []string {
	t.Helper()

	var names []string

	for _, file := range parsedConfigPackage(t) {
		for _, decl := range file.Decls {
			funcDecl, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || !isConfigMethod(funcDecl) {
				continue
			}

			if strings.HasPrefix(funcDecl.Name.Name, prefix) {
				names = append(names, funcDecl.Name.Name)
			}
		}
	}

	slices.Sort(names)

	return names
}

// configMethodsCalledBy names every prefixed method that the named Config
// method calls on its own receiver. A call on anything else is not a step of it.
func configMethodsCalledBy(t *testing.T, method, prefix string) map[string]bool {
	t.Helper()

	called := make(map[string]bool)

	for _, name := range configMethodCallOrder(t, method, prefix) {
		called[name] = true
	}

	return called
}

// configMethodCallOrder is the same reading in the order the source makes the
// calls. The set is derived from it rather than walked separately: two passes
// over one method would be free to disagree about what a call is.
func configMethodCallOrder(t *testing.T, method, prefix string) []string {
	t.Helper()

	decl := configMethodDecl(t, method)
	receiver := receiverName(decl)

	var order []string

	ast.Inspect(decl.Body, func(node ast.Node) bool {
		name := methodCallOnReceiver(node, receiver)
		if strings.HasPrefix(name, prefix) {
			order = append(order, name)
		}

		return true
	})

	return order
}

// writesToTheConfig reports whether a method's signature leaves it nowhere to
// put its answer but the receiver.
func writesToTheConfig(decl *ast.FuncDecl) bool {
	takesNothing := decl.Type.Params == nil || len(decl.Type.Params.List) == 0
	returnsNothing := decl.Type.Results == nil || len(decl.Type.Results.List) == 0

	return takesNothing && returnsNothing
}

// methodCalls reports whether a method calls another one on its own receiver.
func methodCalls(t *testing.T, decl *ast.FuncDecl, method string) bool {
	t.Helper()

	receiver := receiverName(decl)
	found := false

	ast.Inspect(decl.Body, func(node ast.Node) bool {
		if methodCallOnReceiver(node, receiver) == method {
			found = true
		}

		return !found
	})

	return found
}

// methodCallOnReceiver names the method a node calls on the given receiver, or
// "" when the node is not such a call.
func methodCallOnReceiver(node ast.Node, receiver string) string {
	call, isCall := node.(*ast.CallExpr)
	if !isCall {
		return ""
	}

	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return ""
	}

	ident, isIdent := selector.X.(*ast.Ident)
	if !isIdent || ident.Name != receiver {
		return ""
	}

	return selector.Sel.Name
}

// isConfigMethod reports whether a declaration is a method on Config, pointer
// receiver or not.
func isConfigMethod(decl *ast.FuncDecl) bool {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return false
	}

	return receiverTypeName(decl.Recv.List[0].Type) == "Config"
}

// receiverName is the identifier a method refers to its receiver by.
func receiverName(decl *ast.FuncDecl) string {
	if decl.Recv == nil || len(decl.Recv.List) == 0 || len(decl.Recv.List[0].Names) == 0 {
		return ""
	}

	return decl.Recv.List[0].Names[0].Name
}

// configMethodDecl finds one method on Config by name, failing when it is gone
// — a renamed ladder must break this suite loudly rather than leave it checking
// nothing. It insists on the receiver because the package has more than one
// Validate: Color carries its own.
func configMethodDecl(t *testing.T, name string) *ast.FuncDecl {
	t.Helper()

	for _, file := range parsedConfigPackage(t) {
		for _, decl := range file.Decls {
			funcDecl, isFunc := decl.(*ast.FuncDecl)
			if isFunc && isConfigMethod(funcDecl) && funcDecl.Name.Name == name {
				return funcDecl
			}
		}
	}

	t.Fatalf(
		"internal/config declares no Config.%s; it was renamed or removed, and "+
			"the guardrail that reads it now checks nothing",
		name,
	)

	return nil
}

// defaultLiteralSites reads the shared defaults as the source states them: for
// each schema struct, one set of field names per composite literal of it that
// newDefaultConfig() reaches.
//
// Per literal rather than merged per type, because merging lets two literals of
// one struct cover for each other — each omitting a field the other sets, and
// neither stating a default. That is the exact shape the mode indicator table
// had when this check first ran, five literals deep.
//
// It follows plain function calls out of newDefaultConfig() because the builder
// is written as one — defaultGrid(), defaultHints() and the rest are that
// function's body, split up. Platform overrides are deliberately not followed:
// applyPlatformDefaults() adjusts a default that already exists, and a field
// that only a platform sets has no shared default at all.
func defaultLiteralSites(t *testing.T) map[string][]map[string]bool {
	t.Helper()

	files := parsedConfigPackage(t)

	declarations := make(map[string]*ast.FuncDecl)

	for _, file := range files {
		for _, decl := range file.Decls {
			funcDecl, isFunc := decl.(*ast.FuncDecl)
			if isFunc && funcDecl.Recv == nil {
				declarations[funcDecl.Name.Name] = funcDecl
			}
		}
	}

	sites := make(map[string][]map[string]bool)
	visited := make(map[string]bool)

	var walk func(name string)

	walk = func(name string) {
		decl, declared := declarations[name]
		if !declared || visited[name] {
			return
		}

		visited[name] = true

		ast.Inspect(decl.Body, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.CompositeLit:
				recordLiteralSite(typed, sites)
			case *ast.CallExpr:
				if callee, isIdent := typed.Fun.(*ast.Ident); isIdent {
					walk(callee.Name)
				}
			}

			return true
		})
	}

	walk(defaultsBuilder)

	if len(sites) == 0 {
		t.Fatalf(
			"found no struct literals under %s(); the AST walk is broken",
			defaultsBuilder,
		)
	}

	return sites
}

// recordLiteralSite notes the fields one keyed literal of a named struct type
// sets. A literal whose type is not a bare name is skipped — an unkeyed or
// elided one names nothing, and the schema structs are never written that way.
// Skipping fails closed: an unread literal makes its fields look undefaulted,
// which reports, rather than making them look defaulted, which would not.
func recordLiteralSite(lit *ast.CompositeLit, sites map[string][]map[string]bool) {
	ident, isIdent := lit.Type.(*ast.Ident)
	if !isIdent {
		return
	}

	fields := make(map[string]bool)

	for _, element := range lit.Elts {
		pair, isPair := element.(*ast.KeyValueExpr)
		if !isPair {
			continue
		}

		if key, isKey := pair.Key.(*ast.Ident); isKey {
			fields[key.Name] = true
		}
	}

	sites[ident.Name] = append(sites[ident.Name], fields)
}

// parsedConfigPackage parses every non-test Go file in internal/config,
// platform files included: the validators are spread across a dozen of them.
// Build tags are not honored, so a Validate* method declared for one platform
// only would be demanded of the shared ladder. There are none today, and the
// demand would be the right one to raise — a validator that runs on one
// platform and silently not on another is the failure this file exists for.
func parsedConfigPackage(t *testing.T) []*ast.File {
	t.Helper()

	return parsedGoFiles(t, filepath.Join(findRepoRoot(t), "internal", "config"))
}
