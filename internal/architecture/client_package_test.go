package architecture_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// clientPackage is the public client package, relative to the module root.
const clientPackage = "client"

// clientReach is everything the public client package may build on: inside
// the module, the endpoint lookup and the error vocabulary it uses, and
// outside it, the named-pipe libraries Windows dials through.
var clientReach = struct {
	module   []string
	external []string
}{
	module:   []string{clientPackage, "internal/derrors", "internal/ipcendpoint"},
	external: []string{"github.com/Microsoft/go-winio", "golang.org/x/sys/"},
}

// TestTheClientPackageStaysThin pins what importing github.com/y3owk1n/neru/client
// costs a Go program: the standard library, the endpoint lookup, and the
// pipe libraries Windows needs. A client that reached the daemon's own
// packages would pull ports, config and every adapter behind them into the
// program, and cgo with them.
func TestTheClientPackageStaysThin(t *testing.T) {
	importsByDir := map[string][]string{}

	for _, file := range goFiles(t) {
		dir := filepath.ToSlash(filepath.Dir(file.relPath))
		importsByDir[dir] = append(importsByDir[dir], fullImportsOf(t, file.absPath)...)
	}

	seen := map[string]bool{}
	pending := []string{clientPackage}

	for len(pending) > 0 {
		dir := pending[0]
		pending = pending[1:]

		if seen[dir] {
			continue
		}

		seen[dir] = true

		if len(importsByDir[dir]) == 0 && dir == clientPackage {
			t.Fatal("found no imports in client; the walk is not reading the package")
		}

		for _, imported := range importsByDir[dir] {
			if inModule, found := strings.CutPrefix(imported, modulePrefix); found {
				if !slices.Contains(clientReach.module, inModule) {
					t.Errorf("client reaches %s through %s; keep the client on the endpoint lookup",
						imported, dir)
				}

				pending = append(pending, inModule)

				continue
			}

			first, _, _ := strings.Cut(imported, "/")
			if !strings.Contains(first, ".") {
				continue
			}

			allowed := slices.ContainsFunc(clientReach.external, func(prefix string) bool {
				return strings.HasPrefix(imported, prefix)
			})
			if !allowed {
				t.Errorf("client reaches %s through %s; a client program should not need it",
					imported, dir)
			}
		}
	}
}

// fullImportsOf returns every import path in the file at path, the standard
// library and other modules included, where importsOf keeps only this
// module's.
func fullImportsOf(t *testing.T, path string) []string {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v", path, err)
	}

	imports := make([]string, 0, len(parsed.Imports))

	for _, spec := range parsed.Imports {
		importPath, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			t.Fatalf("%s: import %s: %v", path, spec.Path.Value, unquoteErr)
		}

		imports = append(imports, importPath)
	}

	return imports
}
