//go:build !windows

package ipc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// isolateEndpoint points both endpoint candidates at directories this test owns
// and returns the runtime-directory base. Setting XDG_RUNTIME_DIR also switches
// off the /run/user probe, so a real daemon on the developer's machine cannot
// be picked up mid-test.
func isolateEndpoint(t *testing.T) (string, string) {
	t.Helper()

	runtimeDir := shortTempDir(t)
	tempDir := shortTempDir(t)

	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("TMPDIR", tempDir)

	return runtimeDir, tempDir
}

// listenAt binds a Unix socket at path, creating its directory with mode.
func listenAt(t *testing.T, path string, dirPerms os.FileMode) {
	t.Helper()

	mkdirErr := os.MkdirAll(filepath.Dir(path), dirPerms)
	if mkdirErr != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), mkdirErr)
	}

	chmodErr := os.Chmod(filepath.Dir(path), dirPerms)
	if chmodErr != nil {
		t.Fatalf("Chmod(%s) error = %v", filepath.Dir(path), chmodErr)
	}

	listener, listenErr := (&net.ListenConfig{}).Listen(t.Context(), "unix", path)
	if listenErr != nil {
		t.Fatalf("Listen(unix, %s) error = %v", path, listenErr)
	}

	t.Cleanup(func() { _ = listener.Close() })
}

// The endpoint's own mode must never be the only gate: the directory it sits in
// is owner-only too, so no other account can even reach the socket to try.
func TestListenEndpoint_PutsTheSocketInAnOwnerOnlyDirectory(t *testing.T) {
	runtimeDir, _ := isolateEndpoint(t)

	path := daemonEndpointPath()
	if want := filepath.Join(runtimeDir, endpointDirName, SocketName); path != want {
		t.Fatalf("daemonEndpointPath() = %s, want %s", path, want)
	}

	listener, listenErr := listenEndpoint(context.Background(), path)
	if listenErr != nil {
		t.Fatalf("listenEndpoint() error = %v", listenErr)
	}

	t.Cleanup(func() { _ = listener.Close() })

	dirInfo, dirErr := os.Stat(filepath.Dir(path))
	if dirErr != nil {
		t.Fatalf("Stat(%s) error = %v", filepath.Dir(path), dirErr)
	}

	if perm := dirInfo.Mode().Perm(); perm != endpointDirPerms {
		t.Errorf("endpoint directory mode = %#o, want %#o", perm, endpointDirPerms)
	}

	socketInfo, socketErr := os.Stat(path)
	if socketErr != nil {
		t.Fatalf("Stat(%s) error = %v", path, socketErr)
	}

	if perm := socketInfo.Mode().Perm(); perm != os.FileMode(DefaultSocketPerms) {
		t.Errorf("socket mode = %#o, want %#o", perm, DefaultSocketPerms)
	}
}

// A directory left over from an earlier run keeps whatever mode it had, so the
// daemon tightens the one it owns rather than trusting it.
func TestListenEndpoint_TightensADirectoryLeftOpen(t *testing.T) {
	runtimeDir, _ := isolateEndpoint(t)

	dir := filepath.Join(runtimeDir, endpointDirName)

	mkdirErr := os.MkdirAll(dir, 0o777)
	if mkdirErr != nil {
		t.Fatalf("MkdirAll(%s) error = %v", dir, mkdirErr)
	}

	chmodErr := os.Chmod(dir, 0o777)
	if chmodErr != nil {
		t.Fatalf("Chmod(%s) error = %v", dir, chmodErr)
	}

	listener, listenErr := listenEndpoint(context.Background(), daemonEndpointPath())
	if listenErr != nil {
		t.Fatalf("listenEndpoint() error = %v", listenErr)
	}

	t.Cleanup(func() { _ = listener.Close() })

	info, statErr := os.Stat(dir)
	if statErr != nil {
		t.Fatalf("Stat(%s) error = %v", dir, statErr)
	}

	if perm := info.Mode().Perm(); perm != endpointDirPerms {
		t.Errorf("endpoint directory mode = %#o, want it tightened to %#o", perm, endpointDirPerms)
	}
}

// A symlink where the endpoint directory belongs would put the socket wherever
// whoever planted it decided, so it is refused instead of followed.
func TestPrepareEndpointDir_RefusesASymlink(t *testing.T) {
	runtimeDir, _ := isolateEndpoint(t)

	target := filepath.Join(runtimeDir, "elsewhere")

	mkdirErr := os.Mkdir(target, endpointDirPerms)
	if mkdirErr != nil {
		t.Fatalf("Mkdir(%s) error = %v", target, mkdirErr)
	}

	dir := filepath.Join(runtimeDir, endpointDirName)

	linkErr := os.Symlink(target, dir)
	if linkErr != nil {
		t.Fatalf("Symlink(%s, %s) error = %v", target, dir, linkErr)
	}

	err := prepareEndpointDir(dir)
	if err == nil {
		t.Fatal("prepareEndpointDir() error = nil, want a refusal to use a symlinked directory")
	}

	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("prepareEndpointDir() error = %v, want it to name the problem", err)
	}
}

