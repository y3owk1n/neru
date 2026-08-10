package architecture_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// This is the one justfile reader the suite shares.
//
// Two guardrails ask the justfile questions, and they are different questions.
// foundation_slice_test.go asks which packages the test-foundation recipe
// passes to `go test` and what reaches that recipe; justfile_doc_test.go asks
// what `just --list` prints for every recipe it shows. Both need the same
// underlying answer first — what the recipes are, what each one's body and
// attributes say, and which recipes run which — so that answer is computed
// once, here, and nothing in this file states a rule.
//
// The alternative is two parsers, and the cost is not the duplicated lines. It
// is that "what a recipe is" would be defined twice: where a header ends, that
// `name := value` is an assignment and not a recipe, that a blank line or a
// column-one comment breaks an attribute run. Those cases were learned one at
// a time, and a second copy inherits whichever ones its author happened to
// hit.
//
// What lives here is the justfile-specific reading and nothing else. The two
// generic helpers it leans on, readRepoFile and withoutComments, stay in
// foundation_slice_test.go beside the CI-workflow reader that also uses them:
// neither is about justfiles, and moving them here would point the same borrow
// the other way.
//
// Reading the file as text rather than shelling out to `just` is deliberate,
// per ADR 0011: the guardrails in this package hold on every CI leg regardless
// of what is installed on it.
const justfileName = "justfile"

// justRecipes is the justfile read as a graph, keyed by recipe name.
type justRecipes map[string]justRecipe

// justRecipe is one recipe: the indented lines under its header, the
// attributes declared immediately above it, and the recipes running it runs —
// its header dependencies plus any `just <name>` call in that body.
type justRecipe struct {
	body string
	runs []string
	// attributes is the `[…]` lines between the recipe's comment block and its
	// header, joined as they appear. justfile_doc_test.go reads the declared
	// summary out of them.
	attributes string
}

// mustFind returns a recipe, failing the test when the justfile has none by
// that name. Every caller names a recipe this repo is supposed to have, so its
// absence is a rename nobody finished, not a condition to handle.
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

	recipes := parseJustfileText(readRepoFile(t, justfileName))
	if len(recipes) == 0 {
		t.Fatalf("%s defines no recipes; the header match is broken", justfileName)
	}

	return recipes
}

// parseJustfileText is parseJustfile over text already in hand, so a guardrail
// can hold the parser itself to a fixture rather than only to the checkout.
func parseJustfileText(text string) justRecipes {
	var (
		recipes = justRecipes{}
		bodies  = map[string][]string{}
		pending string
		current string
	)

	for line := range strings.Lines(text) {
		line = strings.TrimRight(line, "\n")

		switch {
		// A blank line ends an attribute run. `just` requires an attribute to
		// sit directly above the header it decorates and errors on anything
		// between, so a pending attribute separated from one decorates nothing.
		case line == "":
			pending = ""
		case strings.HasPrefix(line, " "), strings.HasPrefix(line, "\t"):
			if current != "" {
				bodies[current] = append(bodies[current], line)
			}
		// A comment in column one interrupts nothing: the recipe it decorates
		// has not started yet. It does end an attribute run, for the reason
		// above.
		case strings.HasPrefix(line, "#"):
			pending = ""
		case strings.HasPrefix(line, "["):
			pending += line + "\n"
		default:
			name, deps, isHeader := justHeaderLine(line)
			if !isHeader {
				current = ""
				pending = ""

				continue
			}

			current = name
			recipes[name] = justRecipe{runs: deps, attributes: pending}
			pending = ""
		}
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
