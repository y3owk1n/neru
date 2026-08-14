package ipc

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// Running out of budget has to be distinguishable from a stream that simply
// ended, or an oversized command would be reported as malformed JSON.
func TestBoundedReader_Read(t *testing.T) {
	t.Parallel()

	const payload = "hello"

	tests := []struct {
		name      string
		input     string
		remaining int64
		want      string
		wantErr   error
	}{
		{
			name:      "reads a payload that fits",
			input:     payload,
			remaining: 16,
			want:      payload,
			wantErr:   nil,
		},
		{
			// A decoder stops as soon as it has a whole JSON value, so it never
			// asks for the byte past the budget; io.ReadAll always does.
			name:      "spends the budget exactly",
			input:     payload,
			remaining: 5,
			want:      payload,
			wantErr:   errCommandTooLarge,
		},
		{
			name:      "refuses a payload past the budget",
			input:     payload + " world",
			remaining: 5,
			want:      payload,
			wantErr:   errCommandTooLarge,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			reader := &boundedReader{
				reader:    strings.NewReader(testCase.input),
				remaining: testCase.remaining,
			}

			read, err := io.ReadAll(reader)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("io.ReadAll() error = %v, want %v", err, testCase.wantErr)
			}

			if string(read) != testCase.want {
				t.Errorf("io.ReadAll() = %q, want %q", read, testCase.want)
			}
		})
	}
}

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
