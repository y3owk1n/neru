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
