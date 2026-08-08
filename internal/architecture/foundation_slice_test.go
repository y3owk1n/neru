package architecture_test

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// These tests pin the cross-platform foundation slice: the package list that
// `just test-foundation` runs, documented in docs/DEVELOPMENT.md as the fast
// check to run before or during Linux/Windows work.
//
// The slice only earns that description if it holds every package whose
// behavior really is identical on every target. Kept by hand it drifted both
// ways — nine eligible packages missing, one platform-dependent package listed
// — and nothing noticed, because nothing checked. This is the check.
//
// The rule lives here and nowhere else. `just list-foundation-packages` prints
// what this file computes rather than deciding for itself, so there is no
// second definition of "platform-free" to disagree with the first.
const (
	justfileName     = "justfile"
	foundationRecipe = "test-foundation"
)

// ciWorkflow is what CI actually runs. It invokes the individual just recipes
// rather than the justfile's own `ci` recipe, so a recipe reachable only from
// `ci` is still ungated — which is why this file checks both.
const ciWorkflow = ".github/workflows/ci.yml"

// localCIRecipe is the recipe CONTRIBUTING.md and AGENTS.md call "exactly what
// CI gates your PR on". That claim is only true while it reaches everything
// ciWorkflow reaches.
const localCIRecipe = "ci"

// listFoundationEnv makes the test print the slice it computes, one package per
// line, so `just list-foundation-packages` can show it without a second
// implementation. Unset — every other run, including CI — it prints nothing.
const listFoundationEnv = "NERU_LIST_FOUNDATION"

// foundationGOARCH holds the architecture constant while GOOS varies. The rule
// is about operating systems; letting the arch move too would report a package
// split by *_arm64.go as platform-dependent, which is a different concern with
// a different answer.
const foundationGOARCH = "amd64"

// minFoundationPackages guards against a vacuous pass. The walk finds a few
// dozen packages today; a bug that matched none — a renamed directory, a walk
// that never descended — would satisfy every assertion below while checking
// nothing at all.
const minFoundationPackages = 25

// foundationExemptions are packages the slice runs even though they carry
// platform-tagged source, mapped to why running them anyway is honest.
//
// An exemption is a claim that the package's platform files are narrow enough
// that a failure in it is still a real cross-platform regression rather than a
// host-specific one. TestFoundationExemptionsStayHonest holds each entry to
// that: the package must still compile, with tests, on every target, and must
// still be genuinely ineligible. So this list can only shrink.
var foundationExemptions = map[string]string{
	"./internal/config": "its four platform files hold applyPlatformDefaults and " +
		"nothing else; the schema, the validators and the loader they gate are " +
		"shared, and they are what Linux and Windows work breaks",
}

// TestFoundationSliceMatchesTheRecipe fails when the hand-kept package list in
// the test-foundation recipe stops matching the packages that qualify.
func TestFoundationSliceMatchesTheRecipe(t *testing.T) {
	want := foundationSlice(t)

	if os.Getenv(listFoundationEnv) != "" {
		for _, pkg := range want {
			// This print is the output of `just list-foundation-packages`.
			//nolint:forbidigo
			fmt.Println(pkg)
		}
	}

	if len(want) < minFoundationPackages {
		t.Fatalf(
			"found %d foundation packages, expected at least %d; the walk or the "+
				"build-constraint match is broken, not the recipe",
			len(want),
			minFoundationPackages,
		)
	}

	got := recipePackages(t, parseJustfile(t))

	for _, pkg := range want {
		if !slices.Contains(got, pkg) {
			t.Errorf(
				"%s compiles to the same files on %s and has tests, but the %s "+
					"recipe in %s does not run it",
				pkg,
				strings.Join(knownOS, ", "),
				foundationRecipe,
				justfileName,
			)
		}
	}

	for _, pkg := range got {
		if slices.Contains(want, pkg) {
			continue
		}

		t.Errorf(
			"the %s recipe in %s runs %s, which does not compile to the same "+
				"files on %s; drop it, or add it to foundationExemptions with a reason",
			foundationRecipe,
			justfileName,
			pkg,
			strings.Join(knownOS, ", "),
		)
	}

	if t.Failed() {
		t.Logf(
			"the %s recipe should run exactly these packages "+
				"(`just list-foundation-packages` prints them):\n%s",
			foundationRecipe,
			strings.Join(want, "\n"),
		)
	}
}

