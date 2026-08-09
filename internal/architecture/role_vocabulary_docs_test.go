package architecture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// TestConfigurationDocsCoverTheRoleVocabulary keeps the documented role table
// in step with the code. The table is the only place a user can see the whole
// vocabulary offline, and a role missing from it is effectively undiscoverable.
func TestConfigurationDocsCoverTheRoleVocabulary(t *testing.T) {
	docPath := filepath.Join(findRepoRoot(t), "docs", "CONFIGURATION.md")

	contents, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", docPath, err)
	}

	doc := string(contents)

	for _, mapping := range element.RoleVocabulary {
		// The table renders each semantic role as an inline code span, which is
		// specific enough not to match prose mentioning the same word.
		if !strings.Contains(doc, "`"+string(mapping.Semantic)+"`") {
			t.Errorf(
				"semantic role %q is missing from docs/CONFIGURATION.md; "+
					"add it to the Clickable roles table",
				mapping.Semantic,
			)
		}
	}
}

// TestConfigurationDocsMarkTheAXSubroleNames keeps the documented role table
// honest about the AX names AppKit declares as subroles: an element reports
// them in its subrole while its role stays generic, and the table is where a
// user would otherwise learn the wrong shape.
func TestConfigurationDocsMarkTheAXSubroleNames(t *testing.T) {
	docPath := filepath.Join(findRepoRoot(t), "docs", "CONFIGURATION.md")

	contents, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", docPath, err)
	}

	doc := string(contents)

	for name := range element.AXSubroleNames {
		if !strings.Contains(doc, "`"+name+"` †") {
			t.Errorf(
				"docs/CONFIGURATION.md does not mark `%s` with the subrole footnote †",
				name,
			)
		}
	}
}

// TestConfigurationDocsUseTheCurrentRoleVocabulary catches documentation that
// still tells users to write native role names without a vocabulary prefix,
// which is now a configuration error.
func TestConfigurationDocsUseTheCurrentRoleVocabulary(t *testing.T) {
	repoRoot := findRepoRoot(t)

	docs := []string{
		filepath.Join(repoRoot, "docs", "CONFIGURATION.md"),
		filepath.Join(repoRoot, "docs", "CLI.md"),
		filepath.Join(repoRoot, "docs", "TROUBLESHOOTING.md"),
		filepath.Join(repoRoot, "docs", "TIPS_TRICKS.md"),
	}

	for _, path := range docs {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read %s: %v", path, err)
		}

		for line := range strings.SplitSeq(string(contents), "\n") {
			// A bare AX name is legitimate when the line is quoting the error
			// that a bare AX name now produces.
			if strings.Contains(line, "unknown role") {
				continue
			}

			// Otherwise native AX names must carry their vocabulary prefix.
			if strings.Contains(line, `"AX`) && !strings.Contains(line, "ax:") {
				t.Errorf(
					"%s: bare AX role name in %q; write a semantic role or prefix it with ax:",
					filepath.Base(path), strings.TrimSpace(line),
				)
			}
		}
	}
}
