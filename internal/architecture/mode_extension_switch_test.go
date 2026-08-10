package architecture_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

// domainPackageDir is where domain.Mode and its constants are declared, as the
// shared walker spells a directory, and domainPackagePath is how a file in
// internal/app/modes imports it.
const (
	domainPackageDir  = "internal/domain"
	domainPackagePath = modulePath + "/" + domainPackageDir
)

// domainModeTypeName is the type whose constants name a navigation mode. The
// constants are read out of the domain package rather than listed here, so a
// sixth mode is covered by this pin on the day it is declared.
const domainModeTypeName = "Mode"

// modeConstantFloor is the fewest domain.Mode constants this pin expects to
// read. Six are declared today — idle and the five registered modes — so five
// leaves room for one to be retired and still catches a reader that has stopped
// recognizing how they are written.
//
// It is the floor that matters most here. A switch is recognized by the
// constants its arms name, so a reader that found none would report every mode
// switch in the package as compliant while reading none of them.
const modeConstantFloor = 5

// modeSwitchFloor is the fewest switch statements expected in
// internal/app/modes. Five are there today — one of them the exempt mode switch
// below, the other four discriminating on a key, a permission answer, a
// modifier and, tagless, on a hint index — so three catches a walk that has
// lost the package and never
// fires on a switch being refactored away.
const modeSwitchFloor = 3

// modeSwitchRule is the rule a failure states, in the imperative, with the
// document that states it.
const modeSwitchRule = "behavior only some modes have is an optional extension — a " +
	"narrow unexported interface in extensions.go, named for what it does and " +
	"reached with a comma-ok assertion through activeModeExtension — never an arm " +
	"of a switch over domain.Mode (internal/app/modes/AGENTS.md, the " +
	"optional-extension contract)"

// knownModeSwitchExemptions are the switches over domain.Mode that internal/app/modes
// still holds, keyed by "<file>:<scope>" as the failure below spells a site.
//
// Each entry carries the reason it is still here, so the list can only shrink:
// TestKnownModeSwitchExemptionsAreStillReal fails on an entry that no longer
// names a mode switch, and a new one fails the guardrail rather than joining the
// list. Add an entry only when converting the switch is a design change that
// cannot ride along, never to silence one.
var knownModeSwitchExemptions = map[string]string{
	"internal/app/modes/indicator_polling.go:handlerState.modeIndicatorEnabled": "" +
		"every arm reads a different field of config.ModeIndicator — one per " +
		"registered mode, plus the idle arm — so the axis is carried by all five " +
		"modes, which by this guide's own criterion (monitor-move refresh is core " +
		"\"because all five modes participate\") makes it a sixth method on Mode " +
		"rather than an optional extension. That is a change to the interface, its " +
		"five implementations and every test fake that satisfies it, which is a " +
		"different change from landing this pin",
}

// TestModeHandlerHasNoSwitchOverDomainMode fails when a switch in
// internal/app/modes discriminates on a domain.Mode value.
//
// The bug it prevents is behavior only some modes have written as a switch: an
// arm per mode, empty arms for the modes that do not participate, and a new
// mode silently taking the empty one. The extension model exists so that a mode
// answers for itself — a narrow interface it either implements or does not, with
// TestModeExtensionMatrix (extensions_test.go) failing until every registered
// mode has stated which — and a switch reintroduces the shape that model
// replaced, in a place the matrix cannot see: it iterates the registered
// extension list, so an axis nobody added to that list is an axis it never asks
// about.
//
// The half of this that is already covered is the other half. exhaustive is
// enabled (.golangci.yml runs with default: all and does not disable it) and it
// fires on an *incomplete* switch over domain.Mode, default clause or not. A
// complete one lints clean — and a complete switch is exactly what a careful
// contributor writes, listing all six constants and giving three of them an
// empty arm. So the case the linter cannot see is the case this targets, and
// catching only the incomplete one would duplicate the linter while missing the
// regression. Both are reported here, because a pin that agreed with the linter
// only by accident would be worth less than one that states the whole rule.
//
// What a syntactic reader cannot see, and a reviewer has to: an if/else chain
// over the same constants, which is the same breach and is indistinguishable
// from the single mode comparisons the package legitimately makes (grid.go,
// monitor.go); a switch over the mode *name* strings (domain.ModeName*) rather
// than the Mode constants; and a switch whose tag is a domain.Mode value with no
// arm naming a constant, which discriminates on nothing.
//
// Test files are outside this, and that is a scope rather than an exemption. The
// rule's subject is behavior the package implements for a mode, and a table in a
// test is not that — it ships nothing, and no mode added to newModes can fall
// into an empty arm of it.
func TestModeHandlerHasNoSwitchOverDomainMode(t *testing.T) {
	offenders, judged := modeSwitchesIn(
		modeHandlerSourceFiles(t),
		domainModeConstants(t),
	)

	for _, offender := range offenders {
		if _, known := knownModeSwitchExemptions[offender.site()]; known {
			continue
		}

		t.Errorf(
			"%s: %s switches over domain.Mode (%s); %s",
			offender.position, offender.scope,
			strings.Join(offender.arms, ", "), modeSwitchRule,
		)
	}

	assertWalkedAtLeast(
		t, "switch statements in "+modeHandlerPackageDir, judged, modeSwitchFloor,
	)
}

