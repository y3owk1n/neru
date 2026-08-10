package architecture_test

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// guideCitationExemptions are citations a guide deliberately makes to something
// that is not in the tree, mapped to the reason.
//
// It is empty, which is the goal state: a guide naming a test that does not
// exist is the failure this pin is for, and a naming *shape* is written with
// angle brackets (Test<Type>_<Method>_<EdgeCase>) so it is not a citation at
// all. An entry here should be temporary and carry the reason it exists.
//
// TestGuideCitations_ExemptionsAreStillReal holds each entry to both halves of
// its claim — that a guide still makes the citation, and that it still does not
// resolve — so this list can only shrink.
var guideCitationExemptions = map[string]string{}

// guideCitedTestNameFloor and guideCitedTestFileFloor are the fewest citations
// of each kind the guides are expected to carry. Both are tripwires against a
// pattern that has stopped matching rather than measurements of the prose,
// which is why neither is a count the guides have to keep up with: the checkout
// carries a few dozen of each, and a check recognizing none of them reports
// success over every one of them. Below half, the pattern has broken rather
// than the guides having gone quiet.
const (
	guideCitedTestNameFloor = 12
	guideCitedTestFileFloor = 28
)

// generatedGuideDoc is the one markdown file in the checkout nobody writes:
// release-please regenerates it from commit subjects, so a citation inside it
// cannot be corrected in place and a failure naming it would have no fix.
const generatedGuideDoc = "CHANGELOG.md"

// symlinkedGuideDoc is the sibling every AGENTS.md keeps
// (agent_contract_test.go pins it). Reading it would report every AGENTS.md
// citation twice, under a path the author never edits.
const symlinkedGuideDoc = "CLAUDE.md"

// guideTestNameCitation matches a test function name as prose names it. The
// uppercase letter after "Test" is what separates a citation from the word
// "Testing", and from the placeholder form Test<Type>_<Method>_<EdgeCase>,
// which names a shape rather than a test.
var guideTestNameCitation = regexp.MustCompile(`\bTest[A-Z][A-Za-z0-9_]*`)

// guideTestFileCitation matches a test file as prose names it: a bare base
// name, or a path with as much of its directory as the sentence needed.
var guideTestFileCitation = regexp.MustCompile(`[A-Za-z0-9_./-]*[A-Za-z0-9-]_test\.go`)

// globStar is the character a match can only follow when what the prose wrote
// is a file *slot* rather than a file — *integration_darwin_test.go, the
// exemption shape the One Rule is stated in. A slot is a contract about naming,
// pinned by platform_slots_test.go, and not a claim that a file exists.
//
// The other placeholder shape needs no rule: both patterns here require a
// letter or a digit where *_integration_<os>_test.go and
// Test<Type>_<Method>_<EdgeCase> have a bracket, so neither ever matches.
const globStar = '*'

// guideCitationRule is the rule a failure states, in the imperative, with the
// documents that state it. Both checks below cite it, because both are halves
// of one rule and a reader who meets either should be sent to the same place.
const guideCitationRule = "name a test only when it exists, and keep the name " +
	"current when it is renamed (AGENTS.md: a guide file may claim a test exists " +
	"only by naming it, " +
	"docs/adr/0011-a-contract-earns-a-guardrail-when-its-breach-is-silent.md)"

// TestGuideCitations_NameTestsThatExist pins the checkable half of ADR 0011's
// rule that a guide file may claim a test exists only by naming it.
//
// The failure it prevents is a sentence that claims enforcement it does not
// have. Two shipped: the root guide said every platform stub gets a contract
// test when three existed against thirty-eight candidates, and the config guide
// said three of its four links are guarded when it is two links by three tests.
// Both compiled, linted and tested green for months, because neither named
// anything a check could resolve — and a sentence like that is worse than one
// claiming nothing, since it stops the reader looking. Requiring a name is what
// makes the claim checkable; this is the check.
//
// It is one-directional, deliberately. Whether a sentence claims enforcement
// without naming a test is a question about prose that no test can ask, so that
// half stays with the reviewer. What lands here is the consequence: once a name
// is written down, a rename that leaves it behind fails rather than sending the
// next reader to a `go test -run` that matches nothing.
func TestGuideCitations_NameTestsThatExist(t *testing.T) {
	declared := declaredTestFunctions(t)

	assertEveryCitationResolves(t, citationCheck{
		pattern: guideTestNameCitation,
		resolves: func(name string) bool {
			return declared[name]
		},
		subject: "test names",
		missing: "is not a test function in this checkout",
		floor:   guideCitedTestNameFloor,
	})
}

