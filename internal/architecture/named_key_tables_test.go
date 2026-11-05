package architecture_test

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

// Where the named-key vocabulary is spelled on macOS: two Objective-C sources
// carrying three tables of names between them, and the keycode enum all three
// index into.
const (
	namedKeyKeymapSource   = "internal/adapter/platform/darwin/keymap_darwin.m"
	namedKeyEventTapSource = "internal/adapter/platform/darwin/eventtap_darwin.m"
	namedKeyCodeHeader     = "internal/adapter/platform/darwin/keymap.h"

	// namedKeyInboundTable is the dictionary a hotkey string is parsed
	// through. A name it does not carry resolves to 0xFFFF and the binding is
	// refused. namedKeyInboundFunc is the function it is built in, which also
	// holds the code-to-name overrides.
	namedKeyInboundTable = "gSpecialNameToCodeMap"
	namedKeyInboundFunc  = "initializeSpecialKeyMaps"

	// namedKeyOutboundFunc is the switch the event tap names a keystroke with.
	namedKeyOutboundFunc = "specialKeyName"

	// namedKeyCodeEnum is the Objective-C enum both tables index into, and
	// namedKeyCodeSymbol is the prefix its members are spelled with.
	namedKeyCodeEnum   = "KeyCode"
	namedKeyCodeSymbol = "kKeyCode"

	// namedKeyNumpadSymbol prefixes the numpad members of that enum. The
	// numpad folds are deliberately one-directional, so this is what the pin
	// recognizes them by.
	namedKeyNumpadSymbol = namedKeyCodeSymbol + "Numpad"

	// namedKeyClear is the one name the macOS tables spell that the vocabulary
	// deliberately does not declare.
	namedKeyClear = "Clear"

	// namedKeyGoDeclaration names the Go side in failure messages, so a reader
	// is sent to the declaration rather than to this test.
	namedKeyGoDeclaration = "keyvocab.NamedKeys()"
)

// namedKeySite is one place in the darwin bridge that spells named keys: the
// source file, and what inside it does the spelling. They always travel
// together into a failure message, and one of the three is a dictionary while
// the other two are functions, which is why this is a site rather than a
// table.
type namedKeySite struct {
	source string
	what   string
}

// String spells the site the way a failure message names it.
func (site namedKeySite) String() string {
	return site.source + ": " + site.what
}

// The three sites this pin reads.
var (
	namedKeyInboundSite = namedKeySite{
		source: namedKeyKeymapSource,
		what:   namedKeyInboundTable,
	}
	namedKeyFoldSite = namedKeySite{
		source: namedKeyKeymapSource,
		what:   namedKeyInboundFunc,
	}
	namedKeyOutboundSite = namedKeySite{
		source: namedKeyEventTapSource,
		what:   namedKeyOutboundFunc,
	}
)

// carbonStopsAtF20 is the reason four function keys are missing from every
// macOS table, and keyvocab's own functionKeyCount comment carries the same fact.
const carbonStopsAtF20 = "Carbon declares no virtual key code above F20, " +
	"so this key reaches a binding only on Linux and Windows"

// darwinNamedKeyGaps are the named keys no macOS table carries, each with the
// reason it is absent. They are written out rather than skipped. A gap this
// list does not name fails the pin, and a gap it names that macOS has since
// grown a keycode for fails it too, so neither half can rot quietly.
var darwinNamedKeyGaps = map[string]string{
	"F21": carbonStopsAtF20,
	"F22": carbonStopsAtF20,
	"F23": carbonStopsAtF20,
	"F24": carbonStopsAtF20,
	keyvocab.KeyInsert: "Carbon declares no Insert virtual key code, " +
		"so an Insert binding reaches a keystroke only on Linux and Windows — " +
		"the same shape as the F21-F24 gap (docs/adr/0008-a-vocabulary-has-one-home.md)",
}

