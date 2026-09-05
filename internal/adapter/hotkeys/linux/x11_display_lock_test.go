package linux_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The X11 hotkey backend owns one Xlib connection and two goroutines that use
// it: whichever one calls Register or Unregister, and the poll loop that reads
// key events off it. Xlib is thread-safe only when XInitThreads runs before the
// first XOpenDisplay, and nothing in this process calls it, so those two must
// never be inside the connection at the same time. Manager.mu does not buy
// that: it guards the Go maps, and the poll loop deliberately does not hold it
// while reading the connection.
//
// The rule is therefore about a lock the compiler cannot see the need for, in
// C calls no race detector watches, on a path that misbehaves as a corrupted
// connection rather than as a crash at the call site. These tests are what
// keeps it true.
//
// They read the sources instead of running them, and carry no build tag for
// the same reason: the edit they exist to catch — a new Xlib call added to
// x11_cgo.go — is made on the maintainer's macOS host, where neither the file
// nor a Linux-tagged test compiles. A rule checked only on the CI leg is a rule
// found broken later than it was written.

// x11DisplayCalls are the cgo entry points that speak on the hotkey connection.
// Each takes the Display as an argument, and each therefore writes to or reads
// from its buffers — including neru_hotkeys_pending, which is XPending, which
// flushes the request buffer and reads the socket rather than merely peeking at
// a counter.
var x11DisplayCalls = map[string]struct{}{
	"XCloseDisplay":            {},
	"XFlush":                   {},
	"XGrabKey":                 {},
	"XKeysymToKeycode":         {},
	"XNextEvent":               {},
	"XOpenDisplay":             {},
	"XSelectInput":             {},
	"XUngrabKey":               {},
	"neru_hotkeys_pending":     {},
	"neru_hotkeys_root_window": {},
	// XkbSetDetectableAutoRepeat: a request on the connection.
	"neru_hotkeys_set_detectable_autorepeat": {},
}

// x11ConnectionFreeCalls are the cgo calls that touch no connection: type
// conversions, C memory, the keysym table (XStringToKeysym is a lookup in the
// client's own static table and takes no Display), and the accessors that read
// a field out of an XEvent already copied into Go memory.
//
// Anything not in this list and not in x11DisplayCalls fails the classification
// test rather than being assumed harmless — that assumption is what a new
// Xlib call would arrive on.
var x11ConnectionFreeCalls = map[string]struct{}{
	"CString":           {},
	"XStringToKeysym":   {},
	"free":              {},
	"int":               {},
	"neru_xevent_type":  {},
	"neru_xkey_keycode": {},
	"neru_xkey_state":   {},
	"uint":              {},
}

// x11DisplayLockHolders are the functions allowed to call into the connection,
// each of which must take displayMu for the length of its call sequence. They
// are methods on x11HotkeyState rather than on Manager so that the lock and the
// Display it protects are reached through the same receiver.
var x11DisplayLockHolders = map[string]string{
	"grab":         "resolves the keycode and installs the grabs as one sequence",
	"ungrab":       "releases one binding's grabs and flushes them",
	"nextEvent":    "pending-then-read is one sequence; a read another thread beat it to would block holding the lock",
	"closeDisplay": "tears the connection down after the poll loop has exited",
}

// x11DisplayLockCallees are the helpers that speak on the connection without
// taking the lock themselves because every caller already holds it. The claim
// each entry makes is checked, not trusted:
// TestEveryDisplayTouchingHelperIsCalledOnlyFromALockHolder fails on a call
// site outside a holder.
var x11DisplayLockCallees = map[string]string{
	"parseX11Hotkey": "resolves a hotkey string to a keycode against this " +
		"connection's keymap, from inside grab's critical section",
}

// x11DisplayLockExemptions are the functions that reach the connection without
// the lock, with the reason each is sound. An entry here is a claim that no
// second goroutine can be inside the connection at that moment.
var x11DisplayLockExemptions = map[string]string{
	"ensureX11State": "opens the connection, asks it for detectable autorepeat " +
		"and reads the root window before the state is published to x11States " +
		"and before the poll loop starts, so there is no second user of the " +
		"connection yet and no lock to take",
}

// x11HotkeySourceFile is the only file in this package permitted to speak on
// the connection. Scanning the whole directory rather than this file alone is
// what stops the rule being sidestepped by a second cgo file.
const x11HotkeySourceFile = "x11_cgo.go"

