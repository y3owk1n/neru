package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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

	var out bytes.Buffer

	full := append([]string{cliTestRoles, "--config", configPath}, args...)

	cli.RootCmd.SetOut(&out)
	cli.RootCmd.SetErr(&out)
	cli.RootCmd.SetArgs(full)

	t.Cleanup(func() {
		cli.RootCmd.SetOut(nil)
		cli.RootCmd.SetErr(nil)
		cli.RootCmd.SetArgs(nil)
	})

	err := cli.RootCmd.Execute()
	if err != nil {
		t.Fatalf("neru %v failed: %v", full, err)
	}

	return out.String()
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

	var out bytes.Buffer

	cli.RootCmd.SetOut(&out)
	cli.RootCmd.SetErr(&out)
	cli.RootCmd.SetArgs([]string{cliTestRoles, "--config", path, "--explain"})

	t.Cleanup(func() {
		cli.RootCmd.SetOut(nil)
		cli.RootCmd.SetErr(nil)
		cli.RootCmd.SetArgs(nil)
	})

	err := cli.RootCmd.Execute()
	if err == nil {
		t.Fatal("neru roles --explain accepted a config using the retired vocabulary")
	}

	if !strings.Contains(out.String(), `use "button"`) {
		t.Errorf("neru roles --explain did not print the migration hint:\n%s", out.String())
	}
}