// TestKnownModeSwitchExemptionsAreStillReal keeps the exemption list honest: an
// entry that no longer names a switch over domain.Mode is stale and must be
// deleted, or the list becomes a place where the extension contract goes to die.
func TestKnownModeSwitchExemptionsAreStillReal(t *testing.T) {
	offenders, _ := modeSwitchesIn(modeHandlerSourceFiles(t), domainModeConstants(t))

	switching := make(map[string]bool, len(offenders))
	for _, offender := range offenders {
		switching[offender.site()] = true
	}

	for site, reason := range knownModeSwitchExemptions {
		if switching[site] {
			continue
		}

		t.Errorf(
			"known exemption %s no longer switches over domain.Mode (%q); delete "+
				"its entry",
			site, reason,
		)
	}
}

// TestModeSwitchPin_CatchesTheSwitchesTheLinterDoesNot is the ticket's
// acceptance check, kept rather than performed once: each shape a mode switch
// arrives in is applied to a file built for the purpose, and the pin has to
// report it.
//
// The complete switch is the one that matters — it compiles, it lints clean
// under exhaustive, and it is what a contributor writes when they are being
// careful — so a pin that only ever met the tree's own switches could not show
// that it reaches the case it exists for.
func TestModeSwitchPin_CatchesTheSwitchesTheLinterDoesNot(t *testing.T) {
	for _, breach := range modeSwitchBreaches() {
		offenders, _ := modeSwitchesIn(
			[]modeHandlerFile{parseModeSwitchFixture(t, breach.source)},
			fixtureModeConstants(),
		)

		if len(offenders) == 0 {
			t.Errorf("the pin reports nothing when %s", breach.name)
		}
	}
}

// TestModeSwitchPin_LeavesEverySwitchThatIsNotOneAlone pins the other half: the
// shapes that look like a mode switch and are not.
//
// Getting this wrong is how a guardrail like this one gets reverted rather than
// obeyed. The package holds five switches that discriminate on something else,
// and the mode constants appear all over its bodies — every activation names its
// own mode — so a reader that judged a switch by what its arms *do* rather than
// by what they match on would report most of the package.
func TestModeSwitchPin_LeavesEverySwitchThatIsNotOneAlone(t *testing.T) {
	for _, allowed := range modeSwitchNonBreaches() {
		offenders, _ := modeSwitchesIn(
			[]modeHandlerFile{parseModeSwitchFixture(t, allowed.source)},
			fixtureModeConstants(),
		)

		if len(offenders) > 0 {
			t.Errorf(
				"the pin reports %s, which breaks no rule: %s",
				allowed.name, offenders[0].scope,
			)
		}
	}
}

// modeSwitch is one switch statement that discriminates on a domain.Mode value.
type modeSwitch struct {
	// file is the slash-relative path of the file holding the switch.
	file string
	// position is that path with the line number, so a failure is clickable.
	position string
	// scope names the declaration the switch sits in.
	scope string
	// arms are the mode constants the switch matches on, in source order, which
	// is the evidence a failure shows for calling it a mode switch.
	arms []string
}

// site is the key the exemption list uses, stable across line moves — a switch
// that survives an edit above it keeps its entry.
func (s modeSwitch) site() string {
	return fmt.Sprintf("%s:%s", s.file, s.scope)
}

