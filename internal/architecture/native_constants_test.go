package architecture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readNativeSource reads a repo-relative source file, failing with a message
// that names the likely cause. A pin addresses files by path, so a rename shows
// up here rather than as a silent pass over nothing.
func readNativeSource(t *testing.T, repoRelPath string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join(findRepoRoot(t), filepath.FromSlash(repoRelPath)))
	if err != nil {
		t.Fatalf("%s: cannot read native source (renamed?): %v", repoRelPath, err)
	}

	return string(content)
}

// pascalCase spells a config vocabulary value the way its Go constant's name
// ends, so "bottom" becomes "Bottom" and "top_left" becomes "TopLeft".
func pascalCase(value string) string {
	var built strings.Builder

	for word := range strings.SplitSeq(value, "_") {
		if word == "" {
			continue
		}

		built.WriteString(strings.ToUpper(word[:1]))
		built.WriteString(word[1:])
	}

	return built.String()
}
