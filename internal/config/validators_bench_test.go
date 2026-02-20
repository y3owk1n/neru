package config_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/config"
)

func BenchmarkValidateColor(b *testing.B) {
	for b.Loop() {
		_ = config.ValidateColor("#FF0000", "test_color")
	}
}

func BenchmarkValidateHotkey(b *testing.B) {
	for b.Loop() {
		_ = config.ValidateHotkey("Cmd+Shift+Space", "test_hotkey")
	}
}

func BenchmarkValidateHints(b *testing.B) {
	config := &config.Config{
		Hints: config.HintsConfig{
			HintCharacters:   "ABCDEFGH",
			BackgroundColor:  "#000000",
			TextColor:        "#FFFFFF",
			MatchedTextColor: "#FF0000",
			BorderColor:      "#333333",
			FontSize:         14,
		},
	}

	for b.Loop() {
		_ = config.ValidateHints()
	}
}
