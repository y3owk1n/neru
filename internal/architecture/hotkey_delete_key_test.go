package architecture_test

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/keyvocab"
)

// Where each platform turns the base key of a [hotkeys] chord into the
// physical key it grabs. macOS is read through the parser in
// named_key_tables_test.go; the other three are Go switches.
const (
	hotkeyKeyWindowsSource = "internal/adapter/platform/windows/keys.go"
	hotkeyKeyWindowsFunc   = "nameToVirtualKey"

	hotkeyKeyX11Source = "internal/adapter/hotkeys/linux/x11_cgo.go"
	hotkeyKeyX11Func   = "x11KeysymFor"

	// The Wayland listener spells a configured chord through two folds: the
	// binding-side one, which is where "Delete" is given its meaning, and the
	// one it shares with the live side.
	hotkeyKeyEvdevSource      = "internal/adapter/eventtap/linux/global_hotkey_keys.go"
	hotkeyKeyEvdevBindingFunc = "canonicalBindingBaseKey"
	hotkeyKeyEvdevBaseFunc    = "canonicalBaseKey"

	// hotkeyKeyVocabPackage and hotkeyKeyVocabPrefix are how a Go switch spells
	// a named key when it compares against the vocabulary's declaration
	// (keyvocab.KeyDelete) rather than a literal.
	hotkeyKeyVocabPackage = "keyvocab"
	hotkeyKeyVocabPrefix  = "Key"
)

// hotkeyKeyGoTable is one platform's Go-side lookup: the file, and the
// functions whose switch cases spell it, searched in order.
type hotkeyKeyGoTable struct {
	platform string
	source   string
	funcs    []string
}

var hotkeyKeyGoTables = []hotkeyKeyGoTable{
	{platform: osWindows, source: hotkeyKeyWindowsSource, funcs: []string{hotkeyKeyWindowsFunc}},
	{platform: "linux/x11", source: hotkeyKeyX11Source, funcs: []string{hotkeyKeyX11Func}},
	{
		platform: "linux/wayland",
		source:   hotkeyKeyEvdevSource,
		funcs:    []string{hotkeyKeyEvdevBindingFunc, hotkeyKeyEvdevBaseFunc},
	},
}

// TestEveryHotkeyTableBindsDeleteAndBackspaceToOneKey keeps a [hotkeys]
// binding on "Delete" meaning the same physical key on every platform: the
// backspace key, which is what "Backspace" binds and what macOS, the
// reference, calls kVK_Delete. Inside a mode the two names are already one
// key, by the vocabulary's alias; this pins the four hotkey tables, which
// resolve names on their own, to the same answer.
//
// It reads each table as a map from the name a config writes to the symbol
// the table returns for it, and asks that "delete" and "backspace" return the
// same symbol. Which symbol is the platform's business.
func TestEveryHotkeyTableBindsDeleteAndBackspaceToOneKey(t *testing.T) {
	t.Parallel()

	tables := map[string]map[string]string{
		"darwin": lowercaseKeys(readDarwinNamedKeyTables(t).inbound),
	}

	for _, table := range hotkeyKeyGoTables {
		tables[table.platform] = readHotkeyKeyGoTable(t, table)
	}

	deleteName := strings.ToLower(keyvocab.KeyDelete)
	backspaceName := strings.ToLower(keyvocab.KeyBackspace)

	for platform, table := range tables {
		deleteSymbol, deleteKnown := table[deleteName]
		backspaceSymbol, backspaceKnown := table[backspaceName]

		switch {
		case !deleteKnown || !backspaceKnown:
			t.Errorf(
				"%s: hotkey table resolves Delete=%t Backspace=%t; both names must reach a key",
				platform, deleteKnown, backspaceKnown,
			)
		case deleteSymbol != backspaceSymbol:
			t.Errorf(
				"%s: a [hotkeys] Delete grabs %s and Backspace grabs %s; both must name the backspace key",
				platform,
				deleteSymbol,
				backspaceSymbol,
			)
		}
	}
}