// TestFoundationSliceRunsInCI fails when nothing CI runs reaches the
// test-foundation recipe.
//
// TestFoundationSliceMatchesTheRecipe above pins the package *list*; this pins
// that the recipe is executed. Both are needed. The list check would not have
// caught a mistyped flag, a `go test` that stopped being reached at all, or a
// package whose tests only fail in the slice's own invocation — and for weeks
// nothing caught anything, because CI ran every recipe except this one while
// AGENTS.md told agents to trust it.
func TestFoundationSliceRunsInCI(t *testing.T) {
	recipes := parseJustfile(t)

	// Establish the recipe exists before asking what reaches it. "Nothing runs
	// test-foundation" is a true but useless thing to say about a recipe that
	// has been renamed or deleted.
	recipes.mustFind(t, foundationRecipe)

	roots := workflowRecipes(t, recipes)
	if len(roots) == 0 {
		t.Fatalf(
			"%s runs no recipe %s defines; the invocation match is broken, not CI",
			ciWorkflow,
			justfileName,
		)
	}

	if !recipes.reaches(roots, foundationRecipe) {
		t.Errorf(
			"%s runs %s, and none of them reaches %s; CI does not gate on the "+
				"recipe contributors are told to trust",
			ciWorkflow,
			strings.Join(roots, ", "),
			foundationRecipe,
		)
	}

	if !recipes.reaches([]string{localCIRecipe}, foundationRecipe) {
		t.Errorf(
			"the %s recipe in %s does not reach %s, so it is no longer the local "+
				"mirror of CI that CONTRIBUTING.md says it is",
			localCIRecipe,
			justfileName,
			foundationRecipe,
		)
	}
}

// TestFoundationExemptionsStayHonest keeps foundationExemptions from outliving
// its reasons. An entry whose package became platform-free belongs in the
// derived set, not the allowlist; an entry that stopped compiling everywhere
// was never safe to run everywhere.
func TestFoundationExemptionsStayHonest(t *testing.T) {
	byPath := map[string]packageDir{}
	for _, dir := range goPackageDirs(t) {
		byPath[dir.rel] = dir
	}

	for pkg, reason := range foundationExemptions {
		if reason == "" {
			t.Errorf("%s is exempt with no reason given", pkg)
		}

		dir, found := byPath[pkg]
		if !found {
			t.Errorf("%s is exempt but no such package exists; drop the entry", pkg)

			continue
		}

		if isFoundationPackage(t, dir) {
			t.Errorf(
				"%s no longer carries platform-tagged source, so it needs no "+
					"exemption; drop the entry and let it be derived",
				pkg,
			)
		}

		for _, goos := range knownOS {
			built := buildableFiles(t, dir, goos)
			if len(built.source) == 0 || len(built.tests) == 0 {
				t.Errorf(
					"%s is exempt from the foundation slice, but on %s it has %d "+
						"source and %d test files; the slice cannot claim to run it "+
						"everywhere",
					pkg,
					goos,
					len(built.source),
					len(built.tests),
				)
			}
		}
	}
}

// foundationSlice returns the packages the recipe should run, sorted: every
// package that compiles to the same files on every target and has tests, plus
// the exemptions.
func foundationSlice(t *testing.T) []string {
	t.Helper()

	var slice []string

	for _, dir := range goPackageDirs(t) {
		if isFoundationPackage(t, dir) {
			slice = append(slice, dir.rel)
		}
	}

	for pkg := range foundationExemptions {
		slice = append(slice, pkg)
	}

	slices.Sort(slice)

	return slices.Compact(slice)
}

// isFoundationPackage reports whether a package compiles to the identical set
// of files on every target and has tests to run there.
//
// Comparing the matched files, rather than looking for platform-tagged ones,
// is what makes this see everything the old filename check could not: a package
// that is one platform's code by directory has no files at all on the others,
// and a file gated by //go:build !darwin says nothing in its name.
func isFoundationPackage(t *testing.T, dir packageDir) bool {
	t.Helper()

	reference := buildableFiles(t, dir, knownOS[0])
	if len(reference.source) == 0 || len(reference.tests) == 0 {
		return false
	}

	for _, goos := range knownOS[1:] {
		built := buildableFiles(t, dir, goos)
		if !slices.Equal(built.source, reference.source) ||
			!slices.Equal(built.tests, reference.tests) {
			return false
		}
	}

	return true
}

// buildSet is the sorted result of asking the toolchain which of a package's
// files compile for one target.
type buildSet struct {
	source []string
	tests  []string
}

