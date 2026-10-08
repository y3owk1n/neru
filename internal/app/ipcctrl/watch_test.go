package ipcctrl_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/ipcctrl"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/event"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// watchController is a controller whose watch streams bus.
func watchController(bus *event.Bus) *ipcctrl.Controller {
	cfg := config.DefaultConfig()

	return ipcctrl.New(ipcctrl.Deps{
		ConfigService: loader.NewService(cfg, "", zap.NewNop(), nil),
		AppState:      state.NewAppState(),
		Config:        cfg,
		System:        &portmocks.MockSystemPort{},
		Events:        bus,
	})
}

// watchLines runs a watch until ctx ends, handing each line to the returned
// channel as the JSON object a client would read. gate, when not nil, holds
// the first line until it closes.
func watchLines(
	ctx context.Context,
	t *testing.T,
	controller *ipcctrl.Controller,
	gate <-chan struct{},
) (<-chan map[string]any, <-chan error) {
	t.Helper()

	lines := make(chan map[string]any, 256)
	done := make(chan error, 1)

	go func() {
		first := true
		done <- controller.HandleWatch(ctx, ipc.Command{Action: domain.CommandWatch}, func(value any) error {
			if first && gate != nil {
				<-gate
			}

			first = false

			encoded, marshalErr := json.Marshal(value)
			if marshalErr != nil {
				return marshalErr
			}

			var line map[string]any

			unmarshalErr := json.Unmarshal(encoded, &line)
			if unmarshalErr != nil {
				return unmarshalErr
			}

			lines <- line

			return nil
		})
	}()

	return lines, done
}

// nextLine waits for one line.
func nextLine(t *testing.T, lines <-chan map[string]any) map[string]any {
	t.Helper()

	select {
	case line := <-lines:
		return line
	case <-time.After(5 * time.Second):
		t.Fatal("no line within 5s")

		return nil
	}
}

func TestController_HandleWatch_OpensWithTheStatusThenStreamsEvents(t *testing.T) {
	bus := event.NewBus(nil)
	ctx, cancel := context.WithCancel(t.Context())
	lines, done := watchLines(ctx, t, watchController(bus), nil)

	snapshot := nextLine(t, lines)
	if snapshot["event"] != "snapshot" || snapshot["seq"] != float64(0) {
		t.Fatalf("first line = %v, want the snapshot at seq 0", snapshot)
	}

	status, isMap := snapshot["status"].(map[string]any)
	if !isMap || status["mode"] != "idle" || status["enabled"] != true {
		t.Fatalf(
			"snapshot status = %v, want the status neru status --json prints",
			snapshot["status"],
		)
	}

	bus.Publish(event.Event{Name: event.ModeEnter, Mode: "hints"})
	bus.Publish(event.Event{Name: event.ConfigReload, OK: false})

	entered := nextLine(t, lines)
	if entered["seq"] != float64(1) || entered["event"] != "mode_enter" ||
		entered["mode"] != "hints" {
		t.Errorf("second line = %v, want mode_enter hints at seq 1", entered)
	}

	reloaded := nextLine(t, lines)
	if reloaded["event"] != "config_reload" {
		t.Fatalf("third line = %v, want config_reload", reloaded)
	}

	if ok, sent := reloaded["ok"]; !sent || ok != false {
		t.Errorf("config_reload line = %v, want ok: false on the wire", reloaded)
	}

	cancel()

	err := <-done
	if err != nil {
		t.Errorf("HandleWatch() = %v after its client left, want nil", err)
	}
}

func TestController_HandleWatch_CountsWhatASlowReaderMissed(t *testing.T) {
	bus := event.NewBus(nil)
	gate := make(chan struct{})
	lines, _ := watchLines(t.Context(), t, watchController(bus), gate)

	// The handler subscribes before its first line, so these queue behind
	// the held snapshot and overflow its buffer.
	time.Sleep(100 * time.Millisecond)

	const published = 80

	for range published {
		bus.Publish(event.Event{Name: event.Enable})
	}

	close(gate)

	nextLine(t, lines)

	delivered := 0

	for quiet := false; !quiet; {
		select {
		case <-lines:
			delivered++
		case <-time.After(200 * time.Millisecond):
			quiet = true
		}
	}

	// The next event to arrive says how many the reader missed.
	bus.Publish(event.Event{Name: event.Disable})

	last := nextLine(t, lines)

	dropped, carried := last["dropped"].(float64)
	if !carried || delivered+int(dropped) != published {
		t.Errorf(
			"delivered %d and the next line says %v dropped, want the %d published accounted for",
			delivered,
			last["dropped"],
			published,
		)
	}
}

func TestController_HandleWatch_RefusesWithoutABus(t *testing.T) {
	err := watchController(nil).HandleWatch(
		t.Context(),
		ipc.Command{Action: domain.CommandWatch},
		func(any) error {
			t.Error("a watch with no bus emitted a line")

			return nil
		},
	)
	if !derrors.IsNotSupported(err) {
		t.Errorf("HandleWatch() = %v, want not supported", err)
	}
}
