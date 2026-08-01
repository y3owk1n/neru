package architecture_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// These tests enforce the dependency direction documented in
// docs/CROSS_PLATFORM.md ("The Three Tiers") and docs/ARCHITECTURE.md.
//
// The tier model is only worth anything if it holds. Prose did not hold it:
// every violation these tests now catch was present in the tree before they
// were written.

const modulePrefix = "github.com/y3owk1n/neru/"

// TestDomainStaysPure pins the innermost layer. Domain is the pure-Go core: it
// may depend on config and ports, never on an adapter or on application wiring.
// A domain package that imports infra cannot be tested without an OS.
func TestDomainStaysPure(t *testing.T) {
	forbidden := []string{
		"internal/core/infra",
		"internal/app",
		"internal/ui",
		"internal/cli",
	}

	for _, file := range goFiles(t) {
		if !strings.HasPrefix(file.relPath, "internal/core/domain/") {
			continue
		}

		for _, imported := range importsOf(t, file.absPath) {
			for _, banned := range forbidden {
				if !strings.HasPrefix(imported, banned) {
					continue
				}

				t.Errorf(
					"%s imports %s; internal/core/domain must stay pure — "+
						"depend on ports instead and let the composition root inject an adapter",
					file.relPath,
					imported,
				)
			}
		}
	}
}

// TestInfraDoesNotImportApp pins the direction of the hexagon: adapters
// implement ports, so nothing under internal/core may reach up into the
// application or UI layers.
//
// This inversion was real and invisible until the overlay managers moved into
// infra: every overlay backend imported internal/app/components for its render
// models. The fix was to move those render packages down into
// internal/core/infra/overlay/render/, not to allow the edge.
func TestInfraDoesNotImportApp(t *testing.T) {
	for _, file := range goFiles(t) {
		if !strings.HasPrefix(file.relPath, "internal/core/") {
			continue
		}

		for _, imported := range importsOf(t, file.absPath) {
			if !strings.HasPrefix(imported, "internal/app") &&
				!strings.HasPrefix(imported, "internal/ui") {
				continue
			}

			t.Errorf(
				"%s imports %s; internal/core must not depend on the app or UI "+
					"layer — move the shared type down into core, or invert the "+
					"dependency with a port",
				file.relPath,
				imported,
			)
		}
	}
}

// sharedInfraPackages may be imported from anywhere.
//
// These are not OS capabilities behind a port; they are shared vocabulary and
// process-level plumbing, and routing them through an interface would add
// indirection without adding a seam:
//
//   - infra/ipc      the CLI/daemon wire protocol (Command, Response, Code*).
//     The CLI is a client of this protocol by definition.
//   - infra/logger   logger construction and log-path resolution.
//   - infra/platform the SystemPort factory and the contributor-facing Profile
//     that `neru doctor` prints. Selecting an implementation is the composition
//     root's job, and reporting the profile is diagnostics.
//   - infra/overlay  overlay render models the app must name to draw
//     (overlay.Mode, hints.Hint, grid.Style). These are data, not behavior;
//     the behavior is behind ports.OverlayPort.
var sharedInfraPackages = []string{
	"internal/core/infra/ipc",
	"internal/core/infra/logger",
	"internal/core/infra/platform",
	"internal/core/infra/overlay",
}

// compositionRootFiles wire concrete adapters to ports. Reaching into infra is
// precisely their job — they are the one place that is allowed to know which
// implementation exists.
var compositionRootFiles = []string{
	"internal/app/initialization.go",
	"internal/app/app_initialization_steps.go",
	"internal/app/component_factory.go",
	"cmd/neru/main.go",
}

// knownLayeringExceptions are edges that are still wrong but not yet fixed.
//
// They are listed individually, with a reason, so the count can only go down:
// a new violation fails the test, while these stay visible instead of hiding
// behind a broad rule. Deleting an entry once it is fixed is the point.
// It is currently empty, which is the goal state. Add an entry only when a
// violation genuinely cannot be fixed in the same change, never to silence one.
var knownLayeringExceptions = map[string]string{}

