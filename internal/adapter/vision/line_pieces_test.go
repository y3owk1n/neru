package vision

import (
	"image"
	"image/color"
	"image/draw"
	"slices"
	"testing"
)

func TestPiecesFromSymbols_LaysCharactersOutAsCellsPerLineAndWord(t *testing.T) {
	// Two words on one line and one on the next, in an image twice the size
	// of the region it shows. Each character's ink is 8 wide, 4 apart.
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	symbols := []lineSymbol{
		{text: "a", ink: image.Rect(10, 10, 18, 30), line: 0, word: 0},
		{text: "b", ink: image.Rect(22, 12, 30, 30), line: 0, word: 0},
		{text: "c", ink: image.Rect(50, 10, 58, 30), line: 0, word: 1},
		{text: "d", ink: image.Rect(10, 50, 18, 70), line: 1, word: 0},
	}

	lines := piecesFromSymbols(symbols, img, image.Rect(1000, 500, 1100, 550))

	if len(lines) != 2 || len(lines[0]) != 3 || len(lines[1]) != 1 {
		t.Fatalf("lines = %v, want three pieces then one", lines)
	}

	first, second, third := lines[0][0], lines[0][1], lines[0][2]
	if first.Word != 0 || second.Word != 0 || third.Word != 1 {
		t.Fatalf("word numbers = %d %d %d, want 0 0 1", first.Word, second.Word, third.Word)
	}

	// a's and b's cells meet halfway between their ink, at 20 in the image,
	// 1010 in the region, and both span the word's height, 10 to 30.
	if first.Bounds.Max.X != second.Bounds.Min.X || first.Bounds.Max.X != 1010 {
		t.Fatalf(
			"a ends at %d and b starts at %d, want both 1010",
			first.Bounds.Max.X,
			second.Bounds.Min.X,
		)
	}

	if first.Bounds.Min.Y != 505 || second.Bounds.Min.Y != 505 || first.Bounds.Max.Y != 515 {
		t.Fatalf(
			"a spans %v and b %v, want the word's height, 505 to 515",
			first.Bounds,
			second.Bounds,
		)
	}
}

func TestPiecesFromSymbols_KeepsAWordWithCrossedInkWhole(t *testing.T) {
	// A recognizer can give a character a box over the one before it, as a
	// "t" covering the "e" of "the".
	img := image.NewRGBA(image.Rect(0, 0, 100, 40))
	symbols := []lineSymbol{
		{text: "h", ink: image.Rect(10, 10, 18, 30), line: 0, word: 0},
		{text: "e", ink: image.Rect(22, 14, 29, 30), line: 0, word: 0},
		{text: "t", ink: image.Rect(19, 12, 39, 30), line: 0, word: 0},
	}

	lines := piecesFromSymbols(symbols, img, img.Rect)

	if len(lines) != 1 || len(lines[0]) != 1 {
		t.Fatalf("lines = %v, want the word as one piece", lines)
	}

	if piece := lines[0][0]; piece.Text != "het" || piece.Bounds != image.Rect(10, 10, 39, 30) {
		t.Fatalf("piece = %q at %v, want \"het\" over the whole word", piece.Text, piece.Bounds)
	}
}

func TestPiecesFromSymbols_MovesABoundaryInsideACharacterToTheGap(t *testing.T) {
	// Two glyphs, 10 to 20 and 24 to 34, on white. The recognizer reads the
	// first one 4 pixels too wide, so halfway between the boxes is column 23,
	// inside the second glyph's ink. The gap is columns 20 to 23.
	img := image.NewRGBA(image.Rect(0, 0, 60, 40))
	draw.Draw(img, img.Rect, image.NewUniform(color.White), image.Point{}, draw.Src)

	for _, glyph := range []image.Rectangle{image.Rect(10, 10, 20, 30), image.Rect(24, 10, 34, 30)} {
		drawGlyph(img, glyph)
	}

	symbols := []lineSymbol{
		{text: "a", ink: image.Rect(10, 10, 22, 30), line: 0, word: 0},
		{text: "b", ink: image.Rect(24, 10, 34, 30), line: 0, word: 0},
	}

	lines := piecesFromSymbols(symbols, img, img.Rect)

	if len(lines) != 1 || len(lines[0]) != 2 {
		t.Fatalf("lines = %v, want two pieces", lines)
	}

	boundary := lines[0][0].Bounds.Max.X
	if boundary < 20 || boundary > 23 || lines[0][1].Bounds.Min.X != boundary {
		t.Fatalf(
			"a ends at %d and b starts at %d, want both in the gap, 20 to 23",
			boundary,
			lines[0][1].Bounds.Min.X,
		)
	}
}

