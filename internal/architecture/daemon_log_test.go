package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// visibleLogLevels are the zap logger methods that write at or above the
// default threshold, which is info. A daemon logging at its defaults emits
// every one of these, so what they carry reaches a file on disk on a machine
// nobody configured.
//
// Debug is deliberately absent: it is opt-in, and the rule this file pins is
// about what a default install writes down.
var visibleLogLevels = map[string]bool{
	"Info": true, "Warn": true, "Error": true,
	"DPanic": true, "Panic": true, "Fatal": true,
}

// contentFieldNames are the zap field names that mean "the thing itself" rather
// than a fact about it: a config value the user typed, a shell command line
// from their configuration, or what that command printed. AGENTS.md forbids all
// three — counts, durations, IDs and booleans are what a log is entitled to.
//
// The match is on the exact name, which is what leaves the correct idiom
// spelling itself: cmd_length and output_bytes say the same thing about the
// same data and pass, because a length is not the content.
var contentFieldNames = map[string]bool{
	"cmd": true, "command": true, "output": true, "value": true,
}

// TestNoVisibleLogFieldNamesConfigValuesOrCommandOutput pins the half of the
// privacy contract a reviewer cannot hold on their own.
//
// Every field here was correct once. The failure path of an exec step logged
// the command line and its combined output at Error while the success path
// three lines below logged only their sizes, and two config-set paths logged
// the value beside the key while the IPC controller beside them explained in a
// comment why it must not. The regression is invisible in review precisely
// because each site reads like helpful diagnostics.
//
// This reads the call, not the data: a field constructed elsewhere and passed
// in by variable is not caught. That is the trade for a check with no false
// positives — the shape it does catch is the shape every one of these
// regressions took.
func TestNoVisibleLogFieldNamesConfigValuesOrCommandOutput(t *testing.T) {
	var offenders []string

	fset := token.NewFileSet()
	inspected := 0

	for _, file := range goFiles(t) {
		parsed, parseErr := parser.ParseFile(fset, file.absPath, nil, 0)
		if parseErr != nil {
			t.Fatalf("ParseFile(%s) error = %v", file.relPath, parseErr)
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}

			selector, isSelector := call.Fun.(*ast.SelectorExpr)
			if !isSelector || !visibleLogLevels[selector.Sel.Name] {
				return true
			}

			names := zapFieldNames(call.Args)
			if len(names) == 0 {
				return true
			}

			inspected++

			for _, name := range names {
				if !contentFieldNames[name] {
					continue
				}

				offenders = append(offenders,
					fset.Position(call.Pos()).String()+"\t"+
						selector.Sel.Name+"("+strconv.Quote(name)+")")
			}

			return true
		})
	}

	assertWalkedAtLeast(t, "log calls above debug level", inspected, bulkWalkFloor)

	reportOffenders(t, offenders,
		"log call names the content itself rather than a fact about it; "+
			"log a length, a count or an exit code instead, and let the error "+
			"returned to the caller carry the detail")
}

// outputRedirectMarkers are the ways this repository decides where the daemon's
// standard output and standard error go: the two launchd plist keys, and the
// detached launch the macOS installer offers. A file naming one of them is a
// file that answers "where does the daemon's output land".
var outputRedirectMarkers = []string{
	"StandardOutPath", "StandardErrorPath", "nohup",
}

// sharedTempPaths are the directories every local user can read and write.
// macOS mounts /tmp mode 1777 and shares it across users, unlike the per-user
// $TMPDIR, so a log parked there is readable by anyone logged in and its name
// is plantable by anyone who gets there first.
var sharedTempPaths = []string{"/tmp/", "/var/tmp/"}

// TestNoServiceDefinitionWritesDaemonOutputToASharedPath keeps the daemon's
// output in the user's own log directory.
//
// The service definitions are written four times over — the plist the CLI
// generates, the plist template shipped for a hand install, the installer
// script's detached launch, and the Nix modules — so the answer to where a log
// goes is only as good as the copy nobody remembered to change. This judges
// every file that decides it, whatever language it is written in.
//
// A file that redirects nothing drops out of the subject set, which is the
// intended behavior: it has no answer to be wrong about, and adding a redirect
// back puts it under this rule again.
func TestNoServiceDefinitionWritesDaemonOutputToASharedPath(t *testing.T) {
	subjects := 0

	walkRepoFiles(t, findRepoRoot(t), func(file repoFile) {
		// This package's own files name the markers to describe the rule.
		if file.dir == architecturePackageDir {
			return
		}

		// A test file installs no service. One of them asserts that the plist
		// it renders names no shared directory, which means quoting the
		// directory — a subject set that judged it would be judging the rule's
		// own statement of itself.
		if strings.HasSuffix(file.name, "_test.go") {
			return
		}

		// The walk hands over symlinks without following them, and one of them
		// points at a directory (.claude/skills), which is not a file to read.
		info, statErr := os.Lstat(file.abs)
		if statErr != nil {
			t.Fatalf("Lstat(%s) error = %v", file.rel, statErr)
		}

		if !info.Mode().IsRegular() {
			return
		}

		content, readErr := os.ReadFile(file.abs)
		if readErr != nil {
			t.Fatalf("ReadFile(%s) error = %v", file.rel, readErr)
		}

		text := string(content)

		if !containsAny(text, outputRedirectMarkers) {
			return
		}

		subjects++

		for _, shared := range sharedTempPaths {
			if !strings.Contains(text, shared) {
				continue
			}

			t.Errorf(
				"%s decides where the daemon's output goes and names %s, which "+
					"every local user can read and plant a symlink in; use the "+
					"per-user log directory the logger already resolves",
				file.rel, shared,
			)
		}
	})

	assertWalkedAtLeast(t, "files redirecting daemon output", subjects, serviceDefinitionFloor)
}

// serviceDefinitionFloor is the fewest files expected to redirect the daemon's
// output. Four do today — the generated plist, the shipped template, the
// installer script and the home-manager module — so three catches a check that
// has stopped recognizing them without firing when one legitimately stops
// redirecting.
const serviceDefinitionFloor = 3

// containsAny reports whether text contains any of the needles.
func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}

	return false
}

// zapFieldNames returns the field names of the zap.X("name", …) constructors
// among args, which is how every structured log call in this tree is written.
func zapFieldNames(args []ast.Expr) []string {
	var names []string

	for _, arg := range args {
		call, isCall := arg.(*ast.CallExpr)
		if !isCall || len(call.Args) == 0 {
			continue
		}

		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector {
			continue
		}

		pkg, isIdent := selector.X.(*ast.Ident)
		if !isIdent || pkg.Name != "zap" {
			continue
		}

		literal, isLiteral := call.Args[0].(*ast.BasicLit)
		if !isLiteral || literal.Kind != token.STRING {
			continue
		}

		name, unquoteErr := strconv.Unquote(literal.Value)
		if unquoteErr != nil {
			continue
		}

		names = append(names, name)
	}

	return names
}
