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

// TestValidateMouseAction_RejectsInvalidFields pins the refusals, and that they
// hold while the indicator is disabled too. The validator has no early return
// on Enabled, so an invalid value cannot sit dormant until someone turns the
// indicator on.
func TestValidateMouseAction_RejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*config.Config)
	}{
		{
			name: "action that is not a mouse button",
			setup: func(c *config.Config) {
				c.MouseAction.Enabled = true
				c.MouseAction.Actions = []string{"scroll"}
			},
		},
		{
			name: "empty actions while enabled",
			setup: func(c *config.Config) {
				c.MouseAction.Enabled = true
				c.MouseAction.Actions = []string{}
			},
		},
		{
			name: "empty actions while disabled",
			setup: func(c *config.Config) {
				c.MouseAction.Enabled = false
				c.MouseAction.Actions = []string{}
			},
		},
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
				t.Fatal("ValidateMouseAction() accepted an invalid mouse_action config")
			}
		})
	}
}
