package ipcctrl_test

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/ipcctrl"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
)

// testGridCharsToSet is the coordinate character set the config-set cases
// switch the grid to, and testGridLabelsFromChars the labels a grid built from
// it carries.
const (
	testGridCharsToSet      = "asdf"
	testGridLabelsFromChars = "ASDF"
)

func newTestController() *ipcctrl.Controller {
	cfg := config.DefaultConfig()
	appState := state.NewAppState()
	logger, _ := zap.NewDevelopment()
	// WithWritten is what the daemon does with the two halves of its load, and
	// a controller without it cannot re-derive — which is the behavior under
	// test in the config-set cases below.
	configService := loader.NewService(cfg, "", logger, nil).
		WithWritten(config.DefaultConfigForDecoding())

	return ipcctrl.New(ipcctrl.Deps{
		ConfigService: configService,
		AppState:      appState,
		Config:        cfg,
		Logger:        logger,
	})
}

func TestIPCController_HandlePing(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandPing})

	if !commandResponse.Success {
		t.Errorf("Expected success=true, got %v", commandResponse.Success)
	}

	if commandResponse.Message != "pong" {
		t.Errorf("Expected message='pong', got %q", commandResponse.Message)
	}

	if commandResponse.Code != ipc.CodeOK {
		t.Errorf("Expected code=%s, got %s", ipc.CodeOK, commandResponse.Code)
	}
}

func TestIPCController_HandleStart(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	// Disable state first (NewAppState starts with enabled=true)
	controller.AppState.SetEnabled(false)

	// First start should succeed
	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandStart})
	if !commandResponse.Success {
		t.Errorf("Expected success=true, got %v", commandResponse.Success)
	}

	if !controller.AppState.IsEnabled() {
		t.Error("Expected state to be enabled after start")
	}

	// Second start should fail (already running)
	commandResponse = controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandStart})
	if commandResponse.Success {
		t.Error("Expected success=false when already running")
	}

	if commandResponse.Code != ipc.CodeAlreadyRunning {
		t.Errorf("Expected code=%s, got %s", ipc.CodeAlreadyRunning, commandResponse.Code)
	}
}

func TestIPCController_HandleStop(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	// Disable state first (NewAppState starts with enabled=true)
	controller.AppState.SetEnabled(false)

	// Stop when not running should fail
	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandStop})
	if commandResponse.Success {
		t.Error("Expected success=false when not running")
	}

	if commandResponse.Code != ipc.CodeNotRunning {
		t.Errorf("Expected code=%s, got %s", ipc.CodeNotRunning, commandResponse.Code)
	}

	// Start then stop should succeed
	controller.AppState.SetEnabled(true)

	commandResponse = controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandStop})
	if !commandResponse.Success {
		t.Errorf("Expected success=true, got %v", commandResponse.Success)
	}

	if controller.AppState.IsEnabled() {
		t.Error("Expected state to be disabled after stop")
	}
}

func TestIPCController_HandleConfig(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandConfig})
	if !commandResponse.Success {
		t.Errorf("Expected success=true, got %v", commandResponse.Success)
	}

	if commandResponse.Data == nil {
		t.Error("Expected non-nil data with config struct")
	}

	// Verify it's a valid config struct
	if cfg, ok := commandResponse.Data.(*config.Config); !ok {
		t.Errorf("Expected data to be *config.Config, got %T", commandResponse.Data)
	} else if cfg == nil {
		t.Error("Expected valid config struct, got nil")
	}
}

func TestIPCController_HandleActionAndScroll(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	// Test that the scroll handler can be called
	scrollResponse := controller.HandleCommand(ctx, ipc.Command{Action: actionScroll})
	if scrollResponse.Code == ipc.CodeUnknownCommand {
		t.Error("Scroll command should be recognized")
	}
}

func TestIPCController_UnknownCommand(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: "unknown_command"})
	if commandResponse.Success {
		t.Error("Expected success=false for unknown command")
	}

	if commandResponse.Code != ipc.CodeUnknownCommand {
		t.Errorf("Expected code=%s, got %s", ipc.CodeUnknownCommand, commandResponse.Code)
	}
}

func TestIPCController_HandleConfigSet_String(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	// Set a string config field
	commandResponse := controller.HandleCommand(ctx, ipc.Command{
		Action: domain.CommandConfigSet,
		Args:   []string{"hints.hint_characters", "qwerty"},
	})

	if !commandResponse.Success {
		t.Fatalf(
			"Expected success=true, got %v: %s",
			commandResponse.Success,
			commandResponse.Message,
		)
	}

	if commandResponse.Code != ipc.CodeOK {
		t.Fatalf("Expected code=%s, got %s", ipc.CodeOK, commandResponse.Code)
	}

	// Verify the change took effect by dumping the config
	cfgResp := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandConfig})
	if cfg, ok := cfgResp.Data.(*config.Config); ok && cfg != nil {
		if cfg.Hints.HintCharacters != "qwerty" {
			t.Errorf("Expected hint_characters='qwerty', got %q", cfg.Hints.HintCharacters)
		}
	} else {
		t.Error("Failed to read config after set")
	}
}

