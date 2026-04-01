package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func TestDefaultThemeFallbacksMatchResolvedDefaults(t *testing.T) {
	cfg := config.DefaultConfig()

	cases := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "virtual pointer light",
			got:  cfg.VirtualPointer.UI.Color.Light,
			want: config.VirtualPointerColorLight,
		},
		{
			name: "virtual pointer dark",
			got:  cfg.VirtualPointer.UI.Color.Dark,
			want: config.VirtualPointerColorDark,
		},
		{
			name: "hints background light",
			got:  cfg.Hints.UI.BackgroundColor.Light,
			want: config.HintsBackgroundColorLight,
		},
		{
			name: "hints background dark",
			got:  cfg.Hints.UI.BackgroundColor.Dark,
			want: config.HintsBackgroundColorDark,
		},
		{
			name: "hints text light",
			got:  cfg.Hints.UI.TextColor.Light,
			want: config.HintsTextColorLight,
		},
		{
			name: "hints text dark",
			got:  cfg.Hints.UI.TextColor.Dark,
			want: config.HintsTextColorDark,
		},
		{
			name: "hints matched text light",
			got:  cfg.Hints.UI.MatchedTextColor.Light,
			want: config.HintsMatchedTextColorLight,
		},
		{
			name: "hints matched text dark",
			got:  cfg.Hints.UI.MatchedTextColor.Dark,
			want: config.HintsMatchedTextColorDark,
		},
		{
			name: "hints border light",
			got:  cfg.Hints.UI.BorderColor.Light,
			want: config.HintsBorderColorLight,
		},
		{
			name: "hints border dark",
			got:  cfg.Hints.UI.BorderColor.Dark,
			want: config.HintsBorderColorDark,
		},
		{
			name: "grid background light",
			got:  cfg.Grid.UI.BackgroundColor.Light,
			want: config.GridBackgroundColorLight,
		},
		{
			name: "grid background dark",
			got:  cfg.Grid.UI.BackgroundColor.Dark,
			want: config.GridBackgroundColorDark,
		},
		{
			name: "grid text light",
			got:  cfg.Grid.UI.TextColor.Light,
			want: config.GridTextColorLight,
		},
		{
			name: "grid text dark",
			got:  cfg.Grid.UI.TextColor.Dark,
			want: config.GridTextColorDark,
		},
		{
			name: "grid matched text light",
			got:  cfg.Grid.UI.MatchedTextColor.Light,
			want: config.GridMatchedTextColorLight,
		},
		{
			name: "grid matched text dark",
			got:  cfg.Grid.UI.MatchedTextColor.Dark,
			want: config.GridMatchedTextColorDark,
		},
		{
			name: "grid matched background light",
			got:  cfg.Grid.UI.MatchedBackgroundColor.Light,
			want: config.GridMatchedBackgroundColorLight,
		},
		{
			name: "grid matched background dark",
			got:  cfg.Grid.UI.MatchedBackgroundColor.Dark,
			want: config.GridMatchedBackgroundColorDark,
		},
		{
			name: "grid matched border light",
			got:  cfg.Grid.UI.MatchedBorderColor.Light,
			want: config.GridMatchedBorderColorLight,
		},
		{
			name: "grid matched border dark",
			got:  cfg.Grid.UI.MatchedBorderColor.Dark,
			want: config.GridMatchedBorderColorDark,
		},
		{
			name: "grid border light",
			got:  cfg.Grid.UI.BorderColor.Light,
			want: config.GridBorderColorLight,
		},
		{
			name: "grid border dark",
			got:  cfg.Grid.UI.BorderColor.Dark,
			want: config.GridBorderColorDark,
		},
		{
			name: "recursive grid line light",
			got:  cfg.RecursiveGrid.UI.LineColor.Light,
			want: config.RecursiveGridLineColorLight,
		},
		{
			name: "recursive grid line dark",
			got:  cfg.RecursiveGrid.UI.LineColor.Dark,
			want: config.RecursiveGridLineColorDark,
		},
		{
			name: "recursive grid highlight light",
			got:  cfg.RecursiveGrid.UI.HighlightColor.Light,
			want: config.RecursiveGridHighlightColorLight,
		},
		{
			name: "recursive grid highlight dark",
			got:  cfg.RecursiveGrid.UI.HighlightColor.Dark,
			want: config.RecursiveGridHighlightColorDark,
		},
		{
			name: "recursive grid text light",
			got:  cfg.RecursiveGrid.UI.TextColor.Light,
			want: config.RecursiveGridTextColorLight,
		},
		{
			name: "recursive grid text dark",
			got:  cfg.RecursiveGrid.UI.TextColor.Dark,
			want: config.RecursiveGridTextColorDark,
		},
		{
			name: "recursive grid label background light",
			got:  cfg.RecursiveGrid.UI.LabelBackgroundColor.Light,
			want: config.RecursiveGridLabelBackgroundColorLight,
		},
		{
			name: "recursive grid label background dark",
			got:  cfg.RecursiveGrid.UI.LabelBackgroundColor.Dark,
			want: config.RecursiveGridLabelBackgroundColorDark,
		},
		{
			name: "recursive grid subkey preview text light",
			got:  cfg.RecursiveGrid.UI.SubKeyPreviewTextColor.Light,
			want: config.RecursiveGridSubKeyPreviewTextColorLight,
		},
		{
			name: "recursive grid subkey preview text dark",
			got:  cfg.RecursiveGrid.UI.SubKeyPreviewTextColor.Dark,
			want: config.RecursiveGridSubKeyPreviewTextColorDark,
		},
		{
			name: "mode indicator background light",
			got:  cfg.ModeIndicator.UI.BackgroundColor.Light,
			want: config.ModeIndicatorBackgroundColorLight,
		},
		{
			name: "mode indicator background dark",
			got:  cfg.ModeIndicator.UI.BackgroundColor.Dark,
			want: config.ModeIndicatorBackgroundColorDark,
		},
		{
			name: "mode indicator text light",
			got:  cfg.ModeIndicator.UI.TextColor.Light,
			want: config.ModeIndicatorTextColorLight,
		},
		{
			name: "mode indicator text dark",
			got:  cfg.ModeIndicator.UI.TextColor.Dark,
			want: config.ModeIndicatorTextColorDark,
		},
		{
			name: "mode indicator border light",
			got:  cfg.ModeIndicator.UI.BorderColor.Light,
			want: config.ModeIndicatorBorderColorLight,
		},
		{
			name: "mode indicator border dark",
			got:  cfg.ModeIndicator.UI.BorderColor.Dark,
			want: config.ModeIndicatorBorderColorDark,
		},
		{
			name: "sticky modifiers background light",
			got:  cfg.StickyModifiers.UI.BackgroundColor.Light,
			want: config.StickyModifiersBackgroundColorLight,
		},
		{
			name: "sticky modifiers background dark",
			got:  cfg.StickyModifiers.UI.BackgroundColor.Dark,
			want: config.StickyModifiersBackgroundColorDark,
		},
		{
			name: "sticky modifiers text light",
			got:  cfg.StickyModifiers.UI.TextColor.Light,
			want: config.StickyModifiersTextColorLight,
		},
		{
			name: "sticky modifiers text dark",
			got:  cfg.StickyModifiers.UI.TextColor.Dark,
			want: config.StickyModifiersTextColorDark,
		},
		{
			name: "sticky modifiers border light",
			got:  cfg.StickyModifiers.UI.BorderColor.Light,
			want: config.StickyModifiersBorderColorLight,
		},
		{
			name: "sticky modifiers border dark",
			got:  cfg.StickyModifiers.UI.BorderColor.Dark,
			want: config.StickyModifiersBorderColorDark,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.got != testCase.want {
				t.Fatalf("expected %q, got %q", testCase.want, testCase.got)
			}
		})
	}
}
