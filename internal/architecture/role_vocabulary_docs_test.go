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
	docPath := filepath.Join(findRepoRoot(t), "docs", "reference", "configuration.md")

	contents, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", docPath, err)
	}

	rows := semanticRoleRows(t, string(contents))

	for _, mapping := range element.RoleVocabulary {
		// Only a row of the table counts. Common roles such as `button` or
		// `link` turn up in prose all over the page, so a match anywhere would
		// pass with the row gone.
		if !rows["`"+string(mapping.Semantic)+"`"] {
			t.Errorf(
				"semantic role %q has no row in the %s table of "+
					"docs/reference/configuration.md; add one",
				mapping.Semantic, semanticRolesHeading,
			)
		}
	}
}

// semanticRolesHeading opens the table TestConfigurationDocsCoverTheRoleVocabulary
// reads, which runs until the next heading.
const semanticRolesHeading = "#### Semantic roles"

// semanticRoleRows returns the first cell of every row in the semantic roles
// table, backticks included.
func semanticRoleRows(t *testing.T, doc string) map[string]bool {
	t.Helper()

	_, section, found := strings.Cut(doc, "\n"+semanticRolesHeading+"\n")
	if !found {
		t.Fatalf("docs/reference/configuration.md has no %q heading", semanticRolesHeading)
	}

	rows := map[string]bool{}

	for line := range strings.SplitSeq(section, "\n") {
		if strings.HasPrefix(line, "#") {
			break
		}

		cells := strings.Split(line, "|")
		if len(cells) < 3 {
			continue
		}

		rows[strings.TrimSpace(cells[1])] = true
	}

	return rows
}

// TestConfigurationDocsMarkTheAXSubroleNames keeps the documented role table
// honest about the AX names AppKit declares as subroles. An element reports
// them in its subrole while its role stays generic, and the table is where a
// user would otherwise learn the wrong shape.
func TestConfigurationDocsMarkTheAXSubroleNames(t *testing.T) {
	docPath := filepath.Join(findRepoRoot(t), "docs", "reference", "configuration.md")

	contents, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", docPath, err)
	}

	doc := string(contents)

	for name := range element.AXSubroleNames {
		if !strings.Contains(doc, "`"+name+"` †") {
			t.Errorf(
				"docs/reference/configuration.md does not mark `%s` with the subrole footnote †",
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
// vocabulary prefix, the spelling that became a fatal configuration error
// when the role vocabulary landed.
//
// allowBacktickedMentions exempts names wrapped in backticks, markdown's way
// of mentioning a native name without telling the user to write it. The role
// table in docs/reference/configuration.md documents every AX expansion that way. Help
// strings get no such exemption, since they carry no mention-table, so every bare
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
		filepath.Join(repoRoot, "docs", "reference", "configuration.md"),
		filepath.Join(repoRoot, "docs", "reference", "cli.md"),
		filepath.Join(repoRoot, "docs", "guide", "troubleshooting.md"),
		filepath.Join(repoRoot, "docs", "guide", "recipes.md"),
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

// TestCLIHelpUsesTheCurrentRoleVocabulary walks every command's compiled-in
// help for the same stale spelling: short and long descriptions, examples, and
// flag usages. Help is where a user copies a role example from, so a
// bare AX name here directs them straight to a fatal config error.
//
// The mode grammar's missing-value messages are walked too. They never reach
// cobra's help, but an IPC response and binding advice repeat them, so a stale
// role spelling there is said everywhere at once. Its usage sentences need no
// separate walk, since TestModeCommands_RegisterExactlyWhatTheGrammarDeclares
// pins each one to the flag usage this test reads.
func TestCLIHelpUsesTheCurrentRoleVocabulary(t *testing.T) {
	for _, descriptor := range modecmd.All() {
		reportBareAXNames(t,
			"the value message for "+descriptor.Name().Long(), descriptor.ValueMessage(), false)
	}

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
