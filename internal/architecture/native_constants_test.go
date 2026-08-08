package architecture_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// cDefinePattern matches an integer `#define NAME 1` in a C header, ignoring
// macros defined as anything but a plain decimal literal.
var cDefinePattern = regexp.MustCompile(
	`(?m)^#define[ \t]+([A-Za-z_][A-Za-z0-9_]*)[ \t]+(-?[0-9]+)[ \t]*$`,
)

// objcEnumPattern matches the opening line of a `typedef NS_ENUM(type, Name) {`
// declaration; the enum's name is substituted in when the pattern is built.
const objcEnumPattern = `NS_ENUM\([^,]+,[ \t]*%s\)[ \t]*\{`

// objcEnumMemberPattern matches one `Member = 1,` line inside an enum body.
var objcEnumMemberPattern = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)[ \t]*=[ \t]*(-?[0-9]+)`)

// cHeaderIntConstants returns every `#define NAME <int>` in the C header at
// repoRelPath, keyed by macro name. Macros defined as anything else are
// skipped, so a header of mixed macros yields only the numeric ones.
func cHeaderIntConstants(t *testing.T, repoRelPath string) map[string]int64 {
	t.Helper()

	source := readNativeSource(t, repoRelPath)
	constants := make(map[string]int64)

	for _, match := range cDefinePattern.FindAllStringSubmatch(source, -1) {
		constants[match[1]] = parseNativeInt(t, repoRelPath, match[1], match[2])
	}

	return constants
}

// objcEnumIntConstants returns the members of the `typedef NS_ENUM(_, enumName)`
// declared in the Objective-C source at repoRelPath, keyed by member name. It
// fails the test when the enum is not there at all, because a renamed enum is
// the same drift this pin exists to catch.
func objcEnumIntConstants(t *testing.T, repoRelPath, enumName string) map[string]int64 {
	t.Helper()

	source := readNativeSource(t, repoRelPath)

	declaration := regexp.MustCompile(
		fmt.Sprintf(objcEnumPattern, regexp.QuoteMeta(enumName)),
	)

	opening := declaration.FindStringIndex(source)
	if opening == nil {
		t.Fatalf(
			"%s: no `NS_ENUM(..., %s) {` declaration (renamed or removed?)",
			repoRelPath,
			enumName,
		)
	}

	body := source[opening[1]:]

	end := strings.Index(body, "}")
	if end < 0 {
		t.Fatalf("%s: enum %s is never closed by `}`", repoRelPath, enumName)
	}

	members := make(map[string]int64)

	for _, match := range objcEnumMemberPattern.FindAllStringSubmatch(body[:end], -1) {
		members[match[1]] = parseNativeInt(t, repoRelPath, match[1], match[2])
	}

	return members
}

// readNativeSource reads a repo-relative source file, failing with a message
// that names the likely cause: these pins address files by path, so a rename
// shows up here rather than as a silent pass over nothing.
//
// It is the entry point every language-boundary pin in this package shares —
// on both sides of the boundary. Usually the file it reads is the native one,
// which is where the name comes from; a pin whose Go side is behind a build
// tag this test binary was not built with reads that side through here too,
// because it cannot link it.
//
// ADR 0007 (docs/adr/0007-a-shared-derivation-has-one-implementation.md) lets
// a native copy of a Go declaration exist — Go cannot be the one
// implementation for a .m file — and asks for a test holding the copies
// together instead of a deletion. Writing a second such pin means reading the
// source through here and then matching whatever shape the copy takes:
// cHeaderIntConstants and objcEnumIntConstants below cover the two shapes that
// exist today, a numeric macro and an NS_ENUM member, and a copy that is a
// rule rather than a constant brings its own pattern rather than a second
// reader. label_autohide_rule_test.go and
// sub_key_preview_autohide_rule_test.go are that third shape: they read their
// copies through here and then run them, because a rule has no constant to
// compare. What those two have in common — the comparisons, and how a
// condition is split into them — lives in native_rule_test.go.
func readNativeSource(t *testing.T, repoRelPath string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(findRepoRoot(t), filepath.FromSlash(repoRelPath)))
	if err != nil {
		t.Fatalf("%s: cannot read native source (renamed?): %v", repoRelPath, err)
	}

	return string(content)
}

// assertNativeConstantCount fails when a native source declares more (or
// fewer) constants sharing a prefix than the Go declaration has values. It is
// the half of a pin that catches a value deleted from Go and left behind in
// the native code, which comparing each Go value against the native side
// cannot see. goDeclaration names the Go side in the failure message.
//
// A native source that gains an unrelated constant under the same prefix — a
// _COUNT or _DEFAULT sentinel, say — fails here too. That is intended: a
// sentinel sharing the vocabulary's prefix is exactly the thing a reader would
// mistake for a member of it, and adding one should be a decision, not a
// silent pass.
func assertNativeConstantCount(
	t *testing.T,
	repoRelPath, prefix string,
	constants map[string]int64,
	want int,
	goDeclaration string,
) {
	t.Helper()

	var declared []string

	for name := range constants {
		if strings.HasPrefix(name, prefix) {
			declared = append(declared, name)
		}
	}

	if len(declared) != want {
		sort.Strings(declared)

		t.Errorf(
			"%s: declares %d %s* constants (%s) but %s has %d values",
			repoRelPath, len(declared), prefix, strings.Join(declared, ", "), goDeclaration, want,
		)
	}
}

// nativeEnumMemberName spells a Go vocabulary value the way Objective-C names
// an enum member: "bottom" becomes "Bottom", "top_left" becomes "TopLeft".
// Underscore-separated values are handled because the config vocabularies next
// to the ones pinned here use that spelling.
func nativeEnumMemberName(value string) string {
	var built strings.Builder

	for word := range strings.SplitSeq(value, "_") {
		if word == "" {
			continue
		}

		built.WriteString(strings.ToUpper(word[:1]))
		built.WriteString(word[1:])
	}

	return built.String()
}

// parseNativeInt converts a matched native literal, naming the constant it
// belongs to if it somehow does not fit an int64.
func parseNativeInt(t *testing.T, repoRelPath, name, literal string) int64 {
	t.Helper()

	value, err := strconv.ParseInt(literal, 10, 64)
	if err != nil {
		t.Fatalf("%s: %s = %q is not an integer: %v", repoRelPath, name, literal, err)
	}

	return value
}
