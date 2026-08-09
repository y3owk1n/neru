package architecture_test

import (
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// darwinBridgeDir is where the darwin bridge's native source lives, as the
// shared walker spells a directory. It is the bridge directory
// cgo_includes_test.go resolves includes against, named for its platform.
const darwinBridgeDir = bridgePackageDirPrefix + "darwin"

const (
	// nativeHeaderExt is what a subsystem's interface is written in — the
	// interface of a bridge, per ADR 0009.
	nativeHeaderExt = ".h"

	// darwinSourceInfix is the platform token a bridge .m carries, which is
	// part of the file name rather than part of the subsystem's name.
	darwinSourceInfix = "_darwin"
)

// darwinBridgeFileFloor is the fewest native files the darwin bridge holds.
// Thirty-five are there today; below half of that, the walk has stopped
// recognizing them rather than the bridge having shrunk.
const darwinBridgeFileFloor = 15

// darwinEntryPointFloor is the fewest non-static Neru* entry points the bridge
// defines. It is the floor that matters most here: the rule is checked per
// entry point, so a signature scanner that stopped matching would report every
// entry point as compliant while reading none of them. A hundred and thirty-one
// are there today.
const darwinEntryPointFloor = 60

// darwinSubsystemHeaderExemptions are the bridge .m files whose subsystem
// header is not the one their name derives, mapped to the header that declares
// their entry points.
//
// Deriving the subsystem by dropping trailing words off the file name is what
// makes accessibility_element_darwin.m part of the accessibility subsystem and
// accessibility_visibility_darwin.m its own; a file whose name does not lead
// with its subsystem needs an entry here, and the entry is a claim that the
// header named is genuinely where that subsystem's interface lives.
//
// TestDarwinEntryPoints_SubsystemHeaderExemptionsStayHonest holds each entry to
// that claim, so this list can only shrink.
var darwinSubsystemHeaderExemptions = map[string]string{
	// The monitor-select panels are a second .m of the overlay subsystem rather
	// than a subsystem of their own: they draw into the same overlay windows,
	// and both entry points are declared in overlay.h beside the other 31.
	"monitor_select_overlay_darwin.m": "overlay.h",
}

// darwinEntryPointRule is the rule a failure states, in the imperative, with
// the documents that state it. ADR 0009 promised this pin and left it out of
// its own scope; ADR 0011 sharpened what it targets.
const darwinEntryPointRule = "declare every non-static Neru* entry point in its own " +
	"subsystem's header, not in the calling .m, a cgo preamble or another " +
	"subsystem's header (internal/adapter/platform/darwin/AGENTS.md, " +
	"docs/adr/0009-a-bridge-interface-is-its-headers.md)"

// TestDarwinEntryPoints_AreDeclaredInTheirSubsystemHeader pins where a darwin
// bridge entry point's prototype lives.
//
// The bug it prevents is a prototype that drifts away from the interface it
// belongs to. A `Neru*` function declared in *no* header is not that bug and is
// not silent: clang has errored on a call to an undeclared function by default
// since version 16, this repo passes no -W flags of its own, and cgo refuses
// the same call on the Go side — so the naive breach fails the build and names
// the symbol. Six such definitions exist in the bridge today and every one of
// them is static, which is the shape that is meant to have no prototype. ADR
// 0011 counts seven: the one this pin does not count is
// _NeruEnableCursorInBackground, static as well, and outside the Neru*
// PascalCase naming contract by its leading underscore rather than by being
// file-local.
//
// What is silent is the same definition with its prototype written somewhere
// else: at the top of the calling .m, into a cgo preamble (for which there is
// precedent in this package — keymap.go and bridge.go both carry one), or into
// another subsystem's header, where overlay.h's thirty-one entry points hide
// one more. All three compile, lint and link, and each one costs the bridge the
// property ADR 0009 records as its interface: that reading a subsystem's header
// tells you what that subsystem publishes.
//
// So the check is per entry point and one-directional — is this definition
// declared in this subsystem's header — and a declaration anywhere else counts
// for nothing. A header with no .m beside it is left alone, which is what keeps
// ADR 0009's header-only exemption legal: systray.h publishes the systray
// bridge while its .m compiles in internal/adapter/systray/darwin, and nothing
// here asks a header for a definition.
func TestDarwinEntryPoints_AreDeclaredInTheirSubsystemHeader(t *testing.T) {
	t.Parallel()

	bridge := readDarwinBridge(t)

	problems, entryPoints := darwinEntryPointProblems(bridge)

	reportOffenders(t, problems, "prototype of a darwin bridge entry point is not "+
		"in its subsystem's header")

	assertWalkedAtLeast(
		t,
		"non-static Neru* entry points in the darwin bridge",
		entryPoints,
		darwinEntryPointFloor,
	)
}

// TestDarwinEntryPoints_PinCatchesTheSilentBreaches is the ticket's acceptance
// check, kept rather than performed once: each way a prototype can end up
// outside its subsystem's header is applied to a bridge built for the purpose,
// and the pin has to report it.
//
// The fixtures are written here rather than doctored out of the tree because
// every one of these breaches is hypothetical — the tree has none, which is the
// point of landing the pin now. What they exercise is the reader as much as the
// rule: a pin that quietly matched nothing would pass its own drift test only
// if the drifts were compared against a tree it had also read as empty.
func TestDarwinEntryPoints_PinCatchesTheSilentBreaches(t *testing.T) {
	t.Parallel()

	for _, drift := range darwinEntryPointDrifts() {
		problems, _ := darwinEntryPointProblems(drift.bridge)

		if len(problems) == 0 {
			t.Errorf(
				"the pin reports nothing when %s; that breach compiles, lints and "+
					"links, so nothing else would report it either",
				drift.name,
			)
		}
	}
}

// TestDarwinEntryPoints_PinLeavesFileLocalHelpersAlone pins the other half: the
// shapes that look like a breach and are not.
//
// A static function is file-local by construction, so a header declaring it
// would be the mistake. Getting this wrong is the way a guardrail like this one
// gets reverted rather than obeyed — six of the bridge's Neru* definitions are
// static today, and a pin that demanded prototypes for them would ask for six
// declarations that no caller could use.
func TestDarwinEntryPoints_PinLeavesFileLocalHelpersAlone(t *testing.T) {
	t.Parallel()

	for _, allowed := range darwinEntryPointNonBreaches() {
		problems, _ := darwinEntryPointProblems(allowed.bridge)

		if len(problems) > 0 {
			t.Errorf(
				"the pin reports %s, which breaks no rule:\n\t%s",
				allowed.name, strings.Join(problems, "\n\t"),
			)
		}
	}
}

// TestDarwinEntryPoints_SubsystemHeaderExemptionsStayHonest keeps
// darwinSubsystemHeaderExemptions from outliving its reasons. An entry whose .m
// or header is gone describes nothing; one the derivation now finds on its own
// is dead weight; and one whose header declares none of the file's entry points
// has stopped being true, which would leave a whole .m pinned to a header that
// publishes something else.
func TestDarwinEntryPoints_SubsystemHeaderExemptionsStayHonest(t *testing.T) {
	t.Parallel()

	bridge := readDarwinBridge(t)

	for sourceName, headerName := range darwinSubsystemHeaderExemptions {
		if _, present := bridge[sourceName]; !present {
			t.Errorf(
				"%s is exempt but the darwin bridge holds no such file; drop the entry",
				sourceName,
			)

			continue
		}

		if _, present := bridge[headerName]; !present {
			t.Errorf(
				"%s is exempt to %s, which the darwin bridge does not hold; name the "+
					"header its entry points are declared in",
				sourceName, headerName,
			)

			continue
		}

		if derived, found := bridge.derivedSubsystemHeader(sourceName); found {
			t.Errorf(
				"%s is exempt to %s, but its name now derives %s; drop the entry and "+
					"let the subsystem be derived",
				sourceName, headerName, derived,
			)
		}

		assertExemptHeaderStillDeclares(t, bridge, sourceName, headerName)
	}
}

// assertExemptHeaderStillDeclares fails when an exemption's header declares
// none of the entry points the exempt file defines.
func assertExemptHeaderStillDeclares(
	t *testing.T,
	bridge darwinBridge,
	sourceName, headerName string,
) {
	t.Helper()

	declared := bridge.declarations(headerName)

	for _, entryPoint := range bridge.entryPoints(sourceName) {
		if declared[entryPoint] {
			return
		}
	}

	t.Errorf(
		"%s is exempt to %s, which declares none of its entry points; the exemption "+
			"points a whole file at a header that publishes something else",
		sourceName, headerName,
	)
}

// darwinBridge is the darwin bridge's native source, keyed by file base name.
// The bridge is one flat directory, so the base name is what a .m and the
// header beside it have in common and what an exemption is written in terms of.
type darwinBridge map[string]string

// definesNativeSymbols reports whether a bridge file defines symbols rather
// than declaring them.
//
// The answer comes from compiledNativeExtensions (cgo_includes_test.go), which
// is already this package's answer to that question. Asking it once is what
// keeps a .c added to the bridge from being judged by one guardrail and skipped
// by the other.
func definesNativeSymbols(name string) bool {
	return compiledNativeExtensions[filepath.Ext(name)]
}

// readDarwinBridge reads the darwin bridge's definitions and headers out of the
// tree.
//
// Everything below the bridge directory is read, not only the directory itself:
// the bridge is one flat directory today, and a file parked in a subdirectory
// of it should be judged rather than quietly escape. That is also why two files
// sharing a base name are a failure — the map is keyed by base name, which is
// what a .m and the header beside it have in common, and silently keeping one
// of the two would leave the other unchecked.
func readDarwinBridge(t *testing.T) darwinBridge {
	t.Helper()

	bridge := darwinBridge{}

	walkRepoFiles(t, findRepoRoot(t), func(file repoFile) {
		if file.dir != darwinBridgeDir &&
			!strings.HasPrefix(file.dir, darwinBridgeDir+"/") {
			return
		}

		if !definesNativeSymbols(file.name) &&
			!strings.HasSuffix(file.name, nativeHeaderExt) {
			return
		}

		if _, taken := bridge[file.name]; taken {
			t.Fatalf(
				"the darwin bridge holds two files named %s; this pin pairs a "+
					"definition with its header by base name and cannot say which is which",
				file.name,
			)
		}

		bridge[file.name] = readNativeSource(t, file.rel)
	})

	assertWalkedAtLeast(
		t,
		"native files in the darwin bridge",
		len(bridge),
		darwinBridgeFileFloor,
	)

	return bridge
}

// darwinEntryPointProblems reports every entry point whose prototype is not in
// its subsystem's header, and how many entry points it judged. The count is
// what the floor is asserted on: a reader that stopped matching signatures
// would report no problems at all.
func darwinEntryPointProblems(bridge darwinBridge) ([]string, int) {
	var problems []string

	entryPoints := 0

	for _, sourceName := range slices.Sorted(maps.Keys(bridge)) {
		if !definesNativeSymbols(sourceName) {
			continue
		}

		defined := bridge.entryPoints(sourceName)
		entryPoints += len(defined)

		if len(defined) == 0 {
			continue
		}

		headerName, found := bridge.subsystemHeader(sourceName)
		if !found {
			problems = append(problems, fmt.Sprintf(
				"%s\tdefines %d entry point(s) (%s) and the bridge holds no header for "+
					"its subsystem; %s",
				bridgePath(sourceName), len(defined), strings.Join(defined, ", "),
				darwinEntryPointRule,
			))

			continue
		}

		declared := bridge.declarations(headerName)

		for _, entryPoint := range defined {
			if declared[entryPoint] {
				continue
			}

			problems = append(problems, fmt.Sprintf(
				"%s\tdefines %s and %s does not declare it; %s",
				bridgePath(sourceName), entryPoint, headerName, darwinEntryPointRule,
			))
		}
	}

	return problems, entryPoints
}

// bridgePath spells a bridge file the way every other failure in this package
// spells a file: relative to the repository root, so the output is clickable.
func bridgePath(name string) string {
	return darwinBridgeDir + "/" + name
}

// entryPoints returns the non-static Neru* functions the named file defines, in
// source order.
func (bridge darwinBridge) entryPoints(sourceName string) []string {
	var defined []string

	for _, signature := range nativeSignatures(bridge[sourceName]) {
		if signature.defines && !signature.static {
			defined = append(defined, signature.name)
		}
	}

	return defined
}

// declarations returns the Neru* functions the named header declares.
func (bridge darwinBridge) declarations(headerName string) map[string]bool {
	declared := map[string]bool{}

	for _, signature := range nativeSignatures(bridge[headerName]) {
		if !signature.defines {
			declared[signature.name] = true
		}
	}

	return declared
}

// subsystemHeader returns the header a .m publishes its entry points through.
func (bridge darwinBridge) subsystemHeader(sourceName string) (string, bool) {
	if exempt, isExempt := darwinSubsystemHeaderExemptions[sourceName]; isExempt {
		return exempt, true
	}

	return bridge.derivedSubsystemHeader(sourceName)
}

// derivedSubsystemHeader works a .m file's subsystem out of its name: the name
// without the platform token, with trailing underscore-separated words dropped
// until a header of that name is in the bridge.
//
// That is what "one header + _darwin.m pair per subsystem" means for a
// subsystem written as several files — accessibility_element_darwin.m and
// accessibility_window_darwin.m are both the accessibility subsystem — and it
// is why accessibility_visibility_darwin.m is its own: the longest name wins,
// so publishing a narrower header re-homes the file that matches it rather than
// leaving it pinned to the wider one.
func (bridge darwinBridge) derivedSubsystemHeader(sourceName string) (string, bool) {
	stem := strings.TrimSuffix(
		strings.TrimSuffix(sourceName, filepath.Ext(sourceName)),
		darwinSourceInfix,
	)

	for stem != "" {
		if _, found := bridge[stem+nativeHeaderExt]; found {
			return stem + nativeHeaderExt, true
		}

		cut := strings.LastIndex(stem, "_")
		if cut < 0 {
			break
		}

		stem = stem[:cut]
	}

	return "", false
}

// nativeSignature is one Neru* function a native source states: whether the
// parameter list is followed by a body or by a semicolon, and whether the
// declaration specifiers in front of it make it file-local.
type nativeSignature struct {
	name string

	// defines is true when a body follows, which is what tells a definition
	// from a prototype. A call written as a statement ends in a semicolon and
	// reads as a prototype here; that costs nothing, because only a header is
	// ever asked what it declares.
	defines bool

	// static is true for a file-local definition, the shape that is meant to
	// have no prototype at all.
	static bool
}

// nativeEntryPointOpening matches the start of a Neru* signature: the name and
// the parenthesis opening its parameter list.
//
// The leading word boundary is what keeps _NeruEnableCursorInBackground out:
// the naming contract is Neru* PascalCase, and a name that only contains those
// four letters is not an entry point.
var nativeEntryPointOpening = regexp.MustCompile(`\bNeru[A-Za-z0-9_]*[ \t\r\n]*\(`)

// nativeStaticSpecifier matches the keyword that makes a definition file-local.
var nativeStaticSpecifier = regexp.MustCompile(`\bstatic\b`)

// nativeSignatures reads every Neru* signature in a native source.
//
// This is not nativeRuleMethodBody (native_rule_test.go) wearing a different
// name. That one is handed the spelling of one definition and returns its body,
// so that a pin can run the rule written inside it; a rename there has to fail
// rather than pass over nothing, which is why it is addressed by name. This
// asks the opposite question — what does this file state, whose names nobody
// knows in advance — and has no body to read. Neither borrows the other's
// vocabulary: the comparison operators that file centralizes are what a rule is
// made of, and a signature has none.
//
// Reading text rather than preprocessed C, this cannot tell a declaration the
// compiler sees from one behind an inactive #if — but it can refuse to be
// fooled by the two shapes that are not signatures at all. Comments and string
// literals are removed first, so an old prototype left commented out declares
// nothing and a parenthesis inside a string cannot unbalance a parameter list;
// and what follows the parameter list decides what the signature is, so a call
// (followed by a `)` or a `,`) is neither a definition nor a declaration.
//
// What it does not read is a prototype carrying a trailing attribute —
// __attribute__, API_AVAILABLE, CF_RETURNS_RETAINED — between the parameter
// list and the semicolon. The bridge has none today, and the failure direction
// is the safe one: such a header would be reported as declaring nothing rather
// than pass silently.
func nativeSignatures(source string) []nativeSignature {
	readable := stripNativeCommentsAndLiterals(source)

	var signatures []nativeSignature

	for _, span := range nativeEntryPointOpening.FindAllStringIndex(readable, -1) {
		openParen := span[1] - 1

		closeParen := balancedParenEnd(readable, openParen)
		if closeParen < 0 {
			continue
		}

		rest := strings.TrimLeft(readable[closeParen+1:], " \t\r\n")

		defines := strings.HasPrefix(rest, "{")
		if !defines && !strings.HasPrefix(rest, ";") {
			continue
		}

		signatures = append(signatures, nativeSignature{
			name:    strings.TrimRight(readable[span[0]:openParen], " \t\r\n"),
			defines: defines,
			static:  hasStaticSpecifier(readable, span[0]),
		})
	}

	return signatures
}

// hasStaticSpecifier reports whether the declaration specifiers in front of the
// signature starting at nameStart include `static`.
//
// The specifiers are whatever sits between the end of the previous declaration
// or block and the name, which is how a definition is found without assuming
// the return type shares its line — a rewrap by clang-format must not change
// what this reads.
func hasStaticSpecifier(readable string, nameStart int) bool {
	boundary := strings.LastIndexAny(readable[:nameStart], ";{}")

	return nativeStaticSpecifier.MatchString(readable[boundary+1 : nameStart])
}

// balancedParenEnd returns the index of the parenthesis closing the one at
// openParen, or -1 when the source has none.
func balancedParenEnd(readable string, openParen int) int {
	depth := 0

	for index := openParen; index < len(readable); index++ {
		switch readable[index] {
		case '(':
			depth++
		case ')':
			depth--

			if depth == 0 {
				return index
			}
		}
	}

	return -1
}

// stripNativeCommentsAndLiterals empties the comments and the string and
// character literals of a C or Objective-C source, leaving everything else
// where it was.
//
// The comments are the removal that catches something: a prototype left
// commented out in a header must not count as a declaration, which is the same
// "an old copy left beside the new one" case callback_context_layout_test.go
// reports. Emptying the literals is defensive — a parenthesis inside one would
// otherwise be balanced against the parameter list of the call it sits in, and
// a signature scanner should not have to reason about which side of the
// resulting mismatch it errs on.
func stripNativeCommentsAndLiterals(source string) string {
	var stripped strings.Builder

	for index := 0; index < len(source); index++ {
		switch {
		case strings.HasPrefix(source[index:], "//"):
			index = skipPast(source, index, "\n") - 1
		case strings.HasPrefix(source[index:], "/*"):
			index = skipPast(source, index+2, "*/") - 1
		case source[index] == '"' || source[index] == '\'':
			stripped.WriteByte(source[index])
			index = skipLiteral(source, index)
			stripped.WriteByte(source[index])
		default:
			stripped.WriteByte(source[index])
		}
	}

	return stripped.String()
}

// skipPast returns the index just past the next occurrence of end at or after
// from, or the length of the source when there is none.
func skipPast(source string, from int, end string) int {
	offset := strings.Index(source[from:], end)
	if offset < 0 {
		return len(source)
	}

	return from + offset + len(end)
}

// skipLiteral returns the index of the quote closing the one at openQuote,
// honoring backslash escapes, or the last index of the source when the literal
// is never closed.
func skipLiteral(source string, openQuote int) int {
	for index := openQuote + 1; index < len(source); index++ {
		switch source[index] {
		case '\\':
			index++
		case source[openQuote]:
			return index
		}
	}

	return len(source) - 1
}

// darwinEntryPointFixture is a bridge built to exercise the pin, and what it is
// meant to show.
type darwinEntryPointFixture struct {
	name   string
	bridge darwinBridge
}

// The fixture bridge: one subsystem publishing one entry point, and a second
// subsystem beside it for a prototype to be misfiled into.
const (
	fixtureHeader      = "widget.h"
	fixtureSource      = "widget_darwin.m"
	fixtureOtherHeader = "gadget.h"

	fixtureDeclaration = "void NeruShowWidget(int id);\n"
	fixtureDefinition  = "#include \"widget.h\"\n\nvoid NeruShowWidget(int id) {\n" +
		"\tNeruLayOutWidget(id);\n}\n"
)

// compliantDarwinBridge is the fixture with nothing wrong with it, which every
// case below starts from.
func compliantDarwinBridge() darwinBridge {
	return darwinBridge{
		fixtureHeader:      fixtureDeclaration,
		fixtureSource:      fixtureDefinition,
		fixtureOtherHeader: "void NeruShowGadget(void);\n",
	}
}

// darwinEntryPointDrifts are the three ways a prototype ends up outside its
// subsystem's header, plus the file that arrives with no subsystem at all. Each
// compiles, lints and links.
func darwinEntryPointDrifts() []darwinEntryPointFixture {
	inTheCallingSource := compliantDarwinBridge()
	inTheCallingSource[fixtureHeader] = ""
	inTheCallingSource[fixtureSource] = fixtureDeclaration + fixtureDefinition

	inAnotherSubsystemsHeader := compliantDarwinBridge()
	inAnotherSubsystemsHeader[fixtureHeader] = ""
	inAnotherSubsystemsHeader[fixtureOtherHeader] += fixtureDeclaration

	inACgoPreamble := compliantDarwinBridge()
	inACgoPreamble[fixtureHeader] = ""

	commentedOutInItsHeader := compliantDarwinBridge()
	commentedOutInItsHeader[fixtureHeader] = "// " + fixtureDeclaration

	withNoSubsystemHeader := compliantDarwinBridge()
	delete(withNoSubsystemHeader, fixtureHeader)

	return []darwinEntryPointFixture{
		{
			name:   "the prototype is written at the top of the calling .m",
			bridge: inTheCallingSource,
		},
		{
			name:   "the prototype is written into another subsystem's header",
			bridge: inAnotherSubsystemsHeader,
		},
		{
			// A cgo preamble lives in a .go file, which is not the bridge's
			// interface and is not read here; from the header's side the breach
			// looks exactly like a prototype that went missing.
			name:   "the prototype is written into a cgo preamble instead",
			bridge: inACgoPreamble,
		},
		{
			// The one shape a reader of text has to be careful about: a header
			// that still shows the prototype declares nothing.
			name:   "the prototype is left commented out in its own header",
			bridge: commentedOutInItsHeader,
		},
		{
			name:   "the .m arrives with no subsystem header at all",
			bridge: withNoSubsystemHeader,
		},
	}
}

// darwinEntryPointNonBreaches are the shapes the pin must not report.
func darwinEntryPointNonBreaches() []darwinEntryPointFixture {
	withStaticHelper := compliantDarwinBridge()
	withStaticHelper[fixtureSource] += "\nstatic void NeruLayOutWidget(int id) {\n}\n"

	withWrappedStaticHelper := compliantDarwinBridge()
	withWrappedStaticHelper[fixtureSource] += "\nstatic void\nNeruLayOutWidget(\n\tint id\n) {\n}\n"

	withRepeatedPrototype := compliantDarwinBridge()
	withRepeatedPrototype[fixtureSource] = fixtureDeclaration + fixtureDefinition

	withCallsInItsBody := compliantDarwinBridge()
	withCallsInItsBody[fixtureSource] += "\nvoid NeruDrawWidget(void) {\n" +
		"\tNeruLayOutWidget(1);\n\tNeruLog(\"drew )( a widget\");\n}\n"
	withCallsInItsBody[fixtureHeader] += "void NeruDrawWidget(void);\n"

	return []darwinEntryPointFixture{
		{name: "a static helper with no prototype anywhere", bridge: withStaticHelper},
		{
			name:   "a static helper whose specifiers do not share its name's line",
			bridge: withWrappedStaticHelper,
		},
		{
			name:   "a prototype repeated in the .m as well as in its header",
			bridge: withRepeatedPrototype,
		},
		{
			name:   "calls in a body, one of them holding a parenthesis in a string",
			bridge: withCallsInItsBody,
		},
	}
}
