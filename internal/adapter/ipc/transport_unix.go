//go:build !windows

package ipc

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/y3owk1n/neru/internal/derrors"
)

// Unix-domain socket transport for IPC on darwin and linux.
// Does not implement the Windows named-pipe transport.
//
// The endpoint lives inside a directory only its owner can enter, rather than
// directly in the shared temporary directory. Both halves matter: the socket
// itself is 0600, and the directory around it is 0700 and owned by the daemon's
// user, so the socket's own mode is never the only thing between another local
// account and the daemon. It also means the name cannot be taken first by
// somebody else — a squatted endpoint would otherwise stop the daemon binding
// at all, and `/tmp` is world-writable wherever `$TMPDIR` is unset.
const (
	// endpointDirPerms is the mode of the directory holding the endpoint:
	// owner-only, no traversal for anyone else.
	endpointDirPerms fs.FileMode = 0o700

	// endpointDirName is the endpoint directory inside a runtime directory that
	// is already private to one user.
	endpointDirName = "neru"

	// endpointDirPrefix names the endpoint directory inside a shared temporary
	// directory, where the owner has to be part of the name.
	endpointDirPrefix = "neru-"

	// runtimeDirRoot is where a Linux login session's runtime directory lives.
	// It is probed only when $XDG_RUNTIME_DIR is absent; see endpointDirs.
	runtimeDirRoot = "/run/user"

	// endpointDirCandidates is how many directories endpointDirs can name: the
	// session's runtime directory, and the temporary directory behind it.
	endpointDirCandidates = 2
)

// endpointDirs returns the directories the endpoint may live in, most
// preferred first. The daemon binds in the first; the CLI searches the list.
//
// $XDG_RUNTIME_DIR is the right home where a session provides one: private to
// one user, and cleaned up at logout. It is an environment variable, though,
// and a daemon started by systemd does not necessarily share an environment
// with a CLI run from cron or from a bare `ssh host neru status`. Since the
// variable is /run/user/<uid> wherever it is set at all, probing that path
// directly keeps the two halves pointing at the same endpoint anyway.
func endpointDirs() []string {
	uid := os.Getuid()
	dirs := make([]string, 0, endpointDirCandidates)

	sessionDir := sessionRuntimeDir(uid)
	if sessionDir != "" {
		dirs = append(dirs, filepath.Join(sessionDir, endpointDirName))
	}

	// os.TempDir() is per-user and 0700 on macOS and shared on Linux, so the
	// owner goes in the name and prepareEndpointDir insists on the mode.
	return append(dirs, filepath.Join(os.TempDir(), endpointDirPrefix+strconv.Itoa(uid)))
}

// sessionRuntimeDir returns this session's runtime directory, or "" when there
// is none to use.
func sessionRuntimeDir(uid int) string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if filepath.IsAbs(runtimeDir) {
		return runtimeDir
	}

	// Only reached when the variable is absent, which is the whole reason to
	// look here: where it is set at all, it already names this path.
	sessionDir := filepath.Join(runtimeDirRoot, strconv.Itoa(uid))
	if verifyOwnedDir(sessionDir) != nil {
		return ""
	}

	return sessionDir
}

// endpointPath is where the daemon listens.
func endpointPath() string {
	return filepath.Join(endpointDirs()[0], SocketName)
}

// legacyEndpointPath is where neru up to and including v1 put the socket:
// straight in the temporary directory, with its mode as the only gate.
//
// It is still dialed, but only when it is a socket this user owns and only when
// no endpoint exists in the current location. That is enough for an upgraded
// CLI to reach a daemon started by the older binary, so the version handshake
// can tell the user to restart it instead of the CLI reporting that nothing is
// running — and it cannot be abused, because another account can neither create
// a file owned by this user nor unlink one from a sticky directory. Drop it
// once daemons predating the move are no longer a realistic thing to meet.
func legacyEndpointPath() string {
	return filepath.Join(os.TempDir(), SocketName)
}

// clientEndpointPath returns the endpoint a CLI process should dial: the first
// candidate that exists and is a socket this user owns, or the daemon's
// preferred path when there is none to be found.
func clientEndpointPath() string {
	dirs := endpointDirs()

	for _, dir := range dirs {
		candidate := filepath.Join(dir, SocketName)
		if verifyPrivateDir(dir) == nil && verifySocketOwner(candidate) == nil {
			return candidate
		}
	}

	if legacy := legacyEndpointPath(); verifySocketOwner(legacy) == nil {
		return legacy
	}

	return filepath.Join(dirs[0], SocketName)
}

// endpointHint returns extra guidance for a failed connection. The Unix client
// falls back to the previous location on its own, so there is nothing to add.
func endpointHint() string {
	return ""
}

// prepareEndpointDir creates the endpoint directory and refuses to use one this
// user does not exclusively own.
func prepareEndpointDir(dir string) error {
	mkdirErr := os.MkdirAll(dir, endpointDirPerms)
	if mkdirErr != nil {
		return derrors.Wrapf(
			mkdirErr,
			derrors.CodeIPCFailed,
			"cannot create the IPC endpoint directory %s",
			dir,
		)
	}

	// Ownership is checked before the mode is repaired, so a directory planted
	// by somebody else is reported as what it is rather than as a chmod that
	// was refused.
	ownerErr := verifyOwnedDir(dir)
	if ownerErr != nil {
		return ownerErr
	}

	// MkdirAll applies the process umask, and leaves an existing directory's
	// mode alone entirely, so neither case can be assumed to be 0700 already.
	chmodErr := os.Chmod(dir, endpointDirPerms)
	if chmodErr != nil {
		return derrors.Wrapf(
			chmodErr,
			derrors.CodeIPCFailed,
			"cannot restrict the IPC endpoint directory %s to its owner",
			dir,
		)
	}

	return nil
}

