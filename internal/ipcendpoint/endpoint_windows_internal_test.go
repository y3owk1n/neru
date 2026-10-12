//go:build windows

package ipcendpoint

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
)

// The pipe namespace is machine-wide, so the endpoint name has to say which
// user it belongs to. Sharing one name is what the previous release did.
func TestDaemonPath_NamesTheEndpointAfterThisUser(t *testing.T) {
	t.Parallel()

	sid, sidErr := pipeIdentity()
	if sidErr != nil {
		t.Fatalf("pipeIdentity() error = %v", sidErr)
	}

	if !strings.HasPrefix(sid, "S-1-") {
		t.Fatalf("pipeIdentity() = %q, want a SID in string form", sid)
	}

	path := DaemonPath()
	if path == legacyPipePath {
		t.Fatalf("DaemonPath() = %s, want a name of this user's own", path)
	}

	if !strings.HasSuffix(path, sid) {
		t.Errorf("DaemonPath() = %s, want it to end in this user's SID %s", path, sid)
	}

	// A CLI that derived a different name would never find its daemon.
	if client := ClientPath(); client != path {
		t.Errorf("ClientPath() = %s, want the daemon's own endpoint %s", client, path)
	}
}

// The descriptor is what keeps another account from opening the pipe, so it
// grants this user and refuses to inherit anything else.
func TestPipeSecurityDescriptor_GrantsOnlyTheNamedUser(t *testing.T) {
	t.Parallel()

	const sid = "S-1-5-21-1-2-3-1001"

	descriptor := pipeSecurityDescriptor(sid)

	if !strings.HasPrefix(descriptor, "D:P(") {
		t.Errorf("descriptor = %q, want a protected DACL that inherits nothing", descriptor)
	}

	if !strings.Contains(descriptor, ";;;"+sid+")") {
		t.Errorf("descriptor = %q, want its only entry to name %s", descriptor, sid)
	}

	if strings.Count(descriptor, "(") != 1 {
		t.Errorf("descriptor = %q, want exactly one access-control entry", descriptor)
	}
}

// servePipe listens on path as this user and accepts the one connection the
// test dials, so that dial has something real on the other end to be checked.
//
// The listener is closed only once that accept has returned. go-winio's
// listener cannot be closed while an accept is still setting up: a Close that
// lands between the accept's own closed check and its connect call hands the
// accept an invalid-handle error instead of the closed one, the listener loops
// back believing it is still open, and Close waits on it forever. Accepting in
// a loop put a fresh accept in that window on every run, and one in a while
// the cleanup hit it, seen as a ten-minute test timeout on Windows CI.
func servePipe(t *testing.T, path string) {
	t.Helper()

	listener, listenErr := Listen(context.Background(), path)
	if listenErr != nil {
		t.Fatalf("Listen(%s) error = %v", path, listenErr)
	}

	accepted := make(chan net.Conn, 1)

	go func() {
		conn, _ := listener.Accept()
		accepted <- conn
	}()

	t.Cleanup(func() {
		// A test that never dialed leaves the accept pending on a connect
		// issued long ago, which Close cancels cleanly; only a fresh accept
		// is unsafe, and nothing issues one here.
		select {
		case conn := <-accepted:
			if conn != nil {
				_ = conn.Close()
			}
		case <-time.After(time.Second):
		}

		_ = listener.Close()
	})
}

// The name is the only thing the client chose, and a name is what another
// account can take first. The process behind it has to be this user's before
// a single byte goes out.
func TestVerifyPipeServer_RefusesAPipeServedByAnotherUser(t *testing.T) {
	t.Parallel()

	sid, sidErr := pipeIdentity()
	if sidErr != nil {
		t.Fatalf("pipeIdentity() error = %v", sidErr)
	}

	path := pipePrefix + `-test-owner-` + strconv.Itoa(os.Getpid())
	servePipe(t, path)

	conn, dialErr := winio.DialPipeContext(context.Background(), path)
	if dialErr != nil {
		t.Fatalf("DialPipeContext(%s) error = %v", path, dialErr)
	}

	defer func() { _ = conn.Close() }()

	// Only this user's own process can serve the pipe here, so "another user"
	// is played by asking the check to expect someone else on the other end.
	const otherUser = "S-1-5-21-1-2-3-1001"

	refusal := verifyPipeServer(conn, path, otherUser)
	if refusal == nil {
		t.Fatal("verifyPipeServer() accepted a server that is not the expected user")
	}

	if !strings.Contains(refusal.Error(), "rather than to this user") {
		t.Errorf("refusal = %q, want the message the Unix client gives", refusal)
	}

	if !strings.Contains(refusal.Error(), sid) {
		t.Errorf("refusal = %q, want it to name the SID actually serving the pipe", refusal)
	}

	acceptErr := verifyPipeServer(conn, path, sid)
	if acceptErr != nil {
		t.Errorf("verifyPipeServer() error = %v for this user's own server", acceptErr)
	}
}

// An upgraded CLI meeting a daemon started by the previous binary reaches it,
// so the daemon can answer with the version mismatch asking for a restart,
// rather than the CLI reporting a connection failure naming the old pipe.
//
// It skips while something answers on this user's own name, a running daemon
// or a test in another package, since Dial then never needs the old one.
func TestDial_ReachesAnOldDaemonOnThePreviousName(t *testing.T) {
	ctx := context.Background()

	current, currentErr := Dial(ctx, net.Dialer{Timeout: time.Second}, DaemonPath())
	if currentErr == nil {
		_ = current.Close()

		t.Skip("something answers on this user's endpoint")
	}

	servePipe(t, legacyPipePath)

	conn, dialErr := Dial(ctx, net.Dialer{Timeout: time.Second}, DaemonPath())
	if dialErr != nil {
		t.Fatalf("Dial(%s) error = %v, want the old daemon's pipe", DaemonPath(), dialErr)
	}

	defer func() { _ = conn.Close() }()

	if got := conn.RemoteAddr().String(); got != legacyPipePath {
		t.Errorf(
			"Dial(%s) reached %s, want the previous name %s",
			DaemonPath(),
			got,
			legacyPipePath,
		)
	}
}
