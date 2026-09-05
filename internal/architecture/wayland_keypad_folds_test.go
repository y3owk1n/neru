package architecture_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Where the keypad is named. The decision is made once, in C, and transcribed
// into Go three times: the X11 keysym cascade, the evdev fallback table, and
// the test tables mirroring both.
const (
	keypadFoldSource    = "internal/adapter/platform/linux/wayland_keymap.c"
	keypadEvdevSource   = "internal/adapter/eventtap/linux/evdev_keys.go"
	keypadEvdevTestFile = "internal/adapter/eventtap/linux/evdev_keys_test.go"
	keypadX11Source     = "internal/adapter/eventtap/linux/x11_cgo.go"
	keypadX11TestFile   = "internal/adapter/eventtap/linux/x11_keys_test.go"

	// keypadFoldFunc is the C function that decides what a keysym is called.
	keypadFoldFunc = "neru_normalize_xkb_name"

	// keypadFoldTable is the literal inside it that spells the decision.
	keypadFoldTable = "table[]"

	// keypadCharacterFunc is the C function that names a keysym by the
	// character it types before the fold table is consulted, and
	// keypadCharacterRule is the name this pin gives that step. It is what
	// makes KP_0 through KP_9, the keysyms the keypad reports with NumLock on,
	// reach a binding as the bare digit the X11 tap answers for the same keys.
	keypadCharacterFunc = "neru_xkb_keysym_name"
	keypadCharacterRule = "the character-first rule"

	// keypadEvdevTable is the fallback table the evdev tap and the global
	// hotkey reader name a raw key code through, and keypadEvdevCases is the
	// test table that mirrors its keypad half.
	keypadEvdevTable = "evdevKeyNames"
	keypadEvdevCases = "keypadKeyNameCases"

	// keypadEvdevSymbolPrefix is how the keypad key codes are spelled, and
	// therefore how this pin tells a keypad entry from the rest of the table.
	keypadEvdevSymbolPrefix = "evdevKeyKP"

	// The fields of one keypadKeyNameCases row this pin reads: which keysym the
	// key reports with NumLock off, the kernel code it sends, and the name the
	// test expects. The `name` field is the subtest's, and is not read.
	keypadCaseKeysymField = "keysym"
	keypadCaseCodeField   = "code"
	keypadCaseWantField   = "want"

	// keypadX11Func is the X11 keysym cascade, keypadX11FunctionFunc the half of
	// it the function keys were split into, and keypadX11LookupFunc the lookup
	// the test table calls.
	keypadX11Func         = "x11KeysymName"
	keypadX11FunctionFunc = "x11FunctionKeysymName"
	keypadX11LookupFunc   = "x11KeyFromLookup"

	// keypadX11TestFunc is the test that mirrors the keypad half of the
	// cascade, and keypadX11TestVar the table inside it.
	keypadX11TestFunc = "TestX11KeyFromLookupKeypadNavigationKeys"
	keypadX11TestVar  = "testCases"

	// The fields of one such row this pin reads: the call whose keysym argument
	// says which key it is, and the name it expects back.
	keypadX11TestGotField  = "got"
	keypadX11TestWantField = "want"

	// keypadKeysymPrefix marks the keysyms the keypad reports. It is what this
	// pin recognizes its own subject by, on both sides of the boundary.
	keypadKeysymPrefix = "KP_"

	// x11KeysymPrefix is what X11 spells the same keysyms with, in the cascade's
	// case labels and in the comments pinning the test file's protocol numbers.
	x11KeysymPrefix = "XK_"

	// keypadCodeBits is the width of an evdev key code, which is what the
	// fallback table is keyed by.
	keypadCodeBits = 16
)

// keypadFold is one keypad row of the fold table, and what each Go copy is
// expected to say about it.
//
// An absence is a claim that the copy cannot be reached for this keysym, not a
// license to leave an entry out: an empty reason means the copy must carry the
// name, and a reason means it must not. Both halves fail, so a gap that stops
// being a gap is caught the same way a missing entry is.
type keypadFold struct {
	// keysym is the xkb name the keypad reports for this key with NumLock off,
	// which is the key into the C table.
	keysym string

	// foldAbsent is why the C table carries no row for this keysym.
	foldAbsent string

	// evdevAbsent is why the evdev fallback names no key for this keysym.
	evdevAbsent string

	// x11Absent is why the X11 keysym cascade has no case for it.
	x11Absent string
}

// The reasons a keypad keysym reaches no table row, or no Go copy.
const (
	// characterNamesTheKeysym covers the keypad operators and KP_Decimal on
	// the Wayland side: they type a character, so neru_xkb_keysym_name names
	// them by it and the fold table is never consulted. A row for one of them
	// would be dead, and this pin fails on it rather than holding the Go copies
	// to a name the resolver never produces.
	characterNamesTheKeysym = "the character-first rule names this keysym by the character " +
		"it types, so it never reaches the fold table"

	// xlookupAnswersWithACharacter covers the keysyms Xlib's XLookupString
	// resolves to a character — the keypad operators and KP_Decimal, which the
	// keypad reports only with NumLock on. Those reach x11KeyFromLookup through
	// its character branch, so the keysym cascade deliberately has no case for
	// them.
	xlookupAnswersWithACharacter = "XLookupString resolves this keysym to a character, " +
		"so it reaches x11KeyFromLookup through the character branch " +
		"and never through the keysym cascade"

	// xlookupAnswersKeypadEnterLikeReturn covers KP_Enter, which Xlib resolves
	// to the same carriage return it resolves XK_Return to. The keypad key
	// therefore reaches x11KeyFromLookup the way the main Return key does,
	// through the character branch — and what that branch answers with is the
	// main key's question, not the keypad's, so this pin holds neither.
	xlookupAnswersKeypadEnterLikeReturn = "Xlib resolves this keysym to the same character " +
		"it resolves XK_Return to, so the keypad key reaches x11KeyFromLookup " +
		"through the character branch the main Return key does"

	// numLockOnKeysym covers KP_Decimal, which is the same physical key as
	// KP_Delete with NumLock the other way round. The evdev fallback has no
	// keymap and answers for NumLock off only, so it names that key once.
	numLockOnKeysym = "KP_Decimal is the NumLock-on keysym of the key the evdev fallback " +
		"names through KP_Delete, and that table answers for NumLock off only"
)

// keypadFolds is every keypad row of neru_normalize_xkb_name's table, with the
// Go copies each one is expected to reach.
//
// This list is the pin's own claim about the shape of the table, and it is
// checked both ways: a KP_ row the table grows that is not here fails, and a
// row here the table no longer carries fails. The names themselves are never
// written down here — they are read out of the C source, which is the whole
// point.
//
// The keypad operators and KP_Decimal are excused from the table itself: they
// type a character, and the character-first rule names them before the table
// is read, on both Linux backends. What this pin buys for those five is that
// the table does not grow a dead row for them; the name they reach a binding
// as is libxkbcommon's answer, which no source in the tree spells and so
// nothing here can hold a copy to. KP_Decimal is excused from the evdev copy
// besides, and saying so is better than implying a check that is not there.
func keypadFolds() []keypadFold {
	return []keypadFold{
		{keysym: "KP_Home"},
		{keysym: "KP_End"},
		{keysym: "KP_Prior"},
		{keysym: "KP_Next"},
		{keysym: "KP_Up"},
		{keysym: "KP_Down"},
		{keysym: "KP_Left"},
		{keysym: "KP_Right"},
		{keysym: "KP_Insert"},
		{keysym: "KP_Delete"},
		{keysym: "KP_Begin"},
		{
			keysym:     "KP_Add",
			foldAbsent: characterNamesTheKeysym,
			x11Absent:  xlookupAnswersWithACharacter,
		},
		{
			keysym:     "KP_Subtract",
			foldAbsent: characterNamesTheKeysym,
			x11Absent:  xlookupAnswersWithACharacter,
		},
		{
			keysym:     "KP_Multiply",
			foldAbsent: characterNamesTheKeysym,
			x11Absent:  xlookupAnswersWithACharacter,
		},
		{
			keysym:     "KP_Divide",
			foldAbsent: characterNamesTheKeysym,
			x11Absent:  xlookupAnswersWithACharacter,
		},
		{keysym: "KP_Enter", x11Absent: xlookupAnswersKeypadEnterLikeReturn},
		{
			keysym:      "KP_Decimal",
			foldAbsent:  characterNamesTheKeysym,
			evdevAbsent: numLockOnKeysym,
			x11Absent:   xlookupAnswersWithACharacter,
		},
	}
}