// buildableFiles asks go/build which files in dir compile for goos, using the
// same matcher the toolchain uses — so it reads filename suffixes and //go:build
// lines alike, which is the whole point.
//
// Cgo is enabled so that a package's cgo files count as present on the target
// that has them. With it off they would be excluded everywhere, and a native
// package would look identical on all three by virtue of being empty on all
// three.
func buildableFiles(t *testing.T, dir packageDir, goos string) buildSet {
	t.Helper()

	context := build.Default
	context.GOOS = goos
	context.GOARCH = foundationGOARCH
	context.CgoEnabled = true

	var built buildSet

	for _, name := range dir.files {
		matched, err := context.MatchFile(dir.abs, name)
		if err != nil {
			t.Fatalf("MatchFile(%s, %s) for %s error = %v", dir.rel, name, goos, err)
		}

		if !matched {
			continue
		}

		if strings.HasSuffix(name, "_test.go") {
			built.tests = append(built.tests, name)
		} else {
			built.source = append(built.source, name)
		}
	}

	slices.Sort(built.source)
	slices.Sort(built.tests)

	return built
}

// packageDir is one directory of Go source, named the way the recipe names it.
type packageDir struct {
	// rel is the import path as `go test` takes it, e.g. "./internal/domain/hint".
	rel   string
	abs   string
	files []string
}

// goPackageDirs returns every directory in the repo holding Go source, test
// files included — a package whose only tests are platform-tagged is not one
// the slice can run everywhere, and that is invisible without them.
func goPackageDirs(t *testing.T) []packageDir {
	t.Helper()

	repoRoot := findRepoRoot(t)
	byDir := map[string][]string{}

	walkRepoFiles(t, repoRoot, func(file repoFile) {
		if filepath.Ext(file.name) != goExt {
			return
		}

		byDir[file.dir] = append(byDir[file.dir], file.name)
	})

	assertWalkedAtLeast(t, "directories of Go source", len(byDir), bulkWalkFloor)

	dirs := make([]packageDir, 0, len(byDir))

	for relDir, files := range byDir {
		dirs = append(dirs, packageDir{
			rel:   "./" + relDir,
			abs:   filepath.Join(repoRoot, filepath.FromSlash(relDir)),
			files: files,
		})
	}

	slices.SortFunc(dirs, func(a, b packageDir) int { return strings.Compare(a.rel, b.rel) })

	return dirs
}

// recipePackagePattern matches the package arguments in a just recipe. The
// recipe's echo lines carry no "./" token, so nothing else in the body matches.
var recipePackagePattern = regexp.MustCompile(`\./[A-Za-z0-9._/-]+`)

// recipePackages returns the packages the test-foundation recipe passes to
// `go test`, sorted, failing on a package listed twice.
func recipePackages(t *testing.T, recipes justRecipes) []string {
	t.Helper()

	listed := recipePackagePattern.FindAllString(recipes.mustFind(t, foundationRecipe).body, -1)
	slices.Sort(listed)

	deduped := slices.Compact(slices.Clone(listed))
	if len(deduped) != len(listed) {
		t.Errorf(
			"the %s recipe in %s lists a package more than once",
			foundationRecipe,
			justfileName,
		)
	}

	return deduped
}

// justRecipes is the justfile read as a graph, keyed by recipe name.
type justRecipes map[string]justRecipe

// justRecipe is one recipe: the indented lines under its header, and the
// recipes running it runs — its header dependencies plus any `just <name>` call
// in that body.
type justRecipe struct {
	body string
	runs []string
}

// mustFind returns a recipe, failing the test when the justfile has none by
// that name. Every caller here names a recipe this repo is supposed to have, so
// its absence is a rename nobody finished, not a condition to handle.
func (recipes justRecipes) mustFind(t *testing.T, name string) justRecipe {
	t.Helper()

	recipe, defined := recipes[name]
	if !defined {
		t.Fatalf("%s defines no %s recipe", justfileName, name)
	}

	return recipe
}

// reaches reports whether target runs when any of roots does.
func (recipes justRecipes) reaches(roots []string, target string) bool {
	seen := map[string]bool{}
	queue := slices.Clone(roots)

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]

		if name == target {
			return true
		}

		if seen[name] {
			continue
		}

		seen[name] = true
		queue = append(queue, recipes[name].runs...)
	}

	return false
}

// invokedIn returns the recipes text calls with `just <name>`, keeping only
// names the justfile defines — so prose in a workflow step, "Install just on
// Linux", contributes nothing.
func (recipes justRecipes) invokedIn(text string) []string {
	var invoked []string

	for _, match := range justInvocationPattern.FindAllStringSubmatch(text, -1) {
		if _, defined := recipes[match[1]]; defined {
			invoked = append(invoked, match[1])
		}
	}

	return invoked
}

// justInvocationPattern matches a `just <recipe>` call.
var justInvocationPattern = regexp.MustCompile(`\bjust\s+([a-zA-Z_][a-zA-Z0-9_-]*)`)

