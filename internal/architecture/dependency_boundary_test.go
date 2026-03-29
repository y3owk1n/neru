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

const forbiddenImport = "github.com/y3owk1n/neru/internal/core/infra/platform/darwin"

func TestNonDarwinFilesDoNotImportDarwinPlatformPackage(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fileSet := token.NewFileSet()

	walkErr := filepath.WalkDir(repoRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "bin", "build":
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
		if strings.Contains(slashed, "/platform/darwin/") ||
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
