package cli

import (
	_ "embed"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/buildinfo"
)

//go:embed agent_guide.md
var agentGuide string

var agentGuideCmd = &cobra.Command{
	Use:   "agent-guide",
	Short: "Print a guide for AI agents driving this build",
	Long: `Print a short guide for an AI agent or another program that drives Neru:
how to learn what this build accepts, how to read its state, how to drive it,
and the rules a tool built on it follows.

It is written for this build, with its version, protocol and docs link, so an
agent can read it instead of docs for a different release. It needs no
running daemon.`,
	Example: `  neru agent-guide`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := cmd.OutOrStdout().Write([]byte(renderAgentGuide(buildinfo.Version)))

		return err
	},
}

func init() {
	RootCmd.AddCommand(agentGuideCmd)
}

// renderAgentGuide fills in what the guide says about the build reading it.
func renderAgentGuide(version string) string {
	return strings.NewReplacer(
		"{{version}}", version,
		"{{protocol}}", strconv.Itoa(ipc.ProtocolVersion),
		"{{ref}}", buildinfo.Ref(version),
		"{{docs}}", buildinfo.DocsURL("docs/README.md", version),
	).Replace(agentGuide)
}
