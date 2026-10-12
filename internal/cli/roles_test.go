package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/y3owk1n/neru/internal/cli"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// cliTestRoles is the roles command name as registered on RootCmd.
const cliTestRoles = "roles"

// runRolesCmd executes `neru roles` through RootCmd so that flags are parsed
// the way they are in real use, and returns everything it wrote. A config path
// is always supplied so the test never reads the developer's own config.
func runRolesCmd(t *testing.T, configPath string, args ...string) string {
	t.Helper()

	full := append([]string{cliTestRoles, "--config", configPath}, args...)

	stdout, stderr, err := executeRoot(t, full...)
	if err != nil {
		t.Fatalf("neru %v failed: %v\n%s", full, err, stderr)
	}

	return stdout
}

// executeRoot runs args through RootCmd and returns what it wrote to stdout
// and stderr. Like cli.Execute, it prints no usage on a failure. It puts the
// command's flags back to their defaults afterwards, since RootCmd keeps them
// between runs.
func executeRoot(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	cmd, _, findErr := cli.RootCmd.Find(args)
	if findErr != nil {
		t.Fatalf("no command for %v: %v", args, findErr)
	}

	var stdout, stderr bytes.Buffer

	cli.RootCmd.SetOut(&stdout)
	cli.RootCmd.SetErr(&stderr)
	cli.RootCmd.SetArgs(args)

	silenced := cmd.SilenceUsage
	cmd.SilenceUsage = true

	t.Cleanup(func() {
		cli.RootCmd.SetOut(nil)
		cli.RootCmd.SetErr(nil)
		cli.RootCmd.SetArgs(nil)

		cmd.SilenceUsage = silenced

		reset := func(flag *pflag.Flag) {
			_ = flag.Value.Set(flag.DefValue)
			flag.Changed = false
		}

		cmd.Flags().VisitAll(reset)
		cmd.InheritedFlags().VisitAll(reset)
	})

	err := cli.RootCmd.Execute()

	return stdout.String(), stderr.String(), err
}

// writeRolesConfig writes a config with the given clickable_roles array.
func writeRolesConfig(t *testing.T, roles string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")

	contents := `
[hints]
enabled = true
hint_characters = "asdf"
clickable_roles = ` + roles + `
`

	err := os.WriteFile(path, []byte(contents), 0o600)
	if err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	return path
}

// TestRolesCmd_ListsEverySemanticRole keeps the command in step with the
// vocabulary. It is how a user discovers what they may write, so a role
// missing from the listing is effectively missing from the product.
func TestRolesCmd_ListsEverySemanticRole(t *testing.T) {
	output := runRolesCmd(t, writeRolesConfig(t, `["button"]`))

	for _, mapping := range element.RoleVocabulary {
		if !strings.Contains(output, string(mapping.Semantic)) {
			t.Errorf("neru roles output omits semantic role %q", mapping.Semantic)
		}
	}

	if !strings.Contains(output, runtime.GOOS) {
		t.Errorf("neru roles output does not name the platform %q", runtime.GOOS)
	}

	vocabulary, supported := element.CurrentVocabulary()
	if !supported {
		return
	}

	// Roles that apply here must show their native expansion; roles that do
	// not must say so rather than render an empty column.
	for _, mapping := range element.RoleVocabulary {
		native := mapping.Native(vocabulary)
		if len(native) == 0 {
			continue
		}

		if !strings.Contains(output, native[0]) {
			t.Errorf(
				"neru roles output omits the expansion %q of role %q",
				native[0], mapping.Semantic,
			)
		}
	}
}

// TestRolesCmd_MarksAXSubroleNames checks that the listing tells a macOS user
// which AX names are subroles. AppKit delivers those names in AXSubrole while
// AXRole stays generic, so the mark is how the expansion column stays an
// honest description of what an element reports.
func TestRolesCmd_MarksAXSubroleNames(t *testing.T) {
	vocabulary, supported := element.CurrentVocabulary()
	if !supported || vocabulary != element.VocabularyAX {
		t.Skipf("AX names are only listed on macOS, not %s", runtime.GOOS)
	}

	output := runRolesCmd(t, writeRolesConfig(t, `["button"]`))

	for name := range element.AXSubroleNames {
		if !strings.Contains(output, name+" (subrole)") {
			t.Errorf("neru roles output does not mark %q as a subrole:\n%s", name, output)
		}
	}
}

// TestRolesCmd_ExplainReportsEveryEntry covers the --explain path, including
// the entries that do not apply on this platform — the case the command exists
// to make visible.
func TestRolesCmd_ExplainReportsEveryEntry(t *testing.T) {
	entries := []string{"button", "ax:AXGenericElement", "atspi:page tab list", "uia:Custom"}
	path := writeRolesConfig(
		t,
		`["button", "ax:AXGenericElement", "atspi:page tab list", "uia:Custom"]`,
	)

	output := runRolesCmd(t, path, "--explain")

	for _, entry := range entries {
		if !strings.Contains(output, entry) {
			t.Errorf("neru roles --explain omits configured entry %q:\n%s", entry, output)
		}
	}

	if !strings.Contains(output, "resolve to") {
		t.Errorf("neru roles --explain has no summary line:\n%s", output)
	}

	// Exactly two of the three prefixed entries belong to other platforms.
	if !strings.Contains(output, "do not apply on "+runtime.GOOS) {
		t.Errorf("neru roles --explain does not report inapplicable entries:\n%s", output)
	}
}

