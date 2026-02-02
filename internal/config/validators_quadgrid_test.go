package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// TestConfig_ValidateQuadGrid tests the Config.ValidateQuadGrid method.
func TestConfig_ValidateQuadGrid(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr bool
	}{
		{
			name: "valid quadgrid config",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled:          true,
					Keys:             "uijk",
					ResetKey:         ",",
					MinSize:          50,
					MaxDepth:         4,
					LineWidth:        2,
					LabelFontSize:    12,
					HighlightOpacity: 0.5,
					LineColor:        "#FF0000",
					HighlightColor:   "#00FF00",
					LabelColor:       "#0000FF",
				},
			},
			wantErr: false,
		},
		{
			name: "disabled quadgrid config (should skip validation)",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled: false,
					// Invalid values but should be ignored
					Keys: "",
				},
			},
			wantErr: false,
		},
		{
			name: "quadgrid with empty keys - invalid",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled: true,
					Keys:    "",
				},
			},
			wantErr: true,
		},
		{
			name: "quadgrid with incorrect key length - invalid",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled: true,
					Keys:    "abc", // Need 4
				},
			},
			wantErr: true,
		},
		{
			name: "quadgrid with duplicate keys - invalid",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled: true,
					Keys:    "uiju", // Duplicate 'u'
				},
			},
			wantErr: true,
		},
		{
			name: "quadgrid with unicode keys - invalid",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled: true,
					Keys:    "uijé",
				},
			},
			wantErr: true,
		},
		{
			name: "quadgrid with invalid min_size",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled: true,
					Keys:    "uijk",
					MinSize: 5, // Too small (< 10)
				},
			},
			wantErr: true,
		},
		{
			name: "quadgrid with invalid max_depth",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled:  true,
					Keys:     "uijk",
					MinSize:  10,
					MaxDepth: 0, // Too small (< 1)
				},
			},
			wantErr: true,
		},
		{
			name: "quadgrid with invalid reset_key (empty uses default)",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled:          true,
					Keys:             "uijk",
					ResetKey:         "",
					MinSize:          50,
					MaxDepth:         4,
					LineWidth:        2,
					LabelFontSize:    12,
					HighlightOpacity: 0.5,
					LineColor:        "#FF0000",
					HighlightColor:   "#00FF00",
					LabelColor:       "#0000FF",
				},
			},
			wantErr: false,
		},
		{
			name: "quadgrid with valid modifier reset_key",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled:          true,
					Keys:             "uijk",
					ResetKey:         "Ctrl+R",
					MinSize:          50,
					MaxDepth:         4,
					LineWidth:        2,
					LabelFontSize:    12,
					HighlightOpacity: 0.5,
					LineColor:        "#FF0000",
					HighlightColor:   "#00FF00",
					LabelColor:       "#0000FF",
				},
			},
			wantErr: false,
		},
		{
			name: "quadgrid with invalid modifier reset_key",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled:  true,
					Keys:     "uijk",
					MinSize:  10,
					MaxDepth: 10,
					ResetKey: "Ctrl+", // Invalid format
				},
			},
			wantErr: true,
		},
		{
			name: "quadgrid with invalid single char reset_key (length > 1)",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled:  true,
					Keys:     "uijk",
					MinSize:  10,
					MaxDepth: 10,
					ResetKey: "ab",
				},
			},
			wantErr: true,
		},
		{
			name: "quadgrid with unicode reset_key",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled:  true,
					Keys:     "uijk",
					MinSize:  10,
					MaxDepth: 10,
					ResetKey: "é",
				},
			},
			wantErr: true,
		},
		{
			name: "quadgrid with reserved reset_key (backspace)",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled:  true,
					Keys:     "uijk",
					MinSize:  10,
					MaxDepth: 10,
					ResetKey: "backspace",
				},
			},
			wantErr: true,
		},
		{
			name: "quadgrid with conflict between keys and reset_key",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled:  true,
					Keys:     "uijk",
					MinSize:  10,
					MaxDepth: 10,
					ResetKey: "u", // Conflict
				},
			},
			wantErr: true,
		},
		{
			name: "quadgrid with invalid styling (negative width)",
			config: config.Config{
				QuadGrid: config.QuadGridConfig{
					Enabled:   true,
					Keys:      "uijk",
					MinSize:   10,
					MaxDepth:  10,
					LineWidth: -1,
				},
			},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.config.ValidateQuadGrid()
			if (err != nil) != testCase.wantErr {
				t.Errorf("Config.ValidateQuadGrid() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}
