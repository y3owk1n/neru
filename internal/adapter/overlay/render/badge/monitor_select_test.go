package badge_test

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/render/badge"
)

func TestMonitorSelectText_FitIn(t *testing.T) {
	// With squareMeasurer a rune is as wide as the font is tall, and a line as
	// tall. The panel is 0.8 of the monitor and a line fills 0.9 of what the
	// padding leaves.
	installMeasurer(t, squareMeasurer{})

	base := badge.MonitorSelectText{
		Label:        "1",
		Subtitle:     "Studio",
		LabelSize:    96,
		SubtitleSize: 18,
		PadX:         30,
		PadY:         15,
		Gap:          4,
	}

	tests := []struct {
		name         string
		text         badge.MonitorSelectText
		monitor      image.Rectangle
		scale        float64
		wantLabel    float64
		wantSubtitle float64
	}{
		{
			name:         "a roomy monitor draws both as configured",
			text:         base,
			monitor:      image.Rect(0, 0, 1920, 1080),
			scale:        1,
			wantLabel:    96,
			wantSubtitle: 18,
		},
		{
			// 0.8 x 150 less 30 of padding is 90, of which 0.9 is 81. The
			// subtitle and the gap take 22 of it, leaving the label 59.
			name:         "a short monitor takes height from the label",
			text:         base,
			monitor:      image.Rect(0, 0, 1920, 150),
			scale:        1,
			wantLabel:    59,
			wantSubtitle: 18,
		},
		{
			// 0.8 x 250 less 60 of padding is 140, of which 0.9 is 126: twenty
			// runes fit at 6.3, which is under the subtitle's floor.
			name: "a long name shrinks to its floor and no further",
			text: func() badge.MonitorSelectText {
				long := base
				long.Subtitle = "An Unreasonably Long"

				return long
			}(),
			monitor:      image.Rect(0, 0, 250, 1080),
			scale:        1,
			wantLabel:    96,
			wantSubtitle: 8,
		},
		{
			name: "a name that nearly fits shrinks to fit the panel's width",
			text: func() badge.MonitorSelectText {
				long := base
				long.Subtitle = "Twelve Runes"

				return long
			}(),
			// 0.8 x 300 less 60 is 180, of which 0.9 is 162: twelve runes at 13.5.
			monitor:      image.Rect(0, 0, 300, 1080),
			scale:        1,
			wantLabel:    96,
			wantSubtitle: 13,
		},
		{
			name:         "a dense display is fitted in font units",
			text:         base,
			monitor:      image.Rect(0, 0, 3840, 300),
			scale:        2,
			wantLabel:    59,
			wantSubtitle: 18,
		},
		{
			name: "no subtitle leaves the label the whole panel",
			text: func() badge.MonitorSelectText {
				bare := base
				bare.Subtitle = ""

				return bare
			}(),
			monitor:      image.Rect(0, 0, 1920, 150),
			scale:        1,
			wantLabel:    81,
			wantSubtitle: 18,
		},
		{
			name: "a configured size under the floor is drawn as configured",
			text: func() badge.MonitorSelectText {
				small := base
				small.LabelSize = 10
				small.SubtitleSize = 6

				return small
			}(),
			monitor:      image.Rect(0, 0, 1920, 1080),
			scale:        1,
			wantLabel:    10,
			wantSubtitle: 6,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			label, subtitle := test.text.FitIn(test.monitor, test.scale, 0.8)

			if label != test.wantLabel || subtitle != test.wantSubtitle {
				t.Fatalf("FitIn() = %v, %v, want %v, %v",
					label, subtitle, test.wantLabel, test.wantSubtitle)
			}
		})
	}
}