//nolint:gosmopolitan // The fixtures are Chinese text.
func TestPiecesFromSymbols_PlacesWideCharactersFromTheirInk(t *testing.T) {
	// Three 20-pixel characters on a 24-pixel pitch, the middle one in two
	// halves with a 2-pixel gap, as 测 is. The recognizer reads each as a word
	// of its own with boxes that drift right.
	img := image.NewRGBA(image.Rect(0, 0, 100, 40))
	draw.Draw(img, img.Rect, image.NewUniform(color.White), image.Point{}, draw.Src)

	for _, ink := range []image.Rectangle{
		image.Rect(10, 10, 30, 30),
		image.Rect(34, 10, 42, 30),
		image.Rect(44, 10, 54, 30),
		image.Rect(58, 10, 78, 30),
	} {
		drawGlyph(img, ink)
	}

	symbols := []lineSymbol{
		{text: "今", ink: image.Rect(10, 10, 30, 30), line: 0, word: 0},
		{text: "测", ink: image.Rect(44, 10, 56, 30), line: 0, word: 1},
		{text: ",", ink: image.Rect(70, 10, 78, 30), line: 0, word: 2},
	}

	lines := piecesFromSymbols(symbols, img, img.Rect)

	if len(lines) != 1 || len(lines[0]) != 3 {
		t.Fatalf("lines = %v, want three pieces", lines)
	}

	// Each cell starts where the one before ends, and the boundaries between
	// characters fall in the gaps, 30 to 34 and 54 to 58, never in the gap
	// inside 测.
	texts := []string{"今", "测", "，"}
	gaps := [][2]int{{30, 34}, {54, 58}}

	for index, piece := range lines[0] {
		if piece.Word != 0 {
			t.Fatalf("piece %d is in word %d, want the run as one word, 0", index, piece.Word)
		}

		if piece.Text != texts[index] {
			t.Fatalf("piece %d is %q, want %q", index, piece.Text, texts[index])
		}

		if index == 0 {
			continue
		}

		edge, gap := piece.Bounds.Min.X, gaps[index-1]
		if edge != lines[0][index-1].Bounds.Max.X || edge < gap[0] || edge > gap[1] {
			t.Fatalf("%q starts at %d after one ending at %d, want both in the gap %d to %d",
				piece.Text, edge, lines[0][index-1].Bounds.Max.X, gap[0], gap[1])
		}
	}
}

func TestPiecesFromSymbols_KeepsWideWordsApartAtTheirSpace(t *testing.T) {
	// Korean has spaces between words, 18 pixels here against characters 20
	// high. Two such words stay two words.
	img := image.NewRGBA(image.Rect(0, 0, 140, 40))
	draw.Draw(img, img.Rect, image.NewUniform(color.White), image.Point{}, draw.Src)

	inks := []image.Rectangle{
		image.Rect(10, 10, 30, 30), image.Rect(32, 10, 52, 30),
		image.Rect(70, 10, 90, 30), image.Rect(92, 10, 112, 30),
	}
	for _, ink := range inks {
		drawGlyph(img, ink)
	}

	symbols := []lineSymbol{
		{text: "한", ink: inks[0], line: 0, word: 0},
		{text: "국", ink: inks[1], line: 0, word: 0},
		{text: "텍", ink: inks[2], line: 0, word: 1},
		{text: "스", ink: inks[3], line: 0, word: 1},
	}

	lines := piecesFromSymbols(symbols, img, img.Rect)

	if len(lines) != 1 || len(lines[0]) != 4 {
		t.Fatalf("lines = %v, want four pieces", lines)
	}

	words := []int{lines[0][0].Word, lines[0][1].Word, lines[0][2].Word, lines[0][3].Word}
	if !slices.Equal(words, []int{0, 0, 1, 1}) {
		t.Fatalf("word numbers = %v, want two words of two, 0 0 1 1", words)
	}

	if gap := lines[0][2].Bounds.Min.X - lines[0][1].Bounds.Max.X; gap <= 0 {
		t.Fatalf("the words touch, %d apart, want the space between them kept", gap)
	}
}

