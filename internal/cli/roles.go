package cli

import (
	"runtime"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// rolesColumnPadding is the gap between the role name and its expansion.
const rolesColumnPadding = 2

// explainRoles is set by the --explain flag, and rolesJSON by --json.
var explainRoles, rolesJSON bool

// rolesListing is `neru roles --json`.
type rolesListing struct {
	Platform   string        `json:"platform"`
	Vocabulary string        `json:"vocabulary"`
	Roles      []roleListing `json:"roles"`
}

// roleListing is one semantic role and the native roles it expands to here.
type roleListing struct {
	Semantic string   `json:"semantic"`
	Native   []string `json:"native"`
	Subrole  bool     `json:"subrole"`
}

// rolesExplanation is `neru roles --explain --json`.
type rolesExplanation struct {
	Platform string            `json:"platform"`
	Config   string            `json:"config"`
	Entries  []roleExplanation `json:"entries"`
	Native   []string          `json:"native"`
}

// roleExplanation is one configured entry and what it contributes here.
type roleExplanation struct {
	Entry      string   `json:"entry"`
	Vocabulary string   `json:"vocabulary"`
	Native     []string `json:"native"`
	Ignored    string   `json:"ignored"`
}

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
which native roles each entry contributes here and which entries do not apply.

Use --json with either form to print the answer as a JSON object.`,
	Example: `  neru roles             Show the semantic vocabulary for this platform
  neru roles --explain   Show how the loaded config resolves here
  neru roles --json      Print the vocabulary as a JSON object`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if explainRoles {
			return runRolesExplain(cmd)
		}

		if rolesJSON {
			return formatter.PrintJSON(cmd, listRoles())
		}

		runRolesList(cmd)

		return nil
	},
}

func init() {
	rolesCmd.Flags().BoolVar(&explainRoles, "explain", false,
		"resolve the loaded config entry by entry")
	rolesCmd.Flags().BoolVar(&rolesJSON, "json", false, "Print the answer as a JSON object")
	RootCmd.AddCommand(rolesCmd)
}

// listRoles is the semantic vocabulary and its expansion here.
func listRoles() rolesListing {
	vocabulary, _ := element.CurrentVocabulary()

	listing := rolesListing{
		Platform:   runtime.GOOS,
		Vocabulary: string(vocabulary),
		Roles:      make([]roleListing, 0, len(element.RoleVocabulary)),
	}

	for _, mapping := range element.RoleVocabulary {
		listing.Roles = append(listing.Roles, roleListing{
			Semantic: string(mapping.Semantic),
			Native:   append([]string{}, mapping.Native(vocabulary)...),
			Subrole:  vocabulary == element.VocabularyAX && mapping.AXIsSubrole,
		})
	}

	return listing
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
	svc := clientConfigLoader()

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

	resolution := element.ResolveRolesForCurrentPlatform(
		loadResult.Config.Hints.ClickableRoles,
	)

	if rolesJSON {
		return formatter.PrintJSON(cmd, explainRolesJSON(loadResult.ConfigPath, resolution))
	}

	source := loadResult.ConfigPath
	if source == "" {
		source = "built-in defaults (no config file found)"
	}

	cmd.Printf("hints.clickable_roles on %s, from %s\n\n", runtime.GOOS, source)

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

// explainRolesJSON is a resolved configuration as --explain --json prints it.
// A configuration that loaded has no fatal entry, so a diagnostic left on an
// entry is the reason it does not apply here.
func explainRolesJSON(configPath string, resolution element.RoleResolution) rolesExplanation {
	explanation := rolesExplanation{
		Platform: runtime.GOOS,
		Config:   configPath,
		Entries:  make([]roleExplanation, 0, len(resolution.Entries)),
		Native:   append([]string{}, resolution.Native...),
	}

	for _, entry := range resolution.Entries {
		explained := roleExplanation{
			Entry:      entry.Entry,
			Vocabulary: string(entry.Vocabulary),
			Native:     append([]string{}, entry.Native...),
		}

		if entry.Diagnostic != nil {
			explained.Ignored = entry.Diagnostic.Message()
		}

		explanation.Entries = append(explanation.Entries, explained)
	}

	return explanation
}

// printClickableRolesCheck reports how the configured clickable roles resolve
// on this platform, and returns whether they are usable. It is a client-side
// doctor check: a config whose roles all belong to another platform loads
// cleanly but produces no hints at all, which is otherwise invisible until a
// user reports a blank overlay.
//
// The result feeds the doctor exit status, so a health check run from a script
// fails on a configuration that cannot hint anything.
func printClickableRolesCheck(cmd *cobra.Command, loadResult *config.LoadResult) bool {
	if loadResult.ValidationError != nil {
		cmd.Printf("  ❌ %-24s %s\n", "clickable_roles", "config invalid: see neru config validate")

		return false
	}

	check := checkClickableRoles(loadResult)
	if !check.Usable {
		cmd.Printf("  ❌ %-24s %s\n", "clickable_roles",
			"no roles apply on "+runtime.GOOS+"; hints would be empty")
		cmd.Printf("  %-27s %s\n", "", "run: neru roles --explain")

		return false
	}

	cmd.Printf("  ✅ %-24s %d entries → %d native roles on %s\n",
		"clickable_roles", check.Entries, check.Native, runtime.GOOS)

	if check.Ignored > 0 {
		cmd.Printf(
			"  %-27s %d entries do not apply here (neru roles --explain)\n",
			"",
			check.Ignored,
		)
	}

	return true
}

// clickableRolesCheck is how the configured clickable roles resolve here.
type clickableRolesCheck struct {
	Entries int  `json:"entries"`
	Native  int  `json:"native"`
	Ignored int  `json:"ignored"`
	Usable  bool `json:"usable"`
}

// checkClickableRoles resolves the configured clickable roles. An invalid
// configuration resolves to nothing usable.
func checkClickableRoles(loadResult *config.LoadResult) clickableRolesCheck {
	if loadResult.ValidationError != nil {
		return clickableRolesCheck{}
	}

	resolution := element.ResolveRolesForCurrentPlatform(
		loadResult.Config.Hints.ClickableRoles,
	)

	return clickableRolesCheck{
		Entries: len(resolution.Entries),
		Native:  len(resolution.Native),
		Ignored: len(resolution.IgnoredMessages()),
		Usable:  len(resolution.Native) > 0,
	}
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