// modeSwitchesIn returns every switch over domain.Mode in the given files, and
// how many switch statements it judged. The count is what the floor is asserted
// on: a reader that stopped finding switches would report no offenders at all.
func modeSwitchesIn(files []modeHandlerFile, constants map[string]bool) ([]modeSwitch, int) {
	var (
		offenders []modeSwitch
		judged    int
	)

	for _, file := range files {
		qualifier, importsDomain := domainQualifier(file.syntax)

		forEachSwitchStatement(file, func(scope string, stmt *ast.SwitchStmt) {
			judged++

			if !importsDomain {
				return
			}

			arms := modeConstantsMatchedBy(stmt, qualifier, constants)
			if len(arms) == 0 {
				return
			}

			offenders = append(offenders, modeSwitch{
				file: file.path,
				position: fmt.Sprintf(
					"%s:%d", file.path, file.fileSet.Position(stmt.Pos()).Line,
				),
				scope: scope,
				arms:  arms,
			})
		})
	}

	return offenders, judged
}

// forEachSwitchStatement hands visit every switch statement of a file, named for
// the declaration it was written in.
//
// A switch inside a function literal is reported under the function that holds
// the literal rather than under the literal itself. A mode switch is a mode
// switch wherever it is written, and naming the enclosing function is what makes
// an exemption key survive the literal being moved or given a name.
func forEachSwitchStatement(file modeHandlerFile, visit func(scope string, stmt *ast.SwitchStmt)) {
	for _, decl := range file.syntax.Decls {
		scope := "a package-level declaration"

		if funcDecl, isFunc := decl.(*ast.FuncDecl); isFunc {
			scope = funcName(funcDecl)
		}

		ast.Inspect(decl, func(node ast.Node) bool {
			if stmt, isSwitch := node.(*ast.SwitchStmt); isSwitch {
				visit(scope, stmt)
			}

			return true
		})
	}
}

// modeConstantsMatchedBy returns the mode constants a switch discriminates on,
// in source order and without repeats.
//
// Only what the switch matches *on* is read — its tag and its case expressions —
// never an arm's body. A mode constant inside a body is an ordinary use of one:
// every activation in the package names its own mode there, and a reader that
// counted those would call most of the package a mode switch. That also keeps a
// switch nested inside an arm a switch of its own, judged when the walk reaches
// it.
//
// A constant handed to a call is not read either, wherever the call sits. A
// switch on what a helper answers *about* a named mode — `switch h.enabledFor(
// domain.ModeGrid)` — discriminates on the answer, not on the mode, and the
// three call sites this change deleted had exactly that shape one level out. The
// call's own name is still read, so a comparison whose operand is a call —
// `case h.activeMode() == domain.ModeHints` — is reported like any other.
//
// A tagless switch is covered by the same reading: `case mode == domain.ModeGrid`
// puts the constant in the case expression, which is where this looks.
func modeConstantsMatchedBy(
	stmt *ast.SwitchStmt,
	qualifier string,
	constants map[string]bool,
) []string {
	var (
		matched []string
		seen    = make(map[string]bool)
	)

	var read func(node ast.Node) bool

	read = func(node ast.Node) bool {
		if call, isCall := node.(*ast.CallExpr); isCall {
			// The callee is still read; its arguments are not.
			ast.Inspect(call.Fun, read)

			return false
		}

		name, isConstant := qualifiedConstant(node, qualifier)
		if !isConstant || !constants[name] || seen[name] {
			return true
		}

		seen[name] = true
		matched = append(matched, qualifier+"."+name)

		return true
	}

	collect := func(expr ast.Expr) {
		ast.Inspect(expr, read)
	}

	if stmt.Tag != nil {
		collect(stmt.Tag)
	}

	for _, statement := range stmt.Body.List {
		clause, isCase := statement.(*ast.CaseClause)
		if !isCase {
			continue
		}

		for _, expr := range clause.List {
			collect(expr)
		}
	}

	return matched
}

// qualifiedConstant reports the name a node selects from the given package
// qualifier — "ModeGrid" for domain.ModeGrid.
func qualifiedConstant(node ast.Node, qualifier string) (string, bool) {
	selector, isSelector := node.(*ast.SelectorExpr)
	if !isSelector {
		return "", false
	}

	pkg, isIdent := selector.X.(*ast.Ident)
	if !isIdent || pkg.Name != qualifier {
		return "", false
	}

	return selector.Sel.Name, true
}

