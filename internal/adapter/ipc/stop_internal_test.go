package ipc

import (
	"net"
	"testing"
	"time"

	"go.uber.org/zap"
)

// A listener whose Close never returns is not hypothetical: the Windows
// named-pipe listener reaches that state when its close signal is consumed by an
// accept that is aborting a connection, and the retry that follows waits for a
// second signal nobody sends. The failure it produced was a daemon that could
// not exit.
//
// The state is reachable only on Windows, but nothing about waiting on Close is
// platform-specific, so it is reproduced here with a listener that simply never
// returns from it.

// stuckListener never returns from Close until it is released.
type stuckListener struct {
	release chan struct{}

	// closed reports that Close was entered, so a case can tell "gave up
	// waiting" apart from "never tried".
	closed chan struct{}
}

func newStuckListener() *stuckListener {
	return &stuckListener{
		release: make(chan struct{}),
		closed:  make(chan struct{}, 1),
	}
}

func (l *stuckListener) Accept() (net.Conn, error) {
	<-l.release

	return nil, net.ErrClosed
}

func (l *stuckListener) Close() error {
	l.closed <- struct{}{}

	<-l.release

	return nil
}

func (*stuckListener) Addr() net.Addr { return nil }

// promptListener closes immediately, and reports whether it was asked to.
type promptListener struct {
	closed bool
	err    error
}

func (*promptListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }

func (l *promptListener) Close() error {
	l.closed = true

	return l.err
}

func (*promptListener) Addr() net.Addr { return nil }

func TestServer_Stop_GivesUpOnAListenerThatWillNotClose(t *testing.T) {
	listener := newStuckListener()
	defer close(listener.release)

	server := &Server{listener: listener, logger: zap.NewNop()}

	stopped := make(chan error, 1)

	go func() {
		stopped <- server.Stop()
	}()

	select {
	case <-listener.closed:
	case <-time.After(time.Second):
		t.Fatal("Stop never tried to close the listener")
	}

	// Generous, so a slow machine cannot fail this: the point is that Stop
	// returns at all, not exactly when.
	limit := listenerCloseTimeout + 10*time.Second

	select {
	case stopErr := <-stopped:
		if stopErr != nil {
			t.Errorf("Stop() = %v, want nil after abandoning the listener", stopErr)
		}
	case <-time.After(limit):
		t.Fatalf("Stop did not return within %v; a daemon in this state cannot exit", limit)
	}
}

func TestServer_Stop_ClosesTheListener(t *testing.T) {
	listener := &promptListener{}
	server := &Server{listener: listener, logger: zap.NewNop()}

	stopErr := server.Stop()
	if stopErr != nil {
		t.Errorf("Stop() = %v, want nil", stopErr)
	}

	if !listener.closed {
		t.Error("Stop returned without closing the listener")
	}
}

// TestServer_Stop_ReportsARealCloseFailure pins that giving up on a hung close did not
// also swallow the errors a close genuinely reports.
func TestServer_Stop_ReportsARealCloseFailure(t *testing.T) {
	server := &Server{
		listener: &promptListener{err: net.ErrClosed},
		logger:   zap.NewNop(),
	}

	stopErr := server.Stop()
	if stopErr == nil {
		t.Error("Stop() = nil, want the listener's close failure reported")
	}
}

// TestServer_Stop_WithoutAListenerIsHarmless covers the server that failed before it
// ever listened, which shutdown still calls Stop on.
func TestServer_Stop_WithoutAListenerIsHarmless(t *testing.T) {
	server := &Server{logger: zap.NewNop()}

	stopErr := server.Stop()
	if stopErr != nil {
		t.Errorf("Stop() = %v, want nil when there is no listener", stopErr)
	}
}