// TestDarwinNamedKeyTablesArePinnedToTheVocabulary keeps macOS binding the keys
// Neru says it binds.
//
// The named keys are declared once in Go (internal/domain/keyvocab), and the
// config validators, the Linux tap and the Windows tap all read that
// declaration. macOS re-types the set twice in Objective-C, because a .m file
// cannot import a Go package. gSpecialNameToCodeMap turns a written binding
// into a keycode, and specialKeyName turns a keystroke back into a name. This
// is ADR 0007's deliberate exception to the one-implementation rule
// (docs/adr/0007-a-shared-derivation-has-one-implementation.md). Where the
// second implementation is in another language, what the rule asks for is a
// test holding the copies together rather than a deletion.
//
// It fails on four shapes: a native name the vocabulary does not declare, a
// native name spelled differently from the vocabulary's, a named key macOS
// carries no entry for, and a keycode symbol that is not the one the name
// means. The failure it exists for is silent. A binding written against a name
// gSpecialNameToCodeMap does not carry parses to 0xFFFF, parseHotkeyString
// returns NO, and the key simply never fires.
func TestDarwinNamedKeyTablesArePinnedToTheVocabulary(t *testing.T) {
	t.Parallel()

	for _, problem := range darwinNamedKeyVocabularyProblems(readDarwinNamedKeyTables(t)) {
		t.Error(problem)
	}
}

// TestDarwinNamedKeyTablesArePinnedToEachOther keeps the two tables spelling
// one vocabulary between them.
//
// They are not copies of each other. One reads names and one writes them, so
// the inbound table carries the Enter and Backspace aliases and the outbound
// one must not, or a keystroke would reach the mode handler under two
// spellings. Everything else has to match. A name only the inbound table knows
// is a binding that parses and never fires, and a name only the outbound one
// knows is a keystroke no binding can be written for.
func TestDarwinNamedKeyTablesArePinnedToEachOther(t *testing.T) {
	t.Parallel()

	for _, problem := range darwinNamedKeyCrossTableProblems(readDarwinNamedKeyTables(t)) {
		t.Error(problem)
	}
}

// TestDarwinNamedKeyFoldsSpellTheVocabulary pins the names the keymap's
// code-to-name table overrides into place.
//
// Two overrides canonicalize the aliases, so a Return keystroke is never
// reported as "Enter". Two more fold the numpad keys that have a named-key
// equivalent (#1372), and they are on the keymap side only, because the tap reaches
// them through NeruKeyCodeToName and NeruKeyCodeToCharacter rather than through
// specialKeyName. Which numpad key folds where is a macOS decision that
// keymap_integration_darwin_test.go pins, on a tagged macOS-only run. What this
// pin holds to the vocabulary is what a fold is allowed to spell, so a fold
// deleted outright is caught there and not here.
func TestDarwinNamedKeyFoldsSpellTheVocabulary(t *testing.T) {
	t.Parallel()

	if keyvocab.IsNamedKey(namedKeyClear) {
		t.Errorf(
			"%s now declares %q, which %s folds the numpad Clear key to precisely "+
				"because no binding can be written for it; decide what a Clear binding "+
				"means on each platform before widening the vocabulary",
			namedKeyGoDeclaration, namedKeyClear, namedKeyKeymapSource,
		)
	}

	for _, problem := range darwinNamedKeyFoldProblems(readDarwinNamedKeyTables(t)) {
		t.Error(problem)
	}
}