// domainQualifier returns the name a file refers to the domain package by, and
// false when it does not import it — in which case no switch in it can name a
// mode constant.
//
// The alias is read rather than assumed, so renaming the import in a file does
// not take that file out of the pin. A dot-import would, and the repository has
// none: gci and the house import style both keep the package named.
func domainQualifier(file *ast.File) (string, bool) {
	for _, spec := range file.Imports {
		path, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil || path != domainPackagePath {
			continue
		}

		if spec.Name == nil {
			return "domain", true
		}

		return spec.Name.Name, spec.Name.Name != "." && spec.Name.Name != "_"
	}

	return "", false
}

// domainModeConstants returns the constants the domain package declares with
// type Mode.
//
// They are read out of the tree rather than listed in this file for the reason
// the extension matrix iterates the live mode map: a mode added to the domain
// vocabulary is one this pin has to know about, and remembering to add it here
// is exactly the step nobody takes.
func domainModeConstants(t *testing.T) map[string]bool {
	t.Helper()

	fileSet := token.NewFileSet()
	names := make(map[string]bool)

	for _, file := range goFiles(t) {
		if file.dir != domainPackageDir {
			continue
		}

		collectModeConstants(parseRepoGoFile(t, fileSet, file.absPath, file.relPath), names)
	}

	assertWalkedAtLeast(t, "domain."+domainModeTypeName+" constants", len(names), modeConstantFloor)

	return names
}

// collectModeConstants adds the Mode-typed constants a file declares to names.
//
// The type carries down a const block the way Go does — a spec with no type of
// its own continues the last one, and a spec with a value but no type ends it —
// so the iota block that declares the modes is read whole rather than one line
// of it.
func collectModeConstants(file *ast.File, names map[string]bool) {
	for _, decl := range file.Decls {
		genDecl, isGenDecl := decl.(*ast.GenDecl)
		if !isGenDecl || genDecl.Tok != token.CONST {
			continue
		}

		carried := ""

		for _, spec := range genDecl.Specs {
			valueSpec, isValueSpec := spec.(*ast.ValueSpec)
			if !isValueSpec {
				continue
			}

			switch {
			case valueSpec.Type != nil:
				carried = ""
				if ident, isIdent := valueSpec.Type.(*ast.Ident); isIdent {
					carried = ident.Name
				}
			case len(valueSpec.Values) > 0:
				carried = ""
			}

			if carried != domainModeTypeName {
				continue
			}

			for _, name := range valueSpec.Names {
				names[name.Name] = true
			}
		}
	}
}

// modeSwitchFixture is a file built to exercise the pin, and what it is meant to
// show.
type modeSwitchFixture struct {
	name   string
	source string
}

// fixtureModeConstants stands in for what domainModeConstants reads out of the
// tree, so a fixture states its own vocabulary and cannot pass by agreeing with
// a reader that returned nothing.
func fixtureModeConstants() map[string]bool {
	return map[string]bool{
		"ModeIdle": true, "ModeHints": true, "ModeGrid": true,
		"ModeScroll": true, "ModeRecursiveGrid": true, "ModeMonitorSelect": true,
	}
}

// fixturePreamble opens every fixture file below with the import the mode
// constants are reached through.
const fixturePreamble = "package modes\n\nimport \"" + domainPackagePath + "\"\n\n"

