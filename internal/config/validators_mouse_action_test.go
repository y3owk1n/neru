package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestValidateMouseAction(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MouseAction.Enabled = true
	cfg.MouseAction.Actions = []string{"left_click", "mouse_down"}

	err := cfg.ValidateMouseAction()
	if err != nil {
		t.Fatalf("ValidateMouseAction() error = %v", err)
	}
}

func TestValidateMouseActionRejectsNonMouseButtonAction(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MouseAction.Enabled = true
	cfg.MouseAction.Actions = []string{"scroll"}

	err := cfg.ValidateMouseAction()
	if err == nil {
		t.Fatal("expected invalid mouse action indicator action to fail")
	}
}
