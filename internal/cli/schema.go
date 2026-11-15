package cli

import (
	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/configschema"
	"github.com/y3owk1n/neru/internal/ipcschema"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print the JSON Schemas this build speaks",
	Long: `Print a JSON Schema this build of Neru speaks, the same file the repository
publishes for this version, without fetching anything.

Subcommands:
  ipc      The IPC requests, replies and watch lines
  config   config.toml, with each option's type and default

It needs no running daemon and no config.`,
	Example: `  neru schema ipc > ipc.schema.json
  neru schema config | jq '.properties.hints'`,
}

var schemaIPCCmd = &cobra.Command{
	Use:   "ipc",
	Short: "Print the IPC schema",
	Long: `Print the JSON Schema for the IPC protocol: one definition per request,
reply payload and watch line, as docs/reference/ipc.md describes them.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return printSchema(cmd, ipcschema.Render)
	},
}

// schemaConfigName is the subcommand that prints the config schema.
const schemaConfigName = "config"

var schemaConfigCmd = &cobra.Command{
	Use:   schemaConfigName,
	Short: "Print the config schema",
	Long: `Print the JSON Schema for config.toml, with each option's type and default.
Editors that read JSON Schema can validate a config with it.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return printSchema(cmd, configschema.Render)
	},
}

func init() {
	schemaCmd.AddCommand(schemaIPCCmd)
	schemaCmd.AddCommand(schemaConfigCmd)
	RootCmd.AddCommand(schemaCmd)
}

func printSchema(cmd *cobra.Command, render func() ([]byte, error)) error {
	out, err := render()
	if err != nil {
		return err
	}

	_, err = cmd.OutOrStdout().Write(out)

	return err
}
