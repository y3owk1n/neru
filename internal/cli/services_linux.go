//go:build linux

package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/y3owk1n/neru/internal/buildinfo"
	"github.com/y3owk1n/neru/internal/derrors"
)

// Service management on Linux is a systemd user unit, and only that. Neru
// needs a display server, so it belongs to the user manager, ordered against
// the graphical session. Other init systems refuse loudly and name systemd
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md).
const (
	// serviceUnitName stays the plain program name because users type it into
	// `systemctl --user` by hand.
	serviceUnitName = "neru.service"

	// systemdRuntimeMarker is what sd_booted(3) checks. `systemctl` on PATH
	// proves nothing, since it ships on machines running another init.
	systemdRuntimeMarker = "/run/systemd/system"

	// linuxServicesDocs is the page for when service management cannot help.
	linuxServicesDocs = "docs/guide/linux.md#systemd-user-service"

	unitDirPerm  = 0o755
	unitFilePerm = 0o644
)

// systemctlTimeout bounds every call to the user manager. `enable --now` starts
// a Type=simple service, so nothing here legitimately waits on the daemon
// coming up. The budget is for `daemon-reload` on a busy machine.
const systemctlTimeout = 15 * time.Second

// unitEnabled and unitDisabled are the is-enabled answers start and stop leave
// behind.
const (
	unitEnabled  = "enabled"
	unitDisabled = "disabled"
)

// serviceUnitMarker is the line Neru writes into every unit it installs, and
// the only evidence that a neru.service is Neru's to disable and delete. The
// path proves nothing, since units hand-written from the old setup guide sit
// at the same path under the same name.
const serviceUnitMarker = "# Installed by `neru services install`"

// serviceUnitTemplate is the systemd user unit Neru installs. It anchors on
// graphical-session.target rather than default.target, because starting
// before a display server exists only crash loops. After=, WantedBy= and
// PartOf= together let it survive a logout/login cycle instead of lingering
// attached to a dead display.
const serviceUnitTemplate = serviceUnitMarker + `
# Neru rewrites this file on install and deletes it on uninstall. A neru.service
# without the line above is one Neru leaves alone.

[Unit]
Description=Neru keyboard-driven mouse replacement daemon
Documentation=https://github.com/y3owk1n/neru
After=graphical-session.target
PartOf=graphical-session.target

[Service]
Type=simple
ExecStart=NERU_BINARY_PATH launch
Restart=on-failure
RestartSec=5

[Install]
WantedBy=graphical-session.target
`

// renderServiceUnit fills the unit template with the absolute path of the
// binary that is installing itself.
func renderServiceUnit(binaryPath string) string {
	return strings.ReplaceAll(
		serviceUnitTemplate,
		"NERU_BINARY_PATH",
		systemdQuoteExecPath(binaryPath),
	)
}

// systemdQuoteExecPath spells a filesystem path the way an Exec line reads one
// back unchanged. Whitespace splits an Exec line, so the path is
// double-quoted, and inside the quotes a backslash or quote doubles up. A
// percent opens a specifier even inside quotes (%h would become the home
// directory), so it becomes %%.
func systemdQuoteExecPath(path string) string {
	escaped := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`%`, `%%`,
	).Replace(path)

	return `"` + escaped + `"`
}

// systemdIsInit reports whether systemd booted this machine, by the presence of
// its runtime marker directory.
func systemdIsInit(markerDir string) bool {
	info, err := os.Stat(markerDir)

	return err == nil && info.IsDir()
}

// errNotSystemd is the refusal every subcommand returns on a machine systemd
// did not boot. It names the one supported init system.
func errNotSystemd(action string) error {
	return derrors.Newf(
		derrors.CodeNotSupported,
		"services %s manages a systemd user unit, and this machine was not booted "+
			"by systemd; other init systems (runit, OpenRC, s6) are not supported — "+
			"start `neru launch` from your session or supervisor instead, see %s",
		action,
		buildinfo.DocsURL(linuxServicesDocs, buildinfo.Version),
	)
}

// requireSystemd is the first line of every subcommand.
func requireSystemd(action string) error {
	if !systemdIsInit(systemdRuntimeMarker) {
		return errNotSystemd(action)
	}

	return nil
}

// serviceUnitPath is the systemd user unit directory under the XDG config home
// the daemon already honors, so the unit sits beside a relocated config.
func serviceUnitPath() (string, error) {
	configHome, err := xdgConfigHome()
	if err != nil {
		return "", err
	}

	return filepath.Join(configHome, "systemd", "user", serviceUnitName), nil
}

// xdgConfigHome resolves $XDG_CONFIG_HOME per the Base Directory specification.
// The variable wins only when it is set to an absolute path (a relative value
// must be ignored), and otherwise the default is ~/.config.
func xdgConfigHome() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return dir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config"), nil
}

