package cli

import (
	"runtime"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// rolesColumnPadding is the gap between the role name and its expansion.
const rolesColumnPadding = 2

// explainRoles is set by the --explain flag.
var explainRoles bool

// rolesCmd is the CLI roles command.
var rolesCmd = &cobra.Command{
	Use:   "roles",
	Short: "List the accessibility roles accepted in hints.clickable_roles",
	Long: `List the semantic role vocabulary accepted by hints.clickable_roles and
show how each one resolves on this platform.

Roles are written either as a semantic name ("button", "text_field"), which
resolves to the native accessibility roles of whichever platform neru is running
on, or as a native role addressed through its vocabulary prefix:

  ax:AXDisclosureTriangle    macOS Accessibility
  atspi:page tab list        Linux AT-SPI
  uia:Custom                 Windows UI Automation

Prefixed entries for other platforms are ignored rather than rejected, so one
configuration file can serve several machines.

With --explain, the loaded configuration is resolved entry by entry, showing
which native roles each entry contributes here and which entries do not apply.`,
	Example: `  neru roles             Show the semantic vocabulary for this platform
  neru roles --explain   Show how the loaded config resolves here`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if explainRoles {
			return runRolesExplain(cmd)
		}

		runRolesList(cmd)

		return nil
	},
}

func init() {
	rolesCmd.Flags().BoolVar(&explainRoles, "explain", false,
		"resolve the loaded config entry by entry")
	RootCmd.AddCommand(rolesCmd)
}

// runRolesList prints the semantic vocabulary and its expansion here.
func runRolesList(cmd *cobra.Command) {
	vocabulary, supported := element.CurrentVocabulary()
	if !supported {
		cmd.Printf("No accessibility backend on %s: every role resolves to nothing.\n\n",
			runtime.GOOS)
	} else {
		cmd.Printf("Semantic roles on %s (native vocabulary: %s)\n\n",
			runtime.GOOS, vocabulary)
	}

	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, rolesColumnPadding, ' ', 0)

	for _, mapping := range element.RoleVocabulary {
		native := mapping.Native(vocabulary)

		names := make([]string, len(native))
		for index, name := range native {
			names[index] = name
			// AppKit declares some AX names as subroles: an element carries
			// them in AXSubrole while its AXRole stays generic, and neru
			// matches them against both. Say so, or the column misdescribes
			// what an element reports as its role.
			if vocabulary == element.VocabularyAX && element.AXSubroleNames[name] {
				names[index] = name + " (subrole)"
			}
		}

		expansion := strings.Join(names, ", ")
		if len(names) == 0 {
			expansion = "— no equivalent on this platform"
		}

		writeTabbed(writer, string(mapping.Semantic), expansion)
	}

	flushTabbed(writer)

	cmd.Println("")
	cmd.Println("Native roles not covered above are addressed by prefix, e.g. " +
		prefixExample(vocabulary))
}

// runRolesExplain resolves the loaded configuration and reports each entry.
func runRolesExplain(cmd *cobra.Command) error {
	svc := loader.NewService(config.DefaultConfig(), "", nil, nil)

	path := configPath
	if path == "" {
		path = svc.FindConfigFile()
	}

	loadResult := svc.LoadWithValidation(path)
	if loadResult.ValidationError != nil {
		cmd.PrintErrln("Configuration validation failed:")
		cmd.PrintErrln("")
		cmd.PrintErrln("  " + loadResult.ValidationError.Error())

		return &silentError{err: errConfigValidationFailed}
	}

	source := loadResult.ConfigPath
	if source == "" {
		source = "built-in defaults (no config file found)"
	}

	cmd.Printf("hints.clickable_roles on %s, from %s\n\n", runtime.GOOS, source)

	resolution := element.ResolveRolesForCurrentPlatform(
		loadResult.Config.Hints.ClickableRoles,
	)

	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, rolesColumnPadding, ' ', 0)

	for _, entry := range resolution.Entries {
		writeTabbed(writer, entry.Entry, explainEntry(entry))
	}

	flushTabbed(writer)

	cmd.Printf("\n%d configured entries resolve to %d native roles.\n",
		len(resolution.Entries), len(resolution.Native))

	ignored := resolution.IgnoredMessages()
	if len(ignored) > 0 {
		cmd.Printf("%d entries do not apply on %s.\n", len(ignored), runtime.GOOS)
	}

	return nil
}

// printClickableRolesCheck reports how the configured clickable roles resolve
// on this platform, and returns whether they are usable. It is a client-side
// doctor check: a config whose roles all belong to another platform loads
// cleanly but produces no hints at all, which is otherwise invisible until a
// user reports a blank overlay.
//
// The result feeds the doctor exit status, so a health check run from a script
// fails on a configuration that cannot hint anything.
func printClickableRolesCheck(cmd *cobra.Command) bool {
	svc := loader.NewService(config.DefaultConfig(), "", nil, nil)

	path := configPath
	if path == "" {
		path = svc.FindConfigFile()
	}

	loadResult := svc.LoadWithValidation(path)
	if loadResult.ValidationError != nil {
		cmd.Printf("  ❌ %-24s %s\n", "clickable_roles", "config invalid: see neru config validate")

		return false
	}

	resolution := element.ResolveRolesForCurrentPlatform(
		loadResult.Config.Hints.ClickableRoles,
	)

	if len(resolution.Native) == 0 {
		cmd.Printf("  ❌ %-24s %s\n", "clickable_roles",
			"no roles apply on "+runtime.GOOS+"; hints would be empty")
		cmd.Printf("  %-27s %s\n", "", "run: neru roles --explain")

		return false
	}

	cmd.Printf("  ✅ %-24s %d entries → %d native roles on %s\n",
		"clickable_roles", len(resolution.Entries), len(resolution.Native), runtime.GOOS)

	if ignored := len(resolution.IgnoredMessages()); ignored > 0 {
		cmd.Printf("  %-27s %d entries do not apply here (neru roles --explain)\n", "", ignored)
	}

	return true
}

// explainEntry renders the resolution of a single configured entry.
func explainEntry(entry element.ResolvedRoleEntry) string {
	if len(entry.Native) > 0 {
		kind := "semantic"
		if entry.Vocabulary != "" {
			kind = "native/" + string(entry.Vocabulary)
		}

		return strings.Join(entry.Native, ", ") + "  [" + kind + "]"
	}

	if entry.Diagnostic != nil {
		return entry.Diagnostic.Message()
	}

	return "no native roles"
}

// prefixExample returns a prefixed-entry example for the running platform.
func prefixExample(vocabulary element.NativeVocabulary) string {
	switch vocabulary {
	case element.VocabularyATSPI:
		return `"atspi:page tab list"`
	case element.VocabularyUIA:
		return `"uia:Custom"`
	case element.VocabularyAX:
		return `"ax:AXDisclosureTriangle"`
	default:
		return `"ax:...", "atspi:..." or "uia:..."`
	}
}

// writeTabbed writes one aligned two-column row, ignoring write errors: the
// destination is the command's output stream and a failure there is already
// surfaced by the caller's own write failures.
func writeTabbed(writer *tabwriter.Writer, left, right string) {
	_, _ = writer.Write([]byte("  " + left + "\t" + right + "\n"))
}

// flushTabbed flushes the aligned output.
func flushTabbed(writer *tabwriter.Writer) {
	_ = writer.Flush()
}
