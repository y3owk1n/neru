package client_test

import (
	"context"
	"errors"
	"os"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/y3owk1n/neru/client"
	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/derrors"
)

const (
	modeHints      = "hints"
	eventModeEnter = "mode_enter"
)

// isolate points the endpoint lookup at a directory this test owns, so the
// client reaches the test's server and never a daemon running on the machine.
// Windows names its pipe after the user and no variable changes that, so a
// test there would share the one real pipe with a running daemon and with
// the endpoint tests in another package. It skips instead.
func isolate(t *testing.T) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("the Windows pipe name cannot be moved to a test's own")
	}

	//nolint:usetesting // t.TempDir names its directory after the test, which is
	// far too long for a socket path.
	dir, err := os.MkdirTemp("", "neru")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	t.Setenv("TMPDIR", dir)
	t.Setenv("XDG_RUNTIME_DIR", dir)
}

// serve starts a daemon-side server that answers with handler, and streams
// watch with stream when it is set.
func serve(t *testing.T, handler ipc.CommandHandler, stream ipc.StreamHandler) {
	t.Helper()

	server, err := ipc.NewServer(handler, nil)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	if stream != nil {
		server.HandleStream("watch", stream)
	}

	server.Start()

	t.Cleanup(func() { _ = server.Stop() })
}

func testContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	return ctx
}

// TestClient_Status_DecodesTheReply also pins the request a client sends: the
// protocol it speaks, and no version, which the daemon would refuse unless it
// matched the daemon's own build.
func TestClient_Status_DecodesTheReply(t *testing.T) {
	isolate(t)

	requests := make(chan ipc.Command, 1)

	serve(t, func(_ context.Context, cmd ipc.Command) ipc.Response {
		requests <- cmd

		return ipc.Response{Success: true, Data: ipc.StatusData{Enabled: true, Mode: modeHints}}
	}, nil)

	status, err := client.New().Status(testContext(t))
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	if !status.Enabled || status.Mode != modeHints {
		t.Errorf("Status() = %+v, want enabled in hints", status)
	}

	got := <-requests
	if got.Action != "status" || got.Protocol != client.ProtocolVersion || got.Version != "" {
		t.Errorf("request = %+v, want status at protocol %d with no version",
			got, client.ProtocolVersion)
	}
}

func TestClient_Hints_PassesItsFlags(t *testing.T) {
	isolate(t)

	requests := make(chan ipc.Command, 1)

	serve(t, func(_ context.Context, cmd ipc.Command) ipc.Response {
		requests <- cmd

		return ipc.Response{
			Success: true,
			Data:    ipc.HintsData{Hints: []ipc.HintData{{Role: "ax:AXButton", Title: "Save"}}},
		}
	}, nil)

	hints, err := client.New().Hints(testContext(t), "--role", "button")
	if err != nil {
		t.Fatalf("Hints() error = %v", err)
	}

	if len(hints.Hints) != 1 || hints.Hints[0].Title != "Save" {
		t.Errorf("Hints() = %+v, want the one element served", hints)
	}

	got := <-requests
	if got.Action != "query-hints" || !slices.Equal(got.Args, []string{"--role", "button"}) {
		t.Errorf("request = %+v, want query-hints with its flags", got)
	}
}

// TestClient_Watch_NamesTheEventsItWants pins that the events Watch is given
// reach the daemon as --event, and that a refused watch comes back as the
// daemon's code.
func TestClient_Watch_NamesTheEventsItWants(t *testing.T) {
	isolate(t)

	requests := make(chan ipc.Command, 1)

	serve(t, nil, func(_ context.Context, cmd ipc.Command, _ func(any) error) error {
		requests <- cmd

		return derrors.New(derrors.CodeInvalidInput, "unknown event")
	})

	err := client.New().Watch(testContext(t), func(client.WatchLine) error { return nil },
		eventModeEnter, "mode_exit")

	var refusal *client.Error
	if !errors.As(err, &refusal) || refusal.Code != ipc.CodeInvalidInput {
		t.Errorf("Watch() error = %v, want a *client.Error with %s", err, ipc.CodeInvalidInput)
	}

	if got := <-requests; !slices.Equal(got.Args, []string{"--event=mode_enter,mode_exit"}) {
		t.Errorf("watch args = %v, want the events as one --event", got.Args)
	}
}