// TestEveryXlibCallOnTheHotkeyDisplayHoldsTheDisplayLock pins the serialization
// itself: a call into the connection happens in a function that holds
// displayMu, or in one of the exemptions above with its reason written down.
func TestEveryXlibCallOnTheHotkeyDisplayHoldsTheDisplayLock(t *testing.T) {
	// A walk that matched nothing would pass whatever the code did, so what it
	// found is asserted alongside what it judged: a rename in the C bridge that
	// emptied x11DisplayCalls would otherwise turn this green.
	seen := 0

	for _, file := range packageSourceFiles(t) {
		base := filepath.Base(file.name)

		for _, function := range file.functions {
			displayCalls := function.cgoCallsMatching(x11DisplayCalls)
			if len(displayCalls) == 0 {
				continue
			}

			seen += len(displayCalls)

			if base != x11HotkeySourceFile {
				t.Errorf(
					"%s: %s calls %s on the hotkey X11 connection; the connection "+
						"is served from %s alone, so every call on it sits behind "+
						"one displayMu",
					base, function.name, strings.Join(displayCalls, ", "),
					x11HotkeySourceFile,
				)

				continue
			}

			if _, exempt := x11DisplayLockExemptions[function.name]; exempt {
				continue
			}

			if _, callee := x11DisplayLockCallees[function.name]; callee {
				continue
			}

			if _, holder := x11DisplayLockHolders[function.name]; !holder {
				t.Errorf(
					"%s: %s calls %s on the hotkey X11 connection without holding "+
						"displayMu; Xlib is not thread-safe here (no XInitThreads) "+
						"and the poll loop is on the same connection, so route the "+
						"call through an x11HotkeyState method that locks, or add "+
						"an exemption saying why no other goroutine can be inside "+
						"the connection here",
					base, function.name, strings.Join(displayCalls, ", "),
				)
			}
		}
	}

	if seen == 0 {
		t.Errorf(
			"no call in %s matched x11DisplayCalls; the rule cannot be checked "+
				"against a list that names nothing the code calls",
			x11HotkeySourceFile,
		)
	}
}

// TestEveryDisplayLockHolderStillTakesTheLock is the other half: a holder that
// stops locking would leave the first test green, because it only asks which
// function the call is in.
//
// It asks for the deferred unlock as well as the lock, and for the lock to come
// before the first call on the connection. A holder that unlocked early, or
// locked after its first call, would satisfy a bare "does it lock" and protect
// nothing.
func TestEveryDisplayLockHolderStillTakesTheLock(t *testing.T) {
	found := map[string]bool{}

	for _, file := range packageSourceFiles(t) {
		if filepath.Base(file.name) != x11HotkeySourceFile {
			continue
		}

		for _, function := range file.functions {
			why, holder := x11DisplayLockHolders[function.name]
			if !holder {
				continue
			}

			found[function.name] = true

			lock := strings.Index(function.source, "displayMu.Lock()")

			if lock < 0 || !strings.Contains(function.source, "defer s.displayMu.Unlock()") {
				t.Errorf(
					"%s: %s is listed as a displayMu holder (%s) but does not take "+
						"the lock and defer its release; every Xlib call on the "+
						"hotkey connection is serialized on it",
					x11HotkeySourceFile, function.name, why,
				)

				continue
			}

			for _, call := range function.cgoCallsMatching(x11DisplayCalls) {
				if used := strings.Index(function.source, "C."+call); used >= 0 && used < lock {
					t.Errorf(
						"%s: %s calls C.%s before taking displayMu; the lock has to "+
							"come first or it serializes nothing",
						x11HotkeySourceFile, function.name, call,
					)
				}
			}
		}
	}

	for name, why := range x11DisplayLockHolders {
		if !found[name] {
			t.Errorf(
				"%s is listed as a displayMu holder (%s) but no such function "+
					"exists in %s; drop the entry or restore the function",
				name, why, x11HotkeySourceFile,
			)
		}
	}
}

// TestEveryDisplayLockExemptionIsStillReal keeps the one list that weakens the
// rule from outliving what it excuses. An exemption for a function that no
// longer exists, or that no longer touches the connection, is a hole nobody
// meant to leave open — so the list can only shrink.
func TestEveryDisplayLockExemptionIsStillReal(t *testing.T) {
	touching := map[string]bool{}

	for _, file := range packageSourceFiles(t) {
		for _, function := range file.functions {
			if len(function.cgoCallsMatching(x11DisplayCalls)) > 0 {
				touching[function.name] = true
			}
		}
	}

	for name, why := range x11DisplayLockExemptions {
		if !touching[name] {
			t.Errorf(
				"%s is excused from taking displayMu (%s) but no longer calls "+
					"anything on the connection; drop the entry",
				name, why,
			)
		}
	}
}

