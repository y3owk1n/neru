package ipcctrl_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/components"
	"github.com/y3owk1n/neru/internal/app/components/scroll"
	"github.com/y3owk1n/neru/internal/app/ipcctrl"
	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/domain/state"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

const (
	actionGrid   = "grid"
	actionHints  = "hints"
	actionScroll = "scroll"
	actionIdle   = "idle"
)

func newTestModesHandler(
	cfg *config.Config,
	logger *zap.Logger,
	appState *state.AppState,
	actionService *services.ActionService,
) *modes.Handler {
	// Only the fields this test needs; every other dependency is legitimately
	// the zero value, which the deps struct expresses by omission.
	return modes.NewHandler(modes.HandlerDeps{
		Ctx:                   context.Background(),
		Config:                cfg,
		Logger:                logger,
		AppState:              appState,
		CursorState:           state.NewCursorState(),
		ScrollComponent:       &components.ScrollComponent{Context: &scroll.Context{}},
		ActionService:         actionService,
		RefreshHotkeys:        func() {},
		ExecuteActionSequence: func(string, []string) {},
		Shutdown:              func() {},
	})
}

// newToggleTestController builds a controller over the real registration path,
// so these tests exercise the wiring rather than a handler called directly.
func newToggleTestController(
	appState *state.AppState,
	executeMacro func(context.Context, string, []string) error,
) *ipcctrl.Controller {
	cfg := config.DefaultConfig()
	logger := zap.NewNop()
	actionService := services.NewActionService(
		&portmocks.MockAccessibilityPort{},
		&portmocks.MockOverlayPort{},
		&portmocks.MockSystemPort{},
		logger,
	)

	return ipcctrl.New(ipcctrl.Deps{
		ActionService: actionService,
		ConfigService: loader.NewService(cfg, "", logger, nil),
		AppState:      appState,
		Config:        cfg,
		Modes:         newTestModesHandler(cfg, logger, appState, actionService),
		ExecuteMacro:  executeMacro,
		Logger:        logger,
	})
}

func TestHandleCommand_MacroReachesTheRunner(t *testing.T) {
	var (
		gotName string
		gotArgs []string
	)

	controller := newToggleTestController(
		state.NewAppState(),
		func(_ context.Context, name string, args []string) error {
			gotName, gotArgs = name, args

			return nil
		},
	)

	resp := controller.HandleCommand(context.Background(), ipc.Command{
		Action: "macro",
		Args:   []string{"window_click", "100", "70"},
	})

	if !resp.Success {
		t.Fatalf("HandleCommand(macro) = %+v, want success", resp)
	}

	if gotName != "window_click" {
		t.Fatalf("macro name = %q, want %q", gotName, "window_click")
	}

	if strings.Join(gotArgs, ",") != "100,70" {
		t.Fatalf("macro args = %v, want [100 70]", gotArgs)
	}
}

func TestHandleCommand_ToggleStateConvergesThroughTheController(t *testing.T) {
	appState := state.NewAppState()
	appState.SetScrollInverted(true)

	controller := newToggleTestController(appState, nil)

	// Asking twice for the same state must land there both times: that is what
	// a script relies on and what a bare toggle cannot give it.
	for range 2 {
		resp := controller.HandleCommand(context.Background(), ipc.Command{
			Action: "toggle-scroll-invert",
			Args:   []string{"--state=off"},
		})

		if !resp.Success {
			t.Fatalf("HandleCommand(toggle-scroll-invert) = %+v, want success", resp)
		}

		if appState.IsScrollInverted() {
			t.Fatal("scroll inverted = true, want --state=off to converge on off")
		}
	}
}

func TestHandleCommand_StatusReportsTheToggles(t *testing.T) {
	appState := state.NewAppState()
	appState.SetScrollInverted(true)
	appState.SetHiddenForScreenShare(true)

	controller := newToggleTestController(appState, nil)

	resp := controller.HandleCommand(context.Background(), ipc.Command{Action: "status"})
	if !resp.Success {
		t.Fatalf("HandleCommand(status) = %+v, want success", resp)
	}

	status, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("status data = %T, want map[string]any", resp.Data)
	}

	for key, want := range map[string]bool{
		"scroll_inverted":         true,
		"hidden_for_screen_share": true,
	} {
		got, isBool := status[key].(bool)
		if !isBool {
			t.Fatalf("status[%q] = %v (%T), want a bool", key, status[key], status[key])
		}

		if got != want {
			t.Fatalf("status[%q] = %t, want %t", key, got, want)
		}
	}

	// No mode is running, so the session-scoped toggle has no state to report.
	// Null and false are different answers and the payload must not conflate
	// them.
	value, present := status["cursor_follow_selection"]
	if !present {
		t.Fatal("status is missing cursor_follow_selection")
	}

	if follow, isPointer := value.(*bool); !isPointer || follow != nil {
		t.Fatalf("status[cursor_follow_selection] = %v, want nil while no mode runs", value)
	}

	// What a script actually reads is the encoded form, so pin that rather than
	// the Go value: "neru status --json | jq .cursor_follow_selection" has to
	// print null, not false and not a missing key.
	encoded, encodeErr := json.Marshal(status)
	if encodeErr != nil {
		t.Fatalf("json.Marshal(status) error = %v", encodeErr)
	}

	if !strings.Contains(string(encoded), `"cursor_follow_selection":null`) {
		t.Fatalf("encoded status does not report a null toggle:\n%s", encoded)
	}

	if !strings.Contains(string(encoded), `"scroll_inverted":true`) {
		t.Fatalf("encoded status does not report the scroll toggle:\n%s", encoded)
	}
}

func TestHandleCommand_StatusReportsSavedCursorSlots(t *testing.T) {
	controller := newToggleTestController(state.NewAppState(), nil)

	// Empty is an empty object, not a null: a script indexing into it should
	// not have to guard the container as well as the key.
	before := statusField(t, controller, "saved_cursor_slots")
	if slots, ok := before.(map[string]map[string]int); !ok || len(slots) != 0 {
		t.Fatalf("saved_cursor_slots = %v (%T), want an empty object", before, before)
	}

	resp := controller.HandleCommand(context.Background(), ipc.Command{
		Action: "action",
		Args:   []string{"save_cursor_pos", "--slot=home"},
	})
	if !resp.Success {
		t.Fatalf("save_cursor_pos = %+v, want success", resp)
	}

	after := statusField(t, controller, "saved_cursor_slots")

	slots, ok := after.(map[string]map[string]int)
	if !ok {
		t.Fatalf("saved_cursor_slots = %T, want map[string]map[string]int", after)
	}

	home, present := slots["home"]
	if !present {
		t.Fatalf("saved_cursor_slots = %v, want an entry for home", slots)
	}

	// Lowercase x and y, not the X and Y that image.Point would encode.
	if _, hasX := home["x"]; !hasX {
		t.Fatalf("slot home = %v, want lowercase x and y keys", home)
	}

	if _, hasY := home["y"]; !hasY {
		t.Fatalf("slot home = %v, want lowercase x and y keys", home)
	}
}

// statusField reads one key out of the status payload.
func statusField(t *testing.T, controller *ipcctrl.Controller, key string) any {
	t.Helper()

	resp := controller.HandleCommand(context.Background(), ipc.Command{Action: "status"})
	if !resp.Success {
		t.Fatalf("HandleCommand(status) = %+v, want success", resp)
	}

	status, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("status data = %T, want map[string]any", resp.Data)
	}

	value, present := status[key]
	if !present {
		t.Fatalf("status is missing %q", key)
	}

	return value
}