// TestDarwinNamedKeyGapsAreRealKeycodeGaps keeps the exception list above
// honest against the keycode enum both tables index into.
//
// A gap is a claim that macOS has no such key, not a license to leave an entry
// out. So each gap is checked against the KeyCode enum in keymap.h. The day
// Carbon grows an F21, the enum gains kKeyCodeF21 and this fails, which is
// where someone decides whether the tables should carry it.
func TestDarwinNamedKeyGapsAreRealKeycodeGaps(t *testing.T) {
	t.Parallel()

	keyCodes := objcEnumIntConstants(t, namedKeyCodeHeader, namedKeyCodeEnum)

	for _, gap := range slices.Sorted(maps.Keys(darwinNamedKeyGaps)) {
		if !keyvocab.IsNamedKey(gap) {
			t.Errorf(
				"darwinNamedKeyGaps excuses %q, which %s does not declare; "+
					"a gap in a vocabulary that no longer has the key is a stale exception",
				gap, namedKeyGoDeclaration,
			)

			continue
		}

		symbol := namedKeyCodeSymbol + gap
		if _, declared := keyCodes[symbol]; declared {
			t.Errorf(
				"%s: enum %s declares %s, so %q is no longer the keycode gap "+
					"darwinNamedKeyGaps excuses it as (%s)",
				namedKeyCodeHeader, namedKeyCodeEnum, symbol, gap, darwinNamedKeyGaps[gap],
			)
		}
	}
}

// darwinNamedKeyTables is what the pin read out of the two Objective-C sources.
//
// Nothing here is an expectation. Every entry comes out of a .m file, and
// internal/domain/keyvocab is the only expectation this pin has. That is what
// makes it bidirectional, because changing either side alone makes the two stop agreeing.
type darwinNamedKeyTables struct {
	// inbound maps each name gSpecialNameToCodeMap accepts to the keycode
	// symbol it resolves to. It carries the aliases.
	inbound map[string]string

	// outbound maps each keycode symbol specialKeyName answers to the name it
	// returns. It must not carry an alias.
	outbound map[string]string

	// folds maps each keycode symbol the keymap overrides in its code-to-name
	// direction to the name it overrides to: the alias canonicalizations and
	// the numpad folds.
	folds map[string]string
}

// darwinNamedKeyVocabularyProblems holds both tables to the Go declaration:
// every native name is one the vocabulary declares, spelled its way and wired
// to the keycode it means, and every named key outside the documented gaps has
// an entry.
func darwinNamedKeyVocabularyProblems(tables darwinNamedKeyTables) []string {
	var problems []string

	for _, name := range slices.Sorted(maps.Keys(tables.inbound)) {
		problems = append(problems, darwinNamedKeySpellingProblems(namedKeyInboundSite, name)...)
		problems = append(problems, darwinNamedKeySymbolProblems(
			namedKeyInboundSite, name, tables.inbound[name],
		)...)
	}

	for _, symbol := range slices.Sorted(maps.Keys(tables.outbound)) {
		name := tables.outbound[symbol]

		problems = append(problems, darwinNamedKeySpellingProblems(namedKeyOutboundSite, name)...)
		problems = append(
			problems,
			darwinNamedKeySymbolProblems(namedKeyOutboundSite, name, symbol)...,
		)

		if means, isAlias := keyvocab.ResolveAlias(name); isAlias {
			problems = append(problems, fmt.Sprintf(
				"%s answers %s with the alias %q; the tap must emit %q, "+
					"or one keystroke reaches the mode handler under two spellings",
				namedKeyOutboundSite, symbol, name, means,
			))
		}
	}

	return append(problems, darwinNamedKeyCoverageProblems(tables)...)
}

// darwinNamedKeyCoverageProblems is the half a per-entry check cannot see: a
// named key with no macOS entry at all. A key missing from either table is a
// binding a config file may write and macOS will never fire, unless
// darwinNamedKeyGaps says why.
func darwinNamedKeyCoverageProblems(tables darwinNamedKeyTables) []string {
	var problems []string

	outboundNames := darwinNamedKeyOutboundNames(tables)

	for _, name := range keyvocab.NamedKeys() {
		reason, excused := darwinNamedKeyGaps[name]
		_, isAlias := keyvocab.ResolveAlias(name)

		_, accepted := tables.inbound[name]

		// An entry and an excuse are the two answers to "can macOS reach this
		// key", so exactly one of them has to be there. Both means a gap that
		// has been filled, and neither means a key that went missing without one.
		if accepted == excused {
			problems = append(problems, darwinNamedKeyCoverageProblem(
				namedKeyInboundSite, name, reason, excused,
			))
		}

		// An alias is a spelling a config file may write, not one the tap may
		// emit, so the outbound table is expected to be missing exactly those.
		if isAlias {
			continue
		}

		if slices.Contains(outboundNames, name) == excused {
			problems = append(problems, darwinNamedKeyCoverageProblem(
				namedKeyOutboundSite, name, reason, excused,
			))
		}
	}

	return problems
}