// getBinaryPath resolves the running binary to a real path, because a unit file
// outlives whatever PATH or symlink the install was run through.
func getBinaryPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", derrors.Wrap(err, derrors.CodeConfigIOFailed, "failed to locate the neru binary")
	}

	resolved, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", derrors.Wrap(
			err,
			derrors.CodeConfigIOFailed,
			"failed to resolve the neru binary path",
		)
	}

	return resolved, nil
}

// systemctl runs a `systemctl --user` subcommand, returning its combined output
// so a failure can quote systemd's own explanation instead of an exit code.
func systemctl(args ...string) (string, error) {
	// A dead user manager (often a stale DBUS_SESSION_BUS_ADDRESS over SSH)
	// leaves systemctl waiting forever, and `status` must still answer.
	ctx, cancel := context.WithTimeout(context.Background(), systemctlTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "systemctl", append([]string{"--user"}, args...)...)

	output, err := cmd.CombinedOutput()

	return strings.TrimSpace(string(output)), err
}

// runSystemctl is systemctl for the callers that only need the error, with
// systemd's own explanation folded into it. attempt reads as the tail of
// "failed to ...", so it is a verb phrase rather than a single word.
func runSystemctl(attempt string, args ...string) error {
	output, err := systemctl(args...)
	if err == nil {
		return nil
	}

	if output == "" {
		return derrors.Wrapf(err, derrors.CodeExecFailed, "failed to %s", attempt)
	}

	return derrors.Wrapf(err, derrors.CodeExecFailed, "failed to %s: %s", attempt, output)
}

// serviceUnitExists reports whether any neru.service user unit exists, Neru's
// own or one a package manager placed on systemd's search path (`systemctl
// --user cat`). The file check comes first because `cat` also fails when
// there is no user manager to ask.
func serviceUnitExists(unitPath string) bool {
	_, statErr := os.Stat(unitPath)
	if statErr == nil {
		return true
	}

	_, catErr := systemctl("cat", serviceUnitName)

	return catErr == nil
}

func installService() error {
	err := requireSystemd("install")
	if err != nil {
		return err
	}

	unitPath, err := serviceUnitPath()
	if err != nil {
		return err
	}

	if serviceUnitExists(unitPath) {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"a %s user unit already exists (Neru's own lives at %s); "+
				"uninstall it — or the package manager installation that provides it, "+
				"such as nix or home-manager — before installing again",
			serviceUnitName,
			unitPath,
		)
	}

	err = requireScannedUnitDir(filepath.Dir(unitPath))
	if err != nil {
		return err
	}

	binPath, err := getBinaryPath()
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(unitPath), unitDirPerm)
	if err != nil {
		return derrors.Wrap(
			err,
			derrors.CodeConfigIOFailed,
			"failed to create the systemd user unit directory",
		)
	}

	err = os.WriteFile(unitPath, []byte(renderServiceUnit(binPath)), unitFilePerm)
	if err != nil {
		return derrors.Wrap(
			err,
			derrors.CodeConfigIOFailed,
			"failed to write the systemd user unit",
		)
	}

	// A unit file systemd never accepted blocks the next install and starts
	// nothing, so undo the write and tell the manager about the removal.
	err = enableService()
	if err != nil {
		_ = os.Remove(unitPath)
		_, _ = systemctl("daemon-reload")

		return err
	}

	return nil
}

// requireScannedUnitDir refuses an install into a directory the user manager
// will not read. The manager fixed its search path at session start, so a
// $XDG_CONFIG_HOME exported later moves where Neru writes but not where
// systemd looks, and `enable` would report a baffling missing unit. An
// unreachable manager answers nothing, and `enable` reports it.
func requireScannedUnitDir(unitDir string) error {
	searchPath, err := systemctl("show", "--property=UnitPath", "--value")
	if err != nil {
		return nil //nolint:nilerr // no manager to ask; enable is what reports it
	}

	if unitDirIsScanned(unitDir, searchPath) {
		return nil
	}

	return derrors.Newf(
		derrors.CodeInvalidInput,
		"systemd's user manager does not scan %s, so a unit written there would never "+
			"load; its search path was fixed when your session started, and "+
			"$XDG_CONFIG_HOME is %q in this shell. Unset it here, or export it in your "+
			"session and run `systemctl --user import-environment XDG_CONFIG_HOME` "+
			"followed by `systemctl --user daemon-reexec`",
		unitDir,
		os.Getenv("XDG_CONFIG_HOME"),
	)
}

