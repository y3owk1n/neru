package ipcctrl_test

import (
	"context"
	"errors"
	"image"
	"reflect"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/app/ipcctrl"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/state"
	"github.com/y3owk1n/neru/internal/ports"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

const (
	builtInName  = "Built-in Display"
	externalName = "DELL U2720Q"
)

// errCompositorSilent is a query that failed this time, on a platform that can
// answer it.
var errCompositorSilent = errors.New("compositor did not answer")

// twoDisplays is a laptop with an external monitor to its right.
func twoDisplays() []ports.Screen {
	return []ports.Screen{
		{Name: builtInName, Bounds: image.Rect(0, 0, 1512, 982)},
		{Name: externalName, Bounds: image.Rect(1512, 0, 4072, 1440)},
	}
}

// cachingSystem is a platform that caches the cursor client-side, as Wayland
// does, and animates it, as macOS does. It records which of the two a query
// asked for.
type cachingSystem struct {
	*portmocks.MockSystemPort

	synced  int
	settled int
}

func (s *cachingSystem) SyncCursorPosition(context.Context) error {
	s.synced++

	return nil
}

func (s *cachingSystem) SettleCursor(context.Context) error {
	s.settled++

	return nil
}

func queryHandlers(
	system ports.SystemPort,
) map[string]func(context.Context, ipc.Command) ipc.Response {
	cfg := config.DefaultConfig()
	logger := zap.NewNop()

	handler := ipcctrl.NewInfoHandler(ipcctrl.InfoHandlerDeps{
		ConfigService: loader.NewService(cfg, "", logger, nil),
		AppState:      state.NewAppState(),
		Config:        cfg,
		ActionService: services.NewActionService(
			&portmocks.MockAccessibilityPort{},
			&portmocks.MockOverlayPort{},
			system,
			logger,
		),
		System: system,
		Logger: logger,
	})

	handlers := make(map[string]func(context.Context, ipc.Command) ipc.Response)
	handler.RegisterHandlers(handlers)

	return handlers
}

func query(t *testing.T, system ports.SystemPort, action string) ipc.Response {
	t.Helper()

	handle := queryHandlers(system)[action]
	if handle == nil {
		t.Fatalf("no handler registered for %q", action)
	}

	return handle(context.Background(), ipc.Command{Action: action})
}

func TestInfoHandler_QueryDisplays_ReportsEveryDisplayWithItsScale(t *testing.T) {
	t.Parallel()

	system := &portmocks.MockSystemPort{
		ScreensFunc: func(context.Context) ([]ports.Screen, error) { return twoDisplays(), nil },
		ScreenScaleFunc: func(_ context.Context, bounds image.Rectangle) (float64, error) {
			if bounds.Min.X == 0 {
				return 2, nil
			}

			// A platform that cannot read the scale reports 1, as the port says.
			return 0, derrors.New(derrors.CodeNotSupported, "no scale here")
		},
	}

	resp := query(t, system, domain.CommandQueryDisplays)
	if !resp.Success {
		t.Fatalf("query displays failed: %s (%s)", resp.Message, resp.Code)
	}

	want := ipc.DisplaysData{Displays: []ipc.DisplayData{
		{Name: builtInName, X: 0, Y: 0, Width: 1512, Height: 982, Scale: 2},
		{Name: externalName, X: 1512, Y: 0, Width: 2560, Height: 1440, Scale: 1},
	}}
	if !reflect.DeepEqual(resp.Data, want) {
		t.Errorf("data = %+v, want %+v", resp.Data, want)
	}
}

func TestInfoHandler_QueryCursor_NamesTheDisplayHoldingIt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cursor    image.Point
		wantName  string
		wantIndex int
		wantNone  bool
	}{
		{
			name:      "on the first display",
			cursor:    image.Pt(10, 10),
			wantName:  builtInName,
			wantIndex: 0,
		},
		{
			name:      "on the second display",
			cursor:    image.Pt(2000, 700),
			wantName:  externalName,
			wantIndex: 1,
		},
		{name: "on no display", cursor: image.Pt(-50, -50), wantNone: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			system := &portmocks.MockSystemPort{
				ScreensFunc: func(context.Context) ([]ports.Screen, error) { return twoDisplays(), nil },
				CursorPositionFunc: func(context.Context) (image.Point, error) {
					return testCase.cursor, nil
				},
			}

			resp := query(t, system, domain.CommandQueryCursor)
			if !resp.Success {
				t.Fatalf("query cursor failed: %s (%s)", resp.Message, resp.Code)
			}

			data, ok := resp.Data.(ipc.CursorData)
			if !ok {
				t.Fatalf("data is %T, want ipc.CursorData", resp.Data)
			}

			if data.X != testCase.cursor.X || data.Y != testCase.cursor.Y {
				t.Errorf("position = %d,%d, want %v", data.X, data.Y, testCase.cursor)
			}

			if testCase.wantNone {
				if data.Display != nil || data.DisplayIndex != nil {
					t.Errorf("display = %v at %v, want none", data.Display, data.DisplayIndex)
				}

				return
			}

			if data.Display == nil || *data.Display != testCase.wantName {
				t.Errorf("display = %v, want %q", data.Display, testCase.wantName)
			}

			if data.DisplayIndex == nil || *data.DisplayIndex != testCase.wantIndex {
				t.Errorf("display_index = %v, want %d", data.DisplayIndex, testCase.wantIndex)
			}
		})
	}
}

