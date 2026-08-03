//go:build integration && darwin

package services_test

import (
	"context"
	"fmt"
	"image"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/accessibility"
	"github.com/y3owk1n/neru/internal/adapter/accessibility/native"
	"github.com/y3owk1n/neru/internal/adapter/logger"
	overlayAdapter "github.com/y3owk1n/neru/internal/adapter/overlay"
	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/ports"
)

// realAdapterTimeout bounds how long these tests wait on the real system.
//
// They drive live accessibility APIs against whatever window happens to be
// focused, and that call can take arbitrarily long — with the screen locked,
// where the focused "app" is the login window, it does not return at all.
const realAdapterTimeout = 30 * time.Second

// startAdapterWatchdog ends the test run with a diagnosis if a real system
// call never comes back. Each subtest starts its own: a test function holds
// several, and one budget shared across them would eventually blame a locked
// screen for what is only a slow machine. Setup outside a subtest is left to
// the go test -timeout backstop.
//
// A context deadline cannot do this job: several adapters take a context and
// discard it, and the ones that honor it check it only before starting work,
// so nothing interrupts a call already blocked inside the platform API.
// Panicking is what turns an indefinite hang into something a reader can act
// on — the message names the likely cause, and the stack dump that comes with
// it names the call that is stuck.
func startAdapterWatchdog(t *testing.T) {
	t.Helper()

	finished := make(chan struct{})
	t.Cleanup(func() { close(finished) })

	name := t.Name()

	go func() {
		select {
		case <-finished:
		case <-time.After(realAdapterTimeout):
			panic(fmt.Sprintf(
				"%s: a real system adapter did not answer within %s. "+
					"This usually means the screen is locked, or the focused app is not "+
					"responding to accessibility queries, rather than a fault in the code "+
					"under test. The stack below shows which call is blocked.",
				name,
				realAdapterTimeout,
			))
		}
	}()
}

// testThemeProvider is a simple ThemeProvider mock for integration tests.
type testThemeProvider struct {
	darkMode bool
}

func (t *testThemeProvider) IsDarkMode() bool {
	return t.darkMode
}

// TestHintServiceIntegration tests the hint service with real adapters.
func TestHintServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	// Create real adapters (they will be initialized with real infra)
	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true

	// Initialize real adapters like the app does
	accAdapter, overlay, systemPort := initializeRealAdapters(t, cfg, logger)

	// Create hint generator
	hintGen, err := hint.NewAlphabetGenerator(
		cfg.Hints.HintCharacters,
		hint.LabelDirectionFromString(cfg.Hints.LabelDirectionForApp("")),
	)
	if err != nil {
		t.Fatalf("Failed to create hint generator: %v", err)
	}

	// Create hint service
	hintService := services.NewHintService(
		accAdapter,
		overlay,
		systemPort,
		hintGen,
		cfg.Hints,
		logger,
		nil,
	)

	ctx := context.Background()

	t.Run("ShowHints integration", func(t *testing.T) {
		startAdapterWatchdog(t)

		// This tests the full pipeline: accessibility -> hint generation -> overlay.
		// Zero hints is a legitimate outcome (the machine may have no clickable
		// elements on screen), but an error is not.
		hints, err := hintService.ShowHints(ctx, nil, nil)
		if err != nil {
			t.Fatalf("ShowHints failed: %v", err)
		}

		// Whatever the pipeline produced must be individually addressable:
		// every hint needs a non-empty, unique label and a real element, or the
		// user cannot reach it by keyboard.
		seen := make(map[string]int, len(hints))
		for idx, generated := range hints {
			if generated == nil {
				t.Fatalf("hint %d is nil", idx)
			}

			if generated.Label() == "" {
				t.Errorf("hint %d has an empty label", idx)
			}

			if prev, dup := seen[generated.Label()]; dup {
				t.Errorf("hints %d and %d share label %q", prev, idx, generated.Label())
			}

			seen[generated.Label()] = idx

			if generated.Element() == nil {
				t.Errorf("hint %d (%q) has no element", idx, generated.Label())
			}
		}
	})

	t.Run("ShowHints leaves NoOpManager-backed overlay idle", func(t *testing.T) {
		startAdapterWatchdog(t)

		// initializeRealAdapters wires a NoOpManager whose Mode() is pinned to
		// ModeIdle, and Adapter.IsVisible is defined as Mode() != idle. So the
		// adapter must report not-visible no matter what was shown. If this
		// starts failing, IsVisible has stopped delegating to the manager.
		_, err := hintService.ShowHints(ctx, nil, nil)
		if err != nil {
			t.Fatalf("ShowHints failed: %v", err)
		}

		if overlay.IsVisible() {
			t.Error("overlay reported visible while backed by NoOpManager")
		}
	})

	t.Run("HideHints integration", func(t *testing.T) {
		startAdapterWatchdog(t)

		err := hintService.HideHints(ctx)
		if err != nil {
			t.Fatalf("HideHints failed: %v", err)
		}

		if overlay.IsVisible() {
			t.Error("overlay still reports visible after HideHints")
		}

		// Hiding an already-hidden overlay must stay a no-op rather than
		// erroring, since mode exit paths can call it more than once.
		err = hintService.HideHints(ctx)
		if err != nil {
			t.Errorf("second HideHints failed, hide is not idempotent: %v", err)
		}
	})

	t.Run("Service health check", func(t *testing.T) {
		startAdapterWatchdog(t)

		health := hintService.Health(ctx)
		if health == nil {
			t.Fatal("Health returned nil map")
		}

		// Permissions were verified during setup, so every component the
		// service reports on is expected to be healthy here.
		for _, component := range []string{"accessibility", "overlay"} {
			err, present := health[component]
			if !present {
				t.Errorf("Health is missing the %q component: got keys %v", component, health)

				continue
			}

			if err != nil {
				t.Errorf("component %q reported unhealthy: %v", component, err)
			}
		}
	})
}