// unitDirIsScanned reports whether dir appears in a `systemctl show -p
// UnitPath` answer, a space-separated list with no escaping. For a directory
// whose name contains a space the answer is inconclusive and reads as yes,
// because refusing a working install is the worse mistake.
func unitDirIsScanned(unitDir, searchPath string) bool {
	cleaned := filepath.Clean(unitDir)
	if slices.Contains(strings.Fields(searchPath), cleaned) {
		return true
	}

	return strings.Contains(cleaned, " ") && strings.Contains(searchPath, cleaned)
}

// requireOwnUnit stops `uninstall` from disabling or deleting a neru.service
// somebody else owns, such as a nix or home-manager store symlink at Neru's
// path, a distribution unit elsewhere on the search path, or a file written
// by hand from the old setup guide. Ownership is read from the marker line.
// knownToSystemd tells "nothing installed" (a no-op) from "installed
// elsewhere" (refuse).
func requireOwnUnit(unitPath string, knownToSystemd bool) error {
	info, err := os.Lstat(unitPath)
	if err != nil {
		if knownToSystemd {
			return derrors.Newf(
				derrors.CodeInvalidInput,
				"a %s user unit exists, but not at %s, so Neru did not install it; "+
					"remove it through whatever put it on systemd's search path "+
					"(a distribution package, nix, or home-manager)",
				serviceUnitName,
				unitPath,
			)
		}

		return nil
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"%s is a symlink, so Neru did not write it — a package manager such as "+
				"nix or home-manager did; disable the service there instead",
			unitPath,
		)
	}

	contents, err := os.ReadFile(unitPath)
	if err != nil {
		return derrors.Wrapf(
			err,
			derrors.CodeConfigIOFailed,
			"failed to read the systemd user unit at %s to check whether Neru wrote it",
			unitPath,
		)
	}

	if !strings.Contains(string(contents), serviceUnitMarker) {
		return derrors.Newf(
			derrors.CodeInvalidInput,
			"%s carries no %q line, so Neru did not write it — a hand-written unit or "+
				"a configuration tool did; remove it there, and `neru services install` "+
				"will write Neru's own in its place",
			unitPath,
			serviceUnitMarker,
		)
	}

	return nil
}

// enableService reloads so systemd sees the new file, then enables --now so
// it starts in this session and every later one.
func enableService() error {
	err := runSystemctl("reload the systemd user manager", "daemon-reload")
	if err != nil {
		return err
	}

	return runSystemctl("enable the service", "enable", "--now", serviceUnitName)
}

func uninstallService() error {
	err := requireSystemd("uninstall")
	if err != nil {
		return err
	}

	unitPath, err := serviceUnitPath()
	if err != nil {
		return err
	}

	installed := serviceUnitExists(unitPath)

	err = requireOwnUnit(unitPath, installed)
	if err != nil {
		return err
	}

	// Nothing to uninstall is not a failure, and asking systemd about a missing
	// unit would turn the no-op into an error.
	if !installed {
		return nil
	}

	// Failures past here are real, since `disable --now` is idempotent. Hiding
	// one would leave the unit enabled while the CLI printed success.
	err = runSystemctl(
		"stop and disable the service (the unit file at "+unitPath+" was left in place)",
		"disable", "--now", serviceUnitName,
	)
	if err != nil {
		return err
	}

	err = os.Remove(unitPath)
	if err != nil && !os.IsNotExist(err) {
		return derrors.Wrap(
			err,
			derrors.CodeConfigIOFailed,
			"failed to remove the systemd user unit",
		)
	}

	return runSystemctl(
		"reload the systemd user manager after removing "+unitPath+
			" (run `systemctl --user daemon-reload` once it is reachable)",
		"daemon-reload",
	)
}

// requireOwnServiceUnit is the first line of start, stop and restart. Like
// uninstall, they leave a neru.service alone that Neru did not write, since
// disabling a unit nix or home-manager links in removes the link itself.
func requireOwnServiceUnit(action string) error {
	err := requireSystemd(action)
	if err != nil {
		return err
	}

	unitPath, err := serviceUnitPath()
	if err != nil {
		return err
	}

	err = requireOwnUnit(unitPath, serviceUnitExists(unitPath))
	if err != nil {
		return err
	}

	return requireManagerResolves(unitPath)
}

// requireManagerResolves refuses when the user manager resolves neru.service to
// a different file than the one just checked, as when a later
// $XDG_CONFIG_HOME points this shell at an old unit while the manager runs a
// nix one. A manager that cannot answer is left to the systemctl call that
// follows.
func requireManagerResolves(unitPath string) error {
	fragment, err := systemctl("show", "--property=FragmentPath", "--value", serviceUnitName)
	if err != nil || fragment == "" || filepath.Clean(fragment) == filepath.Clean(unitPath) {
		return nil //nolint:nilerr // the systemctl call that follows reports it
	}

	return derrors.Newf(
		derrors.CodeInvalidInput,
		"systemd's user manager loads %s from %s, not from %s where Neru checked it; "+
			"manage that unit with the tool that installed it, or unset $XDG_CONFIG_HOME "+
			"in this shell if it differs from your session's",
		serviceUnitName,
		fragment,
		unitPath,
	)
}