// TestGuideCitations_NameTestFilesThatExist covers the other spelling of the
// same claim. The platform guide names five contract-test *paths* rather than
// the functions inside them (#1432), and doc_links_test.go reaches none of
// them: it reads the root and docs/ only, and matches paths anchored at a
// top-level repo directory, while a nested guide writes the path relative to
// wherever the reader already is.
//
// A citation resolves when some test file's path ends with it, which is what
// lets one rule judge every spelling the guides use — the bare
// extensions_test.go, the partial platform/linux/system_stub_contract_test.go,
// and a full path alike. The looseness is deliberate and bounded: it cannot
// tell a correct path from one missing a directory, and it does catch the
// failure that actually happens, which is a file that was renamed or deleted
// while the sentence naming it stayed.
func TestGuideCitations_NameTestFilesThatExist(t *testing.T) {
	files := testFilePaths(t)

	assertEveryCitationResolves(t, citationCheck{
		pattern: guideTestFileCitation,
		resolves: func(path string) bool {
			return testFileCitationResolves(path, files)
		},
		subject: "test files",
		missing: "matches no test file in this checkout",
		floor:   guideCitedTestFileFloor,
	})
}

// citationCheck is one half of the rule: which citations to find, what makes
// one resolve, and how to say so when it does not.
type citationCheck struct {
	pattern  *regexp.Regexp
	resolves func(citation string) bool
	// subject names what is counted, for the floor's failure message.
	subject string
	// missing completes "cites X, which …" in the imperative failure.
	missing string
	floor   int
}

// assertEveryCitationResolves runs one check over every guide document.
//
// The two checks share this because they are one rule read twice, and a
// difference between them would be a difference in what the rule means: the
// same documents, the same exemption list, the same rule cited in the same
// words. What each supplies is only the shape it looks for and what resolving
// means for that shape.
func assertEveryCitationResolves(t *testing.T, check citationCheck) {
	t.Helper()

	var problems []string

	cited := 0

	for _, doc := range guideDocuments(t) {
		for _, citation := range citations(t, doc, check.pattern) {
			cited++

			if check.resolves(citation) || guideCitationExemptions[citation] != "" {
				continue
			}

			problems = append(problems, fmt.Sprintf(
				"%s\tcites %s, which %s; %s",
				doc.rel, citation, check.missing, guideCitationRule,
			))
		}
	}

	reportOffenders(t, problems, "guide names something that does not exist")

	assertWalkedAtLeast(t, check.subject+" cited by the guides", cited, check.floor)
}

// TestGuideCitations_ExemptionsAreStillReal keeps guideCitationExemptions from
// outliving its reasons, in both directions. An entry no guide cites is dead
// weight that makes the next exemption easier to add; one that now resolves is
// worse, because it silently exempts a citation the checks above would have
// been happy to pass on their own — and would go on exempting it after the test
// it names is deleted again.
func TestGuideCitations_ExemptionsAreStillReal(t *testing.T) {
	if len(guideCitationExemptions) == 0 {
		return
	}

	declared := declaredTestFunctions(t)
	files := testFilePaths(t)
	docs := guideDocuments(t)

	for citation, reason := range guideCitationExemptions {
		if !anyGuideCites(t, docs, citation) {
			t.Errorf(
				"guideCitationExemptions names %q, which no guide cites; drop the "+
					"entry (%s)",
				citation, reason,
			)

			continue
		}

		if declared[citation] || testFileCitationResolves(citation, files) {
			t.Errorf(
				"guideCitationExemptions names %q, which now resolves; drop the "+
					"entry so the citation is checked like every other (%s)",
				citation, reason,
			)
		}
	}
}

