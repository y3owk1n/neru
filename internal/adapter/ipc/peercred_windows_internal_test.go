//go:build windows

package ipc

import "testing"

// Windows answers the "who is connecting" question before the connection
// exists, through the pipe's security descriptor rather than through a check
// on the accepted connection. This pins that the nil is that answer and not an
// omission. TestPipeSecurityDescriptor_GrantsOnlyTheNamedUser in
// internal/ipcendpoint pins the descriptor itself.
func TestAuthorizePeer_DefersToThePipeSecurityDescriptor(t *testing.T) {
	t.Parallel()

	authErr := authorizePeer(nil)
	if authErr != nil {
		t.Fatalf("authorizePeer() error = %v, want the descriptor to have settled it", authErr)
	}
}
