package config

import (
	"path/filepath"
	"runtime"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// The shells general.shell starts with. Windows names cmd.exe by its usual
// path, since a table cannot read %SystemRoot%. An install on another volume
// sets the windows entry.
const (
	defaultUnixShell     = "/bin/bash"
	defaultUnixShellFlag = "-lc"
	defaultWindowsShell  = `C:\Windows\System32\cmd.exe`
	defaultWindowsFlag   = "/c"
	minShellCommandParts = 2
)

func defaultShell() PerPlatform[[]string] {
	return PerPlatform[[]string]{
		Darwin:  []string{defaultUnixShell, defaultUnixShellFlag},
		Linux:   []string{defaultUnixShell, defaultUnixShellFlag},
		Windows: []string{defaultWindowsShell, defaultWindowsFlag},
	}
}

// ShellCommand returns the command an exec step runs on goos, before the step's
// command string is appended. It is general.shell, with the deprecated
// exec_shell and exec_shell_args replacing the shell and its arguments when
// they are set and not empty.
func (g GeneralConfig) ShellCommand(goos string) []string {
	var (
		shell string
		args  []string
	)

	if argv := g.Shell.ForOS(goos); len(argv) > 0 {
		shell, args = argv[0], argv[1:]
	}

	if g.ExecShell != nil && *g.ExecShell != "" {
		shell = *g.ExecShell
	}

	if g.ExecShellArgs != nil && len(*g.ExecShellArgs) > 0 {
		args = *g.ExecShellArgs
	}

	return append([]string{shell}, args...)
}

// validateShell refuses a shell command that cannot run an exec step, and warns
// about each deprecated key that is set. Every entry needs an argument. Only
// the running platform's entry needs an absolute path, so one value such as
// ["/bin/zsh", "-lc"] loads on macOS and Linux, and Windows refuses it.
func (g GeneralConfig) validateShell(warnings *Warnings) error {
	// The entry this machine reads: its GOOS, or linux for every platform
	// that is neither darwin nor windows.
	running := parity.Platform(runtime.GOOS)
	if running != parity.Darwin && running != parity.Windows {
		running = parity.Linux
	}

	for _, entry := range g.Shell.entries() {
		if len(entry.value) < minShellCommandParts {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"general.shell.%s needs the shell and at least one argument, "+
					"since Neru appends the step's command string last",
				entry.platform,
			)
		}

		if entry.platform == running && !filepath.IsAbs(entry.value[0]) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"general.shell.%s must start with an absolute path (got: %q)",
				entry.platform,
				entry.value[0],
			)
		}
	}

	if g.ExecShell != nil && *g.ExecShell != "" {
		if !filepath.IsAbs(*g.ExecShell) {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"general.exec_shell must be an absolute path (got: %q)",
				*g.ExecShell,
			)
		}

		warnings.Addf(
			"general.exec_shell is deprecated and is removed in v2. Move it to general.shell",
		)
	}

	if g.ExecShellArgs != nil && len(*g.ExecShellArgs) > 0 {
		warnings.Addf(
			"general.exec_shell_args is deprecated and is removed in v2. Move it to general.shell",
		)
	}

	return nil
}
