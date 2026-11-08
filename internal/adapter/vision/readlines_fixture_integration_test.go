//go:build integration && (darwin || linux)

package vision

// inkCenter is where a character's ink is centered on a fixture line, in
// points, measured by drawing it alone in red in its place.
type inkCenter struct {
	char string
	x    float64
}

// cjkInk is each character of each line of cjk-16pt-2x.png with its ink
// center, spaces left out.
//
//nolint:gosmopolitan
var cjkInk = [][]inkCenter{
	{
		{"今", 19.8},
		{"天", 35.5},
		{"的", 51.5},
		{"天", 67.2},
		{"气", 83.2},
		{"很", 99.2},
		{"好", 115.2},
		{"，", 130.8},
		{"我", 146.8},
		{"们", 162.5},
		{"去", 178.5},
		{"公", 194.5},
		{"园", 210.2},
		{"散", 226.2},
		{"步", 242.0},
		{"。", 258.2},
	},
	{
		{"「", 17.0},
		{"中", 28.0},
		{"文", 44.0},
		{"」", 55.0},
		{"测", 73.0},
		{"试", 89.2},
		{"（", 107.0},
		{"括", 118.2},
		{"号", 134.0},
		{"）", 145.0},
		{"与", 162.0},
		{"E", 177.8},
		{"n", 186.8},
		{"g", 195.5},
		{"l", 202.2},
		{"i", 206.0},
		{"s", 212.0},
		{"h", 220.5},
		{"混", 234.8},
		{"合", 250.5},
	},
	{
		{"日", 20.0},
		{"本", 35.2},
		{"語", 51.8},
		{"の", 67.0},
		{"テ", 82.2},
		{"キ", 97.5},
		{"ス", 112.5},
		{"ト", 129.0},
	},
}

// maxMeanInkOffset is how far, in points, a box's center may be from its
// character's ink on average.
const maxMeanInkOffset = 1.25