// modeSwitchBreaches are the shapes a mode switch arrives in. Every one of them
// compiles; the first two also lint clean.
func modeSwitchBreaches() []modeSwitchFixture {
	return []modeSwitchFixture{
		{
			name: "a complete switch names every mode and leaves three arms empty",
			source: fixturePreamble + `
func (h *handlerState) resetInput() {
	switch h.appState.CurrentMode() {
	case domain.ModeHints:
		h.hints.Context.Reset()
	case domain.ModeGrid, domain.ModeRecursiveGrid:
		h.grid.Reset()
	case domain.ModeIdle, domain.ModeScroll, domain.ModeMonitorSelect:
	}
}
`,
		},
		{
			name: "a complete switch answers a value, with a default arm as well",
			source: fixturePreamble + `
func (h *handlerState) followsSelection(mode domain.Mode) bool {
	switch mode {
	case domain.ModeHints, domain.ModeGrid, domain.ModeRecursiveGrid:
		return true
	case domain.ModeIdle, domain.ModeScroll, domain.ModeMonitorSelect:
		return false
	default:
		return false
	}
}
`,
		},
		{
			name: "an incomplete switch names one mode, which the linter also reports",
			source: fixturePreamble + `
func (h *handlerState) resetInput() {
	switch h.appState.CurrentMode() {
	case domain.ModeHints:
		h.hints.Context.Reset()
	}
}
`,
		},
		{
			name: "a tagless switch compares against the mode constants instead",
			source: fixturePreamble + `
func (h *handlerState) resetInput() {
	mode := h.appState.CurrentMode()

	switch {
	case mode == domain.ModeHints:
		h.hints.Context.Reset()
	case mode != domain.ModeScroll:
		h.grid.Reset()
	}
}
`,
		},
		{
			name: "the switch is written inside a function literal",
			source: fixturePreamble + `
func (h *handlerState) resetInput() {
	h.each(func(mode domain.Mode) {
		switch mode {
		case domain.ModeHints:
			h.hints.Context.Reset()
		case domain.ModeIdle, domain.ModeGrid, domain.ModeScroll,
			domain.ModeRecursiveGrid, domain.ModeMonitorSelect:
		}
	})
}
`,
		},
		{
			name: "a case compares the answer of a call against a mode constant",
			source: fixturePreamble + `
func (h *handlerState) resetInput() {
	switch {
	case h.activeMode() == domain.ModeHints:
		h.hints.Context.Reset()
	case h.activeMode() == domain.ModeGrid:
		h.grid.Reset()
	}
}
`,
		},
		{
			name: "the domain package is imported under another name",
			source: "package modes\n\nimport neru \"" + domainPackagePath + "\"\n\n" + `
func (h *handlerState) resetInput() {
	switch h.appState.CurrentMode() {
	case neru.ModeHints:
		h.hints.Context.Reset()
	case neru.ModeIdle, neru.ModeGrid, neru.ModeScroll,
		neru.ModeRecursiveGrid, neru.ModeMonitorSelect:
	}
}
`,
		},
	}
}

// modeSwitchNonBreaches are the shapes the pin must not report.
func modeSwitchNonBreaches() []modeSwitchFixture {
	return []modeSwitchFixture{
		{
			name: "a switch on a key, with mode constants only in its arms",
			source: fixturePreamble + `
func (h *handlerState) handleKey(key string) {
	switch key {
	case "g":
		h.activate(domain.ModeGrid)
	case "s":
		h.activate(domain.ModeScroll)
	default:
		h.activate(domain.ModeIdle)
	}
}
`,
		},
		{
			name: "a switch over another vocabulary the domain package publishes",
			source: fixturePreamble + `
func (h *handlerState) handleCommand(command string) {
	switch command {
	case domain.CommandPing:
		h.pong()
	case domain.CommandStop:
		h.stop()
	}
}
`,
		},
		{
			name: "an if that compares the active mode, which the package does everywhere",
			source: fixturePreamble + `
func (h *handlerState) refresh() {
	if h.appState.CurrentMode() == domain.ModeIdle {
		return
	}

	switch h.strategy {
	case "vision":
		h.walkVision()
	}
}
`,
		},
		{
			name: "a switch on what a helper answers about a named mode",
			source: fixturePreamble + `
func (h *handlerState) poll() {
	switch h.indicatorEnabledFor(domain.ModeGrid) {
	case true:
		h.draw()
	case false:
		h.stop()
	}
}
`,
		},
		{
			name: "a mode-typed switch in a file that reaches the constants through no import",
			source: "package modes\n\n" + `
func (h *handlerState) resetInput() {
	switch h.mode {
	case modeHints:
		h.hints.Context.Reset()
	}
}
`,
		},
	}
}

// parseModeSwitchFixture parses one fixture into the shape the pin reads a file
// of the mode handler package in.
func parseModeSwitchFixture(t *testing.T, source string) modeHandlerFile {
	t.Helper()

	fileSet := token.NewFileSet()

	parsed, parseErr := parser.ParseFile(
		fileSet, "fixture.go", source, parser.SkipObjectResolution,
	)
	if parseErr != nil {
		t.Fatalf("parsing the fixture: %v", parseErr)
	}

	return modeHandlerFile{
		path:    modeHandlerPackageDir + "/fixture.go",
		syntax:  parsed,
		fileSet: fileSet,
	}
}