// TestWaylandKeypadFoldTableIsTheTableThisPinDeclares keeps the inventory above
// honest against the C source it describes.
//
// Everything below reads the names out of neru_normalize_xkb_name, so a row it
// never looks at is a keypad key nothing holds together. A row the table gains
// therefore fails here until it is declared, and a declaration the table no
// longer carries fails too.
//
// The whole table is parsed and only its keypad rows are pinned, which is a
// boundary rather than an oversight. The rest of it renames ISO_Left_Tab,
// Prior and Next, and Caps_Lock, and each would need a join this one does not
// have: evdevKeyNames answers those by key code, and only for the keypad does
// the tree say which keysym a code reports — that is what keypadKeyNameCases
// is. Prior and Next could be held to the X11 cascade
// alone, since that side is keyed by keysym, and pinning one copy of two under
// a heading that says "the keypad" is how a pin comes to claim more than it
// holds. A row outside the keypad this pin cannot read is still reported,
// because a row it skipped could be the keypad row it does claim.
func TestWaylandKeypadFoldTableIsTheTableThisPinDeclares(t *testing.T) {
	t.Parallel()

	for _, problem := range keypadFoldTableProblems(readKeypadCopies(t)) {
		t.Error(problem)
	}
}

// TestEvdevKeypadNamesArePinnedToTheWaylandFoldTable keeps the evdev fallback
// spelling the keypad the way the Wayland keymap does.
//
// evdevKeyNames is the table the evdev tap drops to when xkb state creation
// fails, and the only table the global hotkey reader has. It names a raw kernel
// key code without a keymap, so every keypad name in it was copied from
// neru_normalize_xkb_name — and nothing kept it copied. A name that drifts is
// one physical key reaching one binding under Wayland and another under evdev.
//
// keypadKeyNameCases is checked as the third copy it is: the test table carries
// its own `want` column, so a name changed in both the table and its test would
// otherwise pass the suite that owns them.
func TestEvdevKeypadNamesArePinnedToTheWaylandFoldTable(t *testing.T) {
	t.Parallel()

	for _, problem := range keypadEvdevProblems(readKeypadCopies(t)) {
		t.Error(problem)
	}
}

// TestX11KeypadNamesArePinnedToTheWaylandFoldTable keeps the X11 tap spelling
// the keypad the way the Wayland keymap does.
//
// With NumLock off the keypad reports keysyms of its own, and XLookupString
// yields no character for the navigation half of them — so x11KeysymName is the
// only path by which those keys reach the mode handler, and every name in it
// was copied from neru_normalize_xkb_name. The keysyms it deliberately has no
// case for are declared in keypadFolds() with the reason, and a case appearing
// for one of them fails here too: an excuse that stopped being true is how a
// pin comes to cover less than it claims.
func TestX11KeypadNamesArePinnedToTheWaylandFoldTable(t *testing.T) {
	t.Parallel()

	for _, problem := range keypadX11Problems(readKeypadCopies(t)) {
		t.Error(problem)
	}
}

// TestWaylandKeypadFoldPinCatchesDrift keeps the pins above from passing over
// copies that have moved.
//
// A pin over four hand-maintained tables is only worth its line count if it can
// tell a drifted one from the declaration, and the ways these drift are
// specific: a name retyped on one side of the language boundary, an entry left
// out of the copy that was edited second, a test row that agrees with the copy
// it mirrors and with nothing else, an excused keysym quietly growing an entry.
// So each is applied to the tables this pin actually read, and the mutant has
// to fail at least one check. Mutating what was parsed rather than the source
// text keeps this honest across a reformat of any of the five files.
func TestWaylandKeypadFoldPinCatchesDrift(t *testing.T) {
	t.Parallel()

	copies := readKeypadCopies(t)

	for _, drift := range keypadDrifts(copies) {
		if keypadProblems(drift.apply(copies)) == nil {
			t.Errorf(
				"no check tells %s apart from the copies in the tree: %s would pass the pin",
				drift.name, drift.where,
			)
		}
	}
}

// TestWaylandKeypadFoldPinReportsACopyItCannotRead pins the other half of the
// guardrail: a table this pin cannot read must be reported, never skipped. A
// pin that reads nothing and passes is worse than no pin, because it reads as
// coverage.
//
// The cost is that the pin reads one spelling of each copy. Rewriting any of
// them in an equivalent shape — a name built at runtime, a keysym reached
// through a variable, a table renamed — fails here rather than being
// understood, and the failure names the shape it expected, which leaves the
// next author a one-line change to this file alongside the one they are already
// making.
func TestWaylandKeypadFoldPinReportsACopyItCannotRead(t *testing.T) {
	t.Parallel()

	for _, unreadable := range keypadUnreadableSources(t, readKeypadSources(t)) {
		if _, problem := parseKeypadCopies(unreadable.sources); problem == "" {
			t.Errorf(
				"parsing accepted sources with %s; the pin would then hold a copy it never read",
				unreadable.name,
			)
		}
	}
}

// keypadProblems is every check the pins above make. The drift test runs it
// whole, because which check catches a given drift is not the point — that one
// of them does is.
func keypadProblems(copies keypadCopies) []string {
	problems := keypadFoldTableProblems(copies)
	problems = append(problems, keypadEvdevProblems(copies)...)

	return append(problems, keypadX11Problems(copies)...)
}

// keypadDrift is one way the copies could plausibly move apart.
type keypadDrift struct {
	name  string
	where string
	apply func(keypadCopies) keypadCopies
}

// keypadDrifts are the drifts TestWaylandKeypadFoldPinCatchesDrift requires the
// pin to catch, written against the copies read out of the tree so a drift can
// borrow a real key code rather than invent one.
func keypadDrifts(tree keypadCopies) []keypadDrift {
	drifts := []keypadDrift{
		{
			name:  "a keypad name retyped in the C table",
			where: keypadFoldSource,
			apply: keypadDriftIn(func(copies *keypadCopies) { copies.folds["KP_Home"] = "HOME" }),
		},
		{
			name:  "a keypad row dropped from the C table",
			where: keypadFoldSource,
			apply: keypadDriftIn(func(copies *keypadCopies) { delete(copies.folds, "KP_Left") }),
		},
		{
			name:  "a keypad row added to the C table",
			where: keypadFoldSource,
			apply: keypadDriftIn(func(copies *keypadCopies) { copies.folds["KP_Separator"] = "," }),
		},
		{
			name:  "a dead row for a character-named keypad key",
			where: keypadFoldSource,
			apply: keypadDriftIn(func(copies *keypadCopies) { copies.folds["KP_Add"] = "+" }),
		},
		{
			name:  "the character-first rule gone",
			where: keypadFoldSource,
			apply: keypadDriftIn(func(copies *keypadCopies) { copies.namesByCharacter = false }),
		},
	}

	drifts = append(drifts, keypadEvdevDrifts(tree)...)

	return append(drifts, keypadX11Drifts()...)
}

// keypadEvdevDrifts are the drifts on the evdev side: the fallback table, and
// the test table mirroring it.
func keypadEvdevDrifts(tree keypadCopies) []keypadDrift {
	upCode := tree.evdevCases["KP_Up"].code

	return []keypadDrift{
		{
			name:  "an evdev entry dropped",
			where: keypadEvdevTable,
			apply: keypadDriftIn(func(copies *keypadCopies) {
				copies.evdevEntries = slices.DeleteFunc(
					copies.evdevEntries,
					func(entry keypadEvdevEntry) bool {
						return entry.symbol == keypadEvdevSymbolPrefix+"7"
					},
				)
			}),
		},
		{
			name:  "an evdev entry retyped",
			where: keypadEvdevTable,
			apply: keypadDriftIn(func(copies *keypadCopies) {
				for index, entry := range copies.evdevEntries {
					if entry.symbol == keypadEvdevSymbolPrefix+"4" {
						copies.evdevEntries[index].name = "LEFT"
					}
				}
			}),
		},
		{
			name:  "an evdev case dropped",
			where: keypadEvdevCases,
			apply: keypadDriftIn(func(copies *keypadCopies) {
				delete(copies.evdevCases, "KP_Insert")
			}),
		},
		{
			name:  "an evdev case expecting a name the fold table does not give",
			where: keypadEvdevCases,
			apply: keypadDriftIn(func(copies *keypadCopies) {
				pinned := copies.evdevCases["KP_Delete"]
				pinned.want = "Backspace"
				copies.evdevCases["KP_Delete"] = pinned
			}),
		},
		{
			name:  "an evdev case pointing at another key's code",
			where: keypadEvdevCases,
			apply: keypadDriftIn(func(copies *keypadCopies) {
				pinned := copies.evdevCases["KP_Home"]
				pinned.code = upCode
				copies.evdevCases["KP_Home"] = pinned
			}),
		},
		{
			name:  "an evdev case for a keysym keypadFolds() excuses the fallback from",
			where: keypadEvdevCases,
			apply: keypadDriftIn(func(copies *keypadCopies) {
				copies.evdevCases["KP_Decimal"] = keypadEvdevCase{code: upCode, want: "."}
			}),
		},
		{
			name:  "an evdev case for a keysym the fold table does not carry",
			where: keypadEvdevCases,
			apply: keypadDriftIn(func(copies *keypadCopies) {
				copies.evdevCases["KP_Separator"] = keypadEvdevCase{code: upCode, want: ","}
			}),
		},
	}
}

