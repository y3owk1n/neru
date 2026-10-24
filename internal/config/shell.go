package config

import (
	"path/filepath"
	"regexp"
	"strings"

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

// windowsAbsolutePath matches a path rooted at a drive letter or a UNC share.
var windowsAbsolutePath = regexp.MustCompile(`^([A-Za-z]:[\\/]|\\\\)`)

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

// isAbsoluteOn reports whether path is absolute on goos rather than on the
// host, so a shared file's windows entry is checked as a Windows path on every
// machine.
func isAbsoluteOn(goos, path string) bool {
	if parity.Platform(goos) == parity.Windows {
		return windowsAbsolutePath.MatchString(path)
	}

	return strings.HasPrefix(path, "/")
}

// validateShell refuses a shell command that cannot run an exec step on the
// platform it is written for, and warns about each deprecated key that is set.
func (g GeneralConfig) validateShell(warnings *Warnings) error {
	for _, entry := range g.Shell.entries() {
		if len(entry.value) < minShellCommandParts {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"general.shell.%s needs the shell and at least one argument, "+
					"since Neru appends the step's command string last",
				entry.platform,
			)
		}

		if !isAbsoluteOn(string(entry.platform), entry.value[0]) {
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
