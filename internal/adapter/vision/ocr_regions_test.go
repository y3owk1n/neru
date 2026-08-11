package vision

import (
	"image"
	"testing"
)

// regionsFromWords is the seam between an OCR engine and the rest of the
// hint pipeline. Everything that can be wrong about a recognized word without
// the engine being wrong lives here: which coordinate space the box is in, what
// the confidence scale is, and which findings are not worth a hint.

// The text these cases pretend an engine read. Nothing depends on the words;
// they are literals in one place so the cases differ only where they mean to.
const (
	labelSave   = "Save"
	labelSubmit = "Submit"
)

func TestRegionsFromWords_TranslatesIntoTheRequestedRegion(t *testing.T) {
	// The engine reads a capture of a window at (400, 300) and reports a word
	// 10 pixels in. The hint has to land at (410, 320) on the screen, not at
	// (10, 20) in the top-left corner of the display.
	words := []recognizedWord{
		{Text: labelSave, Bounds: image.Rect(10, 20, 60, 40), Confidence: 0.9},
	}

	regions := regionsFromWords(
		words,
		image.Rect(400, 300, 900, 700),
		image.Rect(0, 0, 500, 400),
		0,
	)

	if len(regions) != 1 {
		t.Fatalf("got %d regions, want 1", len(regions))
	}

	want := image.Rect(410, 320, 460, 340)
	if regions[0].Bounds != want {
		t.Errorf("bounds %v, want %v", regions[0].Bounds, want)
	}

	if !regions[0].IsText {
		t.Error("an OCR word produced a region that does not claim to be text")
	}

	if regions[0].Label != labelSave {
		t.Errorf("label %q, want %q", regions[0].Label, labelSave)
	}

	if regions[0].Score != 0.9 {
		t.Errorf("score %v, want 0.9", regions[0].Score)
	}
}

// TestRegionsFromWords_UndoesTheOutputScale covers the HiDPI case the capture
// backend documents: a compositor answers a region request in physical pixels,
// so on a 2x output the frame comes back twice the size that was asked for.
// Dividing the box by that ratio is the difference between hints landing on
// their text and landing a quarter of the way up the window.
func TestRegionsFromWords_UndoesTheOutputScale(t *testing.T) {
	words := []recognizedWord{
		{Text: "OK", Bounds: image.Rect(100, 200, 300, 260), Confidence: 0.8},
	}

	// A 500x400 region captured at 2x comes back as 1000x800.
	regions := regionsFromWords(words, image.Rect(0, 0, 500, 400), image.Rect(0, 0, 1000, 800), 0)

	if len(regions) != 1 {
		t.Fatalf("got %d regions, want 1", len(regions))
	}

	want := image.Rect(50, 100, 150, 130)
	if regions[0].Bounds != want {
		t.Errorf("bounds %v, want %v", regions[0].Bounds, want)
	}
}

func TestRegionsFromWords_DropsFindingsNotWorthAHint(t *testing.T) {
	tests := []struct {
		name string
		word recognizedWord
	}{
		{"empty text", recognizedWord{Text: "", Bounds: image.Rect(0, 0, 10, 10), Confidence: 0.9}},
		{
			"whitespace only",
			recognizedWord{Text: "  \n\t ", Bounds: image.Rect(0, 0, 10, 10), Confidence: 0.9},
		},
		{
			"below the confidence floor",
			recognizedWord{Text: labelSave, Bounds: image.Rect(0, 0, 10, 10), Confidence: 0.2},
		},
		{
			"zero-area box",
			recognizedWord{Text: labelSave, Bounds: image.Rect(5, 5, 5, 5), Confidence: 0.9},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			regions := regionsFromWords(
				[]recognizedWord{test.word},
				image.Rect(0, 0, 100, 100),
				image.Rect(0, 0, 100, 100),
				0.5,
			)
			if len(regions) != 0 {
				t.Errorf("got %d regions, want none: %+v", len(regions), regions)
			}
		})
	}
}

// TestRegionsFromWords_KeepsAWordExactlyAtTheConfidenceFloor pins the boundary
// as inclusive, so a user who writes their observed minimum keeps the word they
// wrote it for.
func TestRegionsFromWords_KeepsAWordExactlyAtTheConfidenceFloor(t *testing.T) {
	words := []recognizedWord{
		{Text: labelSave, Bounds: image.Rect(0, 0, 40, 10), Confidence: 0.5},
	}

	regions := regionsFromWords(words, image.Rect(0, 0, 100, 100), image.Rect(0, 0, 100, 100), 0.5)
	if len(regions) != 1 {
		t.Fatalf("got %d regions, want 1", len(regions))
	}
}