//nolint:gosmopolitan // The fixtures are Chinese text.
func TestPiecesFromSymbols_PlacesWideCharactersOnAGradient(t *testing.T) {
	// Three 20-pixel characters on a 24-pixel pitch, each drawn as three white
	// strokes as 三 is, over a dark gradient like the one behind a video title.
	// Under the run the gradient spreads over nine lumas while the text sits
	// on one, so the text's luma is the most common there. The recognizer
	// gives each character a narrow box of its own.
	img := image.NewRGBA(image.Rect(0, 0, 200, 40))
	for x := range 200 {
		shade := uint8(30 + x*32/200)
		for y := range 40 {
			img.SetRGBA(x, y, color.RGBA{R: shade, G: shade, B: shade, A: 0xFF})
		}
	}

	for _, left := range []int{50, 74, 98} {
		for _, top := range []int{10, 18, 27} {
			stroke := image.Rect(left, top, left+20, top+3)
			draw.Draw(img, stroke, image.NewUniform(color.White), image.Point{}, draw.Src)
		}
	}

	symbols := []lineSymbol{
		{text: "今", ink: image.Rect(52, 10, 58, 30), line: 0, word: 0},
		{text: "天", ink: image.Rect(76, 10, 82, 30), line: 0, word: 1},
		{text: "好", ink: image.Rect(100, 10, 106, 30), line: 0, word: 2},
	}

	lines := piecesFromSymbols(symbols, img, img.Rect)
	if len(lines) != 1 || len(lines[0]) != 3 {
		t.Fatalf("lines = %v, want three pieces", lines)
	}

	// Each cell holds its own character's center, 60, 84 and 108, and the
	// cells stay on the ink, 50 to 118, rather than spreading across the
	// gradient to the frame's edges.
	for index, piece := range lines[0] {
		center := 60 + 24*index
		if center < piece.Bounds.Min.X || center >= piece.Bounds.Max.X ||
			piece.Bounds.Min.X < 48 || piece.Bounds.Max.X > 120 {
			t.Fatalf("%q spans %d to %d, want it around %d within the ink, 50 to 118",
				piece.Text, piece.Bounds.Min.X, piece.Bounds.Max.X, center)
		}
	}
}

// drawGlyph draws a stand-in for a character over box: its outline, two
// pixels thick, so like real text it leaves most of its box background.
func drawGlyph(img *image.RGBA, box image.Rectangle) {
	draw.Draw(img, box, image.NewUniform(color.Black), image.Point{}, draw.Src)
	draw.Draw(img, box.Inset(2), image.NewUniform(color.White), image.Point{}, draw.Src)
}

func TestBackgroundOf_IsTheColorAroundTheBox(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	draw.Draw(
		img,
		img.Rect,
		image.NewUniform(color.RGBA{R: 0x20, G: 0x40, B: 0xF0, A: 0xFF}),
		image.Point{},
		draw.Src,
	)
	draw.Draw(
		img,
		image.Rect(15, 15, 25, 25),
		image.NewUniform(color.Black),
		image.Point{},
		draw.Src,
	)

	if got := backgroundOf(img, image.Rect(10, 10, 30, 30)); got != 0x2040F0 {
		t.Fatalf("background = %06x, want 2040f0", got)
	}

	if got := backgroundOf(img, image.Rect(100, 100, 120, 120)); got != 0 {
		t.Fatalf("background outside the image = %06x, want 0", got)
	}
}

// TestBordersOf_FindsALineAcrossTheLineButNotText covers the borders a word
// is kept from joining across: a line across its whole line counts, and the
// ink of other text or a link's underline under one word does not.
func TestBordersOf_FindsALineAcrossTheLineButNotText(t *testing.T) {
	white, gray, ink := color.RGBA{
		255,
		255,
		255,
		255,
	}, color.RGBA{
		0xD2,
		0xD2,
		0xD0,
		255,
	}, color.RGBA{
		20,
		20,
		20,
		255,
	}

	fill := func(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
		for y := rect.Min.Y; y < rect.Max.Y; y++ {
			for x := rect.Min.X; x < rect.Max.X; x++ {
				img.SetRGBA(x, y, c)
			}
		}
	}

	page := func() *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 200, 120))
		fill(img, img.Rect, white)

		return img
	}

	upper, lower := image.Rect(20, 20, 60, 30), image.Rect(20, 60, 60, 70)
	span := [2]int{20, 160}

	tests := []struct {
		name         string
		draw         func(*image.RGBA)
		box          image.Rectangle
		above, below int
	}{
		{
			name:  "a border between two lines, from the line above",
			draw:  func(img *image.RGBA) { fill(img, image.Rect(0, 45, 200, 46), gray) },
			box:   upper,
			below: 16,
		},
		{
			name:  "a border between two lines, from the line below",
			draw:  func(img *image.RGBA) { fill(img, image.Rect(0, 45, 200, 46), gray) },
			box:   lower,
			above: 15,
		},
		{
			name: "an underline under one word is no border",
			draw: func(img *image.RGBA) { fill(img, image.Rect(20, 32, 60, 33), ink) },
			box:  upper,
		},
		{
			name: "the line of text above is no border",
			draw: func(img *image.RGBA) {
				for x := 20; x < 160; x += 9 {
					fill(img, image.Rect(x, 20, x+5, 30), ink)
				}
			},
			box: lower,
		},
		{name: "a plain page has none", draw: func(*image.RGBA) {}, box: upper},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			img := page()
			test.draw(img)

			above, below := bordersOf(img, test.box, span)
			if above != test.above || below != test.below {
				t.Fatalf(
					"borders = %d above, %d below, want %d, %d",
					above,
					below,
					test.above,
					test.below,
				)
			}
		})
	}
}
