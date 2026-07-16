package services_test

import (
	"context"
	"fmt"
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/ports"
	"github.com/y3owk1n/neru/internal/core/ports/mocks"
)

func TestHintService_ShowHints(t *testing.T) {
	// Create test elements
	testElements := []*element.Element{
		mustNewElement("elem1", image.Rect(10, 10, 50, 50)),
		mustNewElement("elem2", image.Rect(60, 10, 100, 50)),
		mustNewElement("elem3", image.Rect(10, 60, 50, 100)),
	}

	tests := []struct {
		name          string
		setupMocks    func(*mocks.MockAccessibilityPort, *mocks.MockOverlayPort)
		setupGen      func() hint.Generator
		config        config.HintsConfig
		wantErr       bool
		wantHintCount int
		checkHints    func(*testing.T, []*hint.Interface)
		checkOverlay  func(*testing.T, *mocks.MockOverlayPort)
	}{
		{
			name: "successful hint display",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.ClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
					return testElements, nil
				}
			},
			setupGen: func() hint.Generator {
				gen, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)

				return gen
			},
			wantErr:       false,
			wantHintCount: 3, // We have 3 test elements
			checkHints: func(t *testing.T, hints []*hint.Interface) {
				t.Helper()

				if len(hints) != 3 {
					t.Errorf("Expected 3 hints, got %d", len(hints))

					return
				}
				// Check that hints have labels (exact labels depend on generator)
				if hints[0].Label() == "" || hints[1].Label() == "" || hints[2].Label() == "" {
					t.Error("Hints should have non-empty labels")
				}
			},
			checkOverlay: func(t *testing.T, ov *mocks.MockOverlayPort) {
				t.Helper()

				if !ov.IsVisible() {
					t.Error("Overlay should be visible after ShowHints")
				}
			},
		},
		{
			name: "no elements found",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.ClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
					return []*element.Element{}, nil
				}
			},
			setupGen: func() hint.Generator {
				gen, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)

				return gen
			},
			wantErr:       false,
			wantHintCount: 0, // No elements means no hints
			checkHints: func(t *testing.T, hints []*hint.Interface) {
				t.Helper()

				if len(hints) != 0 {
					t.Errorf("Expected 0 hints, got %d", len(hints))
				}
			},
		},
		{
			name: "accessibility error",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.ClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
					return nil, derrors.New(
						derrors.CodeAccessibilityFailed,
						"accessibility permission denied",
					)
				}
			},
			setupGen: func() hint.Generator {
				gen, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)

				return gen
			},
			wantErr:       true,
			wantHintCount: 0,
		},
		{
			name: "large element set",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.ClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
					// Create 100 elements
					elements := make([]*element.Element, 100)

					for index := range 100 {
						elem, _ := element.NewElement(
							element.ID(fmt.Sprintf("elem%d", index)),
							image.Rect(index*10, index*10, index*10+40, index*10+40),
							element.RoleButton,
						)
						elements[index] = elem
					}

					return elements, nil
				}
			},
			setupGen: func() hint.Generator {
				// Use larger character set for more hints
				gen, _ := hint.NewAlphabetGenerator(
					"abcdefghijklmnopqrstuvwxyz",
					hint.LabelDirectionReverse,
				)

				return gen
			},
			wantErr:       false,
			wantHintCount: 100,
			checkHints: func(t *testing.T, hints []*hint.Interface) {
				t.Helper()

				if len(hints) != 100 {
					t.Errorf("Expected 100 hints, got %d", len(hints))
				}
			},
		},
		{
			name: "single element",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.ClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
					return []*element.Element{testElements[0]}, nil
				}
			},
			setupGen: func() hint.Generator {
				gen, _ := hint.NewAlphabetGenerator("ab", hint.LabelDirectionReverse)

				return gen
			},
			wantErr:       false,
			wantHintCount: 1,
			checkHints: func(t *testing.T, hints []*hint.Interface) {
				t.Helper()

				if len(hints) != 1 {
					t.Errorf("Expected 1 hint, got %d", len(hints))
				}
			},
		},
		{
			name: "config-driven filtering",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.ClickableElementsFunc = func(_ context.Context, filter ports.ElementFilter) ([]*element.Element, error) {
					// Verify that config values are properly applied to filter
					if !filter.IncludeMenubar {
						t.Error("IncludeMenubar should be true based on config")
					}

					if !filter.IncludeDock {
						t.Error("IncludeDock should be true based on config")
					}

					if !filter.IncludeNotificationCenter {
						t.Error("IncludeNotificationCenter should be true based on config")
					}

					if !filter.IncludeStageManager {
						t.Error("IncludeStageManager should be true based on config")
					}

					if !filter.IncludePIP {
						t.Error("IncludePIP should be true based on config")
					}

					if !filter.IncludeScreenCapture {
						t.Error("IncludeScreenCapture should be true based on config")
					}

					return testElements, nil
				}
			},
			setupGen: func() hint.Generator {
				gen, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)

				return gen
			},
			config: config.HintsConfig{
				IncludeMenubarHints:       true,
				IncludeDockHints:          true,
				IncludeNCHints:            true,
				IncludeStageManagerHints:  true,
				IncludePIPHints:           true,
				IncludeScreenCaptureHints: true,
			},
			wantErr:       false,
			wantHintCount: 3,
		},
		{
			name: "app-specific clickable roles are applied",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.FocusedAppBundleIDFunc = func(context.Context) (string, error) {
					return "net.imput.helium", nil
				}
				acc.ClickableElementsFunc = func(_ context.Context, filter ports.ElementFilter) ([]*element.Element, error) {
					roles := make(map[element.Role]bool, len(filter.Roles))
					for _, role := range filter.Roles {
						roles[role] = true
					}

					if !roles[element.Role("AXButton")] {
						t.Error("expected global role AXButton to be present")
					}

					if !roles[element.Role("AXHeading")] {
						t.Error("expected app-specific role AXHeading to be present")
					}

					if len(roles) != 2 {
						t.Errorf("expected exactly 2 roles, got %v", filter.Roles)
					}

					return testElements, nil
				}
			},
			setupGen: func() hint.Generator {
				gen, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)

				return gen
			},
			config: config.HintsConfig{
				ClickableRoles: []string{"AXButton"},
				AppConfigs: []config.AppConfig{
					{
						BundleID:            "net.imput.helium",
						AdditionalClickable: []string{"AXHeading"},
					},
				},
			},
			wantErr:       false,
			wantHintCount: 3,
		},
		{
			name: "too many elements shows max hints without error",
			setupMocks: func(acc *mocks.MockAccessibilityPort, _ *mocks.MockOverlayPort) {
				acc.ClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
					elements := make([]*element.Element, 10)

					for index := range elements {
						elements[index] = mustNewElement(
							fmt.Sprintf("elem%d", index),
							image.Rect(index*10, index*10, index*10+40, index*10+40),
						)
					}

					return elements, nil
				}
			},
			setupGen: func() hint.Generator {
				gen, _ := hint.NewAlphabetGenerator("as", hint.LabelDirectionReverse)

				return gen
			},
			wantErr:       false,
			wantHintCount: 8,
			checkHints: func(t *testing.T, hints []*hint.Interface) {
				t.Helper()

				if len(hints) != 8 {
					t.Errorf("Expected 8 hints, got %d", len(hints))
				}
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockAcc, mockOverlay)
			}

			generator := testCase.setupGen()
			logger := logger.Get()

			service := services.NewHintService(
				mockAcc,
				mockOverlay,
				&mocks.MockSystemPort{},
				generator,
				testCase.config,
				logger,
				nil,
			)

			ctx := context.Background()

			// Act
			hints, hintsErr := service.ShowHints(ctx, nil, nil)

			// Assert
			if testCase.wantErr && hintsErr == nil {
				t.Error("ShowHints() expected error, got nil")
			}

			if !testCase.wantErr && hintsErr != nil {
				t.Errorf("ShowHints() unexpected error: %v", hintsErr)
			}

			if testCase.checkHints != nil {
				testCase.checkHints(t, hints)
			}

			if testCase.checkOverlay != nil {
				testCase.checkOverlay(t, mockOverlay)
			}
		})
	}
}

