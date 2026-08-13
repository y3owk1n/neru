//go:build linux

package linux

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/y3owk1n/neru/internal/derrors"
)

// waylandFocusedApplicationPID resolves the focused application's PID on a
// Wayland session that speaks wlr-foreign-toplevel-management (wlroots + KDE).
//
// The protocol exposes the focused window's app_id but deliberately not its
// PID — Wayland clients cannot read each other's process credentials. We
// therefore best-effort match the app_id against running processes in /proc.
// When no match is found the caller gets CodeNotSupported with the app_id in
// the message, keeping the "PID is not natively available on Wayland" contract
// honest rather than inventing a number.
func waylandFocusedApplicationPID() (int, error) {
	appID, ok := WaylandFocusedAppID()
	if !ok || appID == "" {
		return 0, waylandNoFocusedAppError()
	}

	pid, found := resolvePIDByAppID(appID, "/proc")
	if !found {
		// Not the unfocused-desktop sentinel: a window *is* focused, and the
		// heuristic that maps its app_id onto a process missed. Wearing the
		// sentinel here would tell the user to focus a window they already have
		// focused.
		return 0, derrors.Newf(
			derrors.CodeNotSupported,
			"FocusedApplicationPID: Wayland exposes no PID for the focused app (app_id=%q); no /proc match found",
			appID,
		)
	}

	return pid, nil
}

// waylandNoFocusedAppError explains an empty answer from the foreign-toplevel
// manager, which is the wlroots family's version of an unfocused desktop.
//
// It wraps errNoFocusedWindow so the capability surface explains it the way the
// X11 arm's is explained (#1495): the query works, and it answers as soon as a
// window takes focus. The CGO-off build takes the other branch, because there
// the manager is absent rather than idle, and the reassuring sentence would be a
// lie on a binary that can never answer.
//
// What it cannot yet separate is a compositor whose foreign-toplevel manager
// never bound: WaylandFocusedAppID reports that with the same bare false as an
// idle session, so on a CGO build both arrive here. Telling them apart needs the
// wlroots client to say which happened, in both its cgo and nocgo halves.
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

// resolvePIDByAppID scans procRoot (normally "/proc") for a process whose
// identity matches appID and returns its PID. It is a heuristic: Wayland
// app_ids and process names do not always agree, so a miss is expected and
// handled gracefully by the caller. procRoot is a parameter so tests can point
// it at a synthetic /proc tree.
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

// readProcComm returns the trimmed contents of <procDir>/comm, or "".
// Note: the kernel truncates comm to 15 characters, so exe/cmdline are more
// reliable for long names; comm is still a cheap first signal.
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
// process with the given comm, exe basename, and argv[0] basename.
//
// Two forms are compared, both case-insensitively:
//   - the full app_id (covers plain ids like "firefox" or "Alacritty")
//   - the last dotted segment (covers reverse-DNS ids like "org.kde.konsole"
//     → "konsole" and "org.mozilla.firefox" → "firefox")
//
// It is intentionally conservative: an exact equality against one of the
// process name candidates, never a substring match, to avoid false positives
// (e.g. app_id "code" matching an unrelated "codeium" helper).
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