// TestRolesCmd_ExplainRejectsRetiredVocabulary checks that --explain surfaces a
// stale config as an error rather than rendering a misleading empty result.
func TestRolesCmd_ExplainRejectsRetiredVocabulary(t *testing.T) {
	path := writeRolesConfig(t, `["AXButton"]`)

	_, stderr, err := executeRoot(t, cliTestRoles, "--config", path, "--explain")
	if err == nil {
		t.Fatal("neru roles --explain accepted a config using the retired vocabulary")
	}

	if !strings.Contains(stderr, `use "button"`) {
		t.Errorf("neru roles --explain did not print the migration hint:\n%s", stderr)
	}
}

// TestRolesCmd_JSONListsEverySemanticRole pins `neru roles --json`: one entry
// per semantic role, with the native roles it expands to here.
func TestRolesCmd_JSONListsEverySemanticRole(t *testing.T) {
	output := runRolesCmd(t, writeRolesConfig(t, `["button"]`), "--json")

	var listing struct {
		Platform   string `json:"platform"`
		Vocabulary string `json:"vocabulary"`
		Roles      []struct {
			Semantic string   `json:"semantic"`
			Native   []string `json:"native"`
			Subrole  bool     `json:"subrole"`
		} `json:"roles"`
	}

	err := json.Unmarshal([]byte(output), &listing)
	if err != nil {
		t.Fatalf("neru roles --json is not a JSON object: %v\n%s", err, output)
	}

	vocabulary, _ := element.CurrentVocabulary()
	if listing.Platform != runtime.GOOS || listing.Vocabulary != string(vocabulary) {
		t.Errorf("platform, vocabulary = %q, %q, want %q, %q",
			listing.Platform, listing.Vocabulary, runtime.GOOS, vocabulary)
	}

	if len(listing.Roles) != len(element.RoleVocabulary) {
		t.Fatalf("listed %d roles, want %d", len(listing.Roles), len(element.RoleVocabulary))
	}

	for index, mapping := range element.RoleVocabulary {
		got := listing.Roles[index]

		want := append([]string{}, mapping.Native(vocabulary)...)

		if got.Semantic != string(mapping.Semantic) || !slices.Equal(got.Native, want) {
			t.Errorf("role %d = %s %v, want %s %v",
				index, got.Semantic, got.Native, mapping.Semantic, want)
		}

		wantSubrole := vocabulary == element.VocabularyAX && mapping.AXIsSubrole
		if got.Subrole != wantSubrole {
			t.Errorf("%s subrole = %v, want %v", got.Semantic, got.Subrole, wantSubrole)
		}
	}
}

// TestRolesCmd_ExplainJSONReportsEveryEntry pins `neru roles --explain --json`:
// every configured entry in order, and the reason for each one that does not
// apply here.
func TestRolesCmd_ExplainJSONReportsEveryEntry(t *testing.T) {
	entries := []string{"button", "ax:AXGenericElement", "atspi:page tab list", "uia:Custom"}
	path := writeRolesConfig(
		t,
		`["button", "ax:AXGenericElement", "atspi:page tab list", "uia:Custom"]`,
	)

	output := runRolesCmd(t, path, "--explain", "--json")

	var explanation struct {
		Platform string `json:"platform"`
		Config   string `json:"config"`
		Entries  []struct {
			Entry      string   `json:"entry"`
			Vocabulary string   `json:"vocabulary"`
			Native     []string `json:"native"`
			Ignored    string   `json:"ignored"`
		} `json:"entries"`
		Native []string `json:"native"`
	}

	err := json.Unmarshal([]byte(output), &explanation)
	if err != nil {
		t.Fatalf("neru roles --explain --json is not a JSON object: %v\n%s", err, output)
	}

	if explanation.Config != path {
		t.Errorf("config = %q, want %q", explanation.Config, path)
	}

	if len(explanation.Entries) != len(entries) {
		t.Fatalf("explained %d entries, want %d", len(explanation.Entries), len(entries))
	}

	ignored := 0

	for index, entry := range explanation.Entries {
		if entry.Entry != entries[index] {
			t.Errorf("entry %d = %q, want %q", index, entry.Entry, entries[index])
		}

		if entry.Native == nil {
			t.Errorf("%s: native is null, want a list", entry.Entry)
		}

		if entry.Ignored != "" {
			ignored++

			if len(entry.Native) != 0 {
				t.Errorf("%s is ignored but contributes %v", entry.Entry, entry.Native)
			}
		}
	}

	if _, supported := element.CurrentVocabulary(); supported && ignored != 2 {
		t.Errorf("%d entries ignored, want the two prefixed for other platforms", ignored)
	}
}
