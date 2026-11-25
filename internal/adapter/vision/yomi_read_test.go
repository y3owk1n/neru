//go:build darwin || linux || windows

package vision

import (
	"slices"
	"testing"
)

func TestReadingOrder(t *testing.T) {
	tests := []struct {
		name   string
		screen []string
		want   []string
	}{
		{
			"a left-to-right line keeps its order",
			[]string{labelSave, "your", "changes"},
			[]string{labelSave, "your", "changes"},
		},
		{
			"an Arabic line reads right to left",
			[]string{"التغييرات", "واحفظ", "الإعدادات", "افتح", "العربية:"},
			[]string{"العربية:", "افتح", "الإعدادات", "واحفظ", "التغييرات"},
		},
		{
			"a run of Latin words in an Arabic line keeps its order",
			[]string{"اليوم", "Neru", "Go", "يستخدم"},
			[]string{"يستخدم", "Neru", "Go", "اليوم"},
		},
		{
			"a Hebrew line with a number reads right to left",
			[]string{"2026", "בשנת", "פתח"},
			[]string{"פתח", "בשנת", "2026"},
		},
		{
			"an English line with one Arabic word stays left to right",
			[]string{"Open", "العربية", "settings", "now"},
			[]string{"Open", "العربية", "settings", "now"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := readingOrder(tt.screen); !slices.Equal(got, tt.want) {
				t.Errorf("readingOrder(%q) = %q, want %q", tt.screen, got, tt.want)
			}
		})
	}
}
