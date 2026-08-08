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

	walkErr := walkSourceRoots(repoRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if isSkippedWalkDir(repoRoot, path) {
				return filepath.SkipDir
			}

			return nil
		}

		if !cgoSourceExtensions[filepath.Ext(path)] {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		relPath, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return relErr
		}

		for _, match := range relativeIncludePattern.FindAllStringSubmatch(string(content), -1) {
			include := match[1]

			target := filepath.Join(filepath.Dir(path), filepath.FromSlash(include))

			_, statErr := os.Stat(target)
			if statErr == nil {
				continue
			}

			t.Errorf(
				"%s includes %q, which does not resolve; a relative include "+
					"breaks when its package moves a directory deeper, and only "+
					"a cgo build for that platform would otherwise catch it",
				filepath.ToSlash(relPath),
				include,
			)
		}

		return nil
	})
	if walkErr != nil {
		t.Fatalf("WalkDir(%s) error = %v", repoRoot, walkErr)
	}
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

	walkErr := walkSourceRoots(repoRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if isSkippedWalkDir(repoRoot, path) {
				return filepath.SkipDir
			}

			return nil
		}

		if !cgoSourceExtensions[filepath.Ext(path)] {
			return nil
		}

		relPath, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return relErr
		}

		slashed := filepath.ToSlash(relPath)

		// Files inside the platform packages include their own headers.
		if strings.Contains(slashed, "internal/adapter/platform/") {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
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
				slashed,
				include,
			)
		}

		return nil
	})
	if walkErr != nil {
		t.Fatalf("WalkDir(%s) error = %v", repoRoot, walkErr)
	}
}

// walkSourceRoots walks only the directories this repo owns. Walking from the
// repo root would descend into .devbox, where a vendored Rust toolchain ships C
// headers whose relative includes are none of our business.
func walkSourceRoots(repoRoot string, fn func(string, os.DirEntry, error) error) error {
	for _, root := range []string{"internal", "cmd"} {
		walkErr := filepath.WalkDir(filepath.Join(repoRoot, root), fn)
		if walkErr != nil {
			return walkErr
		}
	}

	return nil
}
