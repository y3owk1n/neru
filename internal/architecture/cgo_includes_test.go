package architecture_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path"
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

// bridgePackageDirPrefix is where a bridge lives: internal/adapter/platform/
// <os>/. A relative #include that resolves below it is a bridge header, and
// the directory it resolves into is the bridge package.
const bridgePackageDirPrefix = "internal/adapter/platform/"

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
		if strings.HasPrefix(file.rel, bridgePackageDirPrefix) {
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

// modulePath is this module's import path, which is what turns a directory in
// the checkout into the import a Go file states for it.
const modulePath = "github.com/y3owk1n/neru"

// bridgeConsumerFloor is the fewest package-to-bridge edges expected in the
// checkout — one per bridge a package includes a header of. Nineteen packages
// hold one today; below half of that, the matching has stopped recognizing
// them rather than the repo having stopped including headers.
const bridgeConsumerFloor = 10

// compiledNativeExtensions are the files that make a package compile objects
// of its own. A .h is not one of them: a header declares symbols and defines
// none, so a package holding only headers still has an archive to link and
// still owes the import.
var compiledNativeExtensions = map[string]bool{".m": true, ".c": true}

// bridgeConsumer is what one package outside the bridges did with them: the
// bridges it includes a header of (with the file that does, for the failure
// message), the bridges it imports, and whether it compiles native source of
// its own.
type bridgeConsumer struct {
	includes   map[string]string
	imports    map[string]bool
	ownsNative bool
}

// TestCgoIncludes_HeaderConsumersLinkTheBridge pins the second half of the
// bridge boundary. TestCgoIncludes_NativeBridgePointsAtThePlatformPackages
// above pins where a header may be reached *from*; this pins that the compiled
// objects behind that header are actually linked into the binary.
//
// Including a header only declares the symbols. What links the Objective-C or
// C that defines them is importing the bridge's Go package, whether or not any
// Go symbol there is called — so a package that includes a bridge header
// states that import directly, and a blank import is the right form when it
// calls nothing (ADR 0009, and *Bridge* in CONTEXT.md).
//
// A package that compiles native source of its own is exempt, because it needs
// the header and not the archive: internal/adapter/systray/darwin builds
// systray_darwin.m in its own package and reaches platform/darwin not at all.
// That is the rule stated rather than an exception carved for it — the rule is
// about linkage, and a package holding its own .m has nothing to link.
//
// Test files are left out of both halves. What has to link here is the shipped
// binary, and a test binary that fails to link says so on the spot, in the
// package being tested.
//
// Reaching the archive transitively instead is what this forbids, and the
// failure it prevents is the expensive kind: the whole overlay render tree
// once arrived through overlay/render/overlayutil/native, so refactoring
// overlayutil to stop needing that shim would have broken eight packages at
// once, on macOS only, with an undefined-symbol error naming none of them.
func TestCgoIncludes_HeaderConsumersLinkTheBridge(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fileSet := token.NewFileSet()
	checked := 0

	consumers := map[string]*bridgeConsumer{}

	consumerFor := func(dir string) *bridgeConsumer {
		if consumers[dir] == nil {
			consumers[dir] = &bridgeConsumer{
				includes: map[string]string{},
				imports:  map[string]bool{},
			}
		}

		return consumers[dir]
	}

	walkRepoFiles(t, repoRoot, func(file repoFile) {
		if !isCgoSource(file) || strings.HasSuffix(file.name, "_test.go") {
			return
		}

		// The bridges include their own headers, and their Go files are the
		// package the rule points at.
		if strings.HasPrefix(file.rel, bridgePackageDirPrefix) {
			return
		}

		checked++

		if compiledNativeExtensions[filepath.Ext(file.name)] {
			consumerFor(file.dir).ownsNative = true
		}

		content, readErr := os.ReadFile(file.abs)
		if readErr != nil {
			t.Fatalf("ReadFile(%s) error = %v", file.rel, readErr)
		}

		for _, match := range relativeIncludePattern.FindAllStringSubmatch(string(content), -1) {
			bridgeDir := path.Dir(path.Join(file.dir, match[1]))
			if !strings.HasPrefix(bridgeDir, bridgePackageDirPrefix) {
				continue
			}

			consumerFor(file.dir).includes[bridgeDir] = file.rel
		}

		if isNativeSource(file) {
			return
		}

		parsed, parseErr := parser.ParseFile(fileSet, file.abs, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("ParseFile(%s) error = %v", file.rel, parseErr)
		}

		for _, imported := range parsed.Imports {
			importPath := strings.Trim(imported.Path.Value, `"`)

			bridgeDir, isModuleImport := strings.CutPrefix(importPath, modulePath+"/")
			if !isModuleImport || !strings.HasPrefix(bridgeDir, bridgePackageDirPrefix) {
				continue
			}

			consumerFor(file.dir).imports[bridgeDir] = true
		}
	})

	var offenders []string

	edges := 0

	for consumerDir, consumer := range consumers {
		for bridgeDir, includingFile := range consumer.includes {
			edges++

			if consumer.ownsNative || consumer.imports[bridgeDir] {
				continue
			}

			offenders = append(offenders, fmt.Sprintf(
				"%s\t%s includes a header of %s; add `import _ %q` to a file "+
					"built for that platform",
				includingFile, consumerDir, bridgeDir, modulePath+"/"+bridgeDir,
			))
		}
	}

	reportOffenders(t, offenders,
		"package includes a bridge header, compiles no native source of its "+
			"own, and never imports the bridge whose objects that header declares")

	assertWalkedAtLeast(t, "sources that could include a bridge header", checked, bulkWalkFloor)
	assertWalkedAtLeast(t, "package-to-bridge include edges", edges, bridgeConsumerFloor)
}
