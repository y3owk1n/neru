package cli

import (
	"context"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// ReadTextLinesFunc is set by main to serve ReadTextLinesCmd: read one request
// from in and write the lines read to out.
var ReadTextLinesFunc func(ctx context.Context, in io.Reader, out io.Writer) error

// ReadTextLinesCmd reads the text lines of the image on its stdin accurately
// and writes them to stdout. The daemon runs it as a helper process for each
// accurate read, so a model compiling for a new script holds up no read in
// the daemon. It is hidden, since it is not for users.
var ReadTextLinesCmd = &cobra.Command{
	Use:    "read-text-lines",
	Short:  "Read the text lines of the image on stdin and exit",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if ReadTextLinesFunc == nil {
			return nil
		}

		return ReadTextLinesFunc(cmd.Context(), os.Stdin, os.Stdout)
	},
}

func init() {
	RootCmd.AddCommand(ReadTextLinesCmd)
}
