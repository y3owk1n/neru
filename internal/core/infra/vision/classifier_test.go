//go:build !darwin || darwin

package vision //nolint:testpackage // uses unexported regionClassifier

import (
	"image"
	"testing"
)

func TestRegionClassifier_Button(t *testing.T) {
	classifier := &regionClassifier{}

	// A typical button: 120x30, centered text, high saliency
	region := DetectedRegion{
		Bounds: image.Rect(100, 100, 220, 130),
		Score:  0.8,
		IsText: true,
		Label:  "Submit",
	}

	role, clickable := classifier.Classify(region)
	if role != roleButton {
		t.Errorf("expected AXButton, got %s", role)
	}

	if !clickable {
		t.Errorf("expected clickable")
	}
}

func TestRegionClassifier_Link(t *testing.T) {
	classifier := &regionClassifier{}

	// A link: wide and short text
	region := DetectedRegion{
		Bounds: image.Rect(50, 200, 300, 230),
		Score:  0.6,
		IsText: true,
		Label:  "Click here for more information",
	}

	role, clickable := classifier.Classify(region)
	if role != "AXLink" {
		t.Errorf("expected AXLink, got %s", role)
	}

	if !clickable {
		t.Errorf("expected clickable")
	}
}

func TestRegionClassifier_StaticText(t *testing.T) {
	classifier := &regionClassifier{}

	// Tall narrow text block (aspect ratio < 1.2) with low score — not a button
	region := DetectedRegion{
		Bounds: image.Rect(10, 10, 40, 60),
		Score:  0.2,
		IsText: true,
		Label:  "Hi",
	}

	role, clickable := classifier.Classify(region)
	if role != "AXStaticText" {
		t.Errorf("expected AXStaticText, got %s", role)
	}

	if clickable {
		t.Errorf("expected non-clickable")
	}
}

func TestRegionClassifier_SmallToolbarIcon(t *testing.T) {
	classifier := &regionClassifier{}

	// A 32x32 square icon: not a checkbox (too big), not a large image.
	// Should be clickable via the button heuristic.
	region := DetectedRegion{
		Bounds: image.Rect(0, 0, 32, 32),
		Score:  0.4,
		IsText: false,
	}

	role, clickable := classifier.Classify(region)
	if role != "AXButton" {
		t.Errorf("expected AXButton for small icon, got %s", role)
	}

	if !clickable {
		t.Errorf("expected clickable for small icon")
	}
}

func TestRegionClassifier_LargeImage(t *testing.T) {
	classifier := &regionClassifier{}

	// A 64x64 region is large enough to be an actual image, not an icon.
	// Falls through checkbox and button heuristics due to size.
	region := DetectedRegion{
		Bounds: image.Rect(0, 0, 64, 64),
		Score:  0.5,
		IsText: false,
	}

	role, clickable := classifier.Classify(region)
	if role != "AXImage" {
		t.Errorf("expected AXImage, got %s", role)
	}

	if clickable {
		t.Errorf("expected non-clickable")
	}
}

func TestRegionClassifier_SmallCheckbox(t *testing.T) {
	classifier := &regionClassifier{}

	region := DetectedRegion{
		Bounds: image.Rect(10, 10, 26, 26),
		Score:  0.3,
		IsText: false,
	}

	role, clickable := classifier.Classify(region)
	if role != "AXCheckBox" {
		t.Errorf("expected AXCheckBox, got %s", role)
	}

	if !clickable {
		t.Errorf("expected clickable")
	}
}

func TestMergeRegions_NonOverlapping(t *testing.T) {
	regions := []DetectedRegion{
		{Bounds: image.Rect(0, 0, 50, 50), Score: 0.9},
		{Bounds: image.Rect(100, 0, 150, 50), Score: 0.8},
	}

	merged := MergeRegions(regions, 0.5)
	if len(merged) != 2 {
		t.Errorf("expected 2 regions, got %d", len(merged))
	}
}

func TestMergeRegions_Overlapping(t *testing.T) {
	regions := []DetectedRegion{
		{Bounds: image.Rect(0, 0, 100, 100), Score: 0.9},
		{Bounds: image.Rect(10, 10, 90, 90), Score: 0.5}, // high IoU with first
	}

	merged := MergeRegions(regions, 0.5)
	if len(merged) != 1 {
		t.Errorf("expected 1 merged region, got %d", len(merged))
	}
}

func TestMergeRegions_PartialOverlap(t *testing.T) {
	regions := []DetectedRegion{
		{Bounds: image.Rect(0, 0, 50, 50), Score: 0.9},
		{Bounds: image.Rect(30, 30, 80, 80), Score: 0.8}, // partial overlap
	}

	merged := MergeRegions(regions, 0.5)
	if len(merged) != 2 {
		t.Errorf("expected 2 regions for partial overlap, got %d", len(merged))
	}
}

func TestTestClassifier(t *testing.T) {
	testClassifier := NewTestClassifier()

	region := DetectedRegion{
		Bounds: image.Rect(100, 100, 200, 130),
		Score:  0.7,
		IsText: true,
		Label:  "OK",
	}

	role, clickable := testClassifier.Classify(region)
	if role == "" {
		t.Errorf("expected non-empty role")
	}
	// Should classify as button (aspect ratio ~3.3, score 0.7, text)
	if role != "AXButton" || !clickable {
		t.Errorf("expected AXButton/clickable, got %s/%v", role, clickable)
	}
}
