package architecture_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// docsExemptFromLinkChecking are files that deliberately name paths which do
// not exist.
//
// It is empty, which is the goal state: a doc that wants to name a path that
// does not exist is a doc that is wrong. An entry here should be temporary and
// carry the reason it exists.
var docsExemptFromLinkChecking = map[string]string{}

// repoPathPattern matches the two ways docs name a file: a markdown link
// target, and an inline-code path. Both start at a top-level repo directory so
// prose like "the app/modes handler" is not mistaken for a path.
var repoPathPattern = regexp.MustCompile(
	`(?:\]\(|` + "`" + `)((?:\.\./)*(?:internal|cmd|configs|scripts|resources|assets|protocol|nix)/[A-Za-z0-9_./-]+)`,
)

// Documentation that names a file which no longer exists is worse than
// documentation that says nothing: it sends a contributor to a path, the path
// 404s, and now they distrust the rest of the page too.
//
// It is the cheapest guardrail here and it catches the most common review
// miss: a rename that updated the code and forgot the prose.
func TestDocLinks_DoNotPointAtMissingPaths(t *testing.T) {
	repoRoot := findRepoRoot(t)

	for _, doc := range markdownFiles(t, repoRoot) {
		if _, exempt := docsExemptFromLinkChecking[doc.relPath]; exempt {
			continue
		}

		content, readErr := os.ReadFile(doc.absPath)
		if readErr != nil {
			t.Fatalf("ReadFile(%s) error = %v", doc.relPath, readErr)
		}

		for _, match := range repoPathPattern.FindAllStringSubmatch(string(content), -1) {
			reference := match[1]

			// Links are written relative to the doc; inline code is written
			// from the repo root. Resolving from the doc's directory covers
			// both, since a root-relative path resolves the same way from a
			// doc at the root and needs the ../ prefix from docs/.
			resolved := filepath.Join(
				repoRoot,
				filepath.Dir(doc.relPath),
				filepath.FromSlash(reference),
			)

			_, relErr := os.Stat(resolved)
			if relErr == nil {
				continue
			}

			_, rootErr := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(reference)))
			if rootErr == nil {
				continue
			}

			t.Errorf(
				"%s references %q, which does not exist; update the link or the "+
					"path (each fact has one home — docs/CROSS_PLATFORM.md, "+
					"Documentation Checklist)",
				doc.relPath,
				reference,
			)
		}
	}
}

// TestDocLinks_ExemptionsAreStillReal keeps the exemption list honest, the same
// way knownLayeringExceptions is kept honest: an entry for a file that no
// longer exists is dead weight that makes the next exemption easier to add.
func TestDocLinks_ExemptionsAreStillReal(t *testing.T) {
	repoRoot := findRepoRoot(t)

	for relPath, reason := range docsExemptFromLinkChecking {
		_, statErr := os.Stat(filepath.Join(repoRoot, filepath.FromSlash(relPath)))
		if statErr != nil {
			t.Errorf(
				"exempt doc %s no longer exists (%q); delete its entry",
				relPath,
				reason,
			)
		}
	}
}

type markdownFile struct {
	relPath string
	absPath string
}

// markdownFiles returns the documentation a contributor actually reads: the
// top-level guides and everything under docs/.
//
// It carried its own skip list until the walks in this package were collapsed
// into one — vendored and generated trees are the shared walker's business
// now, and were never this list's real work: the root-and-docs rule below had
// already excluded every one of them.
func markdownFiles(t *testing.T, repoRoot string) []markdownFile {
	t.Helper()

	var found []markdownFile

	walkRepoFiles(t, repoRoot, func(file repoFile) {
		if !strings.HasSuffix(file.rel, ".md") || !isContributorDoc(file.rel) {
			return
		}

		found = append(found, markdownFile{relPath: file.rel, absPath: file.abs})
	})

	assertWalkedAtLeast(t, "contributor documents", len(found), bulkWalkFloor)

	return found
}

// isContributorDoc reports whether a repo-relative path is documentation a
// contributor reads: a guide at the root, or anything under docs/. Everything
// else in the checkout is code, or prose written for a tool rather than a
// person.
func isContributorDoc(relPath string) bool {
	return !strings.Contains(relPath, "/") || strings.HasPrefix(relPath, "docs/")
}
