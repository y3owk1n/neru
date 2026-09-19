package badge

import (
	"image"
	"math"
)

const (
	// monitorSelectMinLabelSize and monitorSelectMinSubtitleSize are the
	// smallest the picker's two lines shrink to. Neither is ever hidden, because the
	// label is the key that picks the monitor, and the subtitle is its name.
	monitorSelectMinLabelSize    = 12
	monitorSelectMinSubtitleSize = 8
)

// MonitorSelectText is the text of one monitor-select panel, with everything
// that decides how large it may be drawn. Sizes, padding and the gap are in
// font units, the units the configuration is written in.
type MonitorSelectText struct {
	Label    string
	Subtitle string

	LabelFamily    string
	SubtitleFamily string

	// LabelSize and SubtitleSize are the configured sizes, and the ceilings.
	LabelSize    float64
	SubtitleSize float64

	// PadX and PadY are the panel's padding on each side, and Gap what
	// separates the label from the subtitle.
	PadX float64
	PadY float64
	Gap  float64
}

// FitIn returns the sizes to draw the label and the subtitle at so that both
// stay inside the panel, which every backend caps at maxFraction of the
// monitor. The panel was clamped and the text was not, so a label at the
// default 96 points, or a long monitor name, ran out past the panel's edge on
// a small monitor.
//
// monitor is in device pixels and scale is device pixels per font unit, so the
// answer is in font units. Each line is fitted to the panel's width. The label
// then gives up height until the two lines and their padding fit the panel's.
// The label is drawn bold and the subtitle regular, on every platform.
//
// It measures with the platform's text layer. A monitor's name is only known
// when the picker is drawn, so unlike a grid's alphabet it cannot be measured
// when a Style is built. The measurement is remembered per string, and the
// picker is drawn once per activation, not per keystroke.
func (t MonitorSelectText) FitIn(
	monitor image.Rectangle,
	scale, maxFraction float64,
) (float64, float64) {
	if scale <= 0 {
		scale = 1
	}

	availableW := float64(monitor.Dx())/scale*maxFraction - t.PadX*paddingSideCount
	availableH := float64(monitor.Dy())/scale*maxFraction - t.PadY*paddingSideCount

	labelSize := t.LabelSize
	subtitleSize := t.SubtitleSize

	labelMetrics := MeasureLabels([]string{t.Label}, t.LabelFamily, t.LabelSize, true)
	if labelMetrics.WidthPerSize > 0 {
		labelSize = min(labelSize, availableW*fitFill/labelMetrics.WidthPerSize)
	}

	var subtitleH float64

	if t.Subtitle != "" {
		subtitleMetrics := MeasureLabels(
			[]string{t.Subtitle}, t.SubtitleFamily, t.SubtitleSize, false,
		)
		if subtitleMetrics.WidthPerSize > 0 {
			subtitleSize = min(subtitleSize, availableW*fitFill/subtitleMetrics.WidthPerSize)
		}

		subtitleSize = wholeSizeAtLeast(
			subtitleSize,
			scale,
			monitorSelectMinSubtitleSize,
			t.SubtitleSize,
		)
		subtitleH = subtitleSize*subtitleMetrics.HeightPerSize + t.Gap
	}

	if labelMetrics.HeightPerSize > 0 {
		labelSize = min(labelSize, (availableH*fitFill-subtitleH)/labelMetrics.HeightPerSize)
	}

	labelSize = wholeSizeAtLeast(labelSize, scale, monitorSelectMinLabelSize, t.LabelSize)

	return labelSize, subtitleSize
}

// wholeSizeAtLeast rounds a fitted size down to a whole device pixel, the way
// FontFit does, and holds it at a floor that itself never exceeds the
// configured size.
func wholeSizeAtLeast(size, scale, floor, configured float64) float64 {
	size = math.Floor(size*scale) / scale

	return max(size, min(floor, configured))
}