// keypadX11Drifts are the drifts on the X11 side: the keysym cascade, and the
// test table mirroring it.
func keypadX11Drifts() []keypadDrift {
	return []keypadDrift{
		{
			name:  "an X11 case dropped",
			where: keypadX11Func,
			apply: keypadDriftIn(func(copies *keypadCopies) { delete(copies.x11Names, "KP_Up") }),
		},
		{
			name:  "an X11 case retyped",
			where: keypadX11Func,
			apply: keypadDriftIn(func(copies *keypadCopies) { copies.x11Names["KP_End"] = "END" }),
		},
		{
			name:  "an X11 case for a keysym keypadFolds() excuses",
			where: keypadX11Func,
			apply: keypadDriftIn(func(copies *keypadCopies) {
				copies.x11Names["KP_Enter"] = "Return"
			}),
		},
		{
			name:  "an X11 test row dropped",
			where: keypadX11TestFunc,
			apply: keypadDriftIn(
				func(copies *keypadCopies) { delete(copies.x11Cases, "KP_Begin") },
			),
		},
		{
			name:  "an X11 test row retyped",
			where: keypadX11TestFunc,
			apply: keypadDriftIn(func(copies *keypadCopies) {
				copies.x11Cases["KP_Insert"] = "INSERT"
			}),
		},
		{
			name:  "an X11 test row for a keysym keypadFolds() excuses",
			where: keypadX11TestFunc,
			apply: keypadDriftIn(func(copies *keypadCopies) { copies.x11Cases["KP_Divide"] = "/" }),
		},
	}
}

// keypadDriftIn clones first, so one drift never leaks into the next.
func keypadDriftIn(mutate func(*keypadCopies)) func(keypadCopies) keypadCopies {
	return func(copies keypadCopies) keypadCopies {
		drifted := copies.clone()
		mutate(&drifted)

		return drifted
	}
}

// keypadDoctored rewrites one of the sources, leaving the rest as the tree
// wrote them, and fails when the text it rewrites is no longer there: a rewrite
// that missed would leave the tree's own source in place, which parses fine —
// and the test would report the pin as strict when it had tested nothing.
func keypadDoctored(
	t *testing.T,
	tree keypadSources,
	path, from, into string,
) keypadSources {
	t.Helper()

	doctored := maps.Clone(tree)
	doctored[path] = mustRewrite(t, tree[path], from, into)

	return doctored
}

// keypadUnreadableSource is one rewrite of a source that this pin must report
// rather than read past.
type keypadUnreadableSource struct {
	name    string
	sources keypadSources
}

// keypadUnreadableSources doctors each copy into an equivalent shape this pin
// does not read.
func keypadUnreadableSources(t *testing.T, tree keypadSources) []keypadUnreadableSource {
	t.Helper()

	unreadable := []keypadUnreadableSource{
		{
			name: "the fold function renamed",
			sources: keypadDoctored(t, tree, keypadFoldSource,
				"static void "+keypadFoldFunc+"(char *buf, size_t buf_size) {",
				"static void neru_canonical_xkb_name(char *buf, size_t buf_size) {",
			),
		},
		{
			name:    "the fold table renamed",
			sources: keypadDoctored(t, tree, keypadFoldSource, "} table[] = {", "} folds[] = {"),
		},
		{
			name: "the character-first resolver renamed",
			sources: keypadDoctored(t, tree, keypadFoldSource,
				"int "+keypadCharacterFunc+"(uint32_t keysym, char *buf, size_t buf_size) {",
				"int neru_xkb_keysym_spell(uint32_t keysym, char *buf, size_t buf_size) {",
			),
		},
		{
			name: "a fold row whose name is built at runtime",
			sources: keypadDoctored(
				t,
				tree,
				keypadFoldSource,
				`{"KP_Enter", "Return"},`,
				`{"KP_Enter", canonical_return},`,
			),
		},
	}

	unreadable = append(unreadable, keypadUnreadableEvdevSources(t, tree)...)

	return append(unreadable, keypadUnreadableX11Sources(t, tree)...)
}

// keypadUnreadableEvdevSources doctors the evdev fallback table and the test
// table mirroring it.
func keypadUnreadableEvdevSources(t *testing.T, tree keypadSources) []keypadUnreadableSource {
	t.Helper()

	return []keypadUnreadableSource{
		{
			name: "the fallback table renamed",
			sources: keypadDoctored(t, tree, keypadEvdevSource,
				"var "+keypadEvdevTable+" = map[uint16]string{",
				"var evdevNames = map[uint16]string{",
			),
		},
		{
			name: "a fallback entry answering with a name built at runtime",
			sources: keypadDoctored(t, tree, keypadEvdevSource,
				"evdevKeyKP7:        evdevKeyNameHome,",
				"evdevKeyKP7:        homeName(),",
			),
		},
		{
			name: "a fallback entry keyed by something other than a key-code constant",
			sources: keypadDoctored(t, tree, keypadEvdevSource,
				"evdevKeyKP0:        evdevKeyNameInsert,",
				"evdevKeyKPZero:     evdevKeyNameInsert,",
			),
		},
		{
			name: "the mirror table renamed",
			sources: keypadDoctored(t, tree, keypadEvdevTestFile,
				"var "+keypadEvdevCases+" = []struct {",
				"var keypadCases = []struct {",
			),
		},
		{
			name: "a mirror row whose keysym is built at runtime",
			sources: keypadDoctored(t, tree, keypadEvdevTestFile,
				`{name: "KEY_KP7", keysym: "KP_Home", code: 71, want: evdevKeyNameHome},`,
				`{name: "KEY_KP7", keysym: keysymOf(71), code: 71, want: evdevKeyNameHome},`,
			),
		},
	}
}

// keypadUnreadableX11Sources doctors the X11 keysym cascade and the test table
// mirroring it.
func keypadUnreadableX11Sources(t *testing.T, tree keypadSources) []keypadUnreadableSource {
	t.Helper()

	return []keypadUnreadableSource{
		{
			name: "the keysym cascade renamed",
			sources: keypadDoctored(t, tree, keypadX11Source,
				"func "+keypadX11Func+"(keysym C.KeySym) string {",
				"func x11NameForKeysym(keysym C.KeySym) string {",
			),
		},
		{
			name: "a cascade arm returning a name built at runtime",
			sources: keypadDoctored(
				t,
				tree,
				keypadX11Source,
				"\t\treturn evdevKeyNameHome",
				"\t\treturn homeName()",
			),
		},
		{
			name: "a cascade arm reached by something other than a keysym constant",
			sources: keypadDoctored(
				t,
				tree,
				keypadX11Source,
				"\tcase C.XK_KP_Delete:",
				"\tcase keysymKPDelete:",
			),
		},
		{
			name: "the mirror test renamed",
			sources: keypadDoctored(t, tree, keypadX11TestFile,
				"func "+keypadX11TestFunc+"(t *testing.T) {",
				"func TestX11KeypadNames(t *testing.T) {",
			),
		},
		{
			name: "a mirror row whose keysym constant says which keysym it is nowhere",
			sources: keypadDoctored(
				t,
				tree,
				keypadX11TestFile,
				"= 0xFF95 // XK_KP_Home",
				"= 0xFF95",
			),
		},
		{
			name: "a mirror row calling something other than the lookup",
			sources: keypadDoctored(t, tree, keypadX11TestFile,
				"got: "+keypadX11LookupFunc+"(0, nil, x11KeysymKPEnd)",
				"got: x11Name(0, nil, x11KeysymKPEnd)",
			),
		},
	}
}

