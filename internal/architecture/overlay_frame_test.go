package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// frameSealMethod is the unexported method every ports.Frame carries. It seals
// the interface to the ports package, and it is how this test recognizes a
// frame type without needing to know the list.
const frameSealMethod = "frame"

// TestFrameCarriesDomainValuesOnly enforces the constraint the overlay Frame
// exists under: a Frame carries domain values, never a resolved Style, a
// render model or a platform handle (#1210, ADR 0003).
//
// It is the reason the layering ban is expressible at all. An app package that
// had to name a render model to describe what should be on screen would keep
// the overlay package in the layering test's shared-infrastructure allowlist
// forever, and #1213 could never remove it. A field typed from an adapter
// package is that leak, caught here rather than three issues later.
func TestFrameCarriesDomainValuesOnly(t *testing.T) {
	repoRoot := findRepoRoot(t)
	portsDir := filepath.Join(repoRoot, "internal", "ports")

	files := parsedGoFiles(t, portsDir)

	frames := frameTypeNames(files)
	if len(frames) == 0 {
		t.Fatal(
			"no ports.Frame implementations found; the seal method was renamed or the check is dead",
		)
	}

	for _, file := range files {
		for _, spec := range typeSpecsIn(file) {
			if !frames[spec.Name.Name] {
				continue
			}

			structType, isStruct := spec.Type.(*ast.StructType)
			if !isStruct {
				continue
			}

			assertFieldsAreDomainValues(t, spec.Name.Name, structType, importPathsIn(file))
		}
	}
}

// assertFieldsAreDomainValues reports every field of a frame whose type comes
// from outside the standard library and the domain.
func assertFieldsAreDomainValues(
	t *testing.T,
	frameName string,
	structType *ast.StructType,
	imports map[string]string,
) {
	t.Helper()

	for _, field := range structType.Fields.List {
		for _, qualifier := range packageQualifiersIn(field.Type) {
			path, known := imports[qualifier]
			if !known {
				t.Errorf(
					"ports.%s has a field qualified by %q, which the file does not import",
					frameName,
					qualifier,
				)

				continue
			}

			if isDomainValuePackage(path) {
				continue
			}

			t.Errorf(
				"ports.%s carries a field from %q; a Frame carries domain values only "+
					"— no Style, render model or platform type (#1210, docs/adr/0003)",
				frameName,
				path,
			)
		}
	}
}

// isDomainValuePackage reports whether a Frame field may be typed from a
// package: the standard library, or the domain this repo models.
func isDomainValuePackage(path string) bool {
	const repo = "github.com/y3owk1n/neru/"

	if !strings.HasPrefix(path, repo) {
		// Anything outside this module is either the standard library or a
		// third-party value type; neither can be an overlay render model.
		return true
	}

	return strings.HasPrefix(path, repo+"internal/domain/")
}

// frameTypeNames returns every type in the parsed files that implements the
// sealed Frame interface, keyed by name.
func frameTypeNames(files []*ast.File) map[string]bool {
	names := make(map[string]bool)

	for _, file := range files {
		for _, decl := range file.Decls {
			funcDecl, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || funcDecl.Name.Name != frameSealMethod {
				continue
			}

			if funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
				continue
			}

			if name := receiverTypeName(funcDecl.Recv.List[0].Type); name != "" {
				names[name] = true
			}
		}
	}

	return names
}

// receiverTypeName names the type a method is declared on, pointer or not.
func receiverTypeName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(typed.X)
	case *ast.Ident:
		return typed.Name
	default:
		return ""
	}
}

// packageQualifiersIn collects every package qualifier a type expression
// names — the "hint" of hint.Interface, the "image" of image.Rectangle.
func packageQualifiersIn(expr ast.Expr) []string {
	var qualifiers []string

	ast.Inspect(expr, func(node ast.Node) bool {
		selector, isSelector := node.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}

		if ident, isIdent := selector.X.(*ast.Ident); isIdent {
			qualifiers = append(qualifiers, ident.Name)
		}

		return true
	})

	return qualifiers
}

// importPathsIn maps the name a file refers to each import by onto its path.
func importPathsIn(file *ast.File) map[string]string {
	paths := make(map[string]string, len(file.Imports))

	for _, imported := range file.Imports {
		path := strings.Trim(imported.Path.Value, `"`)

		name := path[strings.LastIndex(path, "/")+1:]
		if imported.Name != nil {
			name = imported.Name.Name
		}

		paths[name] = path
	}

	return paths
}

// typeSpecsIn returns every type declaration in one parsed file.
func typeSpecsIn(file *ast.File) []*ast.TypeSpec {
	var specs []*ast.TypeSpec

	for _, decl := range file.Decls {
		genDecl, isGen := decl.(*ast.GenDecl)
		if !isGen || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			if typeSpec, isType := spec.(*ast.TypeSpec); isType {
				specs = append(specs, typeSpec)
			}
		}
	}

	return specs
}

// parsedGoFiles parses every non-test Go file in one package directory.
func parsedGoFiles(t *testing.T, dir string) []*ast.File {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error = %v", dir, err)
	}

	fileSet := token.NewFileSet()

	var files []*ast.File

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		parsed, parseErr := parser.ParseFile(fileSet, filepath.Join(dir, name), nil, 0)
		if parseErr != nil {
			t.Fatalf("ParseFile(%s) error = %v", name, parseErr)
		}

		files = append(files, parsed)
	}

	return files
}
