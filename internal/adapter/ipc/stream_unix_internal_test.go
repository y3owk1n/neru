//go:build !windows

package ipc

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
)

// streamTestAction is the action the stream tests register.
const streamTestAction = "watch"

// streamServer is a server whose one stream action runs handler.
func streamServer(handler StreamHandler) *Server {
	server := &Server{
		logger: zap.NewNop(),
		handler: func(_ context.Context, _ Command) Response {
			return Response{Success: false, Code: CodeUnknownCommand}
		},
	}
	server.HandleStream(streamTestAction, handler)

	return server
}

// openStream connects a client to server, sends cmd, and returns the client
// end with a decoder positioned after nothing.
func openStream(t *testing.T, server *Server, cmd Command) (net.Conn, *json.Decoder) {
	t.Helper()

	accepted, dialed := connectedUnixPair(t)

	server.wg.Add(1)

	go server.handleConnection(accepted)

	encodeErr := json.NewEncoder(dialed).Encode(cmd)
	if encodeErr != nil {
		t.Fatalf("sending the command: %v", encodeErr)
	}

	deadlineErr := dialed.SetReadDeadline(time.Now().Add(5 * time.Second))
	if deadlineErr != nil {
		t.Fatalf("SetReadDeadline() error = %v", deadlineErr)
	}

	return dialed, json.NewDecoder(dialed)
}

// readReply decodes the reply that opens every exchange.
func readReply(t *testing.T, decoder *json.Decoder) Response {
	t.Helper()

	var response Response

	decodeErr := decoder.Decode(&response)
	if decodeErr != nil {
		t.Fatalf("decoding the reply: %v", decodeErr)
	}

	return response
}

// waitClosed fails unless done closes within a few seconds.
func waitClosed(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s never happened", what)
	}
}

func TestServer_Stream_RepliesThenStreamsInOrder(t *testing.T) {
	t.Parallel()

	ended := make(chan struct{})
	server := streamServer(func(ctx context.Context, _ Command, emit func(any) error) error {
		defer close(ended)

		for seq := 1; seq <= 3; seq++ {
			emitErr := emit(map[string]int{"seq": seq})
			if emitErr != nil {
				return emitErr
			}
		}

		<-ctx.Done()

		return nil
	})

	client, decoder := openStream(t, server, Command{Action: streamTestAction})

	if reply := readReply(t, decoder); !reply.Success || reply.Code != CodeOK {
		t.Fatalf("reply = %+v, want success with %s", reply, CodeOK)
	}

	for want := 1; want <= 3; want++ {
		var line map[string]int

		decodeErr := decoder.Decode(&line)
		if decodeErr != nil {
			t.Fatalf("decoding line %d: %v", want, decodeErr)
		}

		if line["seq"] != want {
			t.Fatalf("line %d has seq %d", want, line["seq"])
		}
	}

	// The client going away is what ends a stream.
	_ = client.Close()

	waitClosed(t, ended, "the handler ending after the client left")
	server.wg.Wait()
}

func TestServer_Stop_EndsAnOpenStreamPromptly(t *testing.T) {
	t.Parallel()

	server := streamServer(func(ctx context.Context, _ Command, emit func(any) error) error {
		emitErr := emit("started")
		if emitErr != nil {
			return emitErr
		}

		<-ctx.Done()

		return nil
	})

	_, decoder := openStream(t, server, Command{Action: streamTestAction})
	readReply(t, decoder)

	var first string

	decodeErr := decoder.Decode(&first)
	if decodeErr != nil {
		t.Fatalf("decoding the first line: %v", decodeErr)
	}

	start := time.Now()

	stopErr := server.Stop()
	if stopErr != nil {
		t.Fatalf("Stop() error = %v", stopErr)
	}

	// Stop waits up to a second for connections before giving up on them, so
	// a stream it did not end itself would cost that whole second.
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("Stop() took %v with a stream open, want it to end the stream at once", elapsed)
	}
}