// justHeaderLine splits a recipe header into its name and its dependencies:
// `name PARAM…: dep…`. A `name := value` assignment is not a header, and the
// dependency list ends at a comment.
func justHeaderLine(line string) (string, []string, bool) {
	header, rest, found := strings.Cut(line, ":")
	if !found || strings.HasPrefix(rest, "=") {
		return "", nil, false
	}

	name, _, _ := strings.Cut(strings.TrimSpace(header), " ")
	if name == "" {
		return "", nil, false
	}

	listed, _, _ := strings.Cut(rest, "#")

	return name, strings.Fields(listed), true
}

// parseJustfile reads the justfile once and returns every recipe with its body
// and the recipes it runs.
//
// A recipe runs another two ways — as a header dependency, and as a `just
// <name>` call in its body, which is how test-all reaches its work. Following
// only the first would report a recipe as ungated for moving between two forms
// that run the same thing.
func parseJustfile(t *testing.T) justRecipes {
	t.Helper()

	var (
		recipes = justRecipes{}
		bodies  = map[string][]string{}
		current string
	)

	for line := range strings.Lines(readRepoFile(t, justfileName)) {
		line = strings.TrimRight(line, "\n")

		switch {
		case line == "":
		case strings.HasPrefix(line, " "), strings.HasPrefix(line, "\t"):
			if current != "" {
				bodies[current] = append(bodies[current], line)
			}
		// A comment or an attribute in column one interrupts nothing: the
		// recipe it decorates has not started yet.
		case strings.HasPrefix(line, "#"), strings.HasPrefix(line, "["):
		default:
			name, deps, isHeader := justHeaderLine(line)
			if !isHeader {
				current = ""

				continue
			}

			current = name
			recipes[name] = justRecipe{runs: deps}
		}
	}

	if len(recipes) == 0 {
		t.Fatalf("%s defines no recipes; the header match is broken", justfileName)
	}

	for name, lines := range bodies {
		body := strings.Join(lines, "\n")

		recipe := recipes[name]
		recipe.body = body
		recipe.runs = append(recipe.runs, recipes.invokedIn(withoutComments(body))...)
		recipes[name] = recipe
	}

	return recipes
}

// workflowRecipes returns the recipes the CI workflow executes, sorted.
func workflowRecipes(t *testing.T, recipes justRecipes) []string {
	t.Helper()

	invoked := recipes.invokedIn(withoutComments(workflowRunCommands(t)))
	slices.Sort(invoked)

	return slices.Compact(invoked)
}

// runKeyPattern matches a step's `run:` key and captures both its indentation
// and whatever follows on the same line — empty for a block scalar, the whole
// command for an inline one.
var runKeyPattern = regexp.MustCompile(`^(\s*)(?:-\s+)?run:\s*(.*)$`)

// workflowRunCommands returns only the shell the CI workflow's steps run: every
// `run:` value, inline or block scalar, and nothing else in the file.
//
// Reading the whole file instead would blur the difference between "CI runs
// this recipe" and "the recipe's name appears somewhere". Every step here is
// named for what it runs — `- name: Run just test-ci` — so a match on the file
// reports each recipe CI merely claims to run, and would go on reporting it
// after the run: line underneath was deleted.
func workflowRunCommands(t *testing.T) string {
	t.Helper()

	var (
		commands       []string
		blockKeyIndent int
		inBlock        bool
	)

	for line := range strings.Lines(readRepoFile(t, ciWorkflow)) {
		line = strings.TrimRight(line, "\n")

		if inBlock {
			indent := len(line) - len(strings.TrimLeft(line, " "))
			if strings.TrimSpace(line) == "" || indent > blockKeyIndent {
				commands = append(commands, line)

				continue
			}

			inBlock = false
		}

		match := runKeyPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		// `run: |` and `run: >` carry the command on the lines below, indented
		// past the key; anything else is the command itself.
		if strings.HasPrefix(match[2], "|") || strings.HasPrefix(match[2], ">") {
			blockKeyIndent = len(match[1])
			inBlock = true

			continue
		}

		commands = append(commands, match[2])
	}

	if len(commands) == 0 {
		t.Fatalf("%s runs no steps; the run: match is broken, not CI", ciWorkflow)
	}

	return strings.Join(commands, "\n")
}

// withoutComments drops whole-line # comments, shared by the justfile and the
// workflow's shell because both use them. A comment naming a recipe would
// otherwise be read as running it, and the false edge points the wrong way:
// "# `just ci` covers this locally" would satisfy the very assertion it was
// written to excuse.
func withoutComments(text string) string {
	var kept []string

	for line := range strings.Lines(text) {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			kept = append(kept, line)
		}
	}

	return strings.Join(kept, "")
}

// readRepoFile returns the contents of a path relative to the repo root.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()

	path := filepath.Join(findRepoRoot(t), filepath.FromSlash(rel))

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}

	return string(data)
}