// TestEveryDisplayTouchingHelperIsCalledOnlyFromALockHolder checks the claim
// each x11DisplayLockCallees entry makes. A helper that speaks on the
// connection is safe only for as long as nothing calls it from outside a
// critical section, and that is a property of its call sites, not of the helper
// — so a second caller is exactly the change this has to catch.
func TestEveryDisplayTouchingHelperIsCalledOnlyFromALockHolder(t *testing.T) {
	callSites := map[string]int{}

	for _, file := range packageSourceFiles(t) {
		for _, function := range file.functions {
			for _, called := range function.calls {
				why, tracked := x11DisplayLockCallees[called]
				if !tracked {
					continue
				}

				callSites[called]++

				_, holder := x11DisplayLockHolders[function.name]
				if holder || function.name == called {
					continue
				}

				t.Errorf(
					"%s: %s calls %s, which speaks on the hotkey X11 connection "+
						"without taking displayMu (%s); either make this caller a "+
						"lock holder or move the call inside one",
					filepath.Base(file.name), function.name, called, why,
				)
			}
		}
	}

	for name, why := range x11DisplayLockCallees {
		if callSites[name] == 0 {
			t.Errorf(
				"%s is listed as a display-touching helper (%s) but nothing calls "+
					"it; drop the entry so the list keeps describing the code",
				name, why,
			)
		}
	}
}

// TestEveryCgoCallInTheHotkeyPackageIsClassified refuses to let a new cgo call
// arrive unclassified. Without it the rule would only cover the calls that
// existed when it was written, and the next Xlib call added to this package
// would be silently outside it.
func TestEveryCgoCallInTheHotkeyPackageIsClassified(t *testing.T) {
	for _, file := range packageSourceFiles(t) {
		unknown := map[string]struct{}{}

		for _, function := range file.functions {
			for _, call := range function.cgoCalls {
				_, display := x11DisplayCalls[call]
				_, free := x11ConnectionFreeCalls[call]

				if !display && !free {
					unknown[call] = struct{}{}
				}
			}
		}

		for _, call := range sortedKeys(unknown) {
			t.Errorf(
				"%s: C.%s is neither in x11DisplayCalls nor in "+
					"x11ConnectionFreeCalls; say which it is, because a call that "+
					"takes the Display has to be made under displayMu",
				filepath.Base(file.name), call,
			)
		}
	}
}

// parsedFile is one package source file reduced to what these tests ask about.
type parsedFile struct {
	name      string
	functions []parsedFunction
}

// parsedFunction is one function: its name, its source text (for the lock
// check), every C.<name>(...) call it makes and every package-level function it
// calls by plain name.
type parsedFunction struct {
	name     string
	source   string
	cgoCalls []string
	calls    []string
}

// cgoCallsMatching returns the function's cgo calls that appear in want,
// deduplicated and ordered so a failure message reads the same on every run.
func (f parsedFunction) cgoCallsMatching(want map[string]struct{}) []string {
	matched := map[string]struct{}{}

	for _, call := range f.cgoCalls {
		if _, ok := want[call]; ok {
			matched[call] = struct{}{}
		}
	}

	return sortedKeys(matched)
}

// packageSourceFiles parses this package's non-test Go sources regardless of
// build tags. x11_cgo.go is behind linux && cgo, so a build without cgo would
// otherwise leave the rule unchecked on the one file it is about.
func packageSourceFiles(t *testing.T) []parsedFile {
	t.Helper()

	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing package sources: %v", err)
	}

	fileSet := token.NewFileSet()

	var files []parsedFile

	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		contents, readErr := os.ReadFile(name)
		if readErr != nil {
			t.Fatalf("reading %s: %v", name, readErr)
		}

		parsed, parseErr := parser.ParseFile(fileSet, name, contents, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parsing %s: %v", name, parseErr)
		}

		files = append(files, parsedFile{
			name:      name,
			functions: parseFunctions(fileSet, parsed, contents),
		})
	}

	if len(files) == 0 {
		t.Fatal("no package sources found; these rules are about this package's own files")
	}

	return files
}

func parseFunctions(fileSet *token.FileSet, file *ast.File, contents []byte) []parsedFunction {
	var functions []parsedFunction

	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}

		start := fileSet.Position(function.Pos()).Offset
		end := fileSet.Position(function.End()).Offset

		cgoCalls, calls := callsIn(function)

		functions = append(functions, parsedFunction{
			name:     function.Name.Name,
			source:   string(contents[start:end]),
			cgoCalls: cgoCalls,
			calls:    calls,
		})
	}

	return functions
}

// callsIn collects a function body's C.<name>(...) calls and its plain
// name(...) calls. Type conversions such as C.int(x) parse as calls too, which
// is why the classification lists name them.
func callsIn(function *ast.FuncDecl) ([]string, []string) {
	var cgoCalls, calls []string

	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		switch fun := call.Fun.(type) {
		case *ast.Ident:
			calls = append(calls, fun.Name)
		case *ast.SelectorExpr:
			pkg, isIdent := fun.X.(*ast.Ident)
			if isIdent && pkg.Name == "C" {
				cgoCalls = append(cgoCalls, fun.Sel.Name)
			}
		}

		return true
	})

	return cgoCalls, calls
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return keys
}