func TestIPCController_HandleConfigSet_Integer(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	commandResponse := controller.HandleCommand(ctx, ipc.Command{
		Action: domain.CommandConfigSet,
		Args:   []string{"hints.ui.font_size", "14"},
	})

	if !commandResponse.Success {
		t.Fatalf(
			"Expected success=true, got %v: %s",
			commandResponse.Success,
			commandResponse.Message,
		)
	}

	// Verify the change
	cfgResp := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandConfig})
	if cfg, ok := cfgResp.Data.(*config.Config); ok && cfg != nil {
		if cfg.Hints.UI.FontSize != 14 {
			t.Errorf("Expected font_size=14, got %d", cfg.Hints.UI.FontSize)
		}
	} else {
		t.Error("Failed to read config after set")
	}
}

func TestIPCController_HandleConfigSet_Bool(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	commandResponse := controller.HandleCommand(ctx, ipc.Command{
		Action: domain.CommandConfigSet,
		Args:   []string{"general.passthrough_unbounded_keys", "true"},
	})

	if !commandResponse.Success {
		t.Fatalf(
			"Expected success=true, got %v: %s",
			commandResponse.Success,
			commandResponse.Message,
		)
	}

	// Verify the change
	cfgResp := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandConfig})
	if cfg, ok := cfgResp.Data.(*config.Config); ok && cfg != nil {
		if !cfg.General.PassthroughUnboundedKeys {
			t.Error("Expected passthrough_unbounded_keys=true")
		}
	} else {
		t.Error("Failed to read config after set")
	}
}

func TestIPCController_HandleConfigSet_InvalidKey(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	commandResponse := controller.HandleCommand(ctx, ipc.Command{
		Action: domain.CommandConfigSet,
		Args:   []string{"hints.nonexistent", "value"},
	})

	if commandResponse.Success {
		t.Fatal("Expected success=false for invalid key")
	}

	if commandResponse.Code != ipc.CodeInvalidInput {
		t.Fatalf("Expected code=%s, got %s", ipc.CodeInvalidInput, commandResponse.Code)
	}
}

func TestIPCController_HandleConfigSet_MissingArgs(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	commandResponse := controller.HandleCommand(ctx, ipc.Command{
		Action: domain.CommandConfigSet,
		Args:   []string{"hints.hint_characters"},
	})

	if commandResponse.Success {
		t.Fatal("Expected success=false for missing args")
	}

	if commandResponse.Code != ipc.CodeInvalidInput {
		t.Fatalf("Expected code=%s, got %s", ipc.CodeInvalidInput, commandResponse.Code)
	}
}

func TestIPCController_HandleConfigSet_NoReload(t *testing.T) {
	controller := newTestController()
	ctx := context.Background()

	// Default config: grid_cols=3, grid_rows=3, keys="rtyfghvbn" (9 chars).

	// Step 1: change grid_cols to 4 with --no-reload.
	// This creates an unbalanced state (4*3=12 ≠ 9 chars keys).
	// Without --no-reload this would fail validation.
	resp := controller.HandleCommand(ctx, ipc.Command{
		Action: domain.CommandConfigSet,
		Args:   []string{"recursive_grid.grid_cols", "4", "--no-reload"},
	})
	if !resp.Success {
		t.Fatalf("--no-reload grid_cols: expected success, got %v: %s",
			resp.Success, resp.Message)
	}

	if resp.Code != ipc.CodeOK {
		t.Fatalf("--no-reload grid_cols: expected code %s, got %s",
			ipc.CodeOK, resp.Code)
	}

	if !strings.Contains(resp.Message, "reload required") {
		t.Fatalf("--no-reload grid_cols: message should contain 'reload required', got %q",
			resp.Message)
	}

	// Step 2: fix keys to match (4*3=12 chars).
	resp = controller.HandleCommand(ctx, ipc.Command{
		Action: domain.CommandConfigSet,
		Args:   []string{"recursive_grid.keys", "abcdefghijkl", "--no-reload"},
	})
	if !resp.Success {
		t.Fatalf("--no-reload keys: expected success, got %v: %s",
			resp.Success, resp.Message)
	}

	// Step 3: set an unrelated field without --no-reload.
	// This exercises that prior --no-reload changes are visible to the
	// full validation path (deep copies from service, sees 4*3=12=keys).
	resp = controller.HandleCommand(ctx, ipc.Command{
		Action: domain.CommandConfigSet,
		Args:   []string{"recursive_grid.min_size_width", "2"},
	})
	if !resp.Success {
		t.Fatalf("regular set after --no-reload: expected success, got %v: %s",
			resp.Success, resp.Message)
	}

	// Step 4: verify final state.
	cfgResp := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandConfig})

	cfg, ok := cfgResp.Data.(*config.Config)
	if !ok || cfg == nil {
		t.Fatal("failed to read config after no-reload sequence")
	}

	if cfg.RecursiveGrid.GridCols != 4 {
		t.Errorf("expected grid_cols=4, got %d", cfg.RecursiveGrid.GridCols)
	}

	if cfg.RecursiveGrid.Keys != "abcdefghijkl" {
		t.Errorf("expected keys=%q, got %q", "abcdefghijkl", cfg.RecursiveGrid.Keys)
	}

	if cfg.RecursiveGrid.MinSizeWidth != 2 {
		t.Errorf("expected min_size_width=2, got %d", cfg.RecursiveGrid.MinSizeWidth)
	}
}

