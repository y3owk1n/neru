//go:build !windows

package ipc

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shortTempDir is t.TempDir with a name short enough to bind a socket in.
// A Unix socket path is capped near 104 bytes on darwin, and t.TempDir builds
// its directory out of the test's own name, which these tests spend on saying
// what they check.
func shortTempDir(t *testing.T) string {
	t.Helper()

	//nolint:usetesting // t.TempDir names its directory after the test, which is
	// far too long for a socket path here.
	dir, mkdirErr := os.MkdirTemp("", "neru")
	if mkdirErr != nil {
		t.Fatalf("MkdirTemp() error = %v", mkdirErr)
	}

	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	return dir
}

// connectedUnixPair returns the two ends of a real Unix-domain connection: the
// one the daemon would have accepted, and the one a CLI would hold.
func connectedUnixPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()

	path := filepath.Join(shortTempDir(t), SocketName)

	listener, listenErr := (&net.ListenConfig{}).Listen(t.Context(), "unix", path)
	if listenErr != nil {
		t.Fatalf("Listen(unix, %s) error = %v", path, listenErr)
	}

	t.Cleanup(func() { _ = listener.Close() })

	type acceptResult struct {
		conn net.Conn
		err  error
	}

	accepts := make(chan acceptResult, 1)

	go func() {
		conn, err := listener.Accept()
		accepts <- acceptResult{conn: conn, err: err}
	}()

	dialed, dialErr := (&net.Dialer{}).DialContext(t.Context(), "unix", path)
	if dialErr != nil {
		t.Fatalf("Dial(unix, %s) error = %v", path, dialErr)
	}

	t.Cleanup(func() { _ = dialed.Close() })

	result := <-accepts
	if result.err != nil {
		t.Fatalf("Accept() error = %v", result.err)
	}

	t.Cleanup(func() { _ = result.conn.Close() })

	return result.conn, dialed
}

// The kernel is asked who connected, and this user's own CLI is let through.
func TestAuthorizePeer_AcceptsThisUsersOwnProcess(t *testing.T) {
	t.Parallel()

	accepted, _ := connectedUnixPair(t)

	err := authorizePeer(accepted)
	if err != nil {
		t.Fatalf("authorizePeer() error = %v, want this user's own connection accepted", err)
	}
}

// peerUID must report the identity the kernel recorded, not a zero it never
// read — a silent zero would read as root and pass the check.
func TestPeerUID_ReportsTheConnectingProcessesOwner(t *testing.T) {
	t.Parallel()

	accepted, _ := connectedUnixPair(t)

	unixConn, ok := accepted.(*net.UnixConn)
	if !ok {
		t.Fatalf("accepted connection is %T, want *net.UnixConn", accepted)
	}

	uid, uidErr := peerUID(unixConn)
	if uidErr != nil {
		t.Fatalf("peerUID() error = %v", uidErr)
	}

	if int(uid) != os.Getuid() {
		t.Errorf("peerUID() = %d, want this process's uid %d", uid, os.Getuid())
	}
}

// Anything that is not a Unix socket carries no credentials to check, so it is
// refused rather than waved through on the strength of the endpoint's mode.
func TestAuthorizePeer_RefusesAConnectionWithNoCredentialsToRead(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()

	t.Cleanup(func() {
		_ = server.Close()
		_ = client.Close()
	})

	err := authorizePeer(server)
	if err == nil {
		t.Fatal("authorizePeer() error = nil, want a connection with no peer identity refused")
	}

	if !strings.Contains(err.Error(), "unix socket") {
		t.Errorf("authorizePeer() error = %v, want it to say why the peer is unknown", err)
	}
}
