//go:build linux

package linux

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/adapter/platform/gnomeshell"
	"github.com/y3owk1n/neru/internal/derrors"
)

// waylandFocusedApplicationPID resolves the focused application's PID on a
// Wayland session with wlr-foreign-toplevel-management. The protocol exposes
// the app_id but not the PID, so the app_id is matched against /proc, and a
// miss is CodeNotSupported rather than an invented number.
func waylandFocusedApplicationPID() (int, error) {
	appID, ok := WaylandFocusedAppID()

	return focusedApplicationPIDFromAppID(appID, ok)
}

// gnomeFocusedApplicationPID is the GNOME arm, matching what the extension
// reports. An extension that is not running is not an unfocused desktop.
func gnomeFocusedApplicationPID() (int, error) {
	bridge := gnomeshell.Shared(nil)
	bridge.EnsureStarted()

	window, found, err := bridge.Focused()
	if err != nil {
		return 0, derrors.Wrap(
			err,
			derrors.CodeNotSupported,
			"FocusedApplicationPID: no GNOME focused-app source",
		)
	}

	return focusedApplicationPIDFromAppID(window.AppID, found)
}

func focusedApplicationPIDFromAppID(appID string, ok bool) (int, error) {
	if !ok || appID == "" {
		return 0, waylandNoFocusedAppError()
	}

	pid, found := resolvePIDByAppID(appID, "/proc")
	if !found {
		// Not errNoFocusedWindow, because a window is focused and only the
		// app_id match missed.
		return 0, derrors.Newf(
			derrors.CodeNotSupported,
			"FocusedApplicationPID: Wayland exposes no PID for the focused app (app_id=%q); no /proc match found",
			appID,
		)
	}

	return pid, nil
}

// waylandNoFocusedAppError explains an empty answer from the foreign-toplevel
// manager, the wlroots unfocused desktop. It wraps errNoFocusedWindow like the
// X11 arm (#1495). The CGO-off build does not, since its manager is absent.
//
// It cannot yet separate a manager that never bound, which WaylandFocusedAppID
// reports with the same bare false. That needs the wlroots client, in both its
// cgo and nocgo halves, to say which happened.
func waylandNoFocusedAppError() error {
	if !nativeBackendsCompiledIn {
		return derrors.New(
			derrors.CodeNotSupported,
			"FocusedApplicationPID: this binary was built without CGO, so the "+
				"foreign-toplevel client that reports the focused app_id is absent",
		)
	}

	return derrors.Wrap(
		errNoFocusedWindow,
		derrors.CodeNotSupported,
		"FocusedApplicationPID: no focused app_id available on this Wayland compositor",
	)
}

// resolvePIDByAppID scans procRoot (normally "/proc") for a process matching
// appID and returns its PID. It is a heuristic, so a miss is expected.
func resolvePIDByAppID(appID, procRoot string) (int, bool) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return 0, false
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue // not a process directory
		}

		procDir := filepath.Join(procRoot, entry.Name())
		comm := readProcComm(procDir)
		exeBase := readProcExeBase(procDir)
		cmdlineBase := readProcCmdlineBase(procDir)

		if appIDMatchesProcess(appID, comm, exeBase, cmdlineBase) {
			return pid, true
		}
	}

	return 0, false
}

// readProcComm returns the trimmed contents of <procDir>/comm, or "". The
// kernel truncates comm to 15 characters, so it is only a cheap first signal.
func readProcComm(procDir string) string {
	data, err := os.ReadFile(filepath.Join(procDir, "comm"))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

// readProcExeBase returns the basename of the <procDir>/exe symlink target, or
// "" when it cannot be read (kernel threads, permission denied, race on exit).
func readProcExeBase(procDir string) string {
	target, err := os.Readlink(filepath.Join(procDir, "exe"))
	if err != nil {
		return ""
	}

	// A deleted executable shows up as "/path/to/bin (deleted)".
	target = strings.TrimSuffix(target, " (deleted)")

	return filepath.Base(target)
}

// readProcCmdlineBase returns the basename of argv[0] from <procDir>/cmdline,
// or "". cmdline args are NUL-separated.
func readProcCmdlineBase(procDir string) string {
	data, err := os.ReadFile(filepath.Join(procDir, "cmdline"))
	if err != nil {
		return ""
	}

	argv0, _, _ := strings.Cut(string(data), "\x00")

	argv0 = strings.TrimSpace(argv0)
	if argv0 == "" {
		return ""
	}

	return filepath.Base(argv0)
}

// appIDMatchesProcess reports whether a Wayland app_id plausibly identifies a
// process with the given comm, exe basename, and argv[0] basename. It compares
// the full app_id and its last dotted segment ("org.kde.konsole" to "konsole"),
// case-insensitively. Matches are exact, never substring, so "code" does not
// match "codeium".
func appIDMatchesProcess(appID, comm, exeBase, cmdlineBase string) bool {
	full := strings.ToLower(strings.TrimSpace(appID))
	if full == "" {
		return false
	}

	wants := []string{full}
	if segment := lastDottedSegment(full); segment != "" && segment != full {
		wants = append(wants, segment)
	}

	candidates := []string{
		strings.ToLower(strings.TrimSpace(comm)),
		strings.ToLower(strings.TrimSpace(exeBase)),
		strings.ToLower(strings.TrimSpace(cmdlineBase)),
	}

	for _, want := range wants {
		for _, cand := range candidates {
			if cand != "" && cand == want {
				return true
			}
		}
	}

	return false
}

// lastDottedSegment returns the substring after the final '.' in appID, or ""
// when appID has no interior dot or ends with one.
func lastDottedSegment(appID string) string {
	idx := strings.LastIndex(appID, ".")
	if idx < 0 || idx == len(appID)-1 {
		return ""
	}

	return appID[idx+1:]
}