// guideDocument is one markdown file this pin judges, with its text read once.
type guideDocument struct {
	rel  string
	text string
}

// guideDocuments returns the prose of the checkout: every markdown file the
// shared walker reaches, less the two that cannot be judged.
//
// The set is "all of it" rather than a list of guides on purpose. Deciding
// which documents are contract and which are commentary is the judgement call
// that let the false sentences sit in the first place, and the rule reads the
// same either way — a document naming a test that does not exist is wrong
// whether it is an AGENTS.md, a skill, a review profile or an ADR. The two
// exclusions are structural rather than editorial: one file is generated, and
// one is a symlink to a file already read.
func guideDocuments(t *testing.T) []guideDocument {
	t.Helper()

	var docs []guideDocument

	walkRepoFiles(t, findRepoRoot(t), func(file repoFile) {
		if !strings.HasSuffix(file.name, ".md") {
			return
		}

		if file.name == symlinkedGuideDoc || file.rel == generatedGuideDoc {
			return
		}

		content, readErr := os.ReadFile(file.abs)
		if readErr != nil {
			t.Fatalf("ReadFile(%s) error = %v", file.rel, readErr)
		}

		docs = append(docs, guideDocument{rel: file.rel, text: string(content)})
	})

	assertWalkedAtLeast(t, "guide documents", len(docs), bulkWalkFloor)

	return docs
}

// citations returns the distinct matches pattern finds in a document, less the
// ones that continue a glob, sorted so a failure lists them the same way twice.
func citations(t *testing.T, doc guideDocument, pattern *regexp.Regexp) []string {
	t.Helper()

	var found []string

	for _, span := range pattern.FindAllStringIndex(doc.text, -1) {
		if span[0] > 0 && doc.text[span[0]-1] == globStar {
			continue
		}

		found = append(found, doc.text[span[0]:span[1]])
	}

	slices.Sort(found)

	return slices.Compact(found)
}

// anyGuideCites reports whether any document names citation, by either pattern.
func anyGuideCites(t *testing.T, docs []guideDocument, citation string) bool {
	t.Helper()

	for _, doc := range docs {
		if slices.Contains(citations(t, doc, guideTestNameCitation), citation) ||
			slices.Contains(citations(t, doc, guideTestFileCitation), citation) {
			return true
		}
	}

	return false
}

// declaredTestFunctions returns every test function the checkout declares.
//
// Build-tagged files are parsed rather than built (forEachTestFile), so a guide
// naming an integration test resolves on a host that cannot run it — which is
// the point, since the guides that name those tests are read on every platform.
func declaredTestFunctions(t *testing.T) map[string]bool {
	t.Helper()

	declared := map[string]bool{}

	forEachTestFile(t, func(_ repoFile, _ *token.FileSet, file *ast.File) {
		for _, decl := range file.Decls {
			funcDecl, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || funcDecl.Recv != nil {
				continue
			}

			if strings.HasPrefix(funcDecl.Name.Name, "Test") {
				declared[funcDecl.Name.Name] = true
			}
		}
	})

	return declared
}

// testFilePaths returns the repo-relative path of every test file.
func testFilePaths(t *testing.T) []string {
	t.Helper()

	var paths []string

	walkRepoFiles(t, findRepoRoot(t), func(file repoFile) {
		if strings.HasSuffix(file.name, "_test.go") {
			paths = append(paths, file.rel)
		}
	})

	assertWalkedAtLeast(t, "test files", len(paths), bulkWalkFloor)

	return paths
}

// testFileCitationResolves reports whether a cited path names one of the test
// files in paths. Leading ../ and ./ segments are dropped first: a link in
// docs/ is written relative to that directory, and the anchor it is relative to
// is what this check deliberately does not try to know.
func testFileCitationResolves(cited string, paths []string) bool {
	trimmed := cited
	for strings.HasPrefix(trimmed, "../") || strings.HasPrefix(trimmed, "./") {
		trimmed = trimmed[strings.Index(trimmed, "/")+1:]
	}

	if trimmed == "" {
		return false
	}

	for _, path := range paths {
		if path == trimmed || strings.HasSuffix(path, "/"+trimmed) {
			return true
		}
	}

	return false
}