// darwinNamedKeyCoverageProblem spells the two directions a coverage check
// fails in: a key with no entry and no excuse, or an excused key that has one.
func darwinNamedKeyCoverageProblem(
	site namedKeySite,
	name, reason string,
	excused bool,
) string {
	if excused {
		return fmt.Sprintf(
			"%s carries %q, which darwinNamedKeyGaps excuses as absent (%s); "+
				"if macOS can reach this key now, drop the gap",
			site, name, reason,
		)
	}

	return fmt.Sprintf(
		"%s has no entry for %q, which %s declares; "+
			"add it, or add %q to darwinNamedKeyGaps with the reason macOS cannot reach it",
		site, name, namedKeyGoDeclaration, name,
	)
}

// darwinNamedKeySpellingProblems checks one native name against the vocabulary.
// Case matters, because the tap compares the name it emits against the binding a config
// file wrote, and "PageDown" and "Pagedown" are two keys to a string compare.
func darwinNamedKeySpellingProblems(site namedKeySite, name string) []string {
	display, declared := keyvocab.NamedKeyDisplay(name)
	if !declared {
		return []string{fmt.Sprintf(
			"%s spells %q, which %s does not declare",
			site, name, namedKeyGoDeclaration,
		)}
	}

	if display != name {
		return []string{fmt.Sprintf(
			"%s spells the named key %q; %s spells it %q, and the tap compares them literally",
			site, name, namedKeyGoDeclaration, display,
		)}
	}

	return nil
}

// darwinNamedKeySymbolProblems checks that a name is wired to the keycode of
// the key it means, such as kKeyCodeReturn for both "Return" and its alias "Enter".
// Deriving the symbol from the Go value rather than listing the pairs is what
// the placement-vocabulary pin next door does, and it means a key added to the
// vocabulary needs no edit here.
func darwinNamedKeySymbolProblems(site namedKeySite, name, symbol string) []string {
	means := name
	if resolved, isAlias := keyvocab.ResolveAlias(name); isAlias {
		means = resolved
	}

	if want := namedKeyCodeSymbol + means; symbol != want {
		return []string{fmt.Sprintf(
			"%s wires %q to %s, want %s — %q means the %s key",
			site, name, symbol, want, name, means,
		)}
	}

	return nil
}

// darwinNamedKeyCrossTableProblems holds the two tables to each other, which is
// the half that still bites if the Go declaration and one table drift together.
func darwinNamedKeyCrossTableProblems(tables darwinNamedKeyTables) []string {
	var problems []string

	outboundNames := darwinNamedKeyOutboundNames(tables)

	for _, name := range slices.Sorted(maps.Keys(tables.inbound)) {
		if _, isAlias := keyvocab.ResolveAlias(name); isAlias {
			problems = append(problems, darwinNamedKeyAliasProblems(tables, name)...)

			continue
		}

		if !slices.Contains(outboundNames, name) {
			problems = append(problems, fmt.Sprintf(
				"%s accepts a binding on %q but %s never answers with it: "+
					"the binding parses and no keystroke matches it",
				namedKeyInboundTable, name, namedKeyOutboundFunc,
			))
		}
	}

	for _, name := range outboundNames {
		if _, accepted := tables.inbound[name]; !accepted {
			problems = append(problems, fmt.Sprintf(
				"%s answers with %q but %s does not accept it: "+
					"the tap emits a name no hotkey string can be parsed into",
				namedKeyOutboundFunc, name, namedKeyInboundTable,
			))
		}
	}

	return problems
}

