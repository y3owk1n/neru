//go:build darwin

package clipboard

import "context"

// pbcopyPath is pbcopy's fixed location, which every macOS install has.
const pbcopyPath = "/usr/bin/pbcopy"

// writeText sets the clipboard through pbcopy. pbcopy reads its input in the
// encoding LANG names, and a daemon started at login may have none, so UTF-8
// is named explicitly or anything past ASCII would be mangled.
func writeText(ctx context.Context, text string) error {
	return runCopier(ctx, []string{pbcopyPath}, []string{"LANG=en_US.UTF-8"}, text)
}