// keypadCopies is what the pin read out of the sources. Nothing here is an
// expectation: the C table is the one declaration, and every other field is a
// transcription of it read back out of the file that carries it.
type keypadCopies struct {
	// folds maps each xkb keysym name the C table rewrites to the name it
	// rewrites it to. It carries the whole table, keypad rows and all.
	folds map[string]string

	// namesByCharacter records whether the resolver still names a keysym by
	// the character it types before it reads the table.
	namesByCharacter bool

	// evdevEntries is the keypad half of evdevKeyNames, in the order the table
	// writes it.
	evdevEntries []keypadEvdevEntry

	// evdevCases is keypadKeyNameCases, keyed by the keysym each case records
	// the key as reporting with NumLock off.
	evdevCases map[string]keypadEvdevCase

	// x11Names is the X11 keysym cascade: the name it returns for each keysym
	// it has a case for, keyed by keysym without the XK_ prefix.
	x11Names map[string]string

	// x11Cases is the keypad table in x11_keys_test.go, keyed the same way and
	// valued by the name that test expects.
	x11Cases map[string]string
}

// keypadEvdevEntry is one keypad entry of evdevKeyNames: the constant it is
// written with, the key code that constant carries, and the name it answers
// with. The code is what joins it to the test table, which is written against
// the kernel numbers rather than against the constants.
type keypadEvdevEntry struct {
	symbol string
	code   uint16
	name   string
}

// keypadEvdevCase is one row of keypadKeyNameCases.
type keypadEvdevCase struct {
	// code is the kernel key code the case checks, written as a literal there.
	code uint16

	// want is the name it expects, which is the copy this pin holds.
	want string
}

// clone copies every table, so a drift applied to one is not seen by the next.
func (copies keypadCopies) clone() keypadCopies {
	return keypadCopies{
		folds:            maps.Clone(copies.folds),
		namesByCharacter: copies.namesByCharacter,
		evdevEntries:     slices.Clone(copies.evdevEntries),
		evdevCases:       maps.Clone(copies.evdevCases),
		x11Names:         maps.Clone(copies.x11Names),
		x11Cases:         maps.Clone(copies.x11Cases),
	}
}

// keypadDeclaredKeysyms is the set keypadFolds() declares, which every check
// below asks the same question of: is this keysym one this pin knows about?
func keypadDeclaredKeysyms() map[string]bool {
	declared := make(map[string]bool)

	for _, fold := range keypadFolds() {
		declared[fold.keysym] = true
	}

	return declared
}

// keypadX11Problems holds the X11 keysym cascade and the test table that
// mirrors it to the C fold table.
func keypadX11Problems(copies keypadCopies) []string {
	var problems []string

	for _, fold := range keypadFolds() {
		canonical, folded := copies.folds[fold.keysym]
		if !folded && fold.foldAbsent == "" {
			continue
		}

		problems = append(problems, keypadX11CopyProblems(
			keypadX11Site{
				source: keypadX11Source,
				what:   keypadX11Func,
				names:  copies.x11Names,
				advice: "add a case, or declare in keypadFolds() why the X11 tap " +
					"cannot reach that keysym",
			},
			fold, canonical,
		)...)
		problems = append(problems, keypadX11CopyProblems(
			keypadX11Site{
				source: keypadX11TestFile,
				what:   keypadX11TestFunc,
				names:  copies.x11Cases,
				advice: "add a row, so the name the cascade returns is read back " +
					"by the test that owns it",
			},
			fold, canonical,
		)...)
	}

	declared := keypadDeclaredKeysyms()

	problems = append(
		problems,
		keypadX11UndeclaredProblems(keypadX11Source, keypadX11Func, copies.x11Names, declared)...,
	)

	return append(
		problems,
		keypadX11UndeclaredProblems(
			keypadX11TestFile, keypadX11TestFunc, copies.x11Cases, declared,
		)...,
	)
}

// keypadX11Site is one of the two places the X11 side names a keysym: the
// cascade itself, and the test table mirroring it. They are checked the same
// way, and both fail the same two ways.
type keypadX11Site struct {
	source string
	what   string
	names  map[string]string

	// advice is what a missing name means at this site, since a cascade with no
	// case and a test with no row are fixed differently.
	advice string
}

// keypadX11CopyProblems checks one keypad key at one X11 site.
func keypadX11CopyProblems(site keypadX11Site, fold keypadFold, canonical string) []string {
	name, named := site.names[fold.keysym]

	if fold.x11Absent != "" {
		if !named {
			return nil
		}

		return []string{fmt.Sprintf(
			"%s: %s names %s %q, which keypadFolds() excuses it from (%s); "+
				"if this keysym reaches the cascade now, drop the excuse",
			site.source, site.what, fold.keysym, name, fold.x11Absent,
		)}
	}

	if !named {
		return []string{fmt.Sprintf(
			"%s: %s names %s nothing, which %s folds to %q; %s",
			site.source, site.what, fold.keysym, keypadFoldSource, canonical, site.advice,
		)}
	}

	if name != canonical {
		return []string{fmt.Sprintf(
			"%s: %s names %s %q; %s folds it to %q, "+
				"so one physical key would reach two bindings",
			site.source, site.what, fold.keysym, name, keypadFoldSource, canonical,
		)}
	}

	return nil
}

// keypadX11UndeclaredProblems catches a keypad keysym named on the X11 side
// that keypadFolds() knows nothing about, which is a name held to nothing.
func keypadX11UndeclaredProblems(
	source, what string,
	names map[string]string,
	declared map[string]bool,
) []string {
	var problems []string

	for _, keysym := range slices.Sorted(maps.Keys(names)) {
		if !strings.HasPrefix(keysym, keypadKeysymPrefix) || declared[keysym] {
			continue
		}

		problems = append(problems, fmt.Sprintf(
			"%s: %s names %s %q, which keypadFolds() does not declare; "+
				"add it there, so it is held to %s",
			source, what, keysym, names[keysym], keypadFoldSource,
		))
	}

	return problems
}

// keypadEvdevProblems holds evdevKeyNames and the test table that mirrors it to
// the C fold table.
func keypadEvdevProblems(copies keypadCopies) []string {
	var problems []string

	pinnedCodes := make(map[uint16]bool)

	for _, fold := range keypadFolds() {
		testCase, pinned := copies.evdevCases[fold.keysym]
		if pinned {
			pinnedCodes[testCase.code] = true
		}

		if fold.evdevAbsent != "" {
			if pinned {
				problems = append(problems, fmt.Sprintf(
					"%s: %s pins %s, which keypadFolds() excuses the evdev fallback from (%s); "+
						"if the fallback names that key now, drop the excuse",
					keypadEvdevTestFile, keypadEvdevCases, fold.keysym, fold.evdevAbsent,
				))
			}

			continue
		}

		problems = append(problems, keypadEvdevFoldProblems(copies, fold, testCase, pinned)...)
	}

	problems = append(problems, keypadEvdevCoverageProblems(copies, pinnedCodes)...)

	return problems
}

// keypadEvdevFoldProblems checks one keypad key the evdev fallback is expected
// to name: the test table's column, and the entry it stands for.
func keypadEvdevFoldProblems(
	copies keypadCopies,
	fold keypadFold,
	testCase keypadEvdevCase,
	pinned bool,
) []string {
	canonical, folded := copies.folds[fold.keysym]
	if !folded {
		return nil
	}

	if !pinned {
		return []string{fmt.Sprintf(
			"%s: %s has no case for %s, which %s folds to %q; "+
				"the evdev name for that key is pinned by nothing",
			keypadEvdevTestFile, keypadEvdevCases, fold.keysym, keypadFoldSource, canonical,
		)}
	}

	var problems []string

	if testCase.want != canonical {
		problems = append(problems, fmt.Sprintf(
			"%s: %s expects %q for %s; %s folds it to %q",
			keypadEvdevTestFile, keypadEvdevCases, testCase.want,
			fold.keysym, keypadFoldSource, canonical,
		))
	}

	entry, answered := keypadEvdevEntryFor(copies.evdevEntries, testCase.code)
	if !answered {
		return append(problems, fmt.Sprintf(
			"%s: %s answers nothing for the code %d that %s gives %s, "+
				"so that keypad key reaches no binding",
			keypadEvdevSource, keypadEvdevTable, testCase.code,
			keypadEvdevCases, fold.keysym,
		))
	}

	if entry.name != canonical {
		problems = append(problems, fmt.Sprintf(
			"%s: %s answers %s with %q; %s folds %s to %q, "+
				"so one physical key would reach two bindings",
			keypadEvdevSource, keypadEvdevTable, entry.symbol, entry.name,
			keypadFoldSource, fold.keysym, canonical,
		))
	}

	return problems
}