// TestRegionsFromWords_RefusesAnUnmappableRegion is the safety case: with no
// region to translate into there is no correct answer, and returning boxes in
// image-local coordinates would put hints on the wrong part of the screen
// rather than nowhere.
func TestRegionsFromWords_RefusesAnUnmappableRegion(t *testing.T) {
	words := []recognizedWord{
		{Text: labelSave, Bounds: image.Rect(0, 0, 40, 10), Confidence: 0.9},
	}

	for _, test := range []struct {
		name        string
		region      image.Rectangle
		imageBounds image.Rectangle
	}{
		{"empty region", image.Rectangle{}, image.Rect(0, 0, 100, 100)},
		{"empty image", image.Rect(0, 0, 100, 100), image.Rectangle{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if regions := regionsFromWords(
				words,
				test.region,
				test.imageBounds,
				0,
			); regions != nil {
				t.Errorf("got %v, want nil", regions)
			}
		})
	}
}

// TestRegionsFromWords_ClipsToTheRequestedRegion keeps a rounding error at the
// far edge of a scaled capture from producing a hint one pixel outside the
// window that was asked about.
func TestRegionsFromWords_ClipsToTheRequestedRegion(t *testing.T) {
	words := []recognizedWord{
		{Text: "Edge", Bounds: image.Rect(90, 90, 130, 130), Confidence: 0.9},
	}

	regions := regionsFromWords(words, image.Rect(0, 0, 100, 100), image.Rect(0, 0, 100, 100), 0)
	if len(regions) != 1 {
		t.Fatalf("got %d regions, want 1", len(regions))
	}

	want := image.Rect(90, 90, 100, 100)
	if regions[0].Bounds != want {
		t.Errorf("bounds %v, want %v", regions[0].Bounds, want)
	}
}

// elementsFromRegions is the last shared step before the hint pipeline sees a
// vision result, and it is shared because both backends reach it the same way:
// a classified region becomes an element whose role is native, whose title is
// the recognized text, and whose ID is stable for the same box.

func TestElementsFromRegions_CarriesTheClassificationOntoTheElement(t *testing.T) {
	classifier := &regionClassifier{roles: atspiClassifierRoles}

	regions := []DetectedRegion{
		{Bounds: image.Rect(100, 100, 220, 130), Score: 0.8, IsText: true, Label: labelSubmit},
	}

	elements, skipped := elementsFromRegions(regions, classifier)
	if skipped != 0 {
		t.Errorf("skipped %d regions, want 0", skipped)
	}

	if len(elements) != 1 {
		t.Fatalf("got %d elements, want 1", len(elements))
	}

	if got := string(elements[0].Role()); got != atspiClassifierRoles.Button {
		t.Errorf("role %q, want %q", got, atspiClassifierRoles.Button)
	}

	if elements[0].Title() != labelSubmit {
		t.Errorf("title %q, want %q", elements[0].Title(), labelSubmit)
	}

	if !elements[0].IsClickable() {
		t.Error("a classified button produced a non-clickable element")
	}
}

func TestElementsFromRegions_SkipsDegenerateRegions(t *testing.T) {
	classifier := &regionClassifier{roles: axClassifierRoles}

	regions := []DetectedRegion{
		{Bounds: image.Rect(5, 5, 5, 5), Score: 0.9, IsText: true, Label: "gone"},
		{Bounds: image.Rect(0, 0, 120, 30), Score: 0.9, IsText: true, Label: "kept"},
	}

	elements, skipped := elementsFromRegions(regions, classifier)
	if len(elements) != 1 {
		t.Fatalf("got %d elements, want 1", len(elements))
	}

	if skipped != 1 {
		t.Errorf("skipped %d, want 1", skipped)
	}
}

// TestElementsFromRegions_GivesTheSameBoxTheSameID keeps hint identity stable
// across a refresh that finds the same control again.
func TestElementsFromRegions_GivesTheSameBoxTheSameID(t *testing.T) {
	classifier := &regionClassifier{roles: axClassifierRoles}
	region := DetectedRegion{
		Bounds: image.Rect(10, 20, 130, 50),
		Score:  0.9,
		IsText: true,
		Label:  "OK",
	}

	first, _ := elementsFromRegions([]DetectedRegion{region}, classifier)
	second, _ := elementsFromRegions([]DetectedRegion{region}, classifier)

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("got %d and %d elements, want 1 each", len(first), len(second))
	}

	if first[0].ID() != second[0].ID() {
		t.Errorf("IDs %q and %q differ for the same box", first[0].ID(), second[0].ID())
	}
}
