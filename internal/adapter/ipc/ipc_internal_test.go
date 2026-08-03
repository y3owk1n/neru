package ipc

import (
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"
)

// A handler is free to outlive the deadline set when the connection was
// accepted — an action sequence can sleep, or wait for the user to finish a
// mode. The reply must still be attempted, or a command that succeeded would
// be reported to its caller as a timeout.
func TestWriteResponse_SendsAfterTheAcceptDeadlineHasPassed(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()

	t.Cleanup(func() {
		_ = server.Close()
		_ = client.Close()
	})

	// Stand in for a handler that ran past the connection's original deadline.
	setDeadlineErr := server.SetDeadline(time.Now().Add(-time.Second))
	if setDeadlineErr != nil {
		t.Fatalf("SetDeadline: %v", setDeadlineErr)
	}

	decoded := make(chan Response, 1)

	go func() {
		var response Response

		decodeErr := json.NewDecoder(client).Decode(&response)
		if decodeErr != nil {
			close(decoded)

			return
		}

		decoded <- response
	}()

	writeErr := writeResponse(
		server,
		json.NewEncoder(server),
		Response{Success: true, Message: "ran 3 step(s)", Code: CodeOK},
	)
	if writeErr != nil {
		t.Fatalf("writeResponse() error = %v, want the reply to be sent anyway", writeErr)
	}

	select {
	case response, ok := <-decoded:
		if !ok {
			t.Fatal("client could not decode the response")
		}

		if !response.Success || response.Message != "ran 3 step(s)" {
			t.Fatalf("client received %+v, want the response that was written", response)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the response never reached the client")
	}
}

// The write side still has a bound of its own: a client that stops reading
// must not pin the connection goroutine indefinitely.
func TestWriteResponse_TimesOutWhenTheClientStopsReading(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()

	t.Cleanup(func() {
		_ = server.Close()
		_ = client.Close()
	})

	// net.Pipe is unbuffered and nothing reads the client end, so the write
	// blocks until the deadline writeResponse sets expires.
	done := make(chan error, 1)

	go func() {
		done <- writeResponse(
			server,
			json.NewEncoder(server),
			Response{Success: true, Code: CodeOK},
		)
	}()

	select {
	case writeErr := <-done:
		var netErr net.Error
		if !errors.As(writeErr, &netErr) || !netErr.Timeout() {
			t.Fatalf("writeResponse() error = %v, want a timeout once nobody reads", writeErr)
		}
	case <-time.After(ConnectionWriteTimeout + 5*time.Second):
		t.Fatal("writeResponse() never returned; the write deadline is not being applied")
	}
}
