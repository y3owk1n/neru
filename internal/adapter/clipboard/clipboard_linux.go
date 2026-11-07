//go:build linux

package clipboard

import (
	"context"
	"os"
	"os/exec"

	"github.com/y3owk1n/neru/internal/derrors"
)

// copierCandidates is each copier Neru knows, in the order it is tried, with
// the display socket it needs. wl-copy talks Wayland, and xclip and xsel talk
// X11, Xwayland included.
var copierCandidates = []struct {
	socket string
	argv   []string
}{
	{socket: "WAYLAND_DISPLAY", argv: []string{"wl-copy"}},
	{socket: "DISPLAY", argv: []string{"xclip", "-selection", "clipboard"}},
	{socket: "DISPLAY", argv: []string{"xsel", "--clipboard", "--input"}},
}

// writeText sets the clipboard through the first copier whose display socket
// is exported and that is installed. It looks each time rather than once, so
// a copier installed while Neru runs is picked up on the next copy.
func writeText(ctx context.Context, text string) error {
	argv, ok := findCopier(os.Getenv, exec.LookPath)
	if !ok {
		return derrors.New(
			derrors.CodeNotSupported,
			"no clipboard tool found: install wl-clipboard on Wayland, or xclip or xsel on X11",
		)
	}

	return runCopier(ctx, argv, nil, text)
}

// findCopier returns the first candidate whose socket getenv reports and whose
// command lookPath finds.
func findCopier(
	getenv func(string) string,
	lookPath func(string) (string, error),
) ([]string, bool) {
	for _, candidate := range copierCandidates {
		if getenv(candidate.socket) == "" {
			continue
		}

		_, err := lookPath(candidate.argv[0])
		if err == nil {
			return candidate.argv, true
		}
	}

	return nil, false
}
