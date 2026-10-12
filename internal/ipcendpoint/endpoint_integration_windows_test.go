//go:build integration && windows

package ipcendpoint

import (
	"context"
	"net"
	"testing"
	"time"
)

// An upgraded CLI meeting a daemon started by the previous binary reaches it,
// so the daemon can answer with the version mismatch asking for a restart,
// rather than the CLI reporting a connection failure naming the old pipe.
//
// It is an integration test because it serves the legacy name, which every
// dial of this user's endpoint falls back to. Run beside another package's
// tests, it would answer a dial that expects no daemon. It skips while
// something answers on this user's own name, since Dial then never needs the
// old one.
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