// darwinNamedKeyAliasProblems covers the one asymmetry the two tables are meant
// to have. An alias belongs in the inbound table and nowhere else, and the
// keymap's code-to-name override is what stops the shared keycode from being
// reported under the alias's spelling.
func darwinNamedKeyAliasProblems(tables darwinNamedKeyTables, alias string) []string {
	means, _ := keyvocab.ResolveAlias(alias)
	symbol := namedKeyCodeSymbol + means

	if canonical, overridden := tables.folds[symbol]; !overridden || canonical != means {
		return []string{fmt.Sprintf(
			"%s does not override %s to %q, so the keycode that %q and %q "+
				"share may be reported under either spelling",
			namedKeyFoldSite, symbol, means, alias, means,
		)}
	}

	return nil
}

// darwinNamedKeyFoldProblems holds the code-to-name overrides to the
// vocabulary. A fold may name a key the vocabulary declares, in its canonical
// spelling, or the one name it deliberately does not declare. That name is
// Clear, which exists so the numpad Clear key stops impersonating Delete and
// matches nothing instead.
//
// It also keeps the folds one-directional. A numpad keycode in the inbound
// table would make "Return" synthesize a numpad keypress.
func darwinNamedKeyFoldProblems(tables darwinNamedKeyTables) []string {
	var problems []string

	for _, symbol := range slices.Sorted(maps.Keys(tables.folds)) {
		name := tables.folds[symbol]
		if name == namedKeyClear {
			continue
		}

		problems = append(problems, darwinNamedKeySpellingProblems(namedKeyFoldSite, name)...)

		if means, isAlias := keyvocab.ResolveAlias(name); isAlias {
			problems = append(problems, fmt.Sprintf(
				"%s folds %s to the alias %q; a code-to-name entry must be the canonical %q",
				namedKeyFoldSite, symbol, name, means,
			))
		}
	}

	for _, name := range slices.Sorted(maps.Keys(tables.inbound)) {
		if strings.HasPrefix(tables.inbound[name], namedKeyNumpadSymbol) {
			problems = append(problems, fmt.Sprintf(
				"%s wires %q to %s; the numpad folds are code-to-name only, "+
					"or synthesizing %q would press a numpad key",
				namedKeyInboundSite, name, tables.inbound[name], name,
			))
		}
	}

	return problems
}

// darwinNamedKeyOutboundNames lists what the outbound table can answer with, in
// a stable order.
func darwinNamedKeyOutboundNames(tables darwinNamedKeyTables) []string {
	names := make([]string, 0, len(tables.outbound))
	for _, name := range tables.outbound {
		names = append(names, name)
	}

	slices.Sort(names)

	return slices.Compact(names)
}

// The shapes each table is written in. Addressing them by name means a rename
// surfaces as a failure rather than as a silent pass over a table that is no
// longer there.
var (
	namedKeyInboundFuncPattern = regexp.MustCompile(
		`(?m)^static void ` + namedKeyInboundFunc + `\(void\)[ \t]*\{`,
	)
	namedKeyOutboundFuncPattern = regexp.MustCompile(
		`(?m)^static NSString \*` + namedKeyOutboundFunc + `\(CGKeyCode keyCode\)[ \t]*\{`,
	)

	namedKeyDictOpeningPattern = regexp.MustCompile(
		namedKeyInboundTable + `[ \t]*=[ \t]*\[@\{`,
	)
	namedKeyDictClosingPattern = regexp.MustCompile(`(?m)^[ \t]*\}[ \t]*copy\];`)

	namedKeyDictEntryPattern = regexp.MustCompile(`^@"([^"]*)"[ \t]*:[ \t]*@\((\w+)\),$`)

	// namedKeyFoldOpeningPattern marks a line as an override written against a
	// literal keycode, and namedKeyFoldPattern reads the whole of one. The
	// split is what lets an unreadable override be reported. The enumerate
	// block's `codeToName[code] = name;` is not an override and must be passed
	// over, while `codeToName[@(kKeyCodeNumpadEnter)] = someName;` is one this
	// pin cannot read and must not be.
	namedKeyFoldOpeningPattern = regexp.MustCompile(`^codeToName\[@\(`)
	namedKeyFoldPattern        = regexp.MustCompile(
		`^codeToName\[@\((\w+)\)\][ \t]*=[ \t]*@"([^"]*)";$`,
	)
	namedKeySwitchCasePattern = regexp.MustCompile(`^case[ \t]+(\w+):$`)
	namedKeySwitchNamePattern = regexp.MustCompile(`^return[ \t]+@"([^"]*)";$`)
)