func TestHintService_HideHints(t *testing.T) {
	tests := []struct {
		name       string
		setupMocks func(*mocks.MockOverlayPort)
		wantErr    bool
	}{
		{
			name: successfulHide,
			setupMocks: func(ov *mocks.MockOverlayPort) {
				ov.HideFunc = func(_ context.Context) error {
					return nil
				}
			},
			wantErr: false,
		},
		{
			name: overlayHideError,
			setupMocks: func(ov *mocks.MockOverlayPort) {
				ov.HideFunc = func(_ context.Context) error {
					return derrors.New(
						derrors.CodeOverlayFailed,
						"failed to hide overlay",
					)
				}
			},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}
			generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)
			logger := logger.Get()

			if testCase.setupMocks != nil {
				testCase.setupMocks(mockOverlay)
			}

			service := services.NewHintService(
				mockAcc,
				mockOverlay,
				&mocks.MockSystemPort{},
				generator,
				config.HintsConfig{},
				logger,
				nil,
			)

			ctx := context.Background()
			hideHintsErr := service.HideHints(ctx)

			if (hideHintsErr != nil) != testCase.wantErr {
				t.Errorf("HideHints() error = %v, wantErr %v", hideHintsErr, testCase.wantErr)
			}

			// Only check visibility for successful hide
			if !testCase.wantErr && mockOverlay.IsVisible() {
				t.Error("Overlay should not be visible after successful HideHints")
			}
		})
	}
}

