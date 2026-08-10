package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/logger"
	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/ports/mocks"
)

func TestScrollService_Scroll(t *testing.T) {
	tests := []struct {
		name         string
		direction    services.ScrollDirection
		amount       services.ScrollAmount
		stepOverride int
		setupMocks   func(*mocks.MockAccessibilityPort)
		setupConfig  func(*config.ScrollConfig)
		wantErr      bool
	}{
		{
			name:      "scroll down char",
			direction: services.ScrollDirectionDown,
			amount:    services.ScrollAmountChar,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int, _ action.Modifiers) error {
					if deltaY >= 0 {
						t.Errorf("Expected negative deltaY for scroll down, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "scroll up char",
			direction: services.ScrollDirectionUp,
			amount:    services.ScrollAmountChar,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int, _ action.Modifiers) error {
					if deltaY <= 0 {
						t.Errorf("Expected positive deltaY for scroll up, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "scroll down full",
			direction: services.ScrollDirectionDown,
			amount:    services.ScrollAmountHalfPage,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int, _ action.Modifiers) error {
					// Down = negative deltaY
					if deltaY >= 0 {
						t.Errorf("Expected negative deltaY for scroll down, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "scroll up full",
			direction: services.ScrollDirectionUp,
			amount:    services.ScrollAmountHalfPage,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int, _ action.Modifiers) error {
					// Up = positive deltaY
					if deltaY <= 0 {
						t.Errorf("Expected positive deltaY for scroll up, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "scroll left char",
			direction: services.ScrollDirectionLeft,
			amount:    services.ScrollAmountChar,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, _ int, _ action.Modifiers) error {
					// Left = positive deltaX
					if deltaX <= 0 {
						t.Errorf("Expected positive deltaX for scroll left, got %d", deltaX)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "scroll right char",
			direction: services.ScrollDirectionRight,
			amount:    services.ScrollAmountChar,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, _ int, _ action.Modifiers) error {
					// Right = negative deltaX
					if deltaX >= 0 {
						t.Errorf("Expected negative deltaX for scroll right, got %d", deltaX)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "accessibility error",
			direction: services.ScrollDirectionDown,
			amount:    services.ScrollAmountChar,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, _ int, _ action.Modifiers) error {
					return derrors.New(
						derrors.CodeAccessibilityFailed,
						"scroll permission denied",
					)
				}
			},
			wantErr: true,
		},
		{
			name:         "step override down",
			direction:    services.ScrollDirectionDown,
			amount:       services.ScrollAmountChar,
			stepOverride: 99,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int, _ action.Modifiers) error {
					if deltaY != -99 {
						t.Errorf("Expected deltaY -99 for step override down, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:         "step override up",
			direction:    services.ScrollDirectionUp,
			amount:       services.ScrollAmountChar,
			stepOverride: 77,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int, _ action.Modifiers) error {
					if deltaY != 77 {
						t.Errorf("Expected deltaY 77 for step override up, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:         "step override left",
			direction:    services.ScrollDirectionLeft,
			amount:       services.ScrollAmountChar,
			stepOverride: 42,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, _ int, _ action.Modifiers) error {
					if deltaX != 42 {
						t.Errorf("Expected deltaX 42 for step override left, got %d", deltaX)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:         "step override right",
			direction:    services.ScrollDirectionRight,
			amount:       services.ScrollAmountChar,
			stepOverride: 33,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, _ int, _ action.Modifiers) error {
					if deltaX != -33 {
						t.Errorf("Expected deltaX -33 for step override right, got %d", deltaX)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:         "step override takes precedence over half page config",
			direction:    services.ScrollDirectionDown,
			amount:       services.ScrollAmountHalfPage,
			stepOverride: 5,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int, _ action.Modifiers) error {
					// Should use stepOverride (5), not config.ScrollStepHalf (30)
					if deltaY != -5 {
						t.Errorf("Expected deltaY -5 for step override, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:         "step override takes precedence over full page config",
			direction:    services.ScrollDirectionUp,
			amount:       services.ScrollAmountEnd,
			stepOverride: 8,
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int, _ action.Modifiers) error {
					// Should use stepOverride (8), not config.ScrollStepFull (50)
					if deltaY != 8 {
						t.Errorf("Expected deltaY 8 for step override, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "invert scroll down becomes up",
			direction: services.ScrollDirectionDown,
			amount:    services.ScrollAmountChar,
			setupConfig: func(c *config.ScrollConfig) {
				c.InvertScroll = true
			},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int, _ action.Modifiers) error {
					if deltaY <= 0 {
						t.Errorf("Expected positive deltaY (inverted from down), got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "invert scroll up becomes down",
			direction: services.ScrollDirectionUp,
			amount:    services.ScrollAmountChar,
			setupConfig: func(c *config.ScrollConfig) {
				c.InvertScroll = true
			},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, _, deltaY int, _ action.Modifiers) error {
					if deltaY >= 0 {
						t.Errorf("Expected negative deltaY (inverted from up), got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "invert scroll left becomes right",
			direction: services.ScrollDirectionLeft,
			amount:    services.ScrollAmountChar,
			setupConfig: func(c *config.ScrollConfig) {
				c.InvertScroll = true
			},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, _ int, _ action.Modifiers) error {
					if deltaX >= 0 {
						t.Errorf("Expected negative deltaX (inverted from left), got %d", deltaX)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "invert scroll right becomes left",
			direction: services.ScrollDirectionRight,
			amount:    services.ScrollAmountChar,
			setupConfig: func(c *config.ScrollConfig) {
				c.InvertScroll = true
			},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.ScrollFunc = func(_ context.Context, deltaX, _ int, _ action.Modifiers) error {
					if deltaX <= 0 {
						t.Errorf("Expected positive deltaX (inverted from right), got %d", deltaX)
					}

					return nil
				}
			},
			wantErr: false,
		},
		{
			name:      "scroll down with app override",
			direction: services.ScrollDirectionDown,
			amount:    services.ScrollAmountChar,
			setupConfig: func(c *config.ScrollConfig) {
				step := 25
				half := 200
				full := 1000
				c.AppConfigs = []config.AppConfig{
					{
						BundleID:       "com.apple.Safari",
						ScrollStep:     &step,
						ScrollStepHalf: &half,
						ScrollStepFull: &full,
					},
				}
			},
			setupMocks: func(acc *mocks.MockAccessibilityPort) {
				acc.FocusedAppBundleIDFunc = func(_ context.Context) (string, error) {
					return "com.apple.Safari", nil
				}
				acc.ScrollFunc = func(_ context.Context, _, deltaY int, _ action.Modifiers) error {
					// Safari scroll_step is overridden to 25, so Down is -25
					if deltaY != -25 {
						t.Errorf("Expected deltaY -25 for Safari app override, got %d", deltaY)
					}

					return nil
				}
			},
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			cfg := config.ScrollConfig{
				ScrollStep:     10,
				ScrollStepHalf: 30,
				ScrollStepFull: 50,
			}
			logger := logger.Get()

			if testCase.setupConfig != nil {
				testCase.setupConfig(&cfg)
			}

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockAcc)
			}

			service := services.NewScrollService(
				mockAcc,
				&mocks.MockSystemPort{},
				cfg,
				logger,
			)
			ctx := context.Background()

			scrollErr := service.Scroll(
				ctx,
				testCase.direction,
				testCase.amount,
				testCase.stepOverride,
				0,
			)

			if (scrollErr != nil) != testCase.wantErr {
				t.Errorf("Scroll() error = %v, wantErr %v", scrollErr, testCase.wantErr)
			}
		})
	}
}

func TestScrollService_UpdateConfig(t *testing.T) {
	var gotDeltaY int

	mockAcc := &mocks.MockAccessibilityPort{
		ScrollFunc: func(_ context.Context, _, deltaY int, _ action.Modifiers) error {
			gotDeltaY = deltaY

			return nil
		},
	}
	log := logger.Get()

	service := services.NewScrollService(
		mockAcc,
		&mocks.MockSystemPort{},
		config.ScrollConfig{ScrollStep: 50, ScrollStepFull: 1000},
		log,
	)

	ctx := context.Background()

	// Baseline: a normal-amount scroll uses the configured step.
	err := service.Scroll(ctx, services.ScrollDirectionDown, services.ScrollAmountChar, 0, 0)
	if err != nil {
		t.Fatalf("Scroll() error = %v, want nil", err)
	}

	baseline := gotDeltaY
	if baseline == 0 {
		t.Fatal("Scroll() produced a zero delta; the test cannot detect a config change")
	}

	// UpdateConfig must change what a subsequent scroll actually emits — the
	// point of the call, and what this test previously never checked.
	service.UpdateConfig(config.ScrollConfig{ScrollStep: 100, ScrollStepFull: 2000})

	err = service.Scroll(ctx, services.ScrollDirectionDown, services.ScrollAmountChar, 0, 0)
	if err != nil {
		t.Fatalf("Scroll() after UpdateConfig error = %v, want nil", err)
	}

	if gotDeltaY == baseline {
		t.Errorf(
			"Scroll() delta = %d both before and after UpdateConfig; the new "+
				"ScrollStep was not applied",
			gotDeltaY,
		)
	}
}

// settlingSystemPort is a MockSystemPort that also implements
// ports.CursorSettler, recording settle calls so tests can pin ordering
// against the scroll itself.
type settlingSystemPort struct {
	mocks.MockSystemPort

	settleCalls int
	settleErr   error
}

func (s *settlingSystemPort) SettleCursor(_ context.Context) error {
	s.settleCalls++

	return s.settleErr
}

// TestScrollService_Scroll_SettlesCursorBeforeScroll pins the interleaving fix
// for scrolls fired during an animated relative cursor move: the in-flight
// animation must be settled before the scroll is dispatched, so the scroll
// anchors to the window the user aimed for rather than one the cursor was
// gliding over mid-animation.
func TestScrollService_Scroll_SettlesCursorBeforeScroll(t *testing.T) {
	system := &settlingSystemPort{}

	settledAtScrollTime := -1
	mockAcc := &mocks.MockAccessibilityPort{
		ScrollFunc: func(_ context.Context, _, _ int, _ action.Modifiers) error {
			settledAtScrollTime = system.settleCalls

			return nil
		},
	}

	service := services.NewScrollService(
		mockAcc,
		system,
		config.ScrollConfig{ScrollStep: 10, ScrollStepHalf: 30, ScrollStepFull: 50},
		logger.Get(),
	)

	err := service.Scroll(
		context.Background(),
		services.ScrollDirectionDown,
		services.ScrollAmountChar,
		0,
		0,
	)
	if err != nil {
		t.Fatalf("Scroll() error = %v, want nil", err)
	}

	if settledAtScrollTime != 1 {
		t.Errorf(
			"settle calls observed at scroll time = %d, want 1 (settle must run before the scroll)",
			settledAtScrollTime,
		)
	}
}

// TestScrollService_Scroll_SettleErrorDoesNotBlockScroll pins that a failing
// settle degrades to the pre-settle behavior — the scroll still runs — rather
// than turning a cosmetic animation problem into a lost user action.
func TestScrollService_Scroll_SettleErrorDoesNotBlockScroll(t *testing.T) {
	system := &settlingSystemPort{
		settleErr: derrors.New(derrors.CodeActionFailed, "settle failed"),
	}

	scrolled := false
	mockAcc := &mocks.MockAccessibilityPort{
		ScrollFunc: func(_ context.Context, _, _ int, _ action.Modifiers) error {
			scrolled = true

			return nil
		},
	}

	service := services.NewScrollService(
		mockAcc,
		system,
		config.ScrollConfig{ScrollStep: 10, ScrollStepHalf: 30, ScrollStepFull: 50},
		logger.Get(),
	)

	err := service.Scroll(
		context.Background(),
		services.ScrollDirectionDown,
		services.ScrollAmountChar,
		0,
		0,
	)
	if err != nil {
		t.Fatalf("Scroll() error = %v, want nil (settle failure must not fail the scroll)", err)
	}

	if !scrolled {
		t.Error("Scroll() never reached the accessibility port after a settle error")
	}
}

// TestScrollService_Health pins that scroll reports only the dependency it has.
// Scrolling never draws, so the service holds no overlay port and must not
// claim to have checked one.
func TestScrollService_Health(t *testing.T) {
	accessibilityDown := derrors.New(derrors.CodeNotSupported, "accessibility unavailable")

	tests := []struct {
		name             string
		accessibilityErr error
	}{
		{name: "healthy accessibility", accessibilityErr: nil},
		{name: "unhealthy accessibility", accessibilityErr: accessibilityDown},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{
				HealthFunc: func(_ context.Context) error {
					return testCase.accessibilityErr
				},
			}

			service := services.NewScrollService(
				mockAcc,
				&mocks.MockSystemPort{},
				config.ScrollConfig{},
				logger.Get(),
			)

			checks := service.Health(context.Background())

			if len(checks) != 1 {
				t.Fatalf(
					"Health() reported %d checks, want 1 (accessibility only)",
					len(checks),
				)
			}

			got, ok := checks["accessibility"]
			if !ok {
				t.Fatal(`Health() has no "accessibility" check`)
			}

			if !errors.Is(got, testCase.accessibilityErr) {
				t.Errorf(
					"Health()[accessibility] = %v, want %v",
					got,
					testCase.accessibilityErr,
				)
			}
		})
	}
}

// TestScrollService_Scroll_KeepsNotSupportedCode pins that a backend's refusal
// survives the service. derrors matches only the outermost code, so wrapping a
// CodeNotSupported as CodeActionFailed here would turn "this session cannot
// hold that modifier" into a generic failure by the time it reaches the reply.
func TestScrollService_Scroll_KeepsNotSupportedCode(t *testing.T) {
	refusal := derrors.New(derrors.CodeNotSupported, "no backend to press a modifier through")

	mockAcc := &mocks.MockAccessibilityPort{
		ScrollFunc: func(_ context.Context, _, _ int, _ action.Modifiers) error {
			return refusal
		},
	}

	service := services.NewScrollService(
		mockAcc,
		&mocks.MockSystemPort{},
		config.ScrollConfig{ScrollStep: 10, ScrollStepHalf: 30, ScrollStepFull: 50},
		logger.Get(),
	)

	err := service.Scroll(
		context.Background(),
		services.ScrollDirectionDown,
		services.ScrollAmountChar,
		0,
		action.ModCtrl,
	)
	if err == nil {
		t.Fatal("Scroll() returned nil for a refused modifier")
	}

	if !derrors.IsNotSupported(err) {
		t.Errorf("Scroll() error = %v, want it to still report CodeNotSupported", err)
	}
}

// TestScrollService_Scroll_ForwardsModifiers pins that the modifier set named
// on the action reaches the accessibility port unchanged. A scroll that loses
// its ctrl pans where the user asked for a zoom, so dropping the set silently
// is what this test exists to catch.
func TestScrollService_Scroll_ForwardsModifiers(t *testing.T) {
	tests := []struct {
		name      string
		modifiers action.Modifiers
	}{
		{name: "no modifiers", modifiers: 0},
		{name: "single modifier", modifiers: action.ModCtrl},
		{name: "combined modifiers", modifiers: action.ModCtrl | action.ModShift},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var got action.Modifiers

			mockAcc := &mocks.MockAccessibilityPort{
				ScrollFunc: func(_ context.Context, _, _ int, modifiers action.Modifiers) error {
					got = modifiers

					return nil
				},
			}

			service := services.NewScrollService(
				mockAcc,
				&mocks.MockSystemPort{},
				config.ScrollConfig{ScrollStep: 10, ScrollStepHalf: 30, ScrollStepFull: 50},
				logger.Get(),
			)

			err := service.Scroll(
				context.Background(),
				services.ScrollDirectionDown,
				services.ScrollAmountChar,
				0,
				testCase.modifiers,
			)
			if err != nil {
				t.Fatalf("Scroll() error = %v, want nil", err)
			}

			if got != testCase.modifiers {
				t.Errorf("port received modifiers %q, want %q", got, testCase.modifiers)
			}
		})
	}
}