// keypadEvdevCoverageProblems is the half a per-fold check cannot see: a case
// written against a keysym the fold table does not carry, and a keypad entry no
// case reaches.
func keypadEvdevCoverageProblems(copies keypadCopies, pinnedCodes map[uint16]bool) []string {
	var problems []string

	declared := keypadDeclaredKeysyms()

	for _, keysym := range slices.Sorted(maps.Keys(copies.evdevCases)) {
		if !declared[keysym] {
			problems = append(problems, fmt.Sprintf(
				"%s: %s pins %s, which keypadFolds() does not declare; "+
					"a keysym this pin does not know is one it cannot hold to %s",
				keypadEvdevTestFile, keypadEvdevCases, keysym, keypadFoldSource,
			))
		}
	}

	for _, entry := range copies.evdevEntries {
		if pinnedCodes[entry.code] {
			continue
		}

		problems = append(problems, fmt.Sprintf(
			"%s: %s answers %s with %q and no case in %s pins it; "+
				"a keypad name nothing reads back is one this pin cannot hold to %s",
			keypadEvdevSource, keypadEvdevTable, entry.symbol, entry.name,
			keypadEvdevCases, keypadFoldSource,
		))
	}

	return problems
}

// keypadEvdevEntryFor finds the entry a key code is answered by.
func keypadEvdevEntryFor(entries []keypadEvdevEntry, code uint16) (keypadEvdevEntry, bool) {
	for _, entry := range entries {
		if entry.code == code {
			return entry, true
		}
	}

	return keypadEvdevEntry{}, false
}

// keypadFoldTableProblems holds the declared inventory to the C table.
func keypadFoldTableProblems(copies keypadCopies) []string {
	var problems []string

	declared := keypadDeclaredKeysyms()

	for _, fold := range keypadFolds() {
		_, folded := copies.folds[fold.keysym]

		switch {
		case fold.foldAbsent != "" && folded:
			problems = append(problems, fmt.Sprintf(
				"%s: %s has a row for %q, which keypadFolds() excuses it from (%s); "+
					"that row never runs, so drop it or the excuse",
				keypadFoldSource, keypadFoldTable, fold.keysym, fold.foldAbsent,
			))
		case fold.foldAbsent == "" && !folded:
			problems = append(problems, fmt.Sprintf(
				"%s: %s has no row for %q, which keypadFolds() declares; "+
					"a keypad key the table stopped folding is one the Go copies still name",
				keypadFoldSource, keypadFoldTable, fold.keysym,
			))
		}
	}

	for _, keysym := range slices.Sorted(maps.Keys(copies.folds)) {
		if strings.HasPrefix(keysym, keypadKeysymPrefix) && !declared[keysym] {
			problems = append(problems, fmt.Sprintf(
				"%s: %s folds %q to %q, which keypadFolds() does not declare; "+
					"add it there with the Go copies it has to reach",
				keypadFoldSource, keypadFoldTable, keysym, copies.folds[keysym],
			))
		}
	}

	if !copies.namesByCharacter {
		problems = append(problems, fmt.Sprintf(
			"%s: %s no longer names a keysym by the character it types ahead of "+
				"the table, or is written in a shape this pin does not read; KP_0 "+
				"through KP_9 are the keysyms the keypad reports with NumLock on, and "+
				"the bare digit is what the X11 tap answers for the same keys",
			keypadFoldSource, keypadCharacterRule,
		))
	}

	return problems
}

// The shapes the C fold table is written in. Addressing them by name means a
// rename surfaces as a failure rather than as a silent pass over a table that
// is no longer there.
var (
	keypadFoldFuncPattern = regexp.MustCompile(
		`(?m)^static void ` + keypadFoldFunc + `\(char \*buf, size_t buf_size\)[ \t]*\{`,
	)
	keypadFoldTableOpeningPattern = regexp.MustCompile(
		`(?m)^[ \t]*\}[ \t]*` + regexp.QuoteMeta(keypadFoldTable) + `[ \t]*=[ \t]*\{`,
	)
	keypadFoldTableClosingPattern = regexp.MustCompile(`(?m)^[ \t]*\};`)
	keypadFoldRowPattern          = regexp.MustCompile(
		`^\{("(?:[^"\\]|\\.)*"),[ \t]*("(?:[^"\\]|\\.)*")\},$`,
	)
)

// keypadCharacterFuncPattern finds the resolver the character-first rule lives
// in, by the one signature this pin reads.
var keypadCharacterFuncPattern = regexp.MustCompile(
	`(?m)^int ` + keypadCharacterFunc + `\(uint32_t keysym, char \*buf, size_t buf_size\)[ \t]*\{`,
)

// keypadCharacterRuleFragments are the parts of the character-first rule this
// pin reads, in the order the C source writes them. Together they say: ask
// libxkbcommon for the character, keep it when it is printable and not a
// space, and only then fall through to the name and the fold table. The two
// guards are part of the rule — without them Tab would be named "\t" and Space
// " ", neither of which a binding spells.
var keypadCharacterRuleFragments = []string{
	`xkb_keysym_to_utf8(keysym, buf, buf_size)`,
	`> 0x20`,
	`!= 0x7f`,
	`xkb_keysym_get_name(keysym, buf, buf_size)`,
	keypadFoldFunc + `(buf, buf_size)`,
}

// readKeypadCopies reads every copy out of the tree, failing the test when it
// cannot.
//
// The two Go copies are read as text like the C one, rather than called: both
// are behind //go:build linux (and the X11 one behind cgo besides), so this
// package cannot link them from any host. readNativeSource is the shared entry
// point for exactly that, native side or not.
func readKeypadCopies(t *testing.T) keypadCopies {
	t.Helper()

	copies, problem := parseKeypadCopies(readKeypadSources(t))
	if problem != "" {
		t.Fatal(problem)
	}

	return copies
}

// keypadSources is the text of every file this pin reads, keyed by its
// repo-relative path, so a test can hand it a doctored one.
type keypadSources map[string]string

// readKeypadSources reads all five out of the tree.
func readKeypadSources(t *testing.T) keypadSources {
	t.Helper()

	sources := make(keypadSources)

	for _, path := range []string{
		keypadFoldSource,
		keypadEvdevSource,
		keypadEvdevTestFile,
		keypadX11Source,
		keypadX11TestFile,
	} {
		sources[path] = readNativeSource(t, path)
	}

	return sources
}

// parseKeypadCopies reads the fold table and each transcription of it. The
// second result describes why they could not be read, and is empty when they
// could — an error value would buy nothing here, since the only caller turns it
// straight into a test failure.
func parseKeypadCopies(sources keypadSources) (keypadCopies, string) {
	body, problem := nativeRuleMethodBody(
		sources[keypadFoldSource],
		keypadFoldFuncPattern,
		keypadFoldFunc,
		"static void "+keypadFoldFunc+"(char *buf, size_t buf_size) {",
	)
	if problem != "" {
		return keypadCopies{}, keypadFoldSource + ": " + problem
	}

	folds, problem := parseKeypadFoldTable(body)
	if problem != "" {
		return keypadCopies{}, keypadFoldSource + ": " + problem
	}

	resolver, problem := nativeRuleMethodBody(
		sources[keypadFoldSource],
		keypadCharacterFuncPattern,
		keypadCharacterFunc,
		"int "+keypadCharacterFunc+"(uint32_t keysym, char *buf, size_t buf_size) {",
	)
	if problem != "" {
		return keypadCopies{}, keypadFoldSource + ": " + problem
	}

	copies := keypadCopies{folds: folds, namesByCharacter: keypadNamesByCharacter(resolver)}

	evdev, problem := parseKeypadGoFile(keypadEvdevSource, sources[keypadEvdevSource])
	if problem != "" {
		return keypadCopies{}, problem
	}

	constants := keypadGoConstants{
		strings: make(map[string]string),
		numbers: make(map[string]uint16),
	}
	collectKeypadGoConstants(evdev, constants)

	copies.evdevEntries, problem = parseKeypadEvdevTable(evdev, constants)
	if problem != "" {
		return keypadCopies{}, problem
	}

	copies.evdevCases, problem = parseKeypadEvdevCases(sources[keypadEvdevTestFile], constants)
	if problem != "" {
		return keypadCopies{}, problem
	}

	copies.x11Names, problem = parseKeypadX11Cascade(sources[keypadX11Source], constants)
	if problem != "" {
		return keypadCopies{}, problem
	}

	copies.x11Cases, problem = parseKeypadX11TestTable(sources[keypadX11TestFile], constants)
	if problem != "" {
		return keypadCopies{}, problem
	}

	return copies, ""
}