func TestHintService_RefreshHints(t *testing.T) {
	tests := []struct {
		name           string
		overlayVisible bool
		expectRefresh  bool
		refreshError   error
		wantErr        bool
	}{
		{
			name:           "refresh when visible",
			overlayVisible: true,
			expectRefresh:  true,
			refreshError:   nil,
			wantErr:        false,
		},
		{
			name:           "skip refresh when not visible",
			overlayVisible: false,
			expectRefresh:  false,
			refreshError:   nil,
			wantErr:        false,
		},
		{
			name:           "refresh error when visible",
			overlayVisible: true,
			expectRefresh:  true,
			refreshError:   derrors.New(derrors.CodeOverlayFailed, "overlay refresh failed"),
			wantErr:        true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			mockAcc := &mocks.MockAccessibilityPort{}
			mockOverlay := &mocks.MockOverlayPort{}

			refreshCalled := false
			mockOverlay.IsVisibleFunc = func() bool {
				return testCase.overlayVisible
			}
			mockOverlay.RefreshFunc = func(_ context.Context) error {
				refreshCalled = true

				return testCase.refreshError
			}

			generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)
			logger := logger.Get()

			service := services.NewHintService(
				mockAcc,
				mockOverlay,
				&mocks.MockSystemPort{},
				generator,
				config.HintsConfig{},
				logger,
				nil,
			)

			ctx := context.Background()
			refreshHintsErr := service.RefreshHints(ctx)

			if (refreshHintsErr != nil) != testCase.wantErr {
				t.Errorf("RefreshHints() error = %v, wantErr %v", refreshHintsErr, testCase.wantErr)
			}

			if refreshCalled != testCase.expectRefresh {
				t.Errorf("Refresh called = %v, want %v", refreshCalled, testCase.expectRefresh)
			}
		})
	}
}

