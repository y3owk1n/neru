//go:build !darwin || darwin

package vision

import (
	"image"
	"testing"
)

// The classifier's answer is a native role name, so every case here runs
// against both vocabularies that have a vision backend: macOS's AX names and
// Linux's AT-SPI names. Asserting a literal "AXButton" would pass on the
// developer's machine and say nothing about the platform the OCR backend runs
// on — which is the bug classifier_roles.go exists to prevent. Each case names
// the field it expects rather than the string, and the vocabulary supplies the
// string.
var classifierVocabularies = map[string]classifierRoles{
	"ax":    axClassifierRoles,
	"atspi": atspiClassifierRoles,
}

func TestRegionClassifier_Classify(t *testing.T) {
	tests := []struct {
		name      string
		region    DetectedRegion
		want      func(classifierRoles) string
		clickable bool
	}{
		{
			// A typical button: 120x30, centered text, high saliency.
			name: "text with button geometry",
			region: DetectedRegion{
				Bounds: image.Rect(100, 100, 220, 130),
				Score:  0.8,
				IsText: true,
				Label:  labelSubmit,
			},
			want:      func(r classifierRoles) string { return r.Button },
			clickable: true,
		},
		{
			// A link: wide and short text.
			name: "wide short text",
			region: DetectedRegion{
				Bounds: image.Rect(50, 200, 300, 230),
				Score:  0.6,
				IsText: true,
				Label:  "Click here for more information",
			},
			want:      func(r classifierRoles) string { return r.Link },
			clickable: true,
		},
		{
			// Tall narrow text block (aspect ratio < 1.2) with low score.
			name: "tall narrow low-confidence text",
			region: DetectedRegion{
				Bounds: image.Rect(10, 10, 40, 60),
				Score:  0.2,
				IsText: true,
				Label:  "Hi",
			},
			want:      func(r classifierRoles) string { return r.StaticText },
			clickable: false,
		},
		{
			// A 32x32 square icon: not a checkbox (too big), not a large image.
			name:      "small square icon",
			region:    DetectedRegion{Bounds: image.Rect(0, 0, 32, 32), Score: 0.4},
			want:      func(r classifierRoles) string { return r.Button },
			clickable: true,
		},
		{
			// A 64x64 region is large enough to be an actual image, not an icon.
			name:      "large square region",
			region:    DetectedRegion{Bounds: image.Rect(0, 0, 64, 64), Score: 0.5},
			want:      func(r classifierRoles) string { return r.Image },
			clickable: false,
		},
		{
			name:      "small square region",
			region:    DetectedRegion{Bounds: image.Rect(10, 10, 26, 26), Score: 0.3},
			want:      func(r classifierRoles) string { return r.CheckBox },
			clickable: true,
		},
		{
			// A region with no area is a bug in the caller rather than a
			// finding about the screen, and the classifier says so instead of
			// guessing.
			name: "degenerate region",
			region: DetectedRegion{
				Bounds: image.Rect(10, 10, 10, 10),
				Score:  0.9,
				IsText: true,
			},
			want:      func(r classifierRoles) string { return r.Unknown },
			clickable: false,
		},
	}

	for vocabulary, roles := range classifierVocabularies {
		for _, test := range tests {
			t.Run(vocabulary+"/"+test.name, func(t *testing.T) {
				classifier := &regionClassifier{roles: roles}

				role, clickable := classifier.Classify(test.region)
				if want := test.want(roles); role != want {
					t.Errorf("role %q, want %q", role, want)
				}

				if clickable != test.clickable {
					t.Errorf("clickable %v, want %v", clickable, test.clickable)
				}
			})
		}
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

	// Should classify as button (aspect ratio ~3.3, score 0.7, text) in
	// whatever vocabulary the running platform speaks.
	if want := currentClassifierRoles().Button; role != want || !clickable {
		t.Errorf("expected %s/clickable, got %s/%v", want, role, clickable)
	}
}
