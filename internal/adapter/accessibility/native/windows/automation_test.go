//go:build windows

package windows

import (
	"image"
	"maps"
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/element"
)

// Unit tests for the pure UIA control-type naming used by hint enumeration.
// Does not exercise live UIA (see accessibility integration tests on WIN-VM).
func TestControlTypeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		controlType int32
		wantName    string
		wantKnown   bool
	}{
		{"button", 50000, uiaControlButton, true},
		{"checkbox", 50002, uiaControlCheckBox, true},
		{"combobox", 50003, "ComboBox", true},
		{"edit", 50004, uiaControlEdit, true},
		{"hyperlink", 50005, uiaControlHyperlink, true},
		{"menu item", 50011, "MenuItem", true},
		{"radio button", 50013, "RadioButton", true},
		{"tab item", 50019, "TabItem", true},
		{"split button", 50031, uiaControlSplitButton, true},
		// Control types that were previously discarded are now named, so a
		// config can address them through the uia: prefix.
		{"text", 50020, "Text", true},
		{"custom", 50025, uiaControlCustom, true},
		{"pane", 50033, uiaControlPane, true},
		{"document", 50030, "Document", true},
		{"last known control type", 50040, "AppBar", true},
		{"unknown control type", 99999, roleUnknown, false},
		{"zero control type", 0, roleUnknown, false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			name, known := controlTypeName(testCase.controlType)
			if name != testCase.wantName || known != testCase.wantKnown {
				t.Fatalf(
					"controlTypeName(%d) = (%q, %v), want (%q, %v)",
					testCase.controlType,
					name,
					known,
					testCase.wantName,
					testCase.wantKnown,
				)
			}
		})
	}
}

// TestControlTypeNamesAreContiguous guards the transcription of the
// UIA_*ControlTypeId range. A gap or duplicate would mean an id was mistyped,
// which silently misnames a control type.
func TestControlTypeNamesAreContiguous(t *testing.T) {
	t.Parallel()

	const first, last = 50000, 50040

	seen := make(map[string]int32, len(controlTypeNames))

	for controlType := int32(first); controlType <= last; controlType++ {
		name, ok := controlTypeNames[controlType]
		if !ok {
			t.Errorf("controlTypeNames lacks id %d", controlType)

			continue
		}

		if previous, duplicate := seen[name]; duplicate {
			t.Errorf("controlTypeNames has %q for both %d and %d", name, previous, controlType)
		}

		seen[name] = controlType
	}

	if len(controlTypeNames) != last-first+1 {
		t.Errorf(
			"controlTypeNames has %d entries, want %d",
			len(controlTypeNames), last-first+1,
		)
	}
}

// TestControlTypeNamesCoverTheVocabulary pins enumeration against the semantic
// vocabulary: every UIA name a semantic role expands to must be a name the
// enumerator can actually produce, or that role would never match anything.
func TestControlTypeNamesCoverTheVocabulary(t *testing.T) {
	t.Parallel()

	names := slices.Collect(maps.Values(controlTypeNames))

	for _, mapping := range element.RoleVocabulary {
		for _, native := range mapping.UIA {
			if !slices.Contains(names, native) {
				t.Errorf(
					"semantic role %q expands to UIA control type %q, which the "+
						"enumerator never produces",
					mapping.Semantic, native,
				)
			}
		}
	}
}

// TestClipToFrame_DropsControlsLaidOutPastTheWindowEdge checks the clip that
// stops a scrolled Edge vertical tab strip from placing hints below the window,
// and that a control straddling the edge keeps only its visible part.
func TestClipToFrame_DropsControlsLaidOutPastTheWindowEdge(t *testing.T) {
	t.Parallel()

	frame := image.Rect(0, 0, 1400, 660)

	tests := []struct {
		name   string
		bounds image.Rectangle
		frame  image.Rectangle
		want   image.Rectangle
	}{
		{
			name:   "inside",
			bounds: image.Rect(10, 10, 50, 40),
			frame:  frame,
			want:   image.Rect(10, 10, 50, 40),
		},
		{
			name:   "straddling the bottom edge",
			bounds: image.Rect(10, 640, 50, 700),
			frame:  frame,
			want:   image.Rect(10, 640, 50, 660),
		},
		{
			name:   "below the window",
			bounds: image.Rect(10, 700, 50, 740),
			frame:  frame,
			want:   image.Rectangle{},
		},
		{
			name:   "unknown frame keeps everything",
			bounds: image.Rect(10, 700, 50, 740),
			want:   image.Rect(10, 700, 50, 740),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := clipToFrame(tt.bounds, tt.frame); got != tt.want {
				t.Errorf("clipToFrame(%v, %v) = %v, want %v", tt.bounds, tt.frame, got, tt.want)
			}
		})
	}
}

// TestSameControl_MatchesOnTypeNameAndBounds checks the identity the visible
// check uses when walking up from a hit element: a covering panel, an
// overlapping sibling and a same-shaped neighbor with another name are all
// rejected, so an occluded control is not mistaken for visible.
func TestSameControl_MatchesOnTypeNameAndBounds(t *testing.T) {
	t.Parallel()

	control := winElement{
		bounds: image.Rect(100, 100, 200, 140),
		role:   uiaControlButton,
		name:   "Save",
	}

	tests := []struct {
		name      string
		candidate winElement
		want      bool
	}{
		{name: "the control itself", candidate: control, want: true},
		{
			name:      "a covering panel",
			candidate: winElement{bounds: image.Rect(0, 0, 400, 400), role: uiaControlPane},
			want:      false,
		},
		{
			name: "an overlapping sibling",
			candidate: winElement{
				bounds: image.Rect(150, 90, 300, 150),
				role:   uiaControlButton,
				name:   "Cancel",
			},
			want: false,
		},
		{
			name:      "a same-shaped overlay",
			candidate: winElement{bounds: control.bounds, role: uiaControlCustom, name: "Save"},
			want:      false,
		},
		{
			name:      "a same-shaped neighbor with another name",
			candidate: winElement{bounds: control.bounds, role: uiaControlButton, name: "Delete"},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := sameControl(tt.candidate, control); got != tt.want {
				t.Errorf("sameControl(%+v, control) = %v, want %v", tt.candidate, got, tt.want)
			}
		})
	}
}

// TestPackPoint_LaysOutAWin32POINT pins the register layout ElementFromPoint
// reads: x in the low 32 bits, y in the high 32 bits, negative coordinates
// (a monitor left of the primary) kept as two's complement.
func TestPackPoint_LaysOutAWin32POINT(t *testing.T) {
	t.Parallel()

	if got, want := packPoint(image.Pt(3, 5)), uintptr(3)|uintptr(5)<<32; got != want {
		t.Errorf("packPoint(3, 5) = %#x, want %#x", got, want)
	}

	if got, want := packPoint(image.Pt(-1, 2)), uintptr(0xFFFFFFFF)|uintptr(2)<<32; got != want {
		t.Errorf("packPoint(-1, 2) = %#x, want %#x", got, want)
	}
}
