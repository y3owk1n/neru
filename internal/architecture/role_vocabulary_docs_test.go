package architecture_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/y3owk1n/neru/internal/cli"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
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

// axNativeName matches a native AX role name: "AX" followed by a capitalized
// word. It is what a role example looked like before the vocabulary landed,
// and what the resolver now refuses as RoleDiagnosticUnknownVocabulary unless
// the name carries its ax: prefix.
var axNativeName = regexp.MustCompile(`\bAX[A-Z][A-Za-z]*`)

// bareAXNames returns the AX native names text writes without the ax:
// vocabulary prefix — the spelling that became a fatal configuration error
// when the role vocabulary landed.
//
// allowBacktickedMentions exempts names wrapped in backticks, markdown's way
// of mentioning a native name without telling the user to write it — the role
// table in docs/CONFIGURATION.md documents every AX expansion that way. Help
// strings get no such exemption: they carry no mention-table, so every bare
// AX name in one is advice.
func bareAXNames(text string, allowBacktickedMentions bool) []string {
	var bare []string

	for _, match := range axNativeName.FindAllStringIndex(text, -1) {
		start, end := match[0], match[1]

		if start >= len("ax:") && text[start-len("ax:"):start] == "ax:" {
			continue
		}

		if allowBacktickedMentions &&
			start >= 1 && text[start-1] == '`' &&
			end < len(text) && text[end] == '`' {
			continue
		}

		bare = append(bare, text[start:end])
	}

	return bare
}

// reportBareAXNames fails the test once per bare AX name in text. subject
// names the surface the text came from, the way the failure should cite it.
func reportBareAXNames(t *testing.T, subject, text string, allowBacktickedMentions bool) {
	t.Helper()

	for _, name := range bareAXNames(text, allowBacktickedMentions) {
		t.Errorf(
			"%s says %s, a spelling the resolver refuses; "+
				"write a semantic role or prefix it with ax:",
			subject, name,
		)
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

			subject := filepath.Base(path) + ": " + strconv.Quote(strings.TrimSpace(line))
			reportBareAXNames(t, subject, line, true)
		}
	}
}

// TestModeFlagVocabularyUsesTheCurrentRoleVocabulary keeps the grammar's own
// words current: a flag's usage sentence and its missing-value message are
// what every door — command line, IPC response, binding advice — says, so a
// stale role spelling there is repeated everywhere at once.
func TestModeFlagVocabularyUsesTheCurrentRoleVocabulary(t *testing.T) {
	for _, descriptor := range modecmd.All() {
		reportBareAXNames(t,
			"the usage for "+descriptor.Name().Long(), descriptor.Usage(), false)
		reportBareAXNames(t,
			"the value message for "+descriptor.Name().Long(), descriptor.ValueMessage(), false)
	}
}

// TestCLIHelpUsesTheCurrentRoleVocabulary walks every command's compiled-in
// help — short and long descriptions, examples, and flag usages — for the
// same stale spelling. Help is where a user copies a role example from, so a
// bare AX name here directs them straight to a fatal config error.
func TestCLIHelpUsesTheCurrentRoleVocabulary(t *testing.T) {
	var walk func(*cobra.Command)

	walk = func(command *cobra.Command) {
		for surface, text := range map[string]string{
			"use":     command.Use,
			"short":   command.Short,
			"long":    command.Long,
			"example": command.Example,
		} {
			reportBareAXNames(t,
				"the "+surface+" help of "+strconv.Quote(command.CommandPath()),
				text, false)
		}

		command.Flags().VisitAll(func(flag *pflag.Flag) {
			reportBareAXNames(t,
				"the --"+flag.Name+" usage on "+strconv.Quote(command.CommandPath()),
				flag.Usage, false)
		})

		for _, child := range command.Commands() {
			walk(child)
		}
	}

	walk(cli.RootCmd)
}