func TestHintService_GenerateHintsVisionCombinesSupplementaryAndWindowElements(
	t *testing.T,
) {
	supplementElement := mustNewElement("menubar", image.Rect(10, 0, 60, 20))
	windowElement := mustNewElement("window", image.Rect(10, 40, 60, 90))

	mockAcc := &mocks.MockAccessibilityPort{}
	mockAcc.ClickableElementsFunc = func(
		_ context.Context,
		filter ports.ElementFilter,
	) ([]*element.Element, error) {
		if !filter.SkipWindowElements {
			t.Error("accessibility should not collect window elements when using vision strategy")

			return nil, nil
		}

		return []*element.Element{supplementElement}, nil
	}

	mockSystem := &mocks.MockSystemPort{}
	mockSystem.FocusedWindowBoundsFunc = func(context.Context) (image.Rectangle, bool, error) {
		return image.Rect(0, 0, 200, 200), true, nil
	}

	generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)
	service := services.NewHintService(
		mockAcc,
		&mocks.MockOverlayPort{},
		mockSystem,
		generator,
		config.HintsConfig{
			ClickableRoles:                []string{string(element.RoleButton)},
			IncludeMenubarHints:           true,
			AdditionalMenubarHintsTargets: []string{"Clock"},
			IncludeDockHints:              true,
			IncludeNCHints:                true,
			IncludeStageManagerHints:      true,
			IncludePIPHints:               true,
			IncludeScreenCaptureHints:     true,
		},
		logger.Get(),
		&mockVisionPort{
			detectedElements: []*element.Element{windowElement},
		},
	)

	hints, err := service.GenerateHints(
		context.Background(),
		nil,
		nil,
		"com.example.app",
		config.StrategyVision,
		"",
		false,
	)
	if err != nil {
		t.Fatalf("GenerateHints() unexpected error: %v", err)
	}

	if len(hints) != 2 {
		t.Fatalf("GenerateHints() returned %d hints, want 2", len(hints))
	}

	seen := map[element.ID]int{}
	for _, generatedHint := range hints {
		seen[generatedHint.Element().ID()]++
	}

	if seen[supplementElement.ID()] != 1 {
		t.Errorf("supplementary element count = %d, want 1", seen[supplementElement.ID()])
	}

	if seen[windowElement.ID()] != 1 {
		t.Errorf("window element count = %d, want 1", seen[windowElement.ID()])
	}
}

func TestHintService_GenerateHintsVisionWithNilPortReturnsSupplementaryElements(
	t *testing.T,
) {
	supplementElement := mustNewElement("menubar", image.Rect(10, 0, 60, 20))

	mockAcc := &mocks.MockAccessibilityPort{}
	mockAcc.ClickableElementsFunc = func(
		_ context.Context,
		filter ports.ElementFilter,
	) ([]*element.Element, error) {
		if !filter.SkipWindowElements {
			t.Error("nil vision port should not trigger window AX collection")
		}

		return []*element.Element{supplementElement}, nil
	}

	generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)
	service := services.NewHintService(
		mockAcc,
		&mocks.MockOverlayPort{},
		&mocks.MockSystemPort{},
		generator,
		config.HintsConfig{
			IncludeMenubarHints: true,
		},
		logger.Get(),
		nil,
	)

	hints, err := service.GenerateHints(
		context.Background(),
		nil,
		nil,
		"com.example.app",
		config.StrategyVision,
		"",
		false,
	)
	if err != nil {
		t.Fatalf("GenerateHints() unexpected error: %v", err)
	}

	if len(hints) != 1 {
		t.Fatalf("GenerateHints() returned %d hints, want 1", len(hints))
	}

	if hints[0].Element().ID() != supplementElement.ID() {
		t.Errorf("hint element = %q, want %q", hints[0].Element().ID(), supplementElement.ID())
	}
}

func TestHintService_UpdateGenerator(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	logger := logger.Get()

	// Initial generator
	initialGen, _ := hint.NewAlphabetGenerator("abcd", hint.LabelDirectionReverse)

	service := services.NewHintService(
		mockAcc,
		mockOverlay,
		&mocks.MockSystemPort{},
		initialGen,
		config.HintsConfig{},
		logger,
		nil,
	)

	// Update with new generator
	newGen, _ := hint.NewAlphabetGenerator("efgh", hint.LabelDirectionReverse)
	ctx := context.Background()
	service.UpdateGenerator(ctx, newGen)

	// Test with nil generator (should not crash)
	service.UpdateGenerator(ctx, nil)
}

