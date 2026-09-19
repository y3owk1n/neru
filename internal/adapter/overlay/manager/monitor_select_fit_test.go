package manager_test

import (
	"image"
	"testing"
	"unicode/utf8"

	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
	"github.com/y3owk1n/neru/internal/ports"
)

// squareMeasurer measures every rune as wide as the font is tall.
type squareMeasurer struct{}

func (squareMeasurer) Measure(text, _ string, size float64, _ bool) (ports.TextMetrics, error) {
	return ports.TextMetrics{Width: float64(utf8.RuneCountInString(text)) * size, Height: size}, nil
}

func TestMonitorSelectStyle_FittedTo(t *testing.T) {
	ports.SetTextMeasurer(squareMeasurer{})
	t.Cleanup(func() { ports.SetTextMeasurer(nil) })

	// The shipped defaults: 96 and 18 points, padding and radius on auto.
	defaults := manager.MonitorSelectStyle{
		FontSize: 96, SubtitleFontSize: 18, PaddingX: -1, PaddingY: -1, BorderRadius: -1,
	}

	tests := []struct {
		name         string
		style        manager.MonitorSelectStyle
		target       manager.MonitorSelectTarget
		scale        float64
		wantLabel    int
		wantSubtitle int
	}{
		{
			name:  "a laptop display draws the defaults as configured",
			style: defaults,
			target: manager.MonitorSelectTarget{
				Bounds: image.Rect(
					0,
					0,
					1470,
					956,
				),
				Label:    "1",
				Subtitle: "Built-in Retina Display",
			},
			scale:        1,
			wantLabel:    96,
			wantSubtitle: 18,
		},
		{
			// Auto padding at 96 points is 29 by 14. The panel is 0.8 x 160 =
			// 128 tall, 100 after padding and 90 after the fill. The subtitle
			// and the gap take 22, leaving the label 68.
			name:  "a small monitor shrinks the label into its panel",
			style: defaults,
			target: manager.MonitorSelectTarget{
				Bounds: image.Rect(0, 0, 1280, 160), Label: "1", Subtitle: "Side",
			},
			scale:        1,
			wantLabel:    68,
			wantSubtitle: 18,
		},
		{
			// 0.8 x 400 less 58 of padding is 262, of which 0.9 is 235.8:
			// twenty-three runes fit at 10.
			name:  "a long name shrinks the subtitle into its panel",
			style: defaults,
			target: manager.MonitorSelectTarget{
				Bounds: image.Rect(0, 0, 400, 956), Label: "1", Subtitle: "Built-in Retina Display",
			},
			scale:        1,
			wantLabel:    96,
			wantSubtitle: 10,
		},
		{
			name:  "unset sizes are fitted from the documented defaults",
			style: manager.MonitorSelectStyle{PaddingX: -1, PaddingY: -1},
			target: manager.MonitorSelectTarget{
				Bounds: image.Rect(0, 0, 1470, 956), Label: "1", Subtitle: "Studio",
			},
			scale:        1,
			wantLabel:    96,
			wantSubtitle: 18,
		},
		{
			name:  "a dense display is fitted in font units",
			style: defaults,
			target: manager.MonitorSelectTarget{
				Bounds: image.Rect(0, 0, 2560, 320), Label: "1", Subtitle: "Side",
			},
			scale:        2,
			wantLabel:    68,
			wantSubtitle: 18,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fitted := test.style.FittedTo(test.target, test.scale)

			if fitted.FontSize != test.wantLabel || fitted.SubtitleFontSize != test.wantSubtitle {
				t.Fatalf("FittedTo() sizes = %d, %d, want %d, %d",
					fitted.FontSize, fitted.SubtitleFontSize, test.wantLabel, test.wantSubtitle)
			}

			if fitted.PaddingX != test.style.PaddingX ||
				fitted.FontFamily != test.style.FontFamily {
				t.Fatalf("FittedTo() changed more than the font sizes: %+v", fitted)
			}
		})
	}
}
