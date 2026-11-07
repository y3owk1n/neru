//go:build !windows

package clipboard

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/y3owk1n/neru/internal/derrors"
)

// waitDelay bounds how long a killed copier may keep its pipes open.
const waitDelay = 500 * time.Millisecond

// runCopier runs a command that reads the clipboard's new contents on its
// standard input, such as pbcopy or wl-copy, with extra variables added to the
// environment.
//
// Its output goes nowhere. wl-copy, xclip and xsel fork a child that keeps
// serving the selection after the parent exits, and a captured output pipe
// would keep the wait open until that child exits too.
func runCopier(ctx context.Context, argv []string, env []string, text string) error {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = strings.NewReader(text)

	cmd.Env = append(os.Environ(), env...)
	cmd.WaitDelay = waitDelay

	err := cmd.Run()
	if err != nil {
		return derrors.Wrapf(err, derrors.CodeActionFailed, "%s failed", argv[0])
	}

	return nil
}