func TestHintService_GeneratorReturnsDirectionSpecificInstance(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	logger := logger.Get()

	reverseGen, _ := hint.NewAlphabetGenerator("abcd", hint.LabelDirectionReverse)
	normalGen, _ := hint.NewAlphabetGenerator("abcd", hint.LabelDirectionNormal)

	service := services.NewHintService(
		mockAcc,
		mockOverlay,
		&mocks.MockSystemPort{},
		reverseGen,
		config.HintsConfig{},
		logger,
		nil,
	)

	// Register a normal-direction generator on top of the reverse default.
	ctx := context.Background()
	service.UpdateGenerator(ctx, normalGen)

	// Each direction must resolve to its own generator instance, not the
	// shared default.
	gotReverse := service.Generator(config.LabelDirectionReverse)
	if gotReverse == nil {
		t.Fatal("Generator(reverse) returned nil")
	}

	if gotReverse.LabelDirection() != hint.LabelDirectionReverse {
		t.Errorf(
			"Generator(reverse).LabelDirection() = %v, want %v",
			gotReverse.LabelDirection(),
			hint.LabelDirectionReverse,
		)
	}

	gotNormal := service.Generator(config.LabelDirectionNormal)
	if gotNormal == nil {
		t.Fatal("Generator(normal) returned nil")
	}

	if gotNormal.LabelDirection() != hint.LabelDirectionNormal {
		t.Errorf(
			"Generator(normal).LabelDirection() = %v, want %v",
			gotNormal.LabelDirection(),
			hint.LabelDirectionNormal,
		)
	}

	if gotReverse == gotNormal {
		t.Error("reverse and normal resolved to the same generator instance")
	}

	// Empty direction falls back to the default (reverse) generator.
	gotDefault := service.Generator("")
	if gotDefault == nil {
		t.Fatal("Generator(\"\") returned nil")
	}

	if gotDefault.LabelDirection() != hint.LabelDirectionReverse {
		t.Errorf(
			"Generator(\"\").LabelDirection() = %v, want %v",
			gotDefault.LabelDirection(),
			hint.LabelDirectionReverse,
		)
	}

	// Unknown direction falls back to the default rather than failing.
	gotUnknown := service.Generator("made-up")
	if gotUnknown == nil {
		t.Fatal("Generator(\"made-up\") returned nil")
	}

	if gotUnknown.LabelDirection() != hint.LabelDirectionReverse {
		t.Errorf(
			"Generator(\"made-up\").LabelDirection() = %v, want %v",
			gotUnknown.LabelDirection(),
			hint.LabelDirectionReverse,
		)
	}
}

func TestHintService_GenerateHintsPicksDirectionGenerator(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	logger := logger.Get()

	normalGen, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionNormal)
	reverseGen, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionReverse)

	// Five elements force both algorithms into the two-character tier, where
	// reverse and normal produce *different* label sequences. The exact
	// normal sequence is [A S D FA FS]; the exact reverse sequence is
	// [AA SA DA FA AS]. The 4th and 5th labels expose the difference.
	mockAcc.ClickableElementsFunc = func(_ context.Context, _ ports.ElementFilter) ([]*element.Element, error) {
		return []*element.Element{
			mustNewElement("e1", image.Rect(0, 0, 10, 10)),
			mustNewElement("e2", image.Rect(20, 20, 30, 30)),
			mustNewElement("e3", image.Rect(40, 40, 50, 50)),
			mustNewElement("e4", image.Rect(60, 60, 70, 70)),
			mustNewElement("e5", image.Rect(80, 80, 90, 90)),
		}, nil
	}

	service := services.NewHintService(
		mockAcc,
		mockOverlay,
		&mocks.MockSystemPort{},
		normalGen,
		config.HintsConfig{},
		logger,
		nil,
	)

	ctx := context.Background()
	service.UpdateGenerator(ctx, reverseGen)

	// Without an override, the configured (empty) label direction resolves
	// to the default normal generator. The normal algorithm keeps 3
	// single-char slots ([A S D]) and expands the 4th alphabet slot (F)
	// into 2-char labels starting at [FA].
	hints, err := service.GenerateHints(ctx, nil, nil, "", "", "", false)
	if err != nil {
		t.Fatalf("GenerateHints() unexpected error: %v", err)
	}

	if len(hints) != 5 {
		t.Fatalf("GenerateHints() returned %d hints, want 5", len(hints))
	}

	wantNormalLabels := []string{"A", "S", "D", "FA", "FS"}
	for i, want := range wantNormalLabels {
		if got := hints[i].Label(); got != want {
			t.Errorf("default-direction hint[%d].Label() = %q, want %q", i, got, want)
		}
	}

	// With a reverse override, the override must resolve to the registered
	// reverse generator — not silently fall back to the default normal one.
	// The reverse algorithm fills all 4 single-char slots ([AA SA DA FA])
	// before yielding a 2-char label ([AS]). The 1st and 5th labels (AA, AS)
	// prove the override actually engaged.
	hints, err = service.GenerateHints(ctx, nil, nil, "", "", config.LabelDirectionReverse, false)
	if err != nil {
		t.Fatalf("GenerateHints() with reverse override unexpected error: %v", err)
	}

	if len(hints) != 5 {
		t.Fatalf(
			"GenerateHints() with reverse override returned %d hints, want 5",
			len(hints),
		)
	}

	wantReverseLabels := []string{"AA", "SA", "DA", "FA", "AS"}
	for i, want := range wantReverseLabels {
		if got := hints[i].Label(); got != want {
			t.Errorf(
				"reverse-override hint[%d].Label() = %q, want %q",
				i,
				got,
				want,
			)
		}
	}
}