// A directory belonging to somebody else is reported as that rather than as a
// chmod the kernel happened to refuse.
func TestPrepareEndpointDir_RefusesADirectoryOwnedByAnotherUser(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; every directory is this user's own")
	}

	// The filesystem root is the one directory guaranteed to exist and to
	// belong to uid 0 on both platforms this file compiles for.
	err := prepareEndpointDir("/")
	if err == nil {
		t.Fatal("prepareEndpointDir(/) error = nil, want a refusal to use another user's directory")
	}

	if !strings.Contains(err.Error(), "uid 0") {
		t.Errorf("prepareEndpointDir(/) error = %v, want it to name the owner", err)
	}
}

// Whatever is already sitting on the endpoint path, the daemon only ever
// unlinks a socket of its own.
func TestPrepareEndpoint_RefusesToReplaceSomethingThatIsNotASocket(t *testing.T) {
	runtimeDir, _ := isolateEndpoint(t)

	dir := filepath.Join(runtimeDir, endpointDirName)

	mkdirErr := os.MkdirAll(dir, endpointDirPerms)
	if mkdirErr != nil {
		t.Fatalf("MkdirAll(%s) error = %v", dir, mkdirErr)
	}

	path := filepath.Join(dir, SocketName)

	writeErr := os.WriteFile(path, []byte("not a socket"), 0o600)
	if writeErr != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, writeErr)
	}

	err := prepareEndpoint(path)
	if err == nil {
		t.Fatal("prepareEndpoint() error = nil, want a refusal to replace a regular file")
	}

	if !strings.Contains(err.Error(), "not a socket") {
		t.Errorf("prepareEndpoint() error = %v, want it to say what it found", err)
	}

	_, statErr := os.Stat(path)
	if statErr != nil {
		t.Errorf("the refused file was removed anyway: %v", statErr)
	}
}

// A stale socket of this user's own is the ordinary case after an unclean exit
// and must not stop the daemon starting.
func TestPrepareEndpoint_ClearsThisUsersOwnStaleSocket(t *testing.T) {
	runtimeDir, _ := isolateEndpoint(t)

	path := filepath.Join(runtimeDir, endpointDirName, SocketName)
	listenAt(t, path, endpointDirPerms)

	err := prepareEndpoint(path)
	if err != nil {
		t.Fatalf("prepareEndpoint() error = %v, want the stale socket cleared", err)
	}

	_, statErr := os.Lstat(path)
	if statErr == nil {
		t.Error("prepareEndpoint() left the stale socket in place")
	}
}

// A daemon whose environment named no runtime directory listens under the
// temporary directory instead; a CLI that does have one must still find it.
func TestClientEndpointPath_FindsAnEndpointOutsideTheRuntimeDirectory(t *testing.T) {
	_, tempDir := isolateEndpoint(t)

	want := filepath.Join(tempDir, endpointDirPrefix+strconv.Itoa(os.Getuid()), SocketName)
	listenAt(t, want, endpointDirPerms)

	if got := clientEndpointPath(); got != want {
		t.Errorf("clientEndpointPath() = %s, want the live endpoint %s", got, want)
	}
}

// A daemon started before this release still listens in the temporary directory
// itself. Reaching it is what lets the version handshake tell the user to
// restart it, instead of the CLI reporting that nothing is running.
func TestClientEndpointPath_FallsBackToAnOlderDaemonsEndpoint(t *testing.T) {
	_, tempDir := isolateEndpoint(t)

	want := filepath.Join(tempDir, SocketName)
	listenAt(t, want, 0o755)

	if got := clientEndpointPath(); got != want {
		t.Errorf("clientEndpointPath() = %s, want the previous location %s", got, want)
	}
}

// An endpoint anyone can reach is not one the CLI should hand a command to,
// however plausible its path looks. Here the exposed one sits in the fallback
// location, which the CLI would otherwise have picked up.
func TestClientEndpointPath_IgnoresAnEndpointInAWorldReachableDirectory(t *testing.T) {
	_, tempDir := isolateEndpoint(t)

	reachable := filepath.Join(
		tempDir,
		endpointDirPrefix+strconv.Itoa(os.Getuid()),
		SocketName,
	)
	listenAt(t, reachable, 0o777)

	got := clientEndpointPath()
	if got == reachable {
		t.Fatalf("clientEndpointPath() = %s, want the exposed endpoint ignored", got)
	}

	if want := daemonEndpointPath(); got != want {
		t.Errorf("clientEndpointPath() = %s, want the preferred path %s", got, want)
	}
}

// With nothing listening anywhere, the CLI reports the path the daemon would
// bind, so a failure names somewhere real.
func TestClientEndpointPath_ReportsThePreferredPathWhenNothingIsListening(t *testing.T) {
	isolateEndpoint(t)

	if got, want := clientEndpointPath(), daemonEndpointPath(); got != want {
		t.Errorf("clientEndpointPath() = %s, want %s", got, want)
	}
}