func TestServer_Stream_RefusesPastTheCap(t *testing.T) {
	t.Parallel()

	server := streamServer(func(ctx context.Context, _ Command, emit func(any) error) error {
		emitErr := emit("started")
		if emitErr != nil {
			return emitErr
		}

		<-ctx.Done()

		return nil
	})

	for range maxStreams {
		_, decoder := openStream(t, server, Command{Action: streamTestAction})

		if reply := readReply(t, decoder); !reply.Success {
			t.Fatalf("stream inside the cap refused: %+v", reply)
		}
	}

	_, decoder := openStream(t, server, Command{Action: streamTestAction})

	if reply := readReply(t, decoder); reply.Success || reply.Code != CodeBusy {
		t.Errorf("reply past the cap = %+v, want %s", reply, CodeBusy)
	}

	stopErr := server.Stop()
	if stopErr != nil {
		t.Fatalf("Stop() error = %v", stopErr)
	}
}

func TestServer_Stream_RefusesAClientOfAnotherVersion(t *testing.T) {
	t.Parallel()

	server := streamServer(func(_ context.Context, _ Command, _ func(any) error) error {
		t.Error("the stream started for a client of another version")

		return nil
	})

	_, decoder := openStream(
		t,
		server,
		Command{Action: streamTestAction, Version: "not-this-build"},
	)

	if reply := readReply(t, decoder); reply.Success || reply.Code != CodeVersionMismatch {
		t.Errorf("reply = %+v, want %s", reply, CodeVersionMismatch)
	}

	server.wg.Wait()
}

func TestServer_Stream_HandlerErrorBeforeTheFirstLineIsTheReply(t *testing.T) {
	t.Parallel()

	server := streamServer(func(_ context.Context, _ Command, _ func(any) error) error {
		return derrors.New(derrors.CodeNotSupported, "no events here")
	})

	_, decoder := openStream(t, server, Command{Action: streamTestAction})

	reply := readReply(t, decoder)
	if reply.Success || reply.Code != CodeNotSupported || reply.Message == "" {
		t.Errorf("reply = %+v, want %s naming the error", reply, CodeNotSupported)
	}

	server.wg.Wait()
}

func TestServer_Stream_AClientLeavingFreesItsPlace(t *testing.T) {
	t.Parallel()

	server := streamServer(func(ctx context.Context, _ Command, emit func(any) error) error {
		emitErr := emit("started")
		if emitErr != nil {
			return emitErr
		}

		<-ctx.Done()

		return nil
	})

	clients := make([]net.Conn, 0, maxStreams)

	for range maxStreams {
		client, decoder := openStream(t, server, Command{Action: streamTestAction})
		readReply(t, decoder)

		clients = append(clients, client)
	}

	_ = clients[0].Close()

	// The place frees once the stream notices its client left, so ask until a
	// stream is let in or the wait runs out.
	deadline := time.Now().Add(5 * time.Second)

	for {
		_, decoder := openStream(t, server, Command{Action: streamTestAction})

		reply := readReply(t, decoder)
		if reply.Success {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("a stream was still refused 5s after a client left: %+v", reply)
		}

		time.Sleep(10 * time.Millisecond)
	}

	stopErr := server.Stop()
	if stopErr != nil {
		t.Fatalf("Stop() error = %v", stopErr)
	}
}

func TestServer_Stream_RefusesAStreamAskedForAfterStop(t *testing.T) {
	t.Parallel()

	server := streamServer(func(_ context.Context, _ Command, _ func(any) error) error {
		t.Error("a stream started after Stop, which nothing would end")

		return nil
	})

	stopErr := server.Stop()
	if stopErr != nil {
		t.Fatalf("Stop() error = %v", stopErr)
	}

	_, decoder := openStream(t, server, Command{Action: streamTestAction})

	if reply := readReply(t, decoder); reply.Success {
		t.Errorf("reply = %+v, want a refusal", reply)
	}

	server.wg.Wait()
}
