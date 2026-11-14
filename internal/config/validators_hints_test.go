package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
)

const (
	dirNormal     = "normal"
	dirReverse    = "reverse"
	bundleExample = "com.example"
	bundleOther   = "com.example.other"
)

func TestValidateHints_UIPlacement(t *testing.T) {
	validPlacements := []string{
		"top",
		"center",
		"bottom",
	}

	for _, placement := range validPlacements {
		cfg := config.DefaultConfig()
		cfg.Hints.UI.Placement = placement

		err := cfg.ValidateHints(nil)
		if err != nil {
			t.Fatalf("ValidateHints() expected placement %q to be valid, got %v", placement, err)
		}
	}

	cfg := config.DefaultConfig()

	cfg.Hints.UI.Placement = "floating"

	err := cfg.ValidateHints(nil)
	if err == nil {
		t.Fatal("ValidateHints() expected error for invalid hints.ui.placement")
	}
}

func TestValidateHints_UIShadowBlur(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		blur    int
		wantErr bool
	}{
		{name: "the default blur", blur: config.DefaultHintShadowBlur},
		{name: "a hard edge", blur: 0},
		{name: "the largest blur", blur: config.MaxHintShadowBlur},
		{name: "negative", blur: -1, wantErr: true},
		{name: "past the largest blur", blur: config.MaxHintShadowBlur + 1, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			cfg.Hints.UI.ShadowBlur = test.blur

			err := cfg.ValidateHints(nil)
			if (err != nil) != test.wantErr {
				t.Errorf("ValidateHints() with shadow_blur %d error = %v, want error %v",
					test.blur, err, test.wantErr)
			}
		})
	}
}

func TestValidateHints_UIShadowColors(t *testing.T) {
	t.Parallel()

	fields := map[string]func(*config.Config) *config.Color{
		"background_gradient_color": func(c *config.Config) *config.Color {
			return &c.Hints.UI.BackgroundGradientColor
		},
		"shadow_color":      func(c *config.Config) *config.Color { return &c.Hints.UI.ShadowColor },
		"text_shadow_color": func(c *config.Config) *config.Color { return &c.Hints.UI.TextShadowColor },
	}

	for name, field := range fields {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg := config.DefaultConfig()
			*field(cfg) = config.Color{Light: "#4D000000", Dark: "#4D000000"}

			err := cfg.ValidateHints(nil)
			if err != nil {
				t.Fatalf("ValidateHints() with a translucent %s = %v, want nil", name, err)
			}

			*field(cfg) = config.Color{Light: "yellow", Dark: "yellow"}

			err = cfg.ValidateHints(nil)
			if err == nil {
				t.Fatalf("ValidateHints() with %s = yellow, want an error", name)
			}
		})
	}
}

func TestValidateHints_PositiveUnitFloat(t *testing.T) {
	// merge_iou_threshold cannot be 0
	cfg := config.DefaultConfig()
	cfg.Hints.Vision.MergeIOUThreshold = 0.0

	err := cfg.ValidateHints(nil)
	if err == nil {
		t.Fatal("ValidateHints() expected error for 0.0 merge_iou_threshold")
	}

	// button_min_confidence cannot be 0
	cfg = config.DefaultConfig()
	cfg.Hints.Vision.ButtonMinConfidence = 0.0

	err = cfg.ValidateHints(nil)
	if err == nil {
		t.Fatal("ValidateHints() expected error for 0.0 button_min_confidence")
	}

	// generic_clickable_min_confidence cannot be 0
	cfg = config.DefaultConfig()
	cfg.Hints.Vision.GenericClickableMinConfidence = 0.0

	err = cfg.ValidateHints(nil)
	if err == nil {
		t.Fatal("ValidateHints() expected error for 0.0 generic_clickable_min_confidence")
	}

	// but minimum_confidence CAN be 0
	cfg = config.DefaultConfig()
	cfg.Hints.Vision.MinimumConfidence = 0.0

	err = cfg.ValidateHints(nil)
	if err != nil {
		t.Fatalf("ValidateHints() expected no error for 0.0 minimum_confidence, got %v", err)
	}
}

func TestValidateHints_LabelDirection(t *testing.T) {
	tests := []struct {
		name      string
		direction string
		wantErr   bool
	}{
		{name: "reverse is valid", direction: dirReverse, wantErr: false},
		{name: "normal is valid", direction: dirNormal, wantErr: false},
		{name: "empty defaults to normal (no error)", direction: "", wantErr: false},
		{name: "unknown value is rejected", direction: "sideways", wantErr: true},
		{name: "uppercase is rejected (case sensitive)", direction: "NORMAL", wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Hints.LabelDirection = testCase.direction

			err := cfg.ValidateHints(nil)
			if testCase.wantErr && err == nil {
				t.Fatalf(
					"ValidateHints() expected error for label_direction=%q, got nil",
					testCase.direction,
				)
			}

			if !testCase.wantErr && err != nil {
				t.Fatalf(
					"ValidateHints() unexpected error for label_direction=%q: %v",
					testCase.direction,
					err,
				)
			}
		})
	}
}

func TestValidateHints_LabelAlternation(t *testing.T) {
	tests := []struct {
		alternation string
		wantErr     bool
	}{
		{alternation: domain.LabelAlternationNone, wantErr: false},
		{alternation: domain.LabelAlternationHalves, wantErr: false},
		{alternation: "", wantErr: false},
		{alternation: "thirds", wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.alternation, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Hints.LabelAlternation = testCase.alternation

			err := cfg.ValidateHints(nil)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("ValidateHints() with label_alternation=%q: err = %v, wantErr %v",
					testCase.alternation, err, testCase.wantErr)
			}
		})
	}
}

func TestValidateAppConfigs_LabelDirection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.AppConfigs = []config.AppConfig{
		{
			BundleID:       bundleExample,
			LabelDirection: dirNormal,
		},
	}

	err := cfg.ValidateAppConfigs()
	if err != nil {
		t.Fatalf("ValidateAppConfigs() unexpected error for valid app config: %v", err)
	}

	cfg = config.DefaultConfig()
	cfg.Hints.AppConfigs = []config.AppConfig{
		{
			BundleID:       bundleExample,
			LabelDirection: "diagonal",
		},
	}

	err = cfg.ValidateAppConfigs()
	if err == nil {
		t.Fatal("ValidateAppConfigs() expected error for invalid app label_direction")
	}
}

func TestLabelDirectionForApp(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.LabelDirection = dirReverse
	cfg.Hints.AppConfigs = []config.AppConfig{
		{
			BundleID:       bundleExample,
			LabelDirection: dirNormal,
		},
	}

	// App override takes precedence.
	if got := cfg.Hints.LabelDirectionForApp(bundleExample); got != dirNormal {
		t.Errorf("LabelDirectionForApp(app with override) = %q, want %q", got, dirNormal)
	}

	// Fallback to global config.
	if got := cfg.Hints.LabelDirectionForApp("com.other"); got != dirReverse {
		t.Errorf("LabelDirectionForApp(app without override) = %q, want %q", got, dirReverse)
	}

	// Empty global value normalizes to the default (normal).
	cfg = config.DefaultConfig()
	cfg.Hints.LabelDirection = ""

	if got := cfg.Hints.LabelDirectionForApp(bundleExample); got != dirNormal {
		t.Errorf("LabelDirectionForApp(empty global) = %q, want %q", got, dirNormal)
	}
}
