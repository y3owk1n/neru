//go:build windows

package ipc

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Microsoft/go-winio"
	"go.uber.org/zap"
)

// The pipe namespace is machine-wide, so the endpoint name has to say which
// user it belongs to. Sharing one name is what the previous release did.
func TestDaemonEndpointPath_NamesTheEndpointAfterThisUser(t *testing.T) {
	t.Parallel()

	sid, sidErr := pipeIdentity()
	if sidErr != nil {
		t.Fatalf("pipeIdentity() error = %v", sidErr)
	}

	if !strings.HasPrefix(sid, "S-1-") {
		t.Fatalf("pipeIdentity() = %q, want a SID in string form", sid)
	}

	path := daemonEndpointPath()
	if path == legacyPipePath {
		t.Fatalf("daemonEndpointPath() = %s, want a name of this user's own", path)
	}

	if !strings.HasSuffix(path, sid) {
		t.Errorf("daemonEndpointPath() = %s, want it to end in this user's SID %s", path, sid)
	}

	// A CLI that derived a different name would never find its daemon.
	if client := clientEndpointPath(); client != path {
		t.Errorf("clientEndpointPath() = %s, want the daemon's own endpoint %s", client, path)
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

// Windows answers the "who is connecting" question before the connection
// exists, through the descriptor above rather than through a check on the
// accepted connection. This pins that the nil is that answer and not an
// omission — if the descriptor ever stops being applied, this is the test whose
// premise has gone.
func TestAuthorizePeer_DefersToThePipeSecurityDescriptor(t *testing.T) {
	t.Parallel()

	sid, sidErr := pipeIdentity()
	if sidErr != nil {
		t.Fatalf("pipeIdentity() error = %v", sidErr)
	}

	if pipeSecurityDescriptor(sid) == "" {
		t.Fatal("the pipe carries no security descriptor, so nothing gates the connection")
	}

	authErr := authorizePeer(nil)
	if authErr != nil {
		t.Fatalf("authorizePeer() error = %v, want the descriptor to have settled it", authErr)
	}
}

// servePipe listens on path as this user and accepts connections until the
// test ends, so a dial has something real on the other end to be checked.
func servePipe(t *testing.T, path string) {
	t.Helper()

	listener, listenErr := listenEndpoint(context.Background(), path)
	if listenErr != nil {
		t.Fatalf("listenEndpoint(%s) error = %v", path, listenErr)
	}

	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}

			t.Cleanup(func() { _ = conn.Close() })
		}
	}()
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

// An upgraded CLI meeting a daemon started by the previous binary gets the
// version-mismatch reply asking for a restart, the way the Unix client does,
// rather than a connection failure naming the old pipe.
//
// Not parallel: it relies on nothing listening on this user's endpoint, and
// the integration tests in this package bind exactly that.
func TestDialEndpoint_ReachesAnOldDaemonOnThePreviousName(t *testing.T) {
	listener, listenErr := listenEndpoint(context.Background(), legacyPipePath)
	if listenErr != nil {
		t.Fatalf("listenEndpoint(%s) error = %v", legacyPipePath, listenErr)
	}

	server := &Server{
		listener:   listener,
		logger:     zap.NewNop(),
		socketPath: legacyPipePath,
		handler: func(_ context.Context, _ Command) Response {
			t.Error("the old daemon ran a command from a newer CLI")

			return Response{Success: true, Code: CodeOK}
		},
	}

	server.Start()

	t.Cleanup(func() { _ = server.Stop() })

	client := &Client{socketPath: daemonEndpointPath()}

	response, sendErr := client.SendWithTimeout(
		Command{Action: "ping", Version: "older"},
		PingTimeout,
	)
	if sendErr != nil {
		t.Fatalf("SendWithTimeout() error = %v, want the old daemon's reply", sendErr)
	}

	if response.Code != CodeVersionMismatch {
		t.Fatalf("response code = %s, want %s", response.Code, CodeVersionMismatch)
	}

	if !strings.Contains(response.Message, "restart the neru daemon") {
		t.Errorf("response message = %q, want it to ask for a restart", response.Message)
	}
}