// parseKeypadX11Cascade reads the name x11KeysymName returns for each keysym it
// has a case for, keyed by keysym without its XK_ prefix.
//
// Both halves of the cascade are read, the function keys included: a keypad
// case hiding in the half this pin skipped would be a name held to nothing. So
// every case has to be readable — a label that is not a C.XK_ keysym, or an arm
// returning something other than a name, is reported rather than passed over.
func parseKeypadX11Cascade(source string, constants keypadGoConstants) (map[string]string, string) {
	file, problem := parseKeypadGoFile(keypadX11Source, source)
	if problem != "" {
		return nil, problem
	}

	collectKeypadGoConstants(file, constants)

	names := make(map[string]string)

	for _, funcName := range []string{keypadX11Func, keypadX11FunctionFunc} {
		decl, found := keypadGoFunc(file, funcName)
		if !found {
			return nil, fmt.Sprintf(
				"%s: no func %s (renamed?); the keysym cascade is unpinned",
				keypadX11Source, funcName,
			)
		}

		if problem := parseKeypadX11Switch(funcName, decl, constants, names); problem != "" {
			return nil, keypadX11Source + ": " + problem
		}
	}

	if len(names) == 0 {
		return nil, fmt.Sprintf("%s: %s names no keysyms", keypadX11Source, keypadX11Func)
	}

	return names, ""
}

// parseKeypadX11Switch reads one `case C.XK_Left, C.XK_KP_Left: return name`
// cascade into names.
func parseKeypadX11Switch(
	funcName string,
	decl *ast.FuncDecl,
	constants keypadGoConstants,
	names map[string]string,
) string {
	problem := ""

	ast.Inspect(decl, func(node ast.Node) bool {
		clause, isClause := node.(*ast.CaseClause)
		if !isClause || problem != "" {
			return problem == ""
		}

		problem = parseKeypadX11Clause(funcName, clause, constants, names)

		return problem == ""
	})

	return problem
}

// parseKeypadX11Clause reads one arm of the cascade. The default arm names no
// keysym, so it is passed over — but only once this pin has seen that it names
// none: an arm it cannot read at all is reported, because the alternative is
// skipping a keysym while reporting coverage of it.
func parseKeypadX11Clause(
	funcName string,
	clause *ast.CaseClause,
	constants keypadGoConstants,
	names map[string]string,
) string {
	answer, readable := keypadX11ClauseAnswer(clause, constants)

	if len(clause.List) == 0 {
		if readable || keypadX11ClauseDelegates(clause) {
			return ""
		}

		return fmt.Sprintf(
			"the default arm of %s is one this pin does not read; it reads an arm "+
				"returning a name or delegating to another lookup",
			funcName,
		)
	}

	if !readable {
		return fmt.Sprintf(
			"an arm of %s returns something this pin does not read; it reads arms "+
				"returning a string literal or one of the name constants",
			funcName,
		)
	}

	for _, label := range clause.List {
		keysym, isKeysym := keypadX11Keysym(label)
		if !isKeysym {
			return fmt.Sprintf(
				"%s has a case label that is not a `C.%s<keysym>` constant, "+
					"which this pin does not read",
				funcName, x11KeysymPrefix,
			)
		}

		if existing, named := names[keysym]; named && existing != answer {
			return fmt.Sprintf(
				"%s names %s both %q and %q",
				funcName, keysym, existing, answer,
			)
		}

		names[keysym] = answer
	}

	return ""
}

// keypadX11ClauseAnswer reads the name an arm returns.
func keypadX11ClauseAnswer(clause *ast.CaseClause, constants keypadGoConstants) (string, bool) {
	if len(clause.Body) != 1 {
		return "", false
	}

	returned, isReturn := clause.Body[0].(*ast.ReturnStmt)
	if !isReturn || len(returned.Results) != 1 {
		return "", false
	}

	return keypadGoStringValue(returned.Results[0], constants)
}

// keypadX11ClauseDelegates reports whether an arm hands the keysym to another
// lookup, which is how the cascade is split in two.
func keypadX11ClauseDelegates(clause *ast.CaseClause) bool {
	if len(clause.Body) != 1 {
		return false
	}

	returned, isReturn := clause.Body[0].(*ast.ReturnStmt)
	if !isReturn || len(returned.Results) != 1 {
		return false
	}

	_, isCall := returned.Results[0].(*ast.CallExpr)

	return isCall
}

// keypadX11Keysym reads a `C.XK_KP_Home` case label as the keysym name the C
// fold table is keyed by.
func keypadX11Keysym(label ast.Expr) (string, bool) {
	selector, isSelector := label.(*ast.SelectorExpr)
	if !isSelector {
		return "", false
	}

	pkg, isIdent := selector.X.(*ast.Ident)
	if !isIdent || pkg.Name != "C" {
		return "", false
	}

	keysym, hasPrefix := strings.CutPrefix(selector.Sel.Name, x11KeysymPrefix)

	return keysym, hasPrefix
}

// parseKeypadX11TestTable reads the keypad table in x11_keys_test.go: which
// keysym each case feeds x11KeyFromLookup, and the name it expects back.
func parseKeypadX11TestTable(
	source string,
	constants keypadGoConstants,
) (map[string]string, string) {
	file, problem := parseKeypadGoFile(keypadX11TestFile, source)
	if problem != "" {
		return nil, problem
	}

	keysyms := keypadX11TestKeysyms(file)

	decl, found := keypadGoFunc(file, keypadX11TestFunc)
	if !found {
		return nil, fmt.Sprintf(
			"%s: no func %s (renamed?); the X11 keypad names are mirrored by nothing",
			keypadX11TestFile, keypadX11TestFunc,
		)
	}

	literal, found := keypadGoAssignedLiteral(decl, keypadX11TestVar)
	if !found {
		return nil, fmt.Sprintf(
			"%s: %s assigns no `%s := []struct{...}{...}` literal (rewritten?)",
			keypadX11TestFile, keypadX11TestFunc, keypadX11TestVar,
		)
	}

	cases := make(map[string]string)

	for _, element := range literal.Elts {
		fields, isRow := element.(*ast.CompositeLit)
		if !isRow {
			return nil, fmt.Sprintf(
				"%s: %s holds an element this pin does not read; it reads "+
					"`{name: ..., got: %s(0, nil, <keysym constant>), want: ...}` rows",
				keypadX11TestFile, keypadX11TestVar, keypadX11LookupFunc,
			)
		}

		keysym, want, problem := parseKeypadX11TestCase(fields, keysyms, constants)
		if problem != "" {
			return nil, keypadX11TestFile + ": " + problem
		}

		if _, duplicate := cases[keysym]; duplicate {
			return nil, fmt.Sprintf("%s: %s pins %s twice",
				keypadX11TestFile, keypadX11TestVar, keysym)
		}

		cases[keysym] = want
	}

	if len(cases) == 0 {
		return nil, fmt.Sprintf("%s: %s is empty", keypadX11TestFile, keypadX11TestVar)
	}

	return cases, ""
}

// parseKeypadX11TestCase reads one row of that table.
func parseKeypadX11TestCase(
	fields *ast.CompositeLit,
	keysyms map[string]string,
	constants keypadGoConstants,
) (string, string, string) {
	var (
		keysym, want   string
		readGot, isSet bool
	)

	for _, element := range fields.Elts {
		field, isField := element.(*ast.KeyValueExpr)
		if !isField {
			return "", "", keypadX11TestVar + " holds a row with an unnamed field"
		}

		name, isName := field.Key.(*ast.Ident)
		if !isName {
			return "", "", keypadX11TestVar + " holds a row with a field name this pin cannot read"
		}

		switch name.Name {
		case keypadX11TestGotField:
			keysym, readGot = keypadX11TestKeysymOf(field.Value, keysyms)
		case keypadX11TestWantField:
			want, isSet = keypadGoStringValue(field.Value, constants)
		default:
		}
	}

	if !readGot {
		return "", "", fmt.Sprintf(
			"%s holds a row whose %s this pin does not read; it reads "+
				"`%s(0, nil, <keysym constant>)`, where the constant carries a "+
				"`// %s<keysym>` comment saying which keysym it is",
			keypadX11TestVar, keypadX11TestGotField, keypadX11LookupFunc, x11KeysymPrefix,
		)
	}

	if !isSet {
		return "", "", fmt.Sprintf(
			"%s holds a row whose %s this pin cannot read; it reads string literals "+
				"and the string constants declared beside the table",
			keypadX11TestVar, keypadX11TestWantField,
		)
	}

	return keysym, want, ""
}

