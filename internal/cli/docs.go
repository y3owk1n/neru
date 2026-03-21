package cli

import (
	"github.com/spf13/cobra"
)

// DocsCmd is the CLI docs command for opening Neru documentation in the browser.
//
// macOS: uses open.
// Other platforms: stubbed and returns CodeNotSupported until implemented.
var DocsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Open Neru documentation in the browser",
	Long:  "Open version-aware Neru documentation pages in the default browser.",
}

// DocsCLICmd is the CLI docs cli subcommand.
var DocsCLICmd = &cobra.Command{
	Use:   "cli",
	Short: "Open CLI documentation",
	Long:  "Open the Neru CLI documentation in the default browser.",
	RunE: func(_ *cobra.Command, _ []string) error {
		return openDocsPage("docs/CLI.md")
	},
}

// DocsConfigCmd is the CLI docs config subcommand.
var DocsConfigCmd = &cobra.Command{
	Use:     "config",
	Aliases: []string{"configuration"},
	Short:   "Open configuration documentation",
	Long:    "Open the Neru configuration documentation in the default browser.",
	RunE: func(_ *cobra.Command, _ []string) error {
		return openDocsPage("docs/CONFIGURATION.md")
	},
}

func init() {
	DocsCmd.AddCommand(DocsCLICmd)
	DocsCmd.AddCommand(DocsConfigCmd)
	RootCmd.AddCommand(DocsCmd)
}