// TestInfoHandler_QueryCursor_RefreshesTheCacheWithoutSettling pins that a
// query reports where a hand-moved cursor is now, and that asking never cuts
// a glide short.
func TestInfoHandler_QueryCursor_RefreshesTheCacheWithoutSettling(t *testing.T) {
	t.Parallel()

	system := &cachingSystem{MockSystemPort: &portmocks.MockSystemPort{
		ScreensFunc: func(context.Context) ([]ports.Screen, error) { return twoDisplays(), nil },
	}}

	resp := query(t, system, domain.CommandQueryCursor)
	if !resp.Success {
		t.Fatalf("query cursor failed: %s (%s)", resp.Message, resp.Code)
	}

	if system.synced != 1 {
		t.Errorf("cursor cache synced %d times, want 1", system.synced)
	}

	if system.settled != 0 {
		t.Errorf("cursor animation settled %d times, want 0", system.settled)
	}
}

// TestInfoHandler_Query_TellsUnsupportedFromFailed pins the code a script
// branches on: a platform with no way to answer says ERR_NOT_SUPPORTED, and a
// query that failed this time says ERR_ACTION_FAILED.
func TestInfoHandler_Query_TellsUnsupportedFromFailed(t *testing.T) {
	t.Parallel()

	unsupported := derrors.New(derrors.CodeNotSupported, "not on this backend")

	tests := []struct {
		name   string
		action string
		system *portmocks.MockSystemPort
		want   string
	}{
		{
			name:   "displays unsupported",
			action: domain.CommandQueryDisplays,
			system: &portmocks.MockSystemPort{
				ScreensFunc: func(context.Context) ([]ports.Screen, error) { return nil, unsupported },
			},
			want: ipc.CodeNotSupported,
		},
		{
			name:   "displays scale failed",
			action: domain.CommandQueryDisplays,
			system: &portmocks.MockSystemPort{
				ScreensFunc: func(context.Context) ([]ports.Screen, error) { return twoDisplays(), nil },
				ScreenScaleFunc: func(context.Context, image.Rectangle) (float64, error) {
					return 0, errCompositorSilent
				},
			},
			want: ipc.CodeActionFailed,
		},
		{
			name:   "cursor unsupported",
			action: domain.CommandQueryCursor,
			system: &portmocks.MockSystemPort{
				CursorPositionFunc: func(context.Context) (image.Point, error) {
					return image.Point{}, unsupported
				},
			},
			want: ipc.CodeNotSupported,
		},
		{
			name:   "cursor failed",
			action: domain.CommandQueryCursor,
			system: &portmocks.MockSystemPort{
				CursorPositionFunc: func(context.Context) (image.Point, error) {
					return image.Point{}, errCompositorSilent
				},
			},
			want: ipc.CodeActionFailed,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			resp := query(t, testCase.system, testCase.action)
			if resp.Success {
				t.Fatalf("%s succeeded, want a refusal", testCase.action)
			}

			if resp.Code != testCase.want {
				t.Errorf("code = %s, want %s (message %q)", resp.Code, testCase.want, resp.Message)
			}
		})
	}
}
