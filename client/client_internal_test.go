package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// failingCloseConn is a connection whose Close reports an error. A test cannot
// make a real socket's Close fail.
type failingCloseConn struct {
	net.Conn
}

var errCloseFailed = errors.New("close failed")

func (c failingCloseConn) Close() error {
	_ = c.Conn.Close()

	return errCloseFailed
}

// A failed close must reach the caller. It must not replace the error that
// already failed the call, and a context that closed the connection reports
// its own error.
func TestConverse_ReportsACloseError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		success bool
		stream  bool
		cancel  bool
		wantErr func(error) bool
	}{
		{
			name:    "after the daemon replied",
			success: true,
			wantErr: func(err error) bool { return errors.Is(err, errCloseFailed) },
		},
		{
			name:    "after the daemon ends the stream",
			success: true,
			stream:  true,
			wantErr: func(err error) bool { return errors.Is(err, errCloseFailed) },
		},
		{
			name: "keeps the refusal",
			wantErr: func(err error) bool {
				var refusal *Error

				return errors.As(err, &refusal)
			},
		},
		{
			name:    "keeps the context's error when the caller cancels",
			success: true,
			stream:  true,
			cancel:  true,
			wantErr: func(err error) bool { return errors.Is(err, context.Canceled) },
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			daemon, conn := net.Pipe()
			t.Cleanup(func() { _ = daemon.Close() })

			go func() {
				defer func() { _ = daemon.Close() }()

				var req request

				decodeErr := json.NewDecoder(daemon).Decode(&req)
				if decodeErr != nil {
					return
				}

				encoder := json.NewEncoder(daemon)
				_ = encoder.Encode(reply{Protocol: ProtocolVersion, Success: testCase.success})

				if testCase.stream {
					_ = encoder.Encode(WatchLine{Event: "snapshot"})
				}

				// The daemon keeps the stream open until the client closes
				// it.
				if testCase.cancel {
					_, _ = io.Copy(io.Discard, daemon)
				}
			}()

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			var stream func(*json.Decoder) error
			if testCase.stream {
				stream = func(decoder *json.Decoder) error {
					for {
						var line WatchLine

						decodeErr := decoder.Decode(&line)
						if errors.Is(decodeErr, io.EOF) {
							return nil
						}

						if decodeErr != nil {
							return decodeErr
						}

						if testCase.cancel {
							cancel()
						}
					}
				}
			}

			_, err := converse(
				ctx,
				failingCloseConn{Conn: conn},
				request{Protocol: ProtocolVersion, Action: "watch"},
				stream,
			)
			if !testCase.wantErr(err) {
				t.Fatalf("converse() error = %v, not the error this case wants", err)
			}
		})
	}
}
