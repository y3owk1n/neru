package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// docGoFile is the package-comment file name skipped by file-scanning checks.
const docGoFile = "doc.go"

var portsWithoutOwnMock = map[string]string{}

// TestEveryPortHasAMock enforces the third requirement of Tier 1 in
// docs/CROSS_PLATFORM.md: a port is not done until it has a mock.
//
// Without one, consumers grow hand-rolled fakes in _test.go files that go stale
// the moment the contract changes — which is exactly how two hotkey fakes ended
// up silently under-implementing HotkeyPort.
//
// portsWithoutOwnMock are interfaces named *Port that are composed into a
// larger port rather than injected on their own. They are exercised through
// the mock of the port that embeds them, so a separate mock would be dead code.
//
// All four are sub-interfaces of SystemPort, covered by MockSystemPort.
func TestEveryPortHasAMock(t *testing.T) {
	repoRoot := findRepoRoot(t)
	portNames := interfaceNamesIn(t, filepath.Join(repoRoot, "internal", "ports"))
	mockNames := typeNamesIn(t, filepath.Join(repoRoot, "internal", "ports", "mocks"))

	for _, name := range portNames {
		if !strings.HasSuffix(name, "Port") {
			continue
		}

		if _, exempt := portsWithoutOwnMock[name]; exempt {
			continue
		}

		if _, ok := mockNames["Mock"+name]; !ok {
			t.Errorf(
				"ports.%s has no Mock%s in internal/ports/mocks; every port "+
					"needs a mock (docs/CROSS_PLATFORM.md, Tier 1)",
				name,
				name,
			)
		}
	}
}

// TestMockExemptionsAreStillReal keeps the exemption list from outliving the
// interfaces it names.
func TestMockExemptionsAreStillReal(t *testing.T) {
	repoRoot := findRepoRoot(t)
	portNames := interfaceNamesIn(t, filepath.Join(repoRoot, "internal", "ports"))

	declared := make(map[string]bool, len(portNames))
	for _, name := range portNames {
		declared[name] = true
	}

	for name, reason := range portsWithoutOwnMock {
		if !declared[name] {
			t.Errorf(
				"ports.%s is in the mock-exemption list (%q) but no longer exists; "+
					"remove the entry",
				name,
				reason,
			)
		}
	}
}

// interfaceNamesIn returns the exported interface type names declared in a
// package directory, skipping test files.
func interfaceNamesIn(t *testing.T, dir string) []string {
	t.Helper()

	var names []string

	for _, decl := range typeDeclsIn(t, dir) {
		if _, isInterface := decl.Type.(*ast.InterfaceType); !isInterface {
			continue
		}

		if decl.Name.IsExported() {
			names = append(names, decl.Name.Name)
		}
	}

	return names
}

// typeNamesIn returns every exported type name declared in a package directory.
func typeNamesIn(t *testing.T, dir string) map[string]struct{} {
	t.Helper()

	names := make(map[string]struct{})

	for _, decl := range typeDeclsIn(t, dir) {
		if decl.Name.IsExported() {
			names[decl.Name.Name] = struct{}{}
		}
	}

	return names
}

func typeDeclsIn(t *testing.T, dir string) []*ast.TypeSpec {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error = %v", dir, err)
	}

	fileSet := token.NewFileSet()

	var specs []*ast.TypeSpec

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		parsed, parseErr := parser.ParseFile(fileSet, filepath.Join(dir, name), nil, 0)
		if parseErr != nil {
			t.Fatalf("ParseFile(%s) error = %v", name, parseErr)
		}

		for _, decl := range parsed.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}

			for _, spec := range genDecl.Specs {
				if typeSpec, isType := spec.(*ast.TypeSpec); isType {
					specs = append(specs, typeSpec)
				}
			}
		}
	}

	return specs
}

// TestEveryMockAssertsInterfaceSatisfaction requires every mock type in the
// mocks package to carry a compile-time assertion tying it to its interface
// (var _ ports.X = (*MockX)(nil), plain or inside a var block). Without one,
// a mock can drop or fumble a method and still compile on its own — the
// break only surfaces at some consumer, far from the cause. The assertion
// moves that failure into the mocks package itself. The check is per mock
// type, not per file, so a second mock added to an existing file cannot
// hide behind the first one's assertion.
func TestEveryMockAssertsInterfaceSatisfaction(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRoot(t)
	mocksDir := filepath.Join(repoRoot, "internal", "ports", "mocks")

	entries, err := os.ReadDir(mocksDir)
	if err != nil {
		t.Fatalf("reading mocks dir: %v", err)
	}

	mockTypePattern := regexp.MustCompile(`(?m)^type (Mock\w+) struct`)
	assertionPattern := regexp.MustCompile(`\(\*(Mock\w+)\)\(nil\)`)

	declared := map[string]string{} // mock type -> file
	asserted := map[string]bool{}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") ||
			name == docGoFile {
			continue
		}

		content, readErr := os.ReadFile(filepath.Join(mocksDir, name))
		if readErr != nil {
			t.Fatalf("reading %s: %v", name, readErr)
		}

		for _, match := range mockTypePattern.FindAllStringSubmatch(string(content), -1) {
			declared[match[1]] = name
		}

		for _, match := range assertionPattern.FindAllStringSubmatch(string(content), -1) {
			asserted[match[1]] = true
		}
	}

	if len(declared) == 0 {
		t.Fatal("found no mock types under internal/ports/mocks; the scan is broken")
	}

	for mockType, file := range declared {
		if !asserted[mockType] {
			t.Errorf(
				"internal/ports/mocks/%s declares %s but no compile-time interface "+
					"assertion for it; add `var _ ports.<Interface> = (*%s)(nil)` so "+
					"drift from the interface fails in this package, not at a consumer",
				file, mockType, mockType,
			)
		}
	}
}
