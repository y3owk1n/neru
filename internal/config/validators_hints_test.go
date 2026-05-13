package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestValidateHints_EnabledRequiresClickableRoles(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.Enabled = true
	cfg.Hints.ClickableRoles = nil

	err := cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error when enabled and clickable_roles is empty")
	}
}

func TestValidateHints_BoundaryHighlightGeometry(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Hints.BoundaryHighlight.BorderWidth = -1

	err := cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error for negative boundary border width")
	}

	cfg = config.DefaultConfig()
	cfg.Hints.BoundaryHighlight.BorderRadius = -1

	err = cfg.ValidateHints()
	if err != nil {
		t.Fatalf("ValidateHints() expected no error for -1 (auto) border radius, got %v", err)
	}
}