// TestAppReachesInfraOnlyThroughPorts is the Tier-1 rule: application and
// domain code depends on ports, and only the composition root knows which
// adapter satisfies them.
func TestAppReachesInfraOnlyThroughPorts(t *testing.T) {
	for _, file := range goFiles(t) {
		if !isApplicationLayer(file.relPath) {
			continue
		}

		if slices.Contains(compositionRootFiles, file.relPath) {
			continue
		}

		// Build-tagged files are Tier-2 platform dispatch. They exist to keep
		// platform knowledge out of shared code, so reaching a platform package
		// is their whole purpose.
		if isPlatformDispatchFile(file.base) {
			continue
		}

		if _, known := knownLayeringExceptions[file.relPath]; known {
			continue
		}

		for _, imported := range importsOf(t, file.absPath) {
			if !strings.HasPrefix(imported, "internal/core/infra") {
				continue
			}

			if isSharedInfraPackage(imported) {
				continue
			}

			t.Errorf(
				"%s imports %s; application code must depend on internal/core/ports "+
					"and let the composition root inject the adapter "+
					"(docs/CROSS_PLATFORM.md, The Three Tiers)",
				file.relPath,
				imported,
			)
		}
	}
}

// TestKnownLayeringExceptionsAreStillReal keeps the exception list honest: an
// entry that no longer violates anything is stale and must be deleted, or the
// list slowly becomes a place where rules go to die.
func TestKnownLayeringExceptionsAreStillReal(t *testing.T) {
	for relPath, reason := range knownLayeringExceptions {
		absPath := filepath.Join(findRepoRoot(t), filepath.FromSlash(relPath))

		_, statErr := os.Stat(absPath)
		if statErr != nil {
			t.Errorf("known exception %s no longer exists; remove it from the list", relPath)

			continue
		}

		stillViolates := false

		for _, imported := range importsOf(t, absPath) {
			if strings.HasPrefix(imported, "internal/core/infra") &&
				!isSharedInfraPackage(imported) {
				stillViolates = true

				break
			}
		}

		if !stillViolates {
			t.Errorf(
				"known exception %s no longer imports infra (%q) — delete its entry",
				relPath,
				reason,
			)
		}
	}
}

func isApplicationLayer(relPath string) bool {
	return strings.HasPrefix(relPath, "internal/app/") ||
		strings.HasPrefix(relPath, "internal/ui/") ||
		strings.HasPrefix(relPath, "internal/cli/") ||
		strings.HasPrefix(relPath, "cmd/")
}

func isSharedInfraPackage(imported string) bool {
	for _, shared := range sharedInfraPackages {
		if imported == shared || strings.HasPrefix(imported, shared+"/") {
			return true
		}
	}

	return false
}

// isPlatformDispatchFile reports whether a filename marks a build-tagged
// platform slot, using the same suffix vocabulary as platform_slots_test.go.
func isPlatformDispatchFile(base string) bool {
	name := strings.TrimSuffix(base, ".go")

	for segment := range strings.SplitSeq(name, "_") {
		switch segment {
		case osDarwin, osLinux, osWindows, "other", "unix":
			return true
		}
	}

	return false
}

// importsOf returns the module-relative import paths of a Go file, dropping
// stdlib and third-party imports.
func importsOf(t *testing.T, path string) []string {
	t.Helper()

	fileSet := token.NewFileSet()

	parsed, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v", path, err)
	}

	imports := make([]string, 0, len(parsed.Imports))

	for _, spec := range parsed.Imports {
		importPath := strings.Trim(spec.Path.Value, `"`)

		after, isLocal := strings.CutPrefix(importPath, modulePrefix)
		if !isLocal {
			continue
		}

		imports = append(imports, after)
	}

	return imports
}
