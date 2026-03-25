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

func TestValidateHints_RefreshDelayBounds(t *testing.T) {
	cfg := config.DefaultConfig()

	cfg.Hints.MouseActionRefreshDelay = -1

	err := cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error for negative mouse_action_refresh_delay")
	}

	cfg = config.DefaultConfig()

	cfg.Hints.MouseActionRefreshDelay = config.MaxMouseActionRefreshDelay + 1

	err = cfg.ValidateHints()
	if err == nil {
		t.Fatal("ValidateHints() expected error for too-large mouse_action_refresh_delay")
	}
}
