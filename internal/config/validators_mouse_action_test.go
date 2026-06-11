package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

const testMouseActionLeftClick = "left_click"

func TestValidateMouseAction(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MouseAction.Enabled = true
	cfg.MouseAction.Actions = []string{testMouseActionLeftClick, "mouse_down"}

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

func TestValidateMouseAction_RejectsInvalidFieldsWhenDisabled(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*config.Config)
	}{
		{
			name: "size too small",
			setup: func(c *config.Config) {
				c.MouseAction.Enabled = false
				c.MouseAction.Actions = []string{testMouseActionLeftClick}
				c.MouseAction.UI.Size = 0
			},
		},
		{
			name: "duration too small",
			setup: func(c *config.Config) {
				c.MouseAction.Enabled = false
				c.MouseAction.Actions = []string{testMouseActionLeftClick}
				c.MouseAction.Animation.DurationMS = 0
			},
		},
		{
			name: "invalid easing",
			setup: func(c *config.Config) {
				c.MouseAction.Enabled = false
				c.MouseAction.Actions = []string{testMouseActionLeftClick}
				c.MouseAction.Animation.Easing = "bogus"
			},
		},
		{
			name: "invalid shape",
			setup: func(c *config.Config) {
				c.MouseAction.Enabled = false
				c.MouseAction.Actions = []string{testMouseActionLeftClick}
				c.MouseAction.UI.Shape = "hexagon"
			},
		},
		{
			name: "negative start scale",
			setup: func(c *config.Config) {
				c.MouseAction.Enabled = false
				c.MouseAction.Actions = []string{testMouseActionLeftClick}
				c.MouseAction.Animation.StartScale = -1
			},
		},
		{
			name: "invalid opacity",
			setup: func(c *config.Config) {
				c.MouseAction.Enabled = false
				c.MouseAction.Actions = []string{testMouseActionLeftClick}
				c.MouseAction.Animation.StartOpacity = 1.5
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			tt.setup(cfg)

			err := cfg.ValidateMouseAction()
			if err == nil {
				t.Fatal("expected validation to fail for disabled mouse action with invalid config")
			}
		})
	}
}

func TestValidateMouseAction_RejectsEmptyActions(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MouseAction.Enabled = true
	cfg.MouseAction.Actions = []string{}

	err := cfg.ValidateMouseAction()
	if err == nil {
		t.Fatal("expected validation to fail for empty actions")
	}
}

func TestValidateMouseAction_RejectsEmptyActionsWhenDisabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MouseAction.Enabled = false
	cfg.MouseAction.Actions = []string{}

	err := cfg.ValidateMouseAction()
	if err == nil {
		t.Fatal("expected validation to fail for empty actions even when disabled")
	}
}