func TestIPCController_UpdateConfig(t *testing.T) {
	controller := newTestController()
	ctx := context.Background()
	// Verify initial config is returned
	commandResponse := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandConfig})
	if !commandResponse.Success {
		t.Fatalf("Expected success=true, got %v", commandResponse.Success)
	}

	initialCfg, initialCfgOk := commandResponse.Data.(*config.Config)
	if !initialCfgOk || initialCfg == nil {
		t.Fatal("Expected valid initial config")
	}
	// Create a new config with different values
	newCfg := config.DefaultConfig()
	newCfg.Hints.Enabled = !initialCfg.Hints.Enabled
	// Propagate via UpdateConfig
	controller.UpdateConfig(newCfg)
	// Verify the config handler now returns the updated config
	commandResponse = controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandConfig})
	if !commandResponse.Success {
		t.Fatalf("Expected success=true after update, got %v", commandResponse.Success)
	}

	updatedCfg, initialCfgOk := commandResponse.Data.(*config.Config)
	if !initialCfgOk || updatedCfg == nil {
		t.Fatal("Expected valid updated config")
	}

	if updatedCfg.Hints.Enabled != newCfg.Hints.Enabled {
		t.Errorf("Expected Hints.Enabled=%v after UpdateConfig, got %v",
			newCfg.Hints.Enabled, updatedCfg.Hints.Enabled)
	}
}

// TestIPCController_HandleConfigSet_GridLabelsStayResolved pins that a runtime
// change to the grid labels leaves the config holding the labels in use rather
// than the string as typed. The config carries the resolved form since the
// option stopped being inferred by its consumers, and a raw value slipped in
// here reads as a change on every subsequent reload — rebuilding the grid and
// discarding whatever coordinate the user was part-way through typing.
func TestIPCController_HandleConfigSet_GridLabelsStayResolved(t *testing.T) {
	controller := newTestController()

	ctx := context.Background()

	commandResponse := controller.HandleCommand(ctx, ipc.Command{
		Action: domain.CommandConfigSet,
		Args:   []string{"grid.row_labels", "xy"},
	})

	if !commandResponse.Success {
		t.Fatalf("Expected success=true, got %v: %s",
			commandResponse.Success, commandResponse.Message)
	}

	cfgResp := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandConfig})

	cfg, ok := cfgResp.Data.(*config.Config)
	if !ok || cfg == nil {
		t.Fatal("Failed to read config after set")
	}

	if cfg.Grid.RowLabels != "XY" {
		t.Errorf("Expected grid.row_labels=%q, got %q", "XY", cfg.Grid.RowLabels)
	}
}

// setConfigField runs one `neru config set` through the controller and hands
// back the config the daemon is left holding.
func setConfigField(t *testing.T, controller *ipcctrl.Controller, args ...string) *config.Config {
	t.Helper()

	ctx := context.Background()

	setResp := controller.HandleCommand(ctx, ipc.Command{
		Action: domain.CommandConfigSet,
		Args:   args,
	})
	if !setResp.Success {
		t.Fatalf("config set %v failed: %s", args, setResp.Message)
	}

	cfgResp := controller.HandleCommand(ctx, ipc.Command{Action: domain.CommandConfig})

	cfg, ok := cfgResp.Data.(*config.Config)
	if !ok || cfg == nil {
		t.Fatal("Failed to read config after set")
	}

	return cfg
}

