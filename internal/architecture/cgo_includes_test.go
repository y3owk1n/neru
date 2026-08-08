package architecture_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var relativeIncludePattern = regexp.MustCompile(`#include\s+"(\.\.?/[^"]+)"`)

// cgoSourceExtensions are the files a relative #include can appear in.
// goExt is the Go source extension, named because three files in this package
// test against it and goconst counts the literal.
const goExt = ".go"

var cgoSourceExtensions = map[string]bool{
	goExt: true,
	".h":  true,
	".m":  true,
	".c":  true,
}

// cgoSourceRoots are the trees whose includes are this repo's business — the
// reach the walkSourceRoots helper these tests used to call gave them, kept
// exactly. It is what keeps the whole-checkout walker off .devbox, where a
// vendored Rust toolchain ships C headers whose relative includes resolve
// against rules that are not ours. Widening a guardrail is a decision of its
// own: the answer to a new tree wanting the check is to name it here.
var cgoSourceRoots = []string{"internal/", "cmd/"}

// nativeCgoFloor is the fewest .h/.m/.c files the native bridges hold. The
// files a relative #include can appear in are mostly Go, so the floor on the
// whole set says nothing about the half that carries the includes; this one
// does.
const nativeCgoFloor = 10

// isCgoSource reports whether a walked file is one a relative #include can
// appear in, inside a tree this repo owns.
func isCgoSource(file repoFile) bool {
	if !cgoSourceExtensions[filepath.Ext(file.name)] {
		return false
	}

	for _, root := range cgoSourceRoots {
		if strings.HasPrefix(file.rel, root) {
			return true
		}
	}

	return false
}

// isNativeSource reports whether a walked file is C or Objective-C rather than
// Go.
func isNativeSource(file repoFile) bool {
	return filepath.Ext(file.name) != goExt
}

// A relative #include that no longer resolves is the most expensive mistake in
// this tree to find by hand: `go vet` does not see it (CGO_ENABLED=0 skips the
// file), the cross-platform vet does not see it, and it only surfaces when the
// target OS compiles the package with cgo on. On a macOS host that means a
// Docker run or a red CI job, minutes later.
//
// It fires whenever a package changes depth relative to the headers it
// includes, which is what moving a backend into its own directory does.
//
// The check is trivial because the answer is purely textual: resolve the path
// against the including file's directory and stat it.
func TestCgoIncludes_RelativeIncludesResolve(t *testing.T) {
	repoRoot := findRepoRoot(t)
	checked := 0
	native := 0

	walkRepoFiles(t, repoRoot, func(file repoFile) {
		if !isCgoSource(file) {
			return
		}

		checked++

		if isNativeSource(file) {
			native++
		}

		content, readErr := os.ReadFile(file.abs)
		if readErr != nil {
			t.Fatalf("ReadFile(%s) error = %v", file.rel, readErr)
		}

		for _, match := range relativeIncludePattern.FindAllStringSubmatch(string(content), -1) {
			include := match[1]

			target := filepath.Join(filepath.Dir(file.abs), filepath.FromSlash(include))

			_, statErr := os.Stat(target)
			if statErr == nil {
				continue
			}

			t.Errorf(
				"%s includes %q, which does not resolve; a relative include "+
					"breaks when its package moves a directory deeper, and only "+
					"a cgo build for that platform would otherwise catch it",
				file.rel,
				include,
			)
		}
	})

	assertWalkedAtLeast(t, "sources that could carry a relative include", checked, bulkWalkFloor)
	assertWalkedAtLeast(t, "native sources among them", native, nativeCgoFloor)
}

// TestNativeBridgeIsReachedByRelativeInclude documents the convention the test
// above protects: the Objective-C and C bridges live in internal/adapter/
// platform/<os>/, and adapter packages reach them by relative path rather than
// by a cgo CFLAGS include path.
//
// Nothing enforces that choice, but it is worth pinning that the includes all
// point at the same place, so a contributor copying an existing file does not
// invent a second convention.
func TestCgoIncludes_NativeBridgePointsAtThePlatformPackages(t *testing.T) {
	repoRoot := findRepoRoot(t)
	checked := 0

	walkRepoFiles(t, repoRoot, func(file repoFile) {
		if !isCgoSource(file) {
			return
		}

		// Files inside the platform packages include their own headers.
		if strings.Contains(file.rel, "internal/adapter/platform/") {
			return
		}

		checked++

		content, readErr := os.ReadFile(file.abs)
		if readErr != nil {
			t.Fatalf("ReadFile(%s) error = %v", file.rel, readErr)
		}

		for _, match := range relativeIncludePattern.FindAllStringSubmatch(string(content), -1) {
			include := match[1]
			if strings.Contains(include, "platform/") {
				continue
			}

			t.Errorf(
				"%s includes %q; native headers live in "+
					"internal/adapter/platform/<os>/ and are reached through a "+
					"path containing platform/",
				file.rel,
				include,
			)
		}
	})

	assertWalkedAtLeast(t, "sources outside the platform packages", checked, bulkWalkFloor)
}
