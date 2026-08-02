package architecture_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const forbiddenImport = "github.com/y3owk1n/neru/internal/adapter/platform/darwin"

// skippedWalkDirs are the directories every repository walk in this package
// skips: version control, build outputs and vendored third-party code, none of
// which contain first-party source subject to these guardrails.
var skippedWalkDirs = map[string]bool{
	".git": true, "bin": true, "build": true,
	"node_modules": true, "vendor": true,
}

// isSkippedWalkDir reports whether a directory should be pruned from a walk.
func isSkippedWalkDir(name string) bool { return skippedWalkDirs[name] }

func TestNonDarwinFilesDoNotImportDarwinPlatformPackage(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fileSet := token.NewFileSet()

	walkErr := filepath.WalkDir(repoRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if isSkippedWalkDir(entry.Name()) {
				return filepath.SkipDir
			}

			return nil
		}

		if filepath.Ext(path) != ".go" {
			return nil
		}

		relPath, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return relErr
		}

		slashed := filepath.ToSlash(relPath)

		// A file may reach the darwin bridge when it is darwin-only, and there
		// are two ways to be darwin-only: the filename says so, or the whole
		// directory does. Matching the directory rather than naming packages
		// means a new darwin backend needs no edit here, and
		// TestPlatformPackagesTagEveryFile is what keeps such a directory
		// honest — every file in it must carry the build tag.
		if strings.Contains(slashed, "/darwin/") ||
			strings.HasSuffix(slashed, "_darwin.go") ||
			strings.HasSuffix(slashed, "integration_darwin_test.go") {
			return nil
		}

		parsedFile, parseErr := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}

		for _, imp := range parsedFile.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if importPath == forbiddenImport {
				t.Errorf("%s imports forbidden darwin platform package", slashed)
			}
		}

		return nil
	})
	if walkErr != nil {
		t.Fatalf("WalkDir() error = %v", walkErr)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	return filepath.Dir(filepath.Dir(filepath.Dir(currentFile)))
}
