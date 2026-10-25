package ipc

import (
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"
)

// closeFailingConn is a connection whose Close reports an error. A test cannot
// make a real socket's Close fail.
type closeFailingConn struct {
	net.Conn
}

var (
	errStreamCloseFailed = errors.New("close failed")
	errEach              = errors.New("each failed")
)

func (c closeFailingConn) Close() error {
	_ = c.Conn.Close()

	return errStreamCloseFailed
}

// A failed close must reach the caller once the stream ends. It must not
// replace the error that already ended it.
func TestClient_Stream_ReportsACloseError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		reply       *Response
		eachErr     error
		wantErr     error
		wantSuccess bool
	}{
		{
			name:        "after the daemon ends the stream",
			reply:       &Response{Success: true, Code: CodeOK},
			wantErr:     errStreamCloseFailed,
			wantSuccess: true,
		},
		{
			name:    "after the daemon refuses the stream",
			reply:   &Response{Success: false, Code: CodeBusy},
			wantErr: errStreamCloseFailed,
		},
		{
			name:        "keeps the error each returns",
			reply:       &Response{Success: true, Code: CodeOK},
			eachErr:     errEach,
			wantErr:     errEach,
			wantSuccess: true,
		},
		{
			name: "keeps the receive error when the daemon hung up",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			daemon, conn := net.Pipe()
			t.Cleanup(func() { _ = daemon.Close() })

			go func() {
				defer func() { _ = daemon.Close() }()

				var cmd Command

				decodeErr := json.NewDecoder(daemon).Decode(&cmd)
				if decodeErr != nil || testCase.reply == nil {
					return
				}

				encoder := json.NewEncoder(daemon)
				_ = encoder.Encode(testCase.reply)

				if testCase.reply.Success {
					_ = encoder.Encode(WatchLine{Event: "snapshot"})
				}
			}()

			response, err := streamOver(
				closeFailingConn{Conn: conn},
				Command{Action: "watch"},
				2*time.Second,
				func(json.RawMessage) error { return testCase.eachErr },
			)

			if testCase.wantErr != nil && !errors.Is(err, testCase.wantErr) {
				t.Fatalf("Stream() error = %v, want %v", err, testCase.wantErr)
			}

			if testCase.wantErr == nil && (err == nil || errors.Is(err, errStreamCloseFailed)) {
				t.Fatalf("Stream() error = %v, want the receive error", err)
			}

			if response.Success != testCase.wantSuccess {
				t.Fatalf("response.Success = %v, want %v", response.Success, testCase.wantSuccess)
			}
		})
	}
}
