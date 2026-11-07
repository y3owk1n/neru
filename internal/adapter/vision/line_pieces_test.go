package vision

import (
	"image"
	"image/color"
	"image/draw"
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