// TestClient_Window_WithNoWindowFocused pins a reply that carries no data,
// which is how the daemon answers query-window when nothing has focus.
func TestClient_Window_WithNoWindowFocused(t *testing.T) {
	isolate(t)

	serve(t, func(_ context.Context, _ ipc.Command) ipc.Response {
		return ipc.Response{Success: true}
	}, nil)

	window, err := client.New().Window(testContext(t))
	if err != nil || window != nil {
		t.Fatalf("Window() = %+v, %v, want nil and no error", window, err)
	}
}

// TestClient_FindsADaemonStartedAfterIt pins that a client looks up the
// endpoint on each call. A client made before the daemon starts still reaches
// it, and so does one made before the daemon moves to its other possible
// location.
func TestClient_FindsADaemonStartedAfterIt(t *testing.T) {
	isolate(t)

	neru := client.New()

	isolate(t)

	serve(t, func(_ context.Context, _ ipc.Command) ipc.Response {
		return ipc.Response{Success: true}
	}, nil)

	_, err := neru.Ping(testContext(t))
	if err != nil {
		t.Fatalf("Ping() error = %v, want the daemon started after the client", err)
	}
}

func TestClient_Send_ReturnsARefusalAsAnError(t *testing.T) {
	isolate(t)

	serve(t, func(_ context.Context, _ ipc.Command) ipc.Response {
		return ipc.Response{Code: ipc.CodeInvalidInput, Message: "unknown flag --nope"}
	}, nil)

	_, err := client.New().Send(testContext(t), modeHints, "--nope")

	var refusal *client.Error
	if !errors.As(err, &refusal) || refusal.Code != ipc.CodeInvalidInput {
		t.Fatalf("Send() error = %v, want a *client.Error with %s", err, ipc.CodeInvalidInput)
	}
}

func TestClient_Send_WithNoDaemon(t *testing.T) {
	isolate(t)

	_, err := client.New().Send(testContext(t), "ping")
	if !errors.Is(err, client.ErrNoDaemon) {
		t.Fatalf("Send() error = %v, want ErrNoDaemon", err)
	}
}

func TestClient_Ping_ReturnsTheDaemonsProtocol(t *testing.T) {
	isolate(t)

	serve(t, func(_ context.Context, _ ipc.Command) ipc.Response {
		return ipc.Response{Success: true}
	}, nil)

	protocol, err := client.New().Ping(testContext(t))
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	if protocol != ipc.ProtocolVersion {
		t.Errorf("Ping() = %d, want %d", protocol, ipc.ProtocolVersion)
	}
}

// TestClient_Watch_HandsOverEachLine covers both ways a stream ends: the
// daemon closing it, which returns nil, and the caller canceling, which
// returns the context's error.
func TestClient_Watch_HandsOverEachLine(t *testing.T) {
	tests := []struct {
		name       string
		daemonEnds bool
		wantErr    error
	}{
		{name: "the daemon ends the stream", daemonEnds: true},
		{name: "the caller cancels", wantErr: context.Canceled},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			isolate(t)

			serve(t, nil, func(ctx context.Context, _ ipc.Command, emit func(any) error) error {
				for _, line := range []ipc.WatchLine{
					{Seq: 0, Event: "snapshot", Status: &ipc.StatusData{Mode: "idle"}},
					{Seq: 1, Event: eventModeEnter, Mode: modeHints},
				} {
					err := emit(line)
					if err != nil {
						return err
					}
				}

				if !testCase.daemonEnds {
					<-ctx.Done()
				}

				return nil
			})

			ctx, cancel := context.WithCancel(testContext(t))
			defer cancel()

			var events []string

			err := client.New().Watch(ctx, func(line client.WatchLine) error {
				events = append(events, line.Event)
				if len(events) == 2 && !testCase.daemonEnds {
					cancel()
				}

				return nil
			})
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("Watch() error = %v, want %v", err, testCase.wantErr)
			}

			if !slices.Equal(events, []string{"snapshot", eventModeEnter}) {
				t.Errorf("Watch() handed over %v, want the snapshot then mode_enter", events)
			}
		})
	}
}