// readDarwinNamedKeyTables reads both tables out of the darwin bridge, failing
// the test when it cannot.
func readDarwinNamedKeyTables(t *testing.T) darwinNamedKeyTables {
	t.Helper()

	tables, problem := parseDarwinNamedKeyTables(
		readNativeSource(t, namedKeyKeymapSource),
		readNativeSource(t, namedKeyEventTapSource),
	)
	if problem != "" {
		t.Fatal(problem)
	}

	return tables
}

// parseDarwinNamedKeyTables reads the three name tables out of the two
// Objective-C sources. The second result describes why they could not be read,
// and is empty when they could. An error value would buy nothing here, since
// the only caller turns it straight into a test failure.
func parseDarwinNamedKeyTables(
	keymapSource, eventTapSource string,
) (darwinNamedKeyTables, string) {
	builder, problem := nativeRuleMethodBody(
		keymapSource,
		namedKeyInboundFuncPattern,
		namedKeyInboundFunc,
		"static void "+namedKeyInboundFunc+"(void) {",
	)
	if problem != "" {
		return darwinNamedKeyTables{}, namedKeyKeymapSource + ": " + problem
	}

	inbound, problem := parseDarwinNamedKeyDictionary(builder)
	if problem != "" {
		return darwinNamedKeyTables{}, namedKeyKeymapSource + ": " + problem
	}

	folds, problem := parseDarwinNamedKeyFolds(builder)
	if problem != "" {
		return darwinNamedKeyTables{}, namedKeyKeymapSource + ": " + problem
	}

	switchBody, problem := nativeRuleMethodBody(
		eventTapSource,
		namedKeyOutboundFuncPattern,
		namedKeyOutboundFunc,
		"static NSString *"+namedKeyOutboundFunc+"(CGKeyCode keyCode) {",
	)
	if problem != "" {
		return darwinNamedKeyTables{}, namedKeyEventTapSource + ": " + problem
	}

	outbound, problem := parseDarwinNamedKeySwitch(switchBody)
	if problem != "" {
		return darwinNamedKeyTables{}, namedKeyEventTapSource + ": " + problem
	}

	return darwinNamedKeyTables{inbound: inbound, outbound: outbound, folds: folds}, ""
}

// parseDarwinNamedKeyDictionary reads the `@"Name" : @(kKeyCodeName),` entries
// of the inbound dictionary literal. Every line inside the literal has to be an
// entry, a comment or blank. A line this pin cannot read is one it would
// otherwise skip past, which is how a pin comes to cover less than it claims.
func parseDarwinNamedKeyDictionary(builder string) (map[string]string, string) {
	opening := namedKeyDictOpeningPattern.FindStringIndex(builder)
	if opening == nil {
		return nil, fmt.Sprintf(
			"no `%s = [@{` literal to read the accepted names from (renamed?)",
			namedKeyInboundTable,
		)
	}

	body := builder[opening[1]:]

	closing := namedKeyDictClosingPattern.FindStringIndex(body)
	if closing == nil {
		return nil, fmt.Sprintf(
			"the %s literal is never closed by `} copy];`",
			namedKeyInboundTable,
		)
	}

	entries := make(map[string]string)

	for line := range strings.SplitSeq(body[:closing[0]], "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		entry := namedKeyDictEntryPattern.FindStringSubmatch(trimmed)
		if entry == nil {
			return nil, fmt.Sprintf(
				"%s holds `%s`, which this pin does not read; it reads "+
					"`@\"<name>\" : @(<keycode>),` entries",
				namedKeyInboundTable, trimmed,
			)
		}

		entries[entry[1]] = entry[2]
	}

	if len(entries) == 0 {
		return nil, fmt.Sprintf("the %s literal is empty", namedKeyInboundTable)
	}

	return entries, ""
}