// TestActionServiceIntegration tests the action service with real adapters.
func TestActionServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	cfg := config.DefaultConfig()

	accAdapter, overlayAdapter, systemPort := initializeRealAdapters(t, cfg, logger)

	actionService := services.NewActionService(
		accAdapter,
		overlayAdapter,
		systemPort,
		logger,
	)

	ctx := context.Background()

	t.Run("PerformActionAtPoint left click", func(t *testing.T) {
		startAdapterWatchdog(t)

		err := actionService.PerformActionAtPoint(ctx, "left_click", image.Point{X: 100, Y: 100}, 0)
		if err != nil {
			t.Fatalf("PerformActionAtPoint(left_click) failed: %v", err)
		}
	})

	t.Run("PerformActionAtPoint right click", func(t *testing.T) {
		startAdapterWatchdog(t)

		err := actionService.PerformActionAtPoint(
			ctx,
			"right_click",
			image.Point{X: 200, Y: 200},
			0,
		)
		if err != nil {
			t.Fatalf("PerformActionAtPoint(right_click) failed: %v", err)
		}
	})

	t.Run("PerformActionAtPoint rejects an unknown action", func(t *testing.T) {
		startAdapterWatchdog(t)

		// The action string is parsed before any native call, so an unknown
		// name must be reported rather than silently no-op'ing.
		err := actionService.PerformActionAtPoint(
			ctx,
			"definitely_not_an_action",
			image.Point{X: 10, Y: 10},
			0,
		)
		if err == nil {
			t.Fatal("expected an error for an unknown action name, got nil")
		}

		if code := derrors.GetCode(err); code != derrors.CodeInvalidConfig {
			t.Errorf("unknown action: got code %q, want %q", code, derrors.CodeInvalidConfig)
		}
	})

	// The exact-placement contract for cursor movement — the global
	// top-left-origin, Y-down guarantee that a Y-flip regression would break —
	// is asserted in internal/adapter/accessibility, which owns cursor
	// movement. It is deliberately not repeated here: both packages would be
	// driving the one physical cursor, and `go test ./...` runs them
	// concurrently, so each would intermittently fail the other.

	t.Run("ExecuteAction on element", func(t *testing.T) {
		startAdapterWatchdog(t)

		// This tests the element-based action execution
		// Create a mock element using the constructor
		testElement, err := element.NewElement(
			element.ID("test-button"),
			image.Rect(50, 50, 150, 80),
			element.RoleButton,
		)
		if err != nil {
			t.Fatalf("Failed to create test element: %v", err)
		}

		err = actionService.ExecuteAction(ctx, testElement, action.TypeLeftClick)
		if err != nil {
			t.Fatalf("ExecuteAction failed: %v", err)
		}
	})
}

// TestGridServiceIntegration tests the grid service with real adapters.
func TestGridServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	cfg := config.DefaultConfig()
	cfg.Grid.Enabled = true

	// Initialize real adapters
	_, overlay, systemPort := initializeRealAdapters(t, cfg, logger)

	// Create grid service
	gridService := services.NewGridService(overlay, systemPort, logger)

	ctx := context.Background()

	t.Run("ShowGrid integration", func(t *testing.T) {
		startAdapterWatchdog(t)

		err := gridService.ShowGrid(ctx)
		if err != nil {
			t.Fatalf("ShowGrid failed: %v", err)
		}

		// Re-showing must stay safe: the grid mode re-renders on screen change
		// and monitor moves without hiding first.
		err = gridService.ShowGrid(ctx)
		if err != nil {
			t.Errorf("second ShowGrid failed, show is not idempotent: %v", err)
		}
	})

	t.Run("HideGrid integration", func(t *testing.T) {
		startAdapterWatchdog(t)

		err := gridService.HideGrid(ctx)
		if err != nil {
			t.Fatalf("HideGrid failed: %v", err)
		}

		if overlay.IsVisible() {
			t.Error("overlay still reports visible after HideGrid")
		}

		err = gridService.HideGrid(ctx)
		if err != nil {
			t.Errorf("second HideGrid failed, hide is not idempotent: %v", err)
		}
	})

	t.Run("Grid health check", func(t *testing.T) {
		startAdapterWatchdog(t)

		health := gridService.Health(ctx)
		if health == nil {
			t.Fatal("Health returned nil map")
		}

		for component, err := range health {
			if err != nil {
				t.Errorf("component %q reported unhealthy: %v", component, err)
			}
		}
	})
}