// startService undoes stopService. It enables the unit again, so it starts at
// login too, and starts it now.
func startService() error {
	err := requireOwnServiceUnit("start")
	if err != nil {
		return err
	}

	return runSystemctl("enable and start the service", "enable", "--now", serviceUnitName)
}

// stopService stops and disables the unit, so it stays stopped across logins
// until startService. The unit file stays.
func stopService() error {
	err := requireOwnServiceUnit("stop")
	if err != nil {
		return err
	}

	return runSystemctl("stop and disable the service", "disable", "--now", serviceUnitName)
}

// restartService refuses a unit that stopService disabled, since restarting
// it would run the daemon while it stays off at login.
func restartService() error {
	err := requireOwnServiceUnit("restart")
	if err != nil {
		return err
	}

	if systemctlWord("is-enabled") == unitDisabled {
		return derrors.New(
			derrors.CodeInvalidInput,
			"the service is stopped; run `neru services start` to start it",
		)
	}

	return runSystemctl("restart the service", "restart", serviceUnitName)
}

// serviceUnitState is everything `neru services status` reports, kept apart
// from the wording so the wording can be tested without systemd.
type serviceUnitState struct {
	// systemdBooted is false on a machine running another init.
	systemdBooted bool
	// installed is whether any neru.service user unit exists, ours or not.
	installed bool
	// unitPath is where Neru writes its unit, whether or not it is there yet.
	unitPath string
	// active and enabled are systemctl's own words, passed through so they
	// match `systemctl --user status`.
	active  string
	enabled string
}

// describeServiceStatus puts a gathered state into one line. No unit and no
// systemd are ordinary answers, not errors. `status` has no error to carry a
// code, so its non-systemd answer carries the docs link the other five
// subcommands put in their CodeNotSupported.
func describeServiceStatus(state serviceUnitState, docsURL string) string {
	if !state.systemdBooted {
		return "Service management requires systemd; this machine was not " +
			"booted by systemd, so there is no user unit to report on — see " +
			docsURL
	}

	if !state.installed {
		return "Service not installed: no unit at " + state.unitPath +
			" — run `neru services install` to create it"
	}

	return "Service installed: " + state.active + ", " + state.enabled + " at login"
}

func statusService() serviceStatus {
	docsURL := buildinfo.DocsURL(linuxServicesDocs, buildinfo.Version)

	unitPath, err := serviceUnitPath()
	if err != nil {
		return serviceStatus{
			Message: "Service status unavailable: " + err.Error(),
			Error:   err.Error(),
		}
	}

	state := serviceUnitState{
		systemdBooted: systemdIsInit(systemdRuntimeMarker),
		unitPath:      unitPath,
	}

	if !state.systemdBooted {
		return reportServiceStatus(state, docsURL)
	}

	state.installed = serviceUnitExists(unitPath)
	if !state.installed {
		return reportServiceStatus(state, docsURL)
	}

	state.active = systemctlWord("is-active")
	state.enabled = systemctlWord("is-enabled")

	return reportServiceStatus(state, docsURL)
}

// reportServiceStatus is a gathered state as `neru services status` reports it.
func reportServiceStatus(state serviceUnitState, docsURL string) serviceStatus {
	status := serviceStatus{
		Supported:     state.systemdBooted,
		Installed:     state.installed,
		Running:       state.active == "active",
		StartsAtLogin: state.enabled == unitEnabled,
		Message:       describeServiceStatus(state, docsURL),
	}

	if state.installed {
		for _, word := range []string{state.active, state.enabled} {
			if !systemctlAnswered(word) {
				status.Error = "systemctl could not read the unit: " + word

				break
			}
		}
	}

	return status
}

// systemctlNoAnswer is what systemctlWord returns when systemctl printed
// nothing, as when it timed out.
const systemctlNoAnswer = "unknown"

// systemctlWord reads a one-word query such as is-active or is-enabled.
// Both exit non-zero for a perfectly ordinary answer ("inactive", "disabled"),
// so the word on stdout is the result and the exit status is not.
func systemctlWord(query string) string {
	output, _ := systemctl(query, serviceUnitName)
	if output == "" {
		return systemctlNoAnswer
	}

	return output
}

// systemctlAnswered reports whether word is an answer from systemd. systemd
// answers in single words. A failed read prints a sentence, such as "Failed
// to connect to bus", or nothing.
func systemctlAnswered(word string) bool {
	return word != systemctlNoAnswer && !strings.ContainsAny(word, " \t\n")
}