// parseDarwinNamedKeyFolds reads the `codeToName[@(kKeyCodeName)] = @"Name";`
// overrides out of the builder function.
//
// Unlike the other two tables, these are found inside a longer function rather
// than inside a literal, so "this line is not an override" and "this override
// is one I cannot read" would otherwise be the same answer. And a pin that
// answers them the same way quietly covers fewer folds than it claims. A line
// that starts an override against a literal keycode therefore has to be
// readable in full, and a commented-out one is skipped rather than read as
// live code.
func parseDarwinNamedKeyFolds(builder string) (map[string]string, string) {
	folds := make(map[string]string)

	for line := range strings.SplitSeq(builder, "\n") {
		trimmed := strings.TrimSpace(line)
		if !namedKeyFoldOpeningPattern.MatchString(trimmed) {
			continue
		}

		override := namedKeyFoldPattern.FindStringSubmatch(trimmed)
		if override == nil {
			return nil, fmt.Sprintf(
				"%s holds `%s`, which this pin does not read; it reads "+
					"`codeToName[@(<keycode>)] = @\"<name>\";` overrides",
				namedKeyInboundFunc, trimmed,
			)
		}

		folds[override[1]] = override[2]
	}

	if len(folds) == 0 {
		return nil, namedKeyInboundFunc +
			" writes no `codeToName[@(<keycode>)] = @\"<name>\";` overrides (rewritten?)"
	}

	return folds, ""
}

// parseDarwinNamedKeySwitch reads the `case kKeyCodeName: return @"Name";` arms
// of the outbound switch, keyed by keycode symbol. Arms sharing one return are
// read as one each. An arm returning anything but a string literal or nil is
// reported rather than skipped.
func parseDarwinNamedKeySwitch(body string) (map[string]string, string) {
	arms := make(map[string]string)

	// pending collects the case labels seen since the last return, so labels
	// grouped onto one arm are read as one entry each. It is truncated rather
	// than dropped, because by then every symbol in it is already in arms.
	pending := make([]string, 0, 1)

	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimSpace(line)
		label := namedKeySwitchCasePattern.FindStringSubmatch(trimmed)
		answer := namedKeySwitchNamePattern.FindStringSubmatch(trimmed)

		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "//"),
			trimmed == "{" || trimmed == "}",
			strings.HasPrefix(trimmed, "switch"):
			continue
		case trimmed == "default:":
			pending = pending[:0]
		case label != nil:
			pending = append(pending, label[1])
		case answer != nil:
			if len(pending) == 0 {
				return nil, fmt.Sprintf(
					"%s returns `%s` under no `case` label this pin read",
					namedKeyOutboundFunc, trimmed,
				)
			}

			for _, symbol := range pending {
				arms[symbol] = answer[1]
			}

			pending = pending[:0]
		case trimmed == "return nil;" && len(pending) == 0:
			continue
		default:
			return nil, fmt.Sprintf(
				"%s holds `%s`, which this pin does not read; it reads "+
					"`case <keycode>:` / `return @\"<name>\";` arms and one `default: return nil;`",
				namedKeyOutboundFunc, trimmed,
			)
		}
	}

	if len(arms) == 0 {
		return nil, namedKeyOutboundFunc + " names no key codes"
	}

	return arms, ""
}