// TestScrollServiceIntegration tests the scroll service with real adapters.
func TestScrollServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := logger.Get()

	cfg := config.DefaultConfig()

	// Initialize real adapters
	accAdapter, overlay, systemPort := initializeRealAdapters(t, cfg, logger)

	// Create scroll service
	scrollService := services.NewScrollService(accAdapter, overlay, systemPort, cfg.Scroll, logger)

	ctx := context.Background()

	t.Run("Scroll integration", func(t *testing.T) {
		startAdapterWatchdog(t)

		// Every direction/amount pair must reach the native scroll API without
		// error, not just the one combination that happened to be wired up.
		for _, dir := range []services.ScrollDirection{
			services.ScrollDirectionUp,
			services.ScrollDirectionDown,
			services.ScrollDirectionLeft,
			services.ScrollDirectionRight,
		} {
			for _, amount := range []services.ScrollAmount{
				services.ScrollAmountChar,
				services.ScrollAmountHalfPage,
				services.ScrollAmountEnd,
			} {
				err := scrollService.Scroll(ctx, dir, amount, 0)
				if err != nil {
					t.Errorf("Scroll(dir=%d, amount=%d) failed: %v", dir, amount, err)
				}
			}
		}
	})

	t.Run("Scroll honors a step override", func(t *testing.T) {
		startAdapterWatchdog(t)

		err := scrollService.Scroll(
			ctx,
			services.ScrollDirectionDown,
			services.ScrollAmountChar,
			7,
		)
		if err != nil {
			t.Fatalf("Scroll with step override failed: %v", err)
		}
	})

	t.Run("SetInvertScroll round-trips", func(t *testing.T) {
		startAdapterWatchdog(t)

		original := scrollService.IsScrollInverted()
		t.Cleanup(func() { scrollService.SetInvertScroll(original) })

		scrollService.SetInvertScroll(!original)

		if got := scrollService.IsScrollInverted(); got != !original {
			t.Fatalf(
				"IsScrollInverted after SetInvertScroll(%t) = %t, want %t",
				!original,
				got,
				!original,
			)
		}

		// Inversion must not break the native path.
		err := scrollService.Scroll(
			ctx,
			services.ScrollDirectionDown,
			services.ScrollAmountHalfPage,
			0,
		)
		if err != nil {
			t.Errorf("Scroll with inverted direction failed: %v", err)
		}
	})

	t.Run("Hide integration", func(t *testing.T) {
		startAdapterWatchdog(t)

		err := scrollService.Hide(ctx)
		if err != nil {
			t.Fatalf("Hide failed: %v", err)
		}

		if overlay.IsVisible() {
			t.Error("overlay still reports visible after scroll Hide")
		}
	})
}

// Helper function to initialize real adapters like the app does.
func initializeRealAdapters(
	t *testing.T,
	cfg *config.Config,
	logger *zap.Logger,
) (ports.AccessibilityPort, ports.OverlayPort, ports.SystemPort) {
	t.Helper()

	// Create infrastructure client
	axClient := native.New(logger, nil)

	// Create base accessibility adapter
	accAdapter := accessibility.NewAdapter(
		logger,
		cfg.General.ExcludedApps,
		cfg.Hints.ClickableRoles,
		axClient,
		cfg.Hints.DetectMissionControl,
	)

	// Initialize overlay manager (always use no-op for integration tests)
	overlayManager := &overlayAdapter.NoOpManager{}

	// Initialize system port
	systemPort, err := platform.NewSystemPort()
	if err != nil {
		t.Fatalf("Failed to create system port: %v", err)
	}

	// Accessibility permission is the one legitimate reason these tests cannot
	// run. Gate on it once, explicitly, so that every assertion below is
	// allowed to be strict: from here on any error is a real failure.
	permErr := systemPort.CheckPermissions(context.Background())
	if permErr != nil {
		t.Skipf("accessibility permission not granted, cannot run integration test: %v", permErr)
	}

	// Create overlay adapter
	theme := &testThemeProvider{darkMode: false}
	overlay := overlayAdapter.NewAdapter(overlayManager, theme, systemPort, logger)

	return accAdapter, overlay, systemPort
}