func TestHintService_Health(t *testing.T) {
	mockAcc := &mocks.MockAccessibilityPort{}
	mockOverlay := &mocks.MockOverlayPort{}
	generator, _ := hint.NewAlphabetGenerator("abcd", hint.LabelDirectionReverse)
	logger := logger.Get()

	service := services.NewHintService(
		mockAcc,
		mockOverlay,
		&mocks.MockSystemPort{},
		generator,
		config.HintsConfig{},
		logger,
		nil,
	)

	// Setup mocks
	mockAcc.HealthFunc = func(_ context.Context) error {
		return nil
	}
	mockOverlay.HealthFunc = func(_ context.Context) error {
		return derrors.New(derrors.CodeOverlayFailed, "overlay unhealthy")
	}

	ctx := context.Background()
	health := service.Health(ctx)

	// Check that health map has both keys
	if len(health) != 2 {
		t.Errorf("Health() returned %d entries, want 2", len(health))
	}

	if _, ok := health["accessibility"]; !ok {
		t.Error("Health() missing 'accessibility' key")
	}

	if _, ok := health["overlay"]; !ok {
		t.Error("Health() missing 'overlay' key")
	}

	// Check that overlay has error
	if health["overlay"] == nil {
		t.Error("Health() overlay should have error")
	}

	if health["accessibility"] != nil {
		t.Error("Health() accessibility should not have error")
	}
}

// Helper functions.
func mustNewElement(id string, bounds image.Rectangle) *element.Element {
	element, elementErr := element.NewElement(element.ID(id), bounds, element.RoleButton)
	if elementErr != nil {
		panic(elementErr)
	}

	return element
}

type mockVisionPort struct {
	detectedElements []*element.Element
	detectErr        error
}

func (m *mockVisionPort) DetectElements(
	context.Context,
	image.Rectangle,
	config.HintsVisionConfig,
	bool,
) ([]*element.Element, error) {
	if m.detectErr != nil {
		return nil, m.detectErr
	}

	return m.detectedElements, nil
}

func (m *mockVisionPort) CaptureScreen(context.Context) (*image.RGBA, error) {
	return nil, derrors.New(derrors.CodeBridgeFailed, "capture screen not implemented")
}

func (m *mockVisionPort) Health(context.Context) error {
	return nil
}

func TestHintService_GenerateHintsRejectsSplitWordForNonVisionStrategy(t *testing.T) {
	generator, _ := hint.NewAlphabetGenerator("asdf", hint.LabelDirectionNormal)
	service := services.NewHintService(
		&mocks.MockAccessibilityPort{},
		&mocks.MockOverlayPort{},
		&mocks.MockSystemPort{},
		generator,
		config.HintsConfig{
			Strategy: config.StrategyAXTree,
		},
		logger.Get(),
		nil,
	)

	ctx := context.Background()

	_, err := service.GenerateHints(
		ctx,
		nil,
		nil,
		"",
		config.StrategyAXTree,
		"",
		true, // splitWord
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !derrors.IsCode(err, derrors.CodeInvalidInput) {
		t.Errorf("expected invalid input error, got: %v", err)
	}
}
