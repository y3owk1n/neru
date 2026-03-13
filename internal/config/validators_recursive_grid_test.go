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
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						LineColorLight:        "#FF0000",
						LineColorDark:         "#FF0000",
						HighlightColorLight:   "#00FF00",
						HighlightColorDark:    "#00FF00",
						TextColorLight:        "#0000FF",
						TextColorDark:         "#0000FF",
						SubKeyPreviewFontSize: 6,
					},
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
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      "",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						LineColorLight:        "#FF0000",
						LineColorDark:         "#FF0000",
						HighlightColorLight:   "#00FF00",
						HighlightColorDark:    "#00FF00",
						TextColorLight:        "#0000FF",
						TextColorDark:         "#0000FF",
						SubKeyPreviewFontSize: 6,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "recursive_grid with valid modifier reset_key",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      "Ctrl+R",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						LineColorLight:        "#FF0000",
						LineColorDark:         "#FF0000",
						HighlightColorLight:   "#00FF00",
						HighlightColorDark:    "#00FF00",
						TextColorLight:        "#0000FF",
						TextColorDark:         "#0000FF",
						SubKeyPreviewFontSize: 6,
					},
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
					Enabled:       true,
					GridCols:      3,
					GridRows:      2,
					Keys:          "gcrhtn",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						LineColorLight:        "#FF0000",
						LineColorDark:         "#FF0000",
						HighlightColorLight:   "#00FF00",
						HighlightColorDark:    "#00FF00",
						TextColorLight:        "#0000FF",
						TextColorDark:         "#0000FF",
						SubKeyPreviewFontSize: 6,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid non-square recursive_grid config (2x3)",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      3,
					Keys:          "gcrhtn",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						LineColorLight:        "#FF0000",
						LineColorDark:         "#FF0000",
						HighlightColorLight:   "#00FF00",
						HighlightColorDark:    "#00FF00",
						TextColorLight:        "#0000FF",
						TextColorDark:         "#0000FF",
						SubKeyPreviewFontSize: 6,
					},
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
					UI: config.RecursiveGridUI{
						LineWidth:             -1,
						SubKeyPreviewFontSize: 6,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid recursive_grid config with auto_exit_actions",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						LineColorLight:        "#FF0000",
						LineColorDark:         "#FF0000",
						HighlightColorLight:   "#00FF00",
						HighlightColorDark:    "#00FF00",
						TextColorLight:        "#0000FF",
						TextColorDark:         "#0000FF",
						SubKeyPreviewFontSize: 6,
					},
					AutoExitActions: []string{"left_click", "right_click"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid recursive_grid config with empty auto_exit_actions",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						LineColorLight:        "#FF0000",
						LineColorDark:         "#FF0000",
						HighlightColorLight:   "#00FF00",
						HighlightColorDark:    "#00FF00",
						TextColorLight:        "#0000FF",
						TextColorDark:         "#0000FF",
						SubKeyPreviewFontSize: 6,
					},
					AutoExitActions: []string{},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid recursive_grid auto_exit_actions with unknown action",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						LineColorLight:        "#FF0000",
						LineColorDark:         "#FF0000",
						HighlightColorLight:   "#00FF00",
						HighlightColorDark:    "#00FF00",
						TextColorLight:        "#0000FF",
						TextColorDark:         "#0000FF",
						SubKeyPreviewFontSize: 6,
					},
					AutoExitActions: []string{"unknown_action"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid recursive_grid auto_exit_actions with scroll (IPC-only)",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						LineColorLight:        "#FF0000",
						LineColorDark:         "#FF0000",
						HighlightColorLight:   "#00FF00",
						HighlightColorDark:    "#00FF00",
						TextColorLight:        "#0000FF",
						TextColorDark:         "#0000FF",
						SubKeyPreviewFontSize: 6,
					},
					AutoExitActions: []string{"scroll"},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid recursive_grid auto_exit_actions with move_mouse (IPC-only)",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						LineColorLight:        "#FF0000",
						LineColorDark:         "#FF0000",
						HighlightColorLight:   "#00FF00",
						HighlightColorDark:    "#00FF00",
						TextColorLight:        "#0000FF",
						TextColorDark:         "#0000FF",
						SubKeyPreviewFontSize: 6,
					},
					AutoExitActions: []string{"move_mouse"},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with empty text_color (theme-aware default) - valid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						LineColorLight:        "#FF0000",
						LineColorDark:         "#FF0000",
						HighlightColorLight:   "#00FF00",
						HighlightColorDark:    "#00FF00",
						TextColorLight:        "",
						TextColorDark:         "",
						SubKeyPreviewFontSize: 6,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "recursive_grid with custom label background geometry - valid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:                   2,
						FontSize:                    12,
						LineColorLight:              "#FF0000",
						LineColorDark:               "#FF0000",
						HighlightColorLight:         "#00FF00",
						HighlightColorDark:          "#00FF00",
						TextColorLight:              "#0000FF",
						TextColorDark:               "#0000FF",
						LabelBackground:             true,
						LabelBackgroundPaddingX:     10,
						LabelBackgroundPaddingY:     6,
						LabelBackgroundBorderRadius: 4,
						LabelBackgroundBorderWidth:  2,
						SubKeyPreviewFontSize:       6,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "recursive_grid with invalid label background padding x - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:               2,
						FontSize:                12,
						LineColorLight:          "#FF0000",
						LineColorDark:           "#FF0000",
						HighlightColorLight:     "#00FF00",
						HighlightColorDark:      "#00FF00",
						TextColorLight:          "#0000FF",
						TextColorDark:           "#0000FF",
						LabelBackground:         true,
						LabelBackgroundPaddingX: -2,
						SubKeyPreviewFontSize:   6,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with invalid label background padding y - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:               2,
						FontSize:                12,
						LineColorLight:          "#FF0000",
						LineColorDark:           "#FF0000",
						HighlightColorLight:     "#00FF00",
						HighlightColorDark:      "#00FF00",
						TextColorLight:          "#0000FF",
						TextColorDark:           "#0000FF",
						LabelBackground:         true,
						LabelBackgroundPaddingY: -2,
						SubKeyPreviewFontSize:   6,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with invalid label background border radius - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:                   2,
						FontSize:                    12,
						LineColorLight:              "#FF0000",
						LineColorDark:               "#FF0000",
						HighlightColorLight:         "#00FF00",
						HighlightColorDark:          "#00FF00",
						TextColorLight:              "#0000FF",
						TextColorDark:               "#0000FF",
						LabelBackground:             true,
						LabelBackgroundBorderRadius: -2,
						SubKeyPreviewFontSize:       6,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with invalid label background border width - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:                  2,
						FontSize:                   12,
						LineColorLight:             "#FF0000",
						LineColorDark:              "#FF0000",
						HighlightColorLight:        "#00FF00",
						HighlightColorDark:         "#00FF00",
						TextColorLight:             "#0000FF",
						TextColorDark:              "#0000FF",
						LabelBackground:            true,
						LabelBackgroundBorderWidth: -1,
						SubKeyPreviewFontSize:      6,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with custom label background colors - valid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:                 2,
						FontSize:                  12,
						LineColorLight:            "#FF0000",
						LineColorDark:             "#FF0000",
						HighlightColorLight:       "#00FF00",
						HighlightColorDark:        "#00FF00",
						TextColorLight:            "#0000FF",
						TextColorDark:             "#0000FF",
						LabelBackground:           true,
						LabelBackgroundColorLight: "#CCFFD700",
						LabelBackgroundColorDark:  "#99FFD700",
						SubKeyPreviewFontSize:     6,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "recursive_grid with invalid label background color - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:                 2,
						FontSize:                  12,
						LineColorLight:            "#FF0000",
						LineColorDark:             "#FF0000",
						HighlightColorLight:       "#00FF00",
						HighlightColorDark:        "#00FF00",
						TextColorLight:            "#0000FF",
						TextColorDark:             "#0000FF",
						LabelBackground:           true,
						LabelBackgroundColorLight: "invalid",
						SubKeyPreviewFontSize:     6,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid backspace_key conflicts with keys - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					BackspaceKey:  "u", // Conflicts with keys
					MinSizeWidth:  10,
					MinSizeHeight: 10,
					MaxDepth:      10,
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid backspace_key named key conflicts with key char - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "ui\tk",
					ResetKey:      ",",
					BackspaceKey:  "tab", // Named key resolves to \t which is in keys
					MinSizeWidth:  10,
					MinSizeHeight: 10,
					MaxDepth:      10,
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid backspace_key case-insensitive conflict - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					BackspaceKey:  "U", // Conflicts (case-insensitive)
					MinSizeWidth:  10,
					MinSizeHeight: 10,
					MaxDepth:      10,
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid backspace_key no conflict",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					BackspaceKey:  "x", // Not in keys
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						LineColorLight:        "#FF0000",
						LineColorDark:         "#FF0000",
						HighlightColorLight:   "#00FF00",
						HighlightColorDark:    "#00FF00",
						TextColorLight:        "#0000FF",
						TextColorDark:         "#0000FF",
						SubKeyPreviewFontSize: 6,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "recursive_grid backspace_key modifier combo no conflict",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					BackspaceKey:  testModifierBackspaceKey, // Modifier combo, no conflict
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						LineColorLight:        "#FF0000",
						LineColorDark:         "#FF0000",
						HighlightColorLight:   "#00FF00",
						HighlightColorDark:    "#00FF00",
						TextColorLight:        "#0000FF",
						TextColorDark:         "#0000FF",
						SubKeyPreviewFontSize: 6,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "recursive_grid modifier combo reset_key conflicts with same modifier combo backspace_key - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      testModifierBackspaceKey,
					BackspaceKey:  testModifierBackspaceKey, // Same as reset_key
					MinSizeWidth:  10,
					MinSizeHeight: 10,
					MaxDepth:      10,
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with empty label background colors (theme-aware default) - valid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:                 2,
						FontSize:                  12,
						LineColorLight:            "#FF0000",
						LineColorDark:             "#FF0000",
						HighlightColorLight:       "#00FF00",
						HighlightColorDark:        "#00FF00",
						TextColorLight:            "#0000FF",
						TextColorDark:             "#0000FF",
						LabelBackground:           true,
						LabelBackgroundColorLight: "",
						LabelBackgroundColorDark:  "",
						SubKeyPreviewFontSize:     6,
					},
				},
			},
			wantErr: false,
		},
		// --- SubKeyPreviewAutohideMultiplier tests ---
		{
			name: "recursive_grid with negative sub_key_preview_autohide_multiplier - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:                       2,
						FontSize:                        12,
						LineColorLight:                  "#FF0000",
						LineColorDark:                   "#FF0000",
						HighlightColorLight:             "#00FF00",
						HighlightColorDark:              "#00FF00",
						TextColorLight:                  "#0000FF",
						TextColorDark:                   "#0000FF",
						SubKeyPreviewFontSize:           6,
						SubKeyPreviewAutohideMultiplier: -1.0,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid with zero sub_key_preview_autohide_multiplier (disable autohide) - valid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:                       2,
						FontSize:                        12,
						LineColorLight:                  "#FF0000",
						LineColorDark:                   "#FF0000",
						HighlightColorLight:             "#00FF00",
						HighlightColorDark:              "#00FF00",
						TextColorLight:                  "#0000FF",
						TextColorDark:                   "#0000FF",
						SubKeyPreviewFontSize:           6,
						SubKeyPreviewAutohideMultiplier: 0,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "recursive_grid with positive sub_key_preview_autohide_multiplier - valid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:                       2,
						FontSize:                        12,
						LineColorLight:                  "#FF0000",
						LineColorDark:                   "#FF0000",
						HighlightColorLight:             "#00FF00",
						HighlightColorDark:              "#00FF00",
						TextColorLight:                  "#0000FF",
						TextColorDark:                   "#0000FF",
						SubKeyPreviewFontSize:           6,
						SubKeyPreviewAutohideMultiplier: 2.5,
					},
				},
			},
			wantErr: false,
		},
		// --- Per-depth layer tests ---
		{
			name: "valid recursive_grid with layers",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						SubKeyPreviewFontSize: 6,
					},
					Layers: []config.RecursiveGridLayerConfig{
						{Depth: 0, GridCols: 4, GridRows: 2, Keys: "qwerasdf"},
						{Depth: 1, GridCols: 3, GridRows: 3, Keys: "qweasdzxc"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "recursive_grid layer with negative depth - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						SubKeyPreviewFontSize: 6,
					},
					Layers: []config.RecursiveGridLayerConfig{
						{Depth: -1, GridCols: 2, GridRows: 2, Keys: "abcd"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid layer with duplicate depths - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						SubKeyPreviewFontSize: 6,
					},
					Layers: []config.RecursiveGridLayerConfig{
						{Depth: 0, GridCols: 2, GridRows: 2, Keys: "abcd"},
						{Depth: 0, GridCols: 3, GridRows: 2, Keys: "qwerty"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid layer with wrong key count - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						SubKeyPreviewFontSize: 6,
					},
					Layers: []config.RecursiveGridLayerConfig{
						{Depth: 0, GridCols: 3, GridRows: 2, Keys: "abcd"}, // Need 6 for 3x2
					},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid layer with duplicate keys - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						SubKeyPreviewFontSize: 6,
					},
					Layers: []config.RecursiveGridLayerConfig{
						{Depth: 0, GridCols: 2, GridRows: 2, Keys: "abba"}, // Duplicate 'a' and 'b'
					},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid layer with grid_cols < 2 - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						SubKeyPreviewFontSize: 6,
					},
					Layers: []config.RecursiveGridLayerConfig{
						{Depth: 0, GridCols: 1, GridRows: 2, Keys: "ab"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid layer keys conflict with reset key - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      "a",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						SubKeyPreviewFontSize: 6,
					},
					Layers: []config.RecursiveGridLayerConfig{
						{
							Depth:    0,
							GridCols: 2,
							GridRows: 2,
							Keys:     "abcd",
						}, // 'a' conflicts with reset_key
					},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid layer with depth >= max_depth - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						SubKeyPreviewFontSize: 6,
					},
					Layers: []config.RecursiveGridLayerConfig{
						{
							Depth:    4,
							GridCols: 2,
							GridRows: 2,
							Keys:     "abcd",
						}, // depth 4 == max_depth 4, unreachable
					},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid layer with depth > max_depth - invalid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						SubKeyPreviewFontSize: 6,
					},
					Layers: []config.RecursiveGridLayerConfig{
						{Depth: 10, GridCols: 2, GridRows: 2, Keys: "abcd"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "recursive_grid layer at max valid depth (max_depth-1) - valid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						SubKeyPreviewFontSize: 6,
					},
					Layers: []config.RecursiveGridLayerConfig{
						{
							Depth:    3,
							GridCols: 2,
							GridRows: 2,
							Keys:     "abcd",
						}, // depth 3 < max_depth 4, valid
					},
				},
			},
			wantErr: false,
		},
		{
			name: "recursive_grid empty layers - valid",
			config: config.Config{
				RecursiveGrid: config.RecursiveGridConfig{
					Enabled:       true,
					GridCols:      2,
					GridRows:      2,
					Keys:          "uijk",
					ResetKey:      ",",
					MinSizeWidth:  50,
					MinSizeHeight: 50,
					MaxDepth:      4,
					UI: config.RecursiveGridUI{
						LineWidth:             2,
						FontSize:              12,
						SubKeyPreviewFontSize: 6,
					},
					Layers: []config.RecursiveGridLayerConfig{},
				},
			},
			wantErr: false,
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