// keypadX11TestKeysymOf reads the keysym a `got:` column feeds the lookup.
func keypadX11TestKeysymOf(value ast.Expr, keysyms map[string]string) (string, bool) {
	call, isCall := value.(*ast.CallExpr)
	if !isCall {
		return "", false
	}

	callee, isIdent := call.Fun.(*ast.Ident)
	if !isIdent || callee.Name != keypadX11LookupFunc || len(call.Args) == 0 {
		return "", false
	}

	symbol, isSymbol := call.Args[len(call.Args)-1].(*ast.Ident)
	if !isSymbol {
		return "", false
	}

	keysym, declared := keysyms[symbol.Name]

	return keysym, declared
}

// keypadX11TestKeysyms reads which keysym each pinned X11 protocol number is,
// which the test file records as a `// XK_KP_Home` comment on the constant. The
// numbers themselves are frozen protocol values with no counterpart in the C
// source, so the comment is the only link this pin can follow.
func keypadX11TestKeysyms(file *ast.File) map[string]string {
	keysyms := make(map[string]string)

	for _, decl := range file.Decls {
		genDecl, isGenDecl := decl.(*ast.GenDecl)
		if !isGenDecl || genDecl.Tok != token.CONST {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, isValueSpec := spec.(*ast.ValueSpec)
			if !isValueSpec || valueSpec.Comment == nil || len(valueSpec.Names) != 1 {
				continue
			}

			fields := strings.Fields(valueSpec.Comment.Text())
			if len(fields) == 0 {
				continue
			}

			if keysym, isKeysym := strings.CutPrefix(fields[0], x11KeysymPrefix); isKeysym {
				keysyms[valueSpec.Names[0].Name] = keysym
			}
		}
	}

	return keysyms
}

// keypadGoFunc finds a function declaration by name.
func keypadGoFunc(file *ast.File, name string) (*ast.FuncDecl, bool) {
	for _, decl := range file.Decls {
		funcDecl, isFunc := decl.(*ast.FuncDecl)
		if isFunc && funcDecl.Name.Name == name {
			return funcDecl, true
		}
	}

	return nil, false
}

// keypadGoAssignedLiteral finds the composite literal a function assigns to a
// local variable.
func keypadGoAssignedLiteral(decl *ast.FuncDecl, name string) (*ast.CompositeLit, bool) {
	var (
		literal *ast.CompositeLit
		found   bool
	)

	ast.Inspect(decl, func(node ast.Node) bool {
		assignment, isAssignment := node.(*ast.AssignStmt)
		if !isAssignment || found {
			return !found
		}

		for index, target := range assignment.Lhs {
			assigned, isIdent := target.(*ast.Ident)
			if !isIdent || assigned.Name != name || index >= len(assignment.Rhs) {
				continue
			}

			literal, found = assignment.Rhs[index].(*ast.CompositeLit)
		}

		return !found
	})

	return literal, found
}

// keypadGoConstants are the constants a Go transcription writes its names and
// key codes as. Resolving them is what lets the copies be compared as names
// rather than as identifiers.
type keypadGoConstants struct {
	strings map[string]string
	numbers map[string]uint16
}

// parseKeypadGoFile parses one Go transcription. The build tags on these files
// are irrelevant here: go/parser reads the source, and nothing links it.
func parseKeypadGoFile(repoRelPath, source string) (*ast.File, string) {
	parsed, err := parser.ParseFile(
		token.NewFileSet(),
		filepath.Base(repoRelPath),
		source,
		parser.ParseComments|parser.SkipObjectResolution,
	)
	if err != nil {
		return nil, fmt.Sprintf("%s: cannot parse: %v", repoRelPath, err)
	}

	return parsed, ""
}

// collectKeypadGoConstants records the string and integer constants a file
// declares. Constants declared as anything else are passed over: this is a
// lookup for the values the tables are written with, not a claim about the
// file, and every table entry that fails to resolve through it is reported by
// the parser that needed it.
func collectKeypadGoConstants(file *ast.File, into keypadGoConstants) {
	for _, decl := range file.Decls {
		genDecl, isGenDecl := decl.(*ast.GenDecl)
		if !isGenDecl || genDecl.Tok != token.CONST {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, isValueSpec := spec.(*ast.ValueSpec)
			if !isValueSpec {
				continue
			}

			for index, name := range valueSpec.Names {
				if index >= len(valueSpec.Values) {
					break
				}

				collectKeypadGoConstant(name.Name, valueSpec.Values[index], into)
			}
		}
	}
}

// collectKeypadGoConstant records one constant, if it is a literal of a kind
// these tables are written with.
func collectKeypadGoConstant(name string, value ast.Expr, into keypadGoConstants) {
	literal, isLiteral := value.(*ast.BasicLit)
	if !isLiteral {
		return
	}

	if literal.Kind == token.STRING {
		unquoted, err := strconv.Unquote(literal.Value)
		if err == nil {
			into.strings[name] = unquoted
		}

		return
	}

	if literal.Kind == token.INT {
		number, err := strconv.ParseUint(literal.Value, 0, keypadCodeBits)
		if err == nil {
			into.numbers[name] = uint16(number)
		}
	}
}

// parseKeypadEvdevTable reads the keypad half of evdevKeyNames, and the key
// codes its constants carry.
//
// Every entry of the table has to be readable, not just the keypad ones: an
// entry written in a shape this pin passes over could be the keypad entry it
// claims to be holding, which is how a pin comes to cover less than it says.
func parseKeypadEvdevTable(
	file *ast.File,
	constants keypadGoConstants,
) ([]keypadEvdevEntry, string) {
	literal, found := keypadGoVarLiteral(file, keypadEvdevTable)
	if !found {
		return nil, fmt.Sprintf(
			"%s: no `var %s = map[...]{...}` literal to read the keypad names from "+
				"(renamed, or no longer a literal?)",
			keypadEvdevSource, keypadEvdevTable,
		)
	}

	var entries []keypadEvdevEntry

	for _, element := range literal.Elts {
		entry, problem := parseKeypadEvdevEntry(element, constants)
		if problem != "" {
			return nil, keypadEvdevSource + ": " + problem
		}

		if entry.symbol == "" {
			continue
		}

		if answered, taken := keypadEvdevEntryFor(entries, entry.code); taken {
			return nil, fmt.Sprintf(
				"%s: %s and %s are both the key code %d",
				keypadEvdevSource, answered.symbol, entry.symbol, entry.code,
			)
		}

		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return nil, fmt.Sprintf(
			"%s: %s carries no %s* entries (renamed?)",
			keypadEvdevSource, keypadEvdevTable, keypadEvdevSymbolPrefix,
		)
	}

	return entries, ""
}

// parseKeypadEvdevEntry reads one `evdevKeyKP7: evdevKeyNameHome,` entry. An
// entry outside the keypad is read and then handed back with no symbol: it has
// to be readable, since an entry this pin passed over could be the keypad entry
// it claims to be holding, but it is not a keypad name.
func parseKeypadEvdevEntry(
	element ast.Expr,
	constants keypadGoConstants,
) (keypadEvdevEntry, string) {
	entry, isEntry := element.(*ast.KeyValueExpr)
	if !isEntry {
		return keypadEvdevEntry{}, fmt.Sprintf(
			"%s holds an element this pin does not read; it reads "+
				"`<key constant>: <name>,` entries",
			keypadEvdevTable,
		)
	}

	symbol, isSymbol := entry.Key.(*ast.Ident)
	if !isSymbol {
		return keypadEvdevEntry{}, fmt.Sprintf(
			"%s holds an entry keyed by something other than a key-code constant, "+
				"which this pin does not read",
			keypadEvdevTable,
		)
	}

	name, resolved := keypadGoStringValue(entry.Value, constants)
	if !resolved {
		return keypadEvdevEntry{}, fmt.Sprintf(
			"%s answers %s with a name this pin cannot read; it reads "+
				"string literals and the string constants declared beside the table",
			keypadEvdevTable, symbol.Name,
		)
	}

	if !strings.HasPrefix(symbol.Name, keypadEvdevSymbolPrefix) {
		return keypadEvdevEntry{}, ""
	}

	code, hasCode := constants.numbers[symbol.Name]
	if !hasCode {
		return keypadEvdevEntry{}, fmt.Sprintf(
			"%s is not declared as an integer key code, "+
				"so this pin cannot tell which key %s names",
			symbol.Name, keypadEvdevTable,
		)
	}

	return keypadEvdevEntry{symbol: symbol.Name, code: code, name: name}, ""
}

