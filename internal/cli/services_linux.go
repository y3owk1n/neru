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

// Service management on Linux is a systemd user unit, and only that. Neru is a
// per-session daemon that needs a display server, so it belongs to the user
// manager rather than the system one, and the graphical session is what it is
// ordered against. Other init systems refuse loudly and name systemd
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md).
const (
	// serviceUnitName is the unit systemd knows Neru by. It is also what a user
	// types into every `systemctl --user` command they run by hand, so it stays
	// the plain program name rather than a reverse-DNS label.
	serviceUnitName = "neru.service"

	// systemdRuntimeMarker is the directory systemd creates when it is PID 1.
	// It is what sd_booted(3) checks, and the only reliable answer to "is this
	// a systemd machine" — `systemctl` sitting on PATH says nothing, since the
	// binary ships in packages installed on machines running another init.
	systemdRuntimeMarker = "/run/systemd/system"

	// linuxServicesDocs is the page a user is sent to when service management
	// cannot help them.
	linuxServicesDocs = "docs/LINUX_SETUP.md"

	unitDirPerm  = 0o755
	unitFilePerm = 0o644
)

// systemctlTimeout bounds every call to the user manager. `enable --now` starts
// a Type=simple service, so nothing here legitimately waits on the daemon
// coming up; the budget is for `daemon-reload` on a busy machine.
const systemctlTimeout = 15 * time.Second

