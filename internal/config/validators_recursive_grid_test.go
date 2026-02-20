package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

// TestConfig_ValidateRecursiveGrid tests the Config.ValidateRecursiveGrid method.
func TestConfig_ValidateRecursiveGrid(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Config
		wantErr bool
	}{
		{
			name: "valid recursive_grid config",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:        true,
					GridCols:       2,
					GridRows:       2,
					Keys:           "uijk",
					ResetKey:       ",",
					MinSizeWidth:   50,
					MinSizeHeight:  50,
					MaxDepth:       4,
					LineWidth:      2,
					LabelFontSize:  12,
					LineColor:      "#FF0000",
					HighlightColor: "#00FF00",
					LabelColor:     "#0000FF",
				},
			},
			wantErr: false,
		},
		{
			name: "disabled recursive_grid config (should skip validation)",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled: false,
					// Invalid values but should be ignored
					Keys: "",
				},
			},
			wantErr: false,
		},
		{
			name: "recursive_grid with empty keys - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:  true,
					GridCols: 2,
					GridRows: 2,
					Keys:     "",
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with incorrect key length - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:  true,
					GridCols: 2,
					GridRows: 2,
					Keys:     "abc", // Need 4 for 2x2
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with duplicate keys - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:  true,
					GridCols: 2,
					GridRows: 2,
					Keys:     "uiju", // Duplicate 'u'
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with unicode keys - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:  true,
					GridCols: 2,
					GridRows: 2,
					Keys:     "uijé",
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with invalid min_size_width",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					MinSizeWidth:  5, // Too small (< 10)
					MinSizeHeight: 25,
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with invalid min_size_height",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					MinSizeWidth:  25,
					MinSizeHeight: 5, // Too small (< 10)
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with invalid max_depth",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					MinSizeWidth:  10,
					MinSizeHeight: 10,
					MaxDepth:      0, // Too small (< 1)
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with invalid reset_key (empty uses default)",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:        true,
					GridCols:       2,
					GridRows:       2,
					Keys:           "uijk",
					ResetKey:       "",
					MinSizeWidth:   50,
					MinSizeHeight:  50,
					MaxDepth:       4,
					LineWidth:      2,
					LabelFontSize:  12,
					LineColor:      "#FF0000",
					HighlightColor: "#00FF00",
					LabelColor:     "#0000FF",
				},
			},
			wantErr: false,
		},
		{
			name: "recursive_grid with valid modifier reset_key",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:        true,
					GridCols:       2,
					GridRows:       2,
					Keys:           "uijk",
					ResetKey:       "Ctrl+R",
					MinSizeWidth:   50,
					MinSizeHeight:  50,
					MaxDepth:       4,
					LineWidth:      2,
					LabelFontSize:  12,
					LineColor:      "#FF0000",
					HighlightColor: "#00FF00",
					LabelColor:     "#0000FF",
				},
			},
			wantErr: false,
		},
		{
			name: "recursive_grid with invalid modifier reset_key",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					MinSizeWidth:  10,
					MinSizeHeight: 10,
					MaxDepth:      10,
					ResetKey:      "Ctrl+", // Invalid format
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with invalid single char reset_key (length > 1)",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					MinSizeWidth:  10,
					MinSizeHeight: 10,
					MaxDepth:      10,
					ResetKey:      "ab",
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with unicode reset_key",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					MinSizeWidth:  10,
					MinSizeHeight: 10,
					MaxDepth:      10,
					ResetKey:      "é",
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with reserved reset_key (backspace)",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					MinSizeWidth:  10,
					MinSizeHeight: 10,
					MaxDepth:      10,
					ResetKey:      "backspace",
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with conflict between keys and reset_key",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					MinSizeWidth:  10,
					MinSizeHeight: 10,
					MaxDepth:      10,
					ResetKey:      "u", // Conflict
				},
			},
			wantErr: true,
		},
		{
			name: "valid non-square recursive_grid config (3x2)",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:        true,
					GridCols:       3,
					GridRows:       2,
					Keys:           "gcrhtn",
					ResetKey:       ",",
					MinSizeWidth:   50,
					MinSizeHeight:  50,
					MaxDepth:       4,
					LineWidth:      2,
					LabelFontSize:  12,
					LineColor:      "#FF0000",
					HighlightColor: "#00FF00",
					LabelColor:     "#0000FF",
				},
			},
			wantErr: false,
		},
		{
			name: "valid non-square recursive_grid config (2x3)",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:        true,
					GridCols:       2,
					GridRows:       3,
					Keys:           "gcrhtn",
					ResetKey:       ",",
					MinSizeWidth:   50,
					MinSizeHeight:  50,
					MaxDepth:       4,
					LineWidth:      2,
					LabelFontSize:  12,
					LineColor:      "#FF0000",
					HighlightColor: "#00FF00",
					LabelColor:     "#0000FF",
				},
			},
			wantErr: false,
		},
		{
			name: "non-square grid with wrong key count - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:  true,
					GridCols: 3,
					GridRows: 2,
					Keys:     "uijk", // 4 keys but need 6 for 3x2
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with invalid grid_cols only (rows valid)",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:  true,
					GridCols: 1, // Invalid (< 2)
					GridRows: 3,
					Keys:     "uijk",
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with invalid grid_rows only (cols valid)",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:  true,
					GridCols: 3,
					GridRows: 0, // Invalid (< 2)
					Keys:     "uijk",
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with invalid styling (negative width)",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					MinSizeWidth:  10,
					MinSizeHeight: 10,
					MaxDepth:      10,
					LineWidth:     -1,
				},
			},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.config.ValidateRecursiveGrid()
			if (err != nil) != testCase.wantErr {
				t.Errorf(
					"Config.ValidateRecursiveGrid() error = %v, wantErr %v",
					err,
					testCase.wantErr,
				)
			}
		})
	}
}
