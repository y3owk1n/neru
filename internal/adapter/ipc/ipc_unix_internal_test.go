//go:build !windows

package ipc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// A command is bounded: the daemon refuses one that would make it buffer
// without limit, and says so rather than reporting truncated JSON.
func TestServer_HandleConnection_RefusesAnOversizedCommand(t *testing.T) {
	t.Parallel()

	accepted, dialed := connectedUnixPair(t)

	server := &Server{
		logger: zap.NewNop(),
		handler: func(_ context.Context, _ Command) Response {
			t.Error("handler ran for a command that should have been refused")

			return Response{Success: true, Code: CodeOK}
		},
	}

	server.wg.Add(1)

	go server.handleConnection(accepted)

	oversized, marshalErr := json.Marshal(Command{
		Action: "run",
		Args:   []string{strings.Repeat("x", MaxCommandBytes)},
	})
	if marshalErr != nil {
		t.Fatalf("Marshal() error = %v", marshalErr)
	}

	// The daemon stops reading at the limit, so the tail of this write has
	// nowhere to go and its error says nothing about whether the refusal
	// worked. The deadline is only here so it cannot wait forever to find out.
	writeDeadlineErr := dialed.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if writeDeadlineErr != nil {
		t.Fatalf("SetWriteDeadline() error = %v", writeDeadlineErr)
	}

	_, _ = dialed.Write(oversized)

	readDeadlineErr := dialed.SetReadDeadline(time.Now().Add(5 * time.Second))
	if readDeadlineErr != nil {
		t.Fatalf("SetReadDeadline() error = %v", readDeadlineErr)
	}

	var response Response

	decodeErr := json.NewDecoder(dialed).Decode(&response)
	if decodeErr != nil {
		t.Fatalf("decoding the refusal: %v", decodeErr)
	}

	if response.Success {
		t.Fatal("the daemon accepted a command past the size limit")
	}

	if response.Code != CodeInvalidInput {
		t.Errorf("response code = %s, want %s", response.Code, CodeInvalidInput)
	}

	if !strings.Contains(response.Message, "limit") {
		t.Errorf("response message = %q, want it to name the limit", response.Message)
	}

	server.wg.Wait()
}

// A command comfortably inside the limit is unaffected by the cap.
func TestServer_HandleConnection_AcceptsACommandInsideTheLimit(t *testing.T) {
	t.Parallel()

	accepted, dialed := connectedUnixPair(t)

	server := &Server{
		logger: zap.NewNop(),
		handler: func(_ context.Context, cmd Command) Response {
			return Response{Success: true, Message: cmd.Action, Code: CodeOK}
		},
	}

	server.wg.Add(1)

	go server.handleConnection(accepted)

	deadlineErr := dialed.SetDeadline(time.Now().Add(5 * time.Second))
	if deadlineErr != nil {
		t.Fatalf("SetDeadline() error = %v", deadlineErr)
	}

	encodeErr := json.NewEncoder(dialed).Encode(Command{Action: "status", Version: BuildVersion()})
	if encodeErr != nil {
		t.Fatalf("Encode() error = %v", encodeErr)
	}

	var response Response

	decodeErr := json.NewDecoder(dialed).Decode(&response)
	if decodeErr != nil {
		t.Fatalf("Decode() error = %v", decodeErr)
	}

	if !response.Success || response.Message != "status" {
		t.Fatalf("response = %+v, want the handler's own answer", response)
	}

	server.wg.Wait()
}

// Every reply names the protocol the daemon speaks. The daemon refuses a command
// that asks for a newer one or carries a field outside the contract, and never
// runs it.
func TestServer_HandleConnection_ChecksTheProtocol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		request  string
		wantCode string
		wantRun  bool
	}{
		{"no protocol runs", `{"action":"ping"}`, CodeOK, true},
		{"current protocol runs", `{"action":"ping","protocol":1}`, CodeOK, true},
		{"protocol 2 is refused", `{"action":"ping","protocol":2}`, CodeVersionMismatch, false},
		{"protocol -1 is refused", `{"action":"ping","protocol":-1}`, CodeInvalidInput, false},
		{"unknown field is refused", `{"action":"ping","params":{}}`, CodeInvalidInput, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			accepted, dialed := connectedUnixPair(t)

			ran := false
			server := &Server{
				logger: zap.NewNop(),
				handler: func(_ context.Context, _ Command) Response {
					ran = true

					return Response{Success: true, Code: CodeOK}
				},
			}

			server.wg.Add(1)

			go server.handleConnection(accepted)

			deadlineErr := dialed.SetDeadline(time.Now().Add(5 * time.Second))
			if deadlineErr != nil {
				t.Fatalf("SetDeadline() error = %v", deadlineErr)
			}

			_, writeErr := dialed.Write([]byte(testCase.request + "\n"))
			if writeErr != nil {
				t.Fatalf("Write() error = %v", writeErr)
			}

			var response Response

			decodeErr := json.NewDecoder(dialed).Decode(&response)
			if decodeErr != nil {
				t.Fatalf("Decode() error = %v", decodeErr)
			}

			if response.Code != testCase.wantCode {
				t.Errorf(
					"code = %s (%q), want %s",
					response.Code,
					response.Message,
					testCase.wantCode,
				)
			}

			if response.Protocol != ProtocolVersion {
				t.Errorf("protocol = %d, want %d", response.Protocol, ProtocolVersion)
			}

			server.wg.Wait()

			if ran != testCase.wantRun {
				t.Errorf("handler ran = %v, want %v", ran, testCase.wantRun)
			}
		})
	}
}
