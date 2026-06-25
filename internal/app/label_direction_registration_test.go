//nolint:testpackage // Tests private initialization helpers directly.
package app

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/app/services"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/infra/logger"
	"github.com/y3owk1n/neru/internal/core/ports/mocks"
)

const labelDirNormal = "normal"

// TestRegisterOppositeLabelDirectionGenerator_PreSeedsBothDirections verifies
// that the app initializer registers a generator for the direction opposite
// to the configured one, so the per-activation override path
// (`hints --label-direction <opposite>`) resolves to a real generator instead
// of silently falling back to the default.
func TestRegisterOppositeLabelDirectionGenerator_PreSeedsBothDirections(t *testing.T) {
	tests := []struct {
		name             string
		configuredRaw    string
		expectedPrimary  hint.LabelDirection
		expectedOpposite hint.LabelDirection
	}{
		{
			name:             "default (normal) -> opposite is reverse",
			configuredRaw:    "",
			expectedPrimary:  hint.LabelDirectionNormal,
			expectedOpposite: hint.LabelDirectionReverse,
		},
		{
			name:             "explicit normal -> opposite is reverse",
			configuredRaw:    labelDirNormal,
			expectedPrimary:  hint.LabelDirectionNormal,
			expectedOpposite: hint.LabelDirectionReverse,
		},
		{
			name:             "explicit reverse -> opposite is normal",
			configuredRaw:    "reverse",
			expectedPrimary:  hint.LabelDirectionReverse,
			expectedOpposite: hint.LabelDirectionNormal,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Hints.LabelDirection = testCase.configuredRaw

			primaryGen, primaryGenErr := hint.NewAlphabetGenerator(
				cfg.Hints.HintCharacters,
				hint.LabelDirectionFromString(cfg.Hints.LabelDirectionForApp("")),
			)
			if primaryGenErr != nil {
				t.Fatalf("NewAlphabetGenerator(primary) error: %v", primaryGenErr)
			}

			hintService := services.NewHintService(
				&mocks.MockAccessibilityPort{},
				&mocks.MockOverlayPort{},
				&mocks.MockSystemPort{},
				primaryGen,
				cfg.Hints,
				logger.Get(),
				nil,
			)

			app := &App{
				ctx:    context.Background(),
				logger: zap.NewNop(),
			}

			registerOppositeLabelDirectionGenerator(app, hintService, cfg)

			// Both directions must be resolvable.
			gotPrimary := hintService.Generator(testCase.expectedPrimary.String())
			if gotPrimary == nil {
				t.Fatalf("Generator(%s) returned nil", testCase.expectedPrimary)
			}

			if gotPrimary.LabelDirection() != testCase.expectedPrimary {
				t.Errorf(
					"Generator(%s).LabelDirection() = %v, want %v",
					testCase.expectedPrimary,
					gotPrimary.LabelDirection(),
					testCase.expectedPrimary,
				)
			}

			gotOpposite := hintService.Generator(testCase.expectedOpposite.String())
			if gotOpposite == nil {
				t.Fatalf("Generator(%s) returned nil", testCase.expectedOpposite)
			}

			if gotOpposite.LabelDirection() != testCase.expectedOpposite {
				t.Errorf(
					"Generator(%s).LabelDirection() = %v, want %v",
					testCase.expectedOpposite,
					gotOpposite.LabelDirection(),
					testCase.expectedOpposite,
				)
			}

			// The two generators must be distinct instances.
			if gotPrimary == gotOpposite {
				t.Errorf(
					"expected distinct generators for %s and %s, got the same instance",
					testCase.expectedPrimary,
					testCase.expectedOpposite,
				)
			}
		})
	}
}

// TestHintService_OverrideWithoutOppositeRegistrationFallsBack reproduces
// the user-reported bug: when only the configured direction is registered,
// asking for the opposite via the per-activation override silently returns
// the default generator. This is the regression check the
// `registerOppositeLabelDirectionGenerator` helper exists to prevent.
func TestHintService_OverrideWithoutOppositeRegistrationFallsBack(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.LabelDirection = labelDirNormal

	primaryGen, primaryGenErr := hint.NewAlphabetGenerator(
		cfg.Hints.HintCharacters,
		hint.LabelDirectionFromString(cfg.Hints.LabelDirectionForApp("")),
	)
	if primaryGenErr != nil {
		t.Fatalf("NewAlphabetGenerator(primary) error: %v", primaryGenErr)
	}

	hintService := services.NewHintService(
		&mocks.MockAccessibilityPort{},
		&mocks.MockOverlayPort{},
		&mocks.MockSystemPort{},
		primaryGen,
		cfg.Hints,
		logger.Get(),
		nil,
	)

	// No `UpdateGenerator` call here — the helper is intentionally NOT
	// invoked. This mirrors the buggy state where only the configured
	// direction is registered.
	gotReverse := hintService.Generator(hint.LabelDirectionReverse.String())
	if gotReverse == nil {
		t.Fatal("Generator(reverse) returned nil (expected default fallback)")
	}

	if gotReverse.LabelDirection() != hint.LabelDirectionNormal {
		t.Errorf(
			"Generator(reverse) without opposite registration returned direction %v, want normal (default fallback)",
			gotReverse.LabelDirection(),
		)
	}
}

// TestRegisterOppositeLabelDirectionGenerator_FixesUserBugScenario is the
// end-to-end regression test: with the helper invoked, the per-activation
// override (`hints --label-direction reverse`) must resolve to a real
// `reverse`-direction generator, not the configured `normal` one.
func TestRegisterOppositeLabelDirectionGenerator_FixesUserBugScenario(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.LabelDirection = labelDirNormal

	primaryGen, primaryGenErr := hint.NewAlphabetGenerator(
		cfg.Hints.HintCharacters,
		hint.LabelDirectionFromString(cfg.Hints.LabelDirectionForApp("")),
	)
	if primaryGenErr != nil {
		t.Fatalf("NewAlphabetGenerator(primary) error: %v", primaryGenErr)
	}

	hintService := services.NewHintService(
		&mocks.MockAccessibilityPort{},
		&mocks.MockOverlayPort{},
		&mocks.MockSystemPort{},
		primaryGen,
		cfg.Hints,
		logger.Get(),
		nil,
	)

	app := &App{
		ctx:    context.Background(),
		logger: zap.NewNop(),
	}

	// Simulate the production initialization path.
	registerOppositeLabelDirectionGenerator(app, hintService, cfg)

	// The per-activation `hints --label-direction reverse` override must
	// resolve to a generator with direction Reverse.
	gotReverse := hintService.Generator(hint.LabelDirectionReverse.String())
	if gotReverse == nil {
		t.Fatal("Generator(reverse) returned nil after registration")
	}

	if gotReverse.LabelDirection() != hint.LabelDirectionReverse {
		t.Errorf(
			"Generator(reverse) after registration has direction %v, want %v (this is the user-reported bug)",
			gotReverse.LabelDirection(),
			hint.LabelDirectionReverse,
		)
	}
}