// parseKeypadEvdevCases reads keypadKeyNameCases: which keysym each keypad key
// reports with NumLock off, and the name the test expects for it.
func parseKeypadEvdevCases(
	source string,
	constants keypadGoConstants,
) (map[string]keypadEvdevCase, string) {
	file, problem := parseKeypadGoFile(keypadEvdevTestFile, source)
	if problem != "" {
		return nil, problem
	}

	literal, found := keypadGoVarLiteral(file, keypadEvdevCases)
	if !found {
		return nil, fmt.Sprintf(
			"%s: no `var %s = []struct{...}{...}` literal to read the keypad cases from "+
				"(renamed, or no longer a literal?)",
			keypadEvdevTestFile, keypadEvdevCases,
		)
	}

	cases := make(map[string]keypadEvdevCase)

	for _, element := range literal.Elts {
		fields, isRow := element.(*ast.CompositeLit)
		if !isRow {
			return nil, fmt.Sprintf(
				"%s: %s holds an element this pin does not read; it reads "+
					"`{name: ..., keysym: ..., code: ..., want: ...}` rows",
				keypadEvdevTestFile, keypadEvdevCases,
			)
		}

		keysym, testCase, problem := parseKeypadEvdevCase(fields, constants)
		if problem != "" {
			return nil, keypadEvdevTestFile + ": " + problem
		}

		if _, duplicate := cases[keysym]; duplicate {
			return nil, fmt.Sprintf(
				"%s: %s pins %s twice",
				keypadEvdevTestFile, keypadEvdevCases, keysym,
			)
		}

		cases[keysym] = testCase
	}

	if len(cases) == 0 {
		return nil, fmt.Sprintf("%s: %s is empty", keypadEvdevTestFile, keypadEvdevCases)
	}

	return cases, ""
}

// parseKeypadEvdevCase reads one row of that table.
func parseKeypadEvdevCase(
	fields *ast.CompositeLit,
	constants keypadGoConstants,
) (string, keypadEvdevCase, string) {
	var (
		keysym   string
		testCase keypadEvdevCase
		read     = make(map[string]bool)
	)

	for _, element := range fields.Elts {
		field, isField := element.(*ast.KeyValueExpr)
		if !isField {
			return "", testCase, keypadEvdevCases + " holds a row with an unnamed field"
		}

		name, isName := field.Key.(*ast.Ident)
		if !isName {
			return "", testCase, keypadEvdevCases +
				" holds a row with a field name this pin cannot read"
		}

		switch name.Name {
		case keypadCaseKeysymField:
			keysym, read[name.Name] = keypadGoStringValue(field.Value, constants)
		case keypadCaseCodeField:
			testCase.code, read[name.Name] = keypadGoNumberValue(field.Value)
		case keypadCaseWantField:
			testCase.want, read[name.Name] = keypadGoStringValue(field.Value, constants)
		default:
		}
	}

	for _, field := range []string{
		keypadCaseKeysymField,
		keypadCaseCodeField,
		keypadCaseWantField,
	} {
		if !read[field] {
			return "", testCase, fmt.Sprintf(
				"%s holds a row whose %s this pin cannot read; it reads string literals, "+
					"the string constants declared beside the table, and integer "+
					"literals for the key code",
				keypadEvdevCases, field,
			)
		}
	}

	return keysym, testCase, ""
}

// keypadGoVarLiteral finds the composite literal a package-level var is
// declared with.
func keypadGoVarLiteral(file *ast.File, name string) (*ast.CompositeLit, bool) {
	for _, decl := range file.Decls {
		genDecl, isGenDecl := decl.(*ast.GenDecl)
		if !isGenDecl || genDecl.Tok != token.VAR {
			continue
		}

		for _, spec := range genDecl.Specs {
			valueSpec, isValueSpec := spec.(*ast.ValueSpec)
			if !isValueSpec {
				continue
			}

			for index, declared := range valueSpec.Names {
				if declared.Name != name || index >= len(valueSpec.Values) {
					continue
				}

				literal, isLiteral := valueSpec.Values[index].(*ast.CompositeLit)

				return literal, isLiteral
			}
		}
	}

	return nil, false
}

// keypadGoStringValue reads a string written as a literal or as one of the
// constants declared beside the tables.
func keypadGoStringValue(value ast.Expr, constants keypadGoConstants) (string, bool) {
	switch expression := value.(type) {
	case *ast.BasicLit:
		if expression.Kind != token.STRING {
			return "", false
		}

		unquoted, err := strconv.Unquote(expression.Value)

		return unquoted, err == nil
	case *ast.Ident:
		resolved, declared := constants.strings[expression.Name]

		return resolved, declared
	default:
		return "", false
	}
}

// keypadGoNumberValue reads a key code, which keypadKeyNameCases writes as a
// literal on purpose: those are frozen kernel ABI numbers, pinned there rather
// than read back from the table under test.
func keypadGoNumberValue(value ast.Expr) (uint16, bool) {
	literal, isLiteral := value.(*ast.BasicLit)
	if !isLiteral || literal.Kind != token.INT {
		return 0, false
	}

	number, err := strconv.ParseUint(literal.Value, 0, keypadCodeBits)

	return uint16(number), err == nil
}

// parseKeypadFoldTable reads the `{"KP_Home", "Home"},` rows of the fold table.
// Every line inside the literal has to be a row, a comment or blank: a line this
// pin cannot read is one it would otherwise skip past, which is how a pin comes
// to cover less than it claims.
func parseKeypadFoldTable(body string) (map[string]string, string) {
	opening := keypadFoldTableOpeningPattern.FindStringIndex(body)
	if opening == nil {
		return nil, fmt.Sprintf(
			"no `} %s = {` literal to read the fold table from (renamed?)",
			keypadFoldTable,
		)
	}

	rest := body[opening[1]:]

	closing := keypadFoldTableClosingPattern.FindStringIndex(rest)
	if closing == nil {
		return nil, fmt.Sprintf("the %s literal is never closed by `};`", keypadFoldTable)
	}

	folds := make(map[string]string)

	for line := range strings.SplitSeq(rest[:closing[0]], "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "//") {
			continue
		}

		row := keypadFoldRowPattern.FindStringSubmatch(trimmed)
		if row == nil {
			return nil, fmt.Sprintf(
				"%s holds `%s`, which this pin does not read; it reads "+
					"`{\"<keysym>\", \"<name>\"},` rows",
				keypadFoldTable, trimmed,
			)
		}

		keysym, name, unquoted := unquoteKeypadFoldRow(row[1], row[2])
		if !unquoted {
			return nil, fmt.Sprintf(
				"%s holds `%s`, whose strings this pin cannot read",
				keypadFoldTable, trimmed,
			)
		}

		// The C lookup returns on the first row that matches, so a second row
		// for the same keysym is dead code — and reading it would have this pin
		// hold the Go copies to a name the keypad never produces.
		if existing, folded := folds[keysym]; folded {
			return nil, fmt.Sprintf(
				"%s folds %q twice, to %q and then to %q; "+
					"the second row never runs",
				keypadFoldTable, keysym, existing, name,
			)
		}

		folds[keysym] = name
	}

	if len(folds) == 0 {
		return nil, "the " + keypadFoldTable + " literal is empty"
	}

	return folds, ""
}

// unquoteKeypadFoldRow reads the two C string literals of one row. C and Go
// spell the escapes this table uses the same way, so Go's unquoting is the
// right reader for them.
func unquoteKeypadFoldRow(keysymLiteral, nameLiteral string) (string, string, bool) {
	keysym, err := strconv.Unquote(keysymLiteral)
	if err != nil {
		return "", "", false
	}

	name, err := strconv.Unquote(nameLiteral)
	if err != nil {
		return "", "", false
	}

	return keysym, name, true
}

// keypadNamesByCharacter reports whether the resolver still spells "a keysym
// with a printable character is that character, and only otherwise its folded
// name". It is read as fragments in order rather than as whole lines, because
// the rule is a C condition and two calls rather than a table row; what matters
// is that every part of it is there, and that the character comes first.
func keypadNamesByCharacter(resolver string) bool {
	rest := strings.Join(strings.Fields(resolver), " ")

	for _, fragment := range keypadCharacterRuleFragments {
		at := strings.Index(rest, fragment)
		if at < 0 {
			return false
		}

		rest = rest[at+len(fragment):]
	}

	return true
}
