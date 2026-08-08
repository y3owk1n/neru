package architecture_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

const forbiddenImport = "github.com/y3owk1n/neru/internal/adapter/platform/darwin"

func TestNonDarwinFilesDoNotImportDarwinPlatformPackage(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fileSet := token.NewFileSet()
	checked := 0

	walkRepoFiles(t, repoRoot, func(file repoFile) {
		if filepath.Ext(file.name) != goExt {
			return
		}

		// A file may reach the darwin bridge when it is darwin-only, and there
		// are two ways to be darwin-only: the filename says so, or the whole
		// directory does. Matching the directory rather than naming packages
		// means a new darwin backend needs no edit here, and
		// TestPlatformPackagesTagEveryFile is what keeps such a directory
		// honest — every file in it must carry the build tag.
		if strings.Contains(file.rel, "/darwin/") ||
			strings.HasSuffix(file.rel, "_darwin.go") ||
			strings.HasSuffix(file.rel, "integration_darwin_test.go") {
			return
		}

		parsedFile, parseErr := parser.ParseFile(fileSet, file.abs, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("ParseFile(%s) error = %v", file.rel, parseErr)
		}

		checked++

		for _, imp := range parsedFile.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if importPath == forbiddenImport {
				t.Errorf("%s imports forbidden darwin platform package", file.rel)
			}
		}
	})

	assertWalkedAtLeast(t, "Go files that may not reach the darwin bridge", checked, bulkWalkFloor)
}