// prepareEndpoint clears the way for a fresh listener, refusing to unlink
// anything that is not this user's own socket.
func prepareEndpoint(path string) error {
	info, lstatErr := os.Lstat(path)
	if lstatErr != nil {
		if errors.Is(lstatErr, fs.ErrNotExist) {
			return nil
		}

		return derrors.Wrapf(
			lstatErr,
			derrors.CodeIPCFailed,
			"cannot inspect the existing IPC endpoint %s",
			path,
		)
	}

	if info.Mode()&fs.ModeSocket == 0 {
		return derrors.Newf(
			derrors.CodeIPCFailed,
			"%s already exists and is not a socket; neru will not replace it",
			path,
		)
	}

	ownerErr := verifyOwner(path, info)
	if ownerErr != nil {
		return ownerErr
	}

	removeErr := os.Remove(path)
	if removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
		return derrors.Wrapf(
			removeErr,
			derrors.CodeIPCFailed,
			"cannot remove the stale IPC endpoint %s",
			path,
		)
	}

	return nil
}

func listenEndpoint(ctx context.Context, path string) (net.Listener, error) {
	dirErr := prepareEndpointDir(filepath.Dir(path))
	if dirErr != nil {
		return nil, dirErr
	}

	prepareErr := prepareEndpoint(path)
	if prepareErr != nil {
		return nil, prepareErr
	}

	listener, listenErr := (&net.ListenConfig{}).Listen(ctx, "unix", path)
	if listenErr != nil {
		return nil, derrors.Wrapf(
			listenErr,
			derrors.CodeIPCFailed,
			"cannot listen on the IPC endpoint %s",
			path,
		)
	}

	// bind() applies the process umask, so the socket is briefly whatever that
	// leaves. The 0700 directory it was created in is what makes that window
	// harmless; this narrows the file itself to its owner regardless.
	chmodErr := os.Chmod(path, DefaultSocketPerms)
	if chmodErr != nil {
		_ = listener.Close()

		return nil, derrors.Wrapf(
			chmodErr,
			derrors.CodeIPCFailed,
			"cannot restrict the IPC endpoint %s to its owner",
			path,
		)
	}

	return listener, nil
}

func dialEndpoint(ctx context.Context, dialer net.Dialer, path string) (net.Conn, error) {
	return dialer.DialContext(ctx, "unix", path)
}

func cleanupEndpoint(path string) error {
	removeErr := os.Remove(path)
	if removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
		return removeErr
	}

	return nil
}

// inspectOwnedDir returns dir's own metadata once it is known to be a real
// directory belonging to this user. A symlink is rejected outright: following
// one would put the endpoint wherever whoever planted it decided.
func inspectOwnedDir(dir string) (fs.FileInfo, error) {
	info, lstatErr := os.Lstat(dir)
	if lstatErr != nil {
		return nil, derrors.Wrapf(
			lstatErr,
			derrors.CodeIPCFailed,
			"cannot inspect the IPC endpoint directory %s",
			dir,
		)
	}

	if !info.Mode().IsDir() {
		return nil, derrors.Newf(
			derrors.CodeIPCFailed,
			"the IPC endpoint directory %s is not a directory",
			dir,
		)
	}

	ownerErr := verifyOwner(dir, info)
	if ownerErr != nil {
		return nil, ownerErr
	}

	return info, nil
}

// verifyOwnedDir reports whether dir is a real directory belonging to this user.
func verifyOwnedDir(dir string) error {
	_, err := inspectOwnedDir(dir)

	return err
}

// verifyPrivateDir additionally requires that nobody else can enter it. The
// daemon repairs the mode of the directory it owns; a client uses this to
// decide whether the endpoint inside is one it should trust.
func verifyPrivateDir(dir string) error {
	info, err := inspectOwnedDir(dir)
	if err != nil {
		return err
	}

	if info.Mode().Perm()&0o077 != 0 {
		return derrors.Newf(
			derrors.CodeIPCFailed,
			"the IPC endpoint directory %s is reachable by other users",
			dir,
		)
	}

	return nil
}

// verifySocketOwner reports whether path is a socket belonging to this user.
func verifySocketOwner(path string) error {
	info, lstatErr := os.Lstat(path)
	if lstatErr != nil {
		return derrors.Wrapf(
			lstatErr,
			derrors.CodeIPCFailed,
			"cannot inspect the IPC endpoint %s",
			path,
		)
	}

	if info.Mode()&fs.ModeSocket == 0 {
		return derrors.Newf(derrors.CodeIPCFailed, "%s is not a socket", path)
	}

	return verifyOwner(path, info)
}

// verifyOwner reports whether info describes something owned by this user.
func verifyOwner(path string, info fs.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return derrors.Newf(derrors.CodeIPCFailed, "cannot read the owner of %s", path)
	}

	if int(stat.Uid) != os.Getuid() {
		return derrors.Newf(
			derrors.CodeIPCFailed,
			"%s belongs to uid %d rather than to this user",
			path,
			stat.Uid,
		)
	}

	return nil
}
