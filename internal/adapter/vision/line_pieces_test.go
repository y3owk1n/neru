package vision

import (
	"image"
	"image/color"
	"image/draw"
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/ports"
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
	// tesseract sometimes gives a character a box over the one before it, as
	// a "t" covering the "e" of "the".
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
	// Two glyphs, 10 to 20 and 24 to 34, on white. tesseract
	// reads the first one 4 pixels too wide, so halfway between the boxes is
	// column 23, inside the second glyph's ink. The gap is columns 20 to 23.
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
	// halves with a 2-pixel gap, as 测 is. tesseract reads each as a word of
	// its own with boxes that drift right.
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

//nolint:gosmopolitan // The fixtures are Chinese and Korean text.
func TestMergeWideLines_ReplacesOnlyLinesReadAsLatinNoise(t *testing.T) {
	piece := func(text string, left, top int) ports.TextPiece {
		return ports.TextPiece{Text: text, Bounds: image.Rect(left, top, left+20, top+20)}
	}

	// eng read the second line as noise in two pieces.
	detected := [][]ports.TextPiece{
		{piece("hello", 0, 0)},
		{piece("G", 0, 40)},
		{piece("E", 20, 40)},
		{piece("한", 0, 80)},
	}
	wide := [][]ports.TextPiece{
		{piece("hello", 0, 0)},
		{piece("括", 0, 40), piece("号", 20, 40)},
		{piece("韓", 0, 80)},
		{piece("新", 0, 120)},
	}

	merged := mergeWideLines(detected, wide)

	texts := make([]string, 0, len(merged))
	for _, line := range merged {
		var text strings.Builder
		for _, p := range line {
			text.WriteString(p.Text)
		}

		texts = append(texts, text.String())
	}

	want := []string{"hello", "한", "括号", "新"}
	if !slices.Equal(texts, want) {
		t.Fatalf("merged lines = %q, want %q", texts, want)
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