// TestIPCController_HandleConfigSet_RelabelsGridFromNewCharacters is the
// acceptance for a derived value whose *source* changed. The labels are
// inferred from the characters when nobody wrote them, and a settled label
// reads exactly like one somebody typed — so applying the change to the
// resolved config found the labels filled in and left the grid drawing an
// alphabet from a character set it no longer uses.
func TestIPCController_HandleConfigSet_RelabelsGridFromNewCharacters(t *testing.T) {
	cfg := setConfigField(t, newTestController(), "grid.characters", testGridCharsToSet)

	if cfg.Grid.RowLabels != testGridLabelsFromChars {
		t.Errorf("Expected grid.row_labels=%q, got %q", testGridLabelsFromChars, cfg.Grid.RowLabels)
	}

	if cfg.Grid.ColLabels != testGridLabelsFromChars {
		t.Errorf("Expected grid.col_labels=%q, got %q", testGridLabelsFromChars, cfg.Grid.ColLabels)
	}
}

// TestIPCController_HandleConfigSet_RecoloursFromTheme is the same class one
// derived value over: the component colors are derived from [theme], so a
// theme change has to reach them.
func TestIPCController_HandleConfigSet_RecoloursFromTheme(t *testing.T) {
	cfg := setConfigField(t, newTestController(), "theme.light.surface", "#123456")

	if got := cfg.Hints.UI.BackgroundColor.Light; got != "#F2123456" {
		t.Errorf("Expected hints background %q, got %q", "#F2123456", got)
	}
}

// TestIPCController_HandleConfigSet_NoReloadRelabelsGrid covers the batching
// path, which persists without reconfiguring. Each change in a batch derives
// from the last one's written half, so the second set below has to see the
// first one's characters and not the labels it produced.
func TestIPCController_HandleConfigSet_NoReloadRelabelsGrid(t *testing.T) {
	controller := newTestController()

	setConfigField(t, controller, "grid.characters", testGridCharsToSet, "--no-reload")

	cfg := setConfigField(t, controller, "grid.hide_unmatched", "true", "--no-reload")

	if cfg.Grid.RowLabels != testGridLabelsFromChars {
		t.Errorf("Expected grid.row_labels=%q, got %q", testGridLabelsFromChars, cfg.Grid.RowLabels)
	}
}

// TestIPCController_HandleConfigSet_KeepsWrittenGridLabels guards the other
// direction: re-deriving fills the option in, it does not take it back from a
// user who set it.
func TestIPCController_HandleConfigSet_KeepsWrittenGridLabels(t *testing.T) {
	controller := newTestController()

	setConfigField(t, controller, "grid.row_labels", "xy")

	cfg := setConfigField(t, controller, "grid.characters", testGridCharsToSet)

	if cfg.Grid.RowLabels != "XY" {
		t.Errorf("Expected grid.row_labels=%q, got %q", "XY", cfg.Grid.RowLabels)
	}

	if cfg.Grid.ColLabels != testGridLabelsFromChars {
		t.Errorf("Expected grid.col_labels=%q, got %q", testGridLabelsFromChars, cfg.Grid.ColLabels)
	}
}

// TestIPCController_HandleConfigSet_RekeysSubgridFromNewCharacters covers the
// third derived value: the subgrid keys are the characters the grid is labeled
// with when nobody wrote them, so changing the characters has to move the
// subgrid with the grid rather than leave it on the set the grid no longer uses.
func TestIPCController_HandleConfigSet_RekeysSubgridFromNewCharacters(t *testing.T) {
	controller := newTestController()

	setConfigField(t, controller, "grid.sublayer_keys", "")

	cfg := setConfigField(t, controller, "grid.characters", testGridCharsToSet)

	if cfg.Grid.SublayerKeys != testGridLabelsFromChars {
		t.Errorf(
			"Expected grid.sublayer_keys=%q, got %q",
			testGridLabelsFromChars, cfg.Grid.SublayerKeys,
		)
	}
}

// TestIPCController_HandleConfigSet_KeepsWrittenSublayerKeys guards the other
// direction, the way the grid labels are guarded: keys the user set stay set
// when the characters they would otherwise be inferred from change.
func TestIPCController_HandleConfigSet_KeepsWrittenSublayerKeys(t *testing.T) {
	controller := newTestController()

	setConfigField(t, controller, "grid.sublayer_keys", "uiop")

	cfg := setConfigField(t, controller, "grid.characters", testGridCharsToSet)

	if cfg.Grid.SublayerKeys != "uiop" {
		t.Errorf("Expected grid.sublayer_keys=%q, got %q", "uiop", cfg.Grid.SublayerKeys)
	}
}