// serviceUnitTemplate is the systemd user unit Neru installs.
//
// The anchor is graphical-session.target rather than default.target: every
// Linux backend needs a display server, so starting before one exists would
// only produce a crash loop. After= orders it behind the session, WantedBy=
// starts it with each new session, and PartOf= stops it when that session ends
// — together they are what makes the unit survive a logout/login cycle rather
// than linger as an orphan attached to a display that is gone.
const serviceUnitTemplate = `[Unit]
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
// back unchanged.
//
// A Linux path may hold any byte but / and NUL, and two of them are load
// bearing to systemd. Whitespace splits an Exec line into words, so
// /opt/my apps/neru would be read as the command /opt/my with the argument
// apps/neru; double quotes are what hold it together, and inside them a
// backslash opens a C-style escape, so a literal backslash or quote has to
// double up. A percent opens a specifier, which is resolved before the line is
// split and regardless of quoting — %h would silently become the home
// directory, %y would fail the unit load — and %% is the literal one.
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
// did not boot. Neru manages a systemd user unit and nothing else, so the
// message names the one init system that is supported instead of implying the
// operation might work after some setup.
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

// serviceUnitPath is where the unit file is written: the systemd user unit
// directory under the XDG config home the daemon already honors, so a user who
// relocated $XDG_CONFIG_HOME finds the unit beside their config rather than in
// a second place nothing else in Neru uses.
func serviceUnitPath() (string, error) {
	configHome, err := xdgConfigHome()
	if err != nil {
		return "", err
	}

	return filepath.Join(configHome, "systemd", "user", serviceUnitName), nil
}

// xdgConfigHome resolves $XDG_CONFIG_HOME per the Base Directory specification:
// the variable wins only when it is set to an absolute path — a relative value
// must be ignored — and otherwise the default is ~/.config.
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
	// A user manager that has gone away — a stale DBUS_SESSION_BUS_ADDRESS
	// inherited over SSH is the usual way — leaves systemctl waiting on a bus
	// that will never answer. `status` is the subcommand a person runs when
	// things are already broken, so it has to come back with something.
	ctx, cancel := context.WithTimeout(context.Background(), systemctlTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "systemctl", append([]string{"--user"}, args...)...)

	output, err := cmd.CombinedOutput()

	return strings.TrimSpace(string(output)), err
}

// runSystemctl is systemctl for the callers that only need the error, with
// systemd's own explanation folded into it. attempt reads as the tail of
// "failed to …", so it is a verb phrase rather than a single word.
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

// serviceUnitExists reports whether a neru.service user unit exists at all: the
// one Neru writes, or one a package manager (nix, home-manager, a distribution
// package) placed on systemd's search path. `systemctl --user cat` is what
// answers the second, and it is also what fails when there is no user manager
// to ask — which is why the file check comes first.
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

	// A unit file systemd never accepted is worse than no unit file: it blocks
	// the next install and starts nothing, so the write is undone — and the
	// manager told about the removal, or it goes on holding a unit whose file
	// is gone.
	err = enableService()
	if err != nil {
		_ = os.Remove(unitPath)
		_, _ = systemctl("daemon-reload")

		return err
	}

	return nil
}

// requireScannedUnitDir refuses an install into a directory the user manager
// will not read.
//
// The manager fixed its unit search path from its own environment when the
// session began. A $XDG_CONFIG_HOME exported in a shell rc afterwards moves
// where Neru writes without moving where systemd looks, and the symptom —
// `enable` reporting that a unit just written does not exist — names neither
// half of the mismatch. Asking the manager for its search path turns that into
// one sentence. A manager that cannot be reached at all answers nothing, and
// the install proceeds so that `enable` is what reports it.
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

// unitDirIsScanned reports whether dir appears in a `systemctl show -p UnitPath`
// answer, which is the search path spelled as space-separated directories.
//
// That spelling has no escaping, so a directory whose own name contains a space
// cannot be told from two directories. There the substring hit is inconclusive
// rather than a match — and an inconclusive answer says yes, because refusing an
// install that would have worked is the worse of the two mistakes.
func unitDirIsScanned(unitDir, searchPath string) bool {
	cleaned := filepath.Clean(unitDir)
	if slices.Contains(strings.Fields(searchPath), cleaned) {
		return true
	}

	return strings.Contains(cleaned, " ") && strings.Contains(searchPath, cleaned)
}

// requireOwnUnit is the mirror of install's refusal, on the way back out: it
// stops `uninstall` from disabling or deleting a neru.service somebody else
// owns.
//
// Neru's own installs are a plain file it wrote. Nix and home-manager put a
// symlink into the store at the same path, and a distribution package puts a
// real unit somewhere else on the search path entirely — deleting the first and
// disabling the second both look like success while breaking a setup Neru does
// not manage. knownToSystemd says whether the manager can resolve a
// neru.service at all, which is the only way to tell "nothing installed" (fine,
// uninstall is a no-op) from "installed elsewhere" (refuse).
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

	return nil
}

// enableService is the half of install that talks to systemd: reload so the new
// file is seen, then enable --now so it starts in this session and in every
// session after it.
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

	err = requireOwnUnit(unitPath, serviceUnitExists(unitPath))
	if err != nil {
		return err
	}

	// Best effort: a unit that was never enabled, or a user manager that has
	// already forgotten it, must not stop the file from being removed.
	_, _ = systemctl("disable", "--now", serviceUnitName)

	err = os.Remove(unitPath)
	if err != nil && !os.IsNotExist(err) {
		return derrors.Wrap(
			err,
			derrors.CodeConfigIOFailed,
			"failed to remove the systemd user unit",
		)
	}

	_, _ = systemctl("daemon-reload")

	return nil
}

// driveService is start, stop and restart, which are the three subcommands
// whose name is also the systemctl verb — so the word is written once rather
// than three times per subcommand.
func driveService(verb string) error {
	err := requireSystemd(verb)
	if err != nil {
		return err
	}

	return runSystemctl(verb+" the service", verb, serviceUnitName)
}

func startService() error {
	return driveService("start")
}

func stopService() error {
	return driveService("stop")
}

func restartService() error {
	return driveService("restart")
}

// serviceUnitState is everything `neru services status` reports, gathered
// before any of it is put into words. Keeping the two apart is what lets the
// wording be tested on a machine with no systemd to ask.
type serviceUnitState struct {
	// systemdBooted is false on a machine running another init, where nothing
	// below can be true.
	systemdBooted bool
	// installed is whether a neru.service user unit exists at all — ours, or
	// one a package manager placed on systemd's search path.
	installed bool
	// unitPath is where Neru writes its unit, whether or not it is there yet.
	unitPath string
	// active and enabled are systemctl's own words ("active", "inactive",
	// "failed"; "enabled", "disabled", "static"), passed through rather than
	// translated so they match what `systemctl --user status` shows.
	active  string
	enabled string
}

// describeServiceStatus puts a gathered state into one line.
//
// A machine with no unit installed is not a failure, and neither is one systemd
// did not boot: both are ordinary answers to "what is the service doing", so
// they read as statements of fact with the next step attached rather than as
// errors. `status` is the one subcommand with no error to carry a code, so the
// non-systemd answer carries the documentation link the other five put in their
// CodeNotSupported instead.
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

func statusService() string {
	docsURL := buildinfo.DocsURL(linuxServicesDocs, buildinfo.Version)

	unitPath, err := serviceUnitPath()
	if err != nil {
		return "Service status unavailable: " + err.Error()
	}

	state := serviceUnitState{
		systemdBooted: systemdIsInit(systemdRuntimeMarker),
		unitPath:      unitPath,
	}

	if !state.systemdBooted {
		return describeServiceStatus(state, docsURL)
	}

	state.installed = serviceUnitExists(unitPath)
	if !state.installed {
		return describeServiceStatus(state, docsURL)
	}

	state.active = systemctlWord("is-active")
	state.enabled = systemctlWord("is-enabled")

	return describeServiceStatus(state, docsURL)
}

// systemctlWord reads a one-word query such as is-active or is-enabled.
// Both exit non-zero for a perfectly ordinary answer ("inactive", "disabled"),
// so the word on stdout is the result and the exit status is not.
func systemctlWord(query string) string {
	output, _ := systemctl(query, serviceUnitName)
	if output == "" {
		return "unknown"
	}

	return output
}
