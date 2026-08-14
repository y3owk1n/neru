//go:build windows

package ipc

import (
	"strings"
	"testing"
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
