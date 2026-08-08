package architecture_test

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// bannerPattern matches a file's own path written as a comment near the top —
// a header line restating where the file lives.
var bannerPattern = regexp.MustCompile(`^// ((?:internal|cmd)/[A-Za-z0-9_./-]+\.go)\s*$`)

// bannerScanLines is how far into a file a path header can appear. It sits
// after the build tag and before the package clause, so a handful of lines is
// enough.
const bannerScanLines = 6

// TestCommentPaths_HeadersMatchTheirFile catches a header comment left behind by a
// rename. Not every file carries one; those that do must be right.
//
// A comment that names a source file is a pointer, and a pointer that no longer
// resolves is worse than no pointer: it sends a reader somewhere that does not
// exist and costs them the time to work out which of the two is wrong.
//
// Renames break these silently. The compiler does not read comments, and a
// grep for the old name finds only the stale comment itself, which reads like
// confirmation that the file is still there.
func TestCommentPaths_HeadersMatchTheirFile(t *testing.T) {
	repoRoot := findRepoRoot(t)

	for _, file := range goFiles(t) {
		content, readErr := os.ReadFile(file.absPath)
		if readErr != nil {
			t.Fatalf("ReadFile(%s) error = %v", file.relPath, readErr)
		}

		lines := strings.Split(string(content), "\n")
		if len(lines) > bannerScanLines {
			lines = lines[:bannerScanLines]
		}

		for _, line := range lines {
			match := bannerPattern.FindStringSubmatch(line)
			if match == nil {
				continue
			}

			if match[1] == file.relPath {
				break
			}

			t.Errorf(
				"%s has a path header reading %q; either correct it or delete the "+
					"line, which only restates where the file already is",
				file.relPath,
				match[1],
			)

			break
		}

		_ = repoRoot
	}
}

// siblingRefPattern matches a comment pointing at another Go file by name,
// as in "see events.go" or "(see darwin/element.go".
var siblingRefPattern = regexp.MustCompile(`\bsee ([a-z][a-z0-9_]*(?:/[a-z][a-z0-9_]*)*\.go)\b`)

// TestCommentPaths_SiblingReferencesResolve catches "see foo.go" pointing at a file that
// has been renamed or moved away.
//
// The reference is resolved anywhere under internal/ rather than beside the
// commenting file, because these point at near neighbors and a package split
// legitimately moves one into a subdirectory.
func TestCommentPaths_SiblingReferencesResolve(t *testing.T) {
	internalPaths := internalFilePaths(t)

	for _, file := range goFiles(t) {
		content, readErr := os.ReadFile(file.absPath)
		if readErr != nil {
			t.Fatalf("ReadFile(%s) error = %v", file.relPath, readErr)
		}

		for line := range strings.SplitSeq(string(content), "\n") {
			if !strings.Contains(line, "//") {
				continue
			}

			for _, match := range siblingRefPattern.FindAllStringSubmatch(line, -1) {
				if resolvesUnderInternal(internalPaths, match[1]) {
					continue
				}

				t.Errorf(
					"%s points at %q, which does not exist under internal/; "+
						"update the reference or drop it",
					file.relPath,
					match[1],
				)
			}
		}
	}
}

// internalFilePaths returns the slash-relative path of every file under
// internal/, indexed once for the whole test: the references being resolved
// number in the hundreds, and a walk apiece would read the tree that many
// times over.
func internalFilePaths(t *testing.T) []string {
	t.Helper()

	var found []string

	walkRepoFiles(t, findRepoRoot(t), func(file repoFile) {
		if strings.HasPrefix(file.rel, "internal/") {
			found = append(found, file.rel)
		}
	})

	assertWalkedAtLeast(t, "files under internal/", len(found), bulkWalkFloor)

	return found
}

// resolvesUnderInternal reports whether name matches a file anywhere in the
// internal tree. A reference may be a bare file name, a trailing directory or
// two ("darwin/element.go"), or the whole path from the repository root, so it
// matches on a path component boundary rather than on the string.
//
// The paths are repo-relative, which is a narrowing: matching against absolute
// paths, as this did before the walks were shared, let the directory the
// checkout happens to sit in complete a reference and pass it.
func resolvesUnderInternal(internalPaths []string, name string) bool {
	return slices.ContainsFunc(internalPaths, func(relPath string) bool {
		return relPath == name || strings.HasSuffix(relPath, "/"+name)
	})
}
