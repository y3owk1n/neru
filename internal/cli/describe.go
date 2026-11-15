package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/describe"
)

// describeJSON is set by --json.
var describeJSON bool

var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "List every word Neru reads, and where each works",
	Long: `List every mode, mode flag, action, config option, event and hook variable
this build of Neru reads, with the platforms each one does something on.

It describes the binary, so it needs no running daemon and no config. A
program that drives Neru can read it once to learn which names this version
accepts and where each works. Each command's own flags are in its --help. The
daemon answers the same object to the IPC command describe.

Without --json it prints a summary and the words that do nothing on this
platform. With --json it prints the whole object. The config schema, with
each option's type and default, is what 'neru schema config' prints.`,
	Example: `  neru describe
  neru describe --json | jq -r '.actions[] | select(.here) | .name'`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		manifest := describe.Manifest()

		if describeJSON {
			return formatter.PrintJSON(cmd, manifest)
		}

		return printDescription(cmd, manifest)
	},
}

func init() {
	describeCmd.Flags().
		BoolVar(&describeJSON, "json", false, "Print the whole description as a JSON object")
	RootCmd.AddCommand(describeCmd)
}

// printDescription prints the counts, then each word that does nothing here
// with the reason.
func printDescription(cmd *cobra.Command, manifest ipc.DescribeData) error {
	platform := manifest.Platform
	if platform == "" {
		platform = "an undeclared platform"
	}

	var out strings.Builder

	fmt.Fprintf(
		&out,
		"neru %s, protocol %d, on %s\n\n",
		manifest.Version,
		manifest.Protocol,
		platform,
	)
	fmt.Fprintf(&out, "Modes:       %s\n", strings.Join(manifest.Modes, ", "))
	fmt.Fprintf(&out, "Mode flags:  %d\n", len(manifest.ModeFlags))
	fmt.Fprintf(&out, "Actions:     %d\n", len(manifest.Actions))
	fmt.Fprintf(&out, "Options:     %d\n", len(manifest.Options))
	fmt.Fprintf(&out, "Events:      %d\n", len(manifest.Events))

	missing := unavailable(manifest)
	if len(missing) == 0 {
		fmt.Fprintf(&out, "\nEvery word works on %s.\n", platform)
	} else {
		fmt.Fprintf(&out, "\nDoes nothing on %s:\n", platform)

		for _, line := range missing {
			fmt.Fprintf(&out, "  %s\n", line)
		}
	}

	_, err := cmd.OutOrStdout().Write([]byte(out.String()))

	return err
}

// unavailable lists each word that does nothing here, with its kind and the
// reason, in the manifest's order.
func unavailable(manifest ipc.DescribeData) []string {
	var lines []string

	add := func(written, kind, note string) {
		line := written + " (" + kind + ")"
		if note != "" {
			line += ": " + note
		}

		lines = append(lines, line)
	}

	for _, flag := range manifest.ModeFlags {
		if !flag.Here {
			add("--"+flag.Name, "mode flag", flag.Note)
		}

		for _, value := range flag.Values {
			if !value.Here {
				add("--"+flag.Name+"="+value.Value, "mode flag", value.Note)
			}
		}
	}

	for _, described := range manifest.Actions {
		if !described.Here {
			add(described.Name, "action", described.Note)
		}
	}

	for _, described := range manifest.Events {
		if !described.Here {
			add(described.Name, "event", described.Note)
		}
	}

	for _, option := range manifest.Options {
		if !option.Here {
			add(option.Name, "option", option.Note)
		}

		for _, value := range option.Values {
			if !value.Here {
				add(option.Name+" = "+value.Value, "option", value.Note)
			}
		}
	}

	return lines
}
