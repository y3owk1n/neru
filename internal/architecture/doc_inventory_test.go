package architecture_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// guardrailFileMention matches a guardrail file as doc.go names it.
var guardrailFileMention = regexp.MustCompile(`[a-z0-9_]+_test\.go`)

// TestDocInventory_ListsEveryGuardrailFile keeps the inventory in doc.go honest.
//
// The package comment is the map of this suite: it is how a contributor asked
// to "add a guardrail" finds whether one already exists, and how a reviewer
// judges whether a rule has a home. Written by hand it drifted — it claimed
// eight files when there were seventeen, so nine guardrails were invisible to
// anyone who read the map instead of the directory (#1319).
//
// A list nobody checks is a list that describes the tree it was written
// against, so this checks it in both directions: a file that is not listed,
// and a listing for a file that is gone.
func TestDocInventory_ListsEveryGuardrailFile(t *testing.T) {
	repoRoot := findRepoRoot(t)
	packagePath := filepath.Join(repoRoot, filepath.FromSlash(architecturePackageDir))

	listed := guardrailFilesNamedInDoc(t, filepath.Join(packagePath, "doc.go"))
	present := guardrailFilesOnDisk(t, packagePath)

	assertWalkedAtLeast(t, "guardrail files on disk", len(present), bulkWalkFloor)

	for _, name := range present {
		if slices.Contains(listed, name) {
			continue
		}

		t.Errorf(
			"%s is not listed in doc.go; the package comment is how a "+
				"contributor finds whether a rule already has a home, and a "+
				"guardrail missing from it is a guardrail nobody knows about",
			name,
		)
	}

	for _, name := range listed {
		if slices.Contains(present, name) {
			continue
		}

		t.Errorf(
			"doc.go lists %s, which no longer exists; delete the entry or "+
				"restore the file",
			name,
		)
	}
}

// guardrailFilesNamedInDoc returns the file names the package comment in
// docPath mentions, sorted and deduplicated. Only the comment is read: a name
// appearing in code would make the inventory look complete without saying
// anything.
func guardrailFilesNamedInDoc(t *testing.T, docPath string) []string {
	t.Helper()

	parsed, parseErr := parser.ParseFile(token.NewFileSet(), docPath, nil, parser.ParseComments)
	if parseErr != nil {
		t.Fatalf("ParseFile(%s) error = %v", docPath, parseErr)
	}

	if parsed.Doc == nil {
		t.Fatalf("%s carries no package comment", docPath)
	}

	named := guardrailFileMention.FindAllString(parsed.Doc.Text(), -1)
	slices.Sort(named)

	return slices.Compact(named)
}

// guardrailFilesOnDisk returns the _test.go file names in the package
// directory, sorted. The directory is read directly rather than walked: the
// package is one flat directory, and this is the list the walker's own callers
// are checked against.
func guardrailFilesOnDisk(t *testing.T, packagePath string) []string {
	t.Helper()

	entries, readErr := os.ReadDir(packagePath)
	if readErr != nil {
		t.Fatalf("ReadDir(%s) error = %v", packagePath, readErr)
	}

	var found []string

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		found = append(found, entry.Name())
	}

	slices.Sort(found)

	return found
}
