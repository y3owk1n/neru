//go:build darwin

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/derrors"
)

const (
	serviceLabel    = "com.y3owk1n.neru"
	launchAgentsDir = "~/Library/LaunchAgents"
	plistFile       = launchAgentsDir + "/" + serviceLabel + ".plist"
)

// daemonStderrFileName is the file the login agent's standard error is
// redirected to, inside the per-user log directory the logger already owns.
//
// Standard output is deliberately not redirected: the logger's console core
// writes every log line there, so a redirect would duplicate app.log into a
// second, unrotated file. Standard error carries what app.log cannot — a panic,
// a native crash, or a startup failure raised before the file sink exists — so
// it is the half worth keeping.
const daemonStderrFileName = "daemon.err.log"

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.y3owk1n.neru</string>
    <key>ProgramArguments</key>
    <array>
        <string>NERU_BINARY_PATH</string>
        <string>launch</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardErrorPath</key>
    <string>NERU_STDERR_PATH</string>
    <key>ProcessType</key>
    <string>Interactive</string>
    <key>LimitLoadToSessionType</key>
    <string>Aqua</string>
    <key>Nice</key>
    <integer>-10</integer>
    <key>ThrottleInterval</key>
    <integer>10</integer>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string>
    </dict>
</dict>
</plist>`

var (
	errServiceAlreadyLoaded = errors.New(
		"service is already loaded; check for existing installations (e.g., nix-darwin, home-manager) and uninstall them first",
	)
	errPlistAlreadyExists  = errors.New("plist file already exists")
	errServiceNotInstalled = derrors.New(
		derrors.CodeInvalidInput,
		"no service is installed; run `neru services install` first",
	)
	errServiceStopped = derrors.New(
		derrors.CodeInvalidInput,
		"the service is stopped; run `neru services start` to start it",
	)
)

// serviceDomain is the per-user launchd domain the agent loads into.
// serviceTarget names the agent inside it, in the form enable, disable,
// bootout and kickstart take.
func serviceDomain() string {
	return "gui/" + strconv.Itoa(os.Getuid())
}

func serviceTarget() string {
	return serviceDomain() + "/" + serviceLabel
}

// launchctl runs a launchctl subcommand, folding launchctl's own explanation
// into the error rather than leaving only an exit status.
func launchctl(args ...string) error {
	output, err := exec.CommandContext(context.Background(), "launchctl", args...).
		CombinedOutput()
	if err != nil {
		return derrors.Wrapf(
			err,
			derrors.CodeExecFailed,
			"launchctl %s: %s",
			args[0],
			strings.TrimSpace(string(output)),
		)
	}

	return nil
}

// plistPath is where the agent's plist lives, expanded.
func plistPath() (string, error) {
	path, err := expandPath(plistFile)
	if err != nil {
		return "", fmt.Errorf("failed to expand plist path: %w", err)
	}

	return path, nil
}

// plistInstalled reports whether anything is at the plist path. It tells a
// stopped service, installed but not loaded, apart from one never installed.
func plistInstalled(path string) bool {
	_, err := os.Lstat(path)

	return err == nil
}

// daemonStderrPath returns the absolute file the login agent's stderr is
// redirected to. The plist is read by launchd, which expands nothing, so this
// resolves the home directory rather than writing a "~" into it.
func daemonStderrPath() (string, error) {
	logDir, err := logger.DefaultLogDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(logDir, daemonStderrFileName), nil
}

// plistTextEscaper escapes what cannot appear literally inside a plist
// <string> element. A filesystem path is arbitrary text as far as XML is
// concerned — a directory called "A&B" is perfectly legal on macOS — and an
// unescaped one produces a plist launchctl refuses to load, leaving a file
// behind that the next install then refuses to write over.
var plistTextEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// renderPlist fills the launchd agent template in with the two absolute paths
// launchd cannot work out for itself: the binary to run, and the file its
// standard error is appended to.
func renderPlist(binPath, stderrPath string) string {
	return strings.NewReplacer(
		"NERU_BINARY_PATH", plistTextEscaper.Replace(binPath),
		"NERU_STDERR_PATH", plistTextEscaper.Replace(stderrPath),
	).Replace(plistTemplate)
}

func getBinaryPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(execPath)
}

func isServiceLoaded() bool {
	cmd := exec.CommandContext(context.Background(), "launchctl", "list", serviceLabel)

	return cmd.Run() == nil
}

func installService() error {
	if isServiceLoaded() {
		return errServiceAlreadyLoaded
	}

	binPath, err := getBinaryPath()
	if err != nil {
		return fmt.Errorf("failed to get binary path: %w", err)
	}

	// launchd opens the redirect itself, before neru runs, and it creates the
	// file but never the directory holding it — so the directory has to exist
	// by the time the agent is bootstrapped or the first launch's stderr, which
	// is the launch most likely to fail, goes nowhere.
	stderrPath, err := daemonStderrPath()
	if err != nil {
		return fmt.Errorf("failed to resolve the log directory: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(stderrPath), logger.DefaultDirPerms)
	if err != nil {
		return fmt.Errorf("failed to create the log directory: %w", err)
	}

	plistContent := renderPlist(binPath, stderrPath)

	expandedDir, err := expandPath(launchAgentsDir)
	if err != nil {
		return fmt.Errorf("failed to expand LaunchAgents path: %w", err)
	}

	expandedPlist := filepath.Join(expandedDir, serviceLabel+".plist")

	_, statErr := os.Stat(expandedPlist)
	if statErr == nil {
		return fmt.Errorf(
			"%w at %s; remove it manually or uninstall first",
			errPlistAlreadyExists,
			expandedPlist,
		)
	}

	const dirPerm = 0o755

	err = os.MkdirAll(expandedDir, dirPerm)
	if err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	const filePerm = 0o644

	err = os.WriteFile(expandedPlist, []byte(plistContent), filePerm)
	if err != nil {
		return fmt.Errorf("failed to write plist: %w", err)
	}

	// launchd refuses to load an agent that `neru services stop` disabled.
	err = launchctl("enable", serviceTarget())
	if err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	err = launchctl("bootstrap", serviceDomain(), expandedPlist)
	if err != nil {
		return fmt.Errorf("failed to load service: %w", err)
	}

	return nil
}

func uninstallService() error {
	expandedPlist, err := plistPath()
	if err != nil {
		return err
	}

	_ = launchctl("bootout", serviceTarget()) // Ignore error if not loaded

	// Clear the disable that a stop leaves behind, or a later plist under this
	// label fails to load.
	_ = launchctl("enable", serviceTarget())

	err = os.Remove(expandedPlist)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	return nil
}

// startService undoes stopService. It enables the agent again, so launchd
// loads it at login too, and starts neru now. A stopped agent is not loaded,
// so startService loads the plist and RunAtLoad starts neru. A loaded agent
// gets a kickstart instead.
func startService() error {
	path, err := plistPath()
	if err != nil {
		return err
	}

	loaded := isServiceLoaded()
	if !loaded && !plistInstalled(path) {
		return errServiceNotInstalled
	}

	err = launchctl("enable", serviceTarget())
	if err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	if loaded {
		err = launchctl("kickstart", serviceTarget())
		if err != nil {
			return fmt.Errorf("failed to start service: %w", err)
		}

		return nil
	}

	err = launchctl("bootstrap", serviceDomain(), path)
	if err != nil {
		return fmt.Errorf("failed to load service: %w", err)
	}

	return nil
}

// stopService stops the agent until startService, across logins too.
//
// The plist sets KeepAlive, so after a plain launchctl stop launchd starts
// neru again within seconds. Disabling the agent keeps launchd from loading it
// at the next login, and unloading it stops neru now. The plist stays, so
// startService can load it again.
func stopService() error {
	path, err := plistPath()
	if err != nil {
		return err
	}

	loaded := isServiceLoaded()
	if !loaded && !plistInstalled(path) {
		return errServiceNotInstalled
	}

	err = launchctl("disable", serviceTarget())
	if err != nil {
		return fmt.Errorf("failed to disable service: %w", err)
	}

	if !loaded {
		return nil
	}

	err = launchctl("bootout", serviceTarget())
	if err != nil {
		return fmt.Errorf("failed to unload service: %w", err)
	}

	return nil
}

// restartService restarts the loaded agent with one kickstart -k, which kills
// whatever runs under it and spawns it again. A stop followed by a start races
// launchd's own KeepAlive relaunch instead.
func restartService() error {
	if !isServiceLoaded() {
		path, err := plistPath()
		if err == nil && plistInstalled(path) {
			return errServiceStopped
		}

		return errServiceNotInstalled
	}

	err := launchctl("kickstart", "-k", serviceTarget())
	if err != nil {
		return fmt.Errorf("failed to restart service: %w", err)
	}

	return nil
}

func statusService() string {
	if isServiceLoaded() {
		return "Service loaded"
	}

	path, err := plistPath()
	if err == nil && plistInstalled(path) {
		return "Service stopped (run `neru services start` to start it)"
	}

	return "Service not loaded"
}

func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		return filepath.Join(home, path[1:]), nil
	}

	return path, nil
}
