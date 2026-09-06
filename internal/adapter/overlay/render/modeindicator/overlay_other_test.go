//go:build !darwin

package modeindicator_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/modeindicator"
	"github.com/y3owk1n/neru/internal/config"
)

// TestOverlay_ResolveLabelText_Semantics pins the shared label rules for every
// non-darwin platform: a per-mode disabled indicator draws nothing, an empty
// custom text falls back to the mode name, and unknown modes resolve to
// nothing. Windows used to re-implement this lookup locally and diverged on
// the first two rules.
func TestOverlay_ResolveLabelText_Semantics(t *testing.T) {
	t.Parallel()

	cfg := config.ModeIndicatorConfig{
		Hints: config.ModeIndicatorModeConfig{Enabled: true, Text: "HINT"},
		Grid:  config.ModeIndicatorModeConfig{Enabled: true},
		Scroll: config.ModeIndicatorModeConfig{
			Enabled: false,
			Text:    "SCROLL",
		},
	}

	overlay, err := modeindicator.NewOverlay(cfg, map[string]string{"window": "Window"}, nil, nil)
	if err != nil {
		t.Fatalf("NewOverlay returned error: %v", err)
	}

	tests := []struct {
		name string
		mode string
		want string
	}{
		{name: "custom text wins", mode: "hints", want: "HINT"},
		{name: "empty text falls back to mode name", mode: "grid", want: "grid"},
		{name: "disabled mode draws nothing even with text", mode: "scroll", want: ""},
		{name: "unknown mode draws nothing", mode: "bogus", want: ""},
		// A declared mode is named by its declaration, and its label is the
		// one the declaration gave it.
		{name: "declared mode shows its declared label", mode: "window", want: "Window"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := overlay.ResolveLabelText(tc.mode); got != tc.want {
				t.Errorf("ResolveLabelText(%q) = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}

// TestOverlay_ResolveModeConfig_UnknownMode pins that unknown modes report
// ok=false rather than a zero config, so callers can distinguish "not
// configured" from "configured empty".
func TestOverlay_ResolveModeConfig_UnknownMode(t *testing.T) {
	t.Parallel()

	overlay, err := modeindicator.NewOverlay(config.ModeIndicatorConfig{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewOverlay returned error: %v", err)
	}

	if _, ok := overlay.ResolveModeConfig("bogus"); ok {
		t.Error("ResolveModeConfig(\"bogus\") reported ok=true, want false")
	}

	if _, ok := overlay.ResolveModeConfig("hints"); !ok {
		t.Error("ResolveModeConfig(\"hints\") reported ok=false, want true")
	}
}