// readHotkeyKeyGoTable reads one Go-side table: for every case in every
// switch of the named functions, the name the case matches — lowercased, with
// a literal, a file-level constant, or a keyvocab.Key* reference all read as
// the name they spell — and the first expression the clause returns.
func readHotkeyKeyGoTable(t *testing.T, table hotkeyKeyGoTable) map[string]string {
	t.Helper()

	fileSet := token.NewFileSet()
	absPath := filepath.Join(findRepoRoot(t), filepath.FromSlash(table.source))
	file := parseRepoGoFile(t, fileSet, absPath, table.source)
	consts := stringConstants(file)

	resolved := make(map[string]string)

	for _, funcName := range table.funcs {
		decl := funcDeclNamed(file, funcName)
		if decl == nil {
			t.Fatalf("%s: %s not found (renamed?)", table.source, funcName)
		}

		ast.Inspect(decl.Body, func(node ast.Node) bool {
			clause, isClause := node.(*ast.CaseClause)
			if !isClause {
				return true
			}

			returned := firstReturnedExpr(clause.Body)
			if returned == "" {
				return true
			}

			for _, expr := range clause.List {
				name, isName := hotkeyKeyCaseName(expr, consts)
				if !isName {
					continue
				}

				if _, seen := resolved[name]; !seen {
					resolved[name] = returned
				}
			}

			return true
		})
	}

	return resolved
}

// hotkeyKeyCaseName reads the name one case expression matches. It answers for
// string literals, file-level string constants, and keyvocab.Key* references;
// anything else is not a named key and is skipped.
func hotkeyKeyCaseName(expr ast.Expr, consts map[string]string) (string, bool) {
	switch expr := expr.(type) {
	case *ast.BasicLit:
		if expr.Kind != token.STRING {
			return "", false
		}

		value, err := strconv.Unquote(expr.Value)
		if err != nil {
			return "", false
		}

		return strings.ToLower(value), true
	case *ast.Ident:
		value, isConst := consts[expr.Name]
		if !isConst {
			return "", false
		}

		return strings.ToLower(value), true
	case *ast.SelectorExpr:
		pkg, isIdent := expr.X.(*ast.Ident)
		if !isIdent || pkg.Name != hotkeyKeyVocabPackage ||
			!strings.HasPrefix(expr.Sel.Name, hotkeyKeyVocabPrefix) {
			return "", false
		}

		return strings.ToLower(strings.TrimPrefix(expr.Sel.Name, hotkeyKeyVocabPrefix)), true
	default:
		return "", false
	}
}

// firstReturnedExpr renders the first result of the first return statement
// among the given statements, or "" when there is none.
func firstReturnedExpr(body []ast.Stmt) string {
	for _, stmt := range body {
		ret, isReturn := stmt.(*ast.ReturnStmt)
		if isReturn && len(ret.Results) > 0 {
			return types.ExprString(ret.Results[0])
		}
	}

	return ""
}

// stringConstants collects the file-level constants declared with a string
// literal, by name.
func stringConstants(file *ast.File) map[string]string {
	consts := make(map[string]string)

	for _, decl := range file.Decls {
		gen, isGen := decl.(*ast.GenDecl)
		if !isGen || gen.Tok != token.CONST {
			continue
		}

		for _, spec := range gen.Specs {
			value, isValue := spec.(*ast.ValueSpec)
			if !isValue {
				continue
			}

			for i, name := range value.Names {
				if i >= len(value.Values) {
					break
				}

				lit, isLit := value.Values[i].(*ast.BasicLit)
				if !isLit || lit.Kind != token.STRING {
					continue
				}

				unquoted, err := strconv.Unquote(lit.Value)
				if err != nil {
					continue
				}

				consts[name.Name] = unquoted
			}
		}
	}

	return consts
}

// funcDeclNamed finds a top-level function by name.
func funcDeclNamed(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if isFunc && fn.Recv == nil && fn.Name.Name == name {
			return fn
		}
	}

	return nil
}

// lowercaseKeys re-keys a name table by the lowercase name, which is how the
// Go tables are read.
func lowercaseKeys(table map[string]string) map[string]string {
	lowered := make(map[string]string, len(table))
	for name, symbol := range table {
		lowered[strings.ToLower(name)] = symbol
	}

	return lowered
}
