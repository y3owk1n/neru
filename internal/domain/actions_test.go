package domain

import (
	"slices"
	"testing"
)

func TestKnownActionNames(t *testing.T) {
	names := KnownActionNames()

	// Verify we have the expected number of action names
	expectedCount := 6
	if len(names) != expectedCount {
		t.Errorf("KnownActionNames() returned %d names, want %d", len(names), expectedCount)
	}

	// Verify all expected actions are present
	expectedActions := []ActionName{
		ActionNameLeftClick,
		ActionNameRightClick,
		ActionNameMiddleClick,
		ActionNameMouseDown,
		ActionNameMouseUp,
		ActionNameScroll,
	}

	for _, expected := range expectedActions {
		if !slices.Contains(names, expected) {
			t.Errorf("KnownActionNames() missing expected action: %s", expected)
		}
	}

	// Verify no duplicates
	seen := make(map[ActionName]bool)
	for _, name := range names {
		if seen[name] {
			t.Errorf("KnownActionNames() contains duplicate: %s", name)
		}
		seen[name] = true
	}
}

func TestIsKnownActionName(t *testing.T) {
	tests := []struct {
		name   string
		action ActionName
		want   bool
	}{
		{
			name:   "left_click is known",
			action: ActionNameLeftClick,
			want:   true,
		},
		{
			name:   "right_click is known",
			action: ActionNameRightClick,
			want:   true,
		},
		{
			name:   "middle_click is known",
			action: ActionNameMiddleClick,
			want:   true,
		},
		{
			name:   "mouse_down is known",
			action: ActionNameMouseDown,
			want:   true,
		},
		{
			name:   "mouse_up is known",
			action: ActionNameMouseUp,
			want:   true,
		},
		{
			name:   "scroll is known",
			action: ActionNameScroll,
			want:   true,
		},
		{
			name:   "unknown action",
			action: ActionName("unknown"),
			want:   false,
		},
		{
			name:   "empty string",
			action: ActionName(""),
			want:   false,
		},
		{
			name:   "exec prefix (not a known action name)",
			action: ActionName(ActionPrefixExec),
			want:   false,
		},
		{
			name:   "random string",
			action: ActionName("random_action"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsKnownActionName(tt.action)
			if got != tt.want {
				t.Errorf("IsKnownActionName(%q) = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

// TestKnownActionNames_Consistency verifies that all actions returned by
// KnownActionNames() are recognized by IsKnownActionName().
func TestKnownActionNames_Consistency(t *testing.T) {
	names := KnownActionNames()

	for _, name := range names {
		if !IsKnownActionName(name) {
			t.Errorf(
				"Inconsistency: KnownActionNames() includes %q but IsKnownActionName(%q) returns false",
				name,
				name,
			)
		}
	}
}

// TestActionConstants verifies that action constants have expected values.
func TestActionConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant ActionName
		expected string
	}{
		{"ActionNameLeftClick", ActionNameLeftClick, "left_click"},
		{"ActionNameRightClick", ActionNameRightClick, "right_click"},
		{"ActionNameMiddleClick", ActionNameMiddleClick, "middle_click"},
		{"ActionNameMouseDown", ActionNameMouseDown, "mouse_down"},
		{"ActionNameMouseUp", ActionNameMouseUp, "mouse_up"},
		{"ActionNameScroll", ActionNameScroll, "scroll"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.constant) != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.constant, tt.expected)
			}
		})
	}
}

// TestActionPrefixExec verifies the exec prefix constant.
func TestActionPrefixExec(t *testing.T) {
	if ActionPrefixExec != "exec" {
		t.Errorf("ActionPrefixExec = %q, want %q", ActionPrefixExec, "exec")
	}
}

// TestActionEnumValues verifies that Action enum values are sequential.
func TestActionEnumValues(t *testing.T) {
	// Verify the iota sequence
	if ActionLeftClick != 0 {
		t.Errorf("ActionLeftClick = %d, want 0", ActionLeftClick)
	}
	if ActionRightClick != 1 {
		t.Errorf("ActionRightClick = %d, want 1", ActionRightClick)
	}
	if ActionMouseUp != 2 {
		t.Errorf("ActionMouseUp = %d, want 2", ActionMouseUp)
	}
	if ActionMouseDown != 3 {
		t.Errorf("ActionMouseDown = %d, want 3", ActionMouseDown)
	}
	if ActionMiddleClick != 4 {
		t.Errorf("ActionMiddleClick = %d, want 4", ActionMiddleClick)
	}
	if ActionMoveMouse != 5 {
		t.Errorf("ActionMoveMouse = %d, want 5", ActionMoveMouse)
	}
	if ActionScroll != 6 {
		t.Errorf("ActionScroll = %d, want 6", ActionScroll)
	}
}

// Benchmark tests.
func BenchmarkKnownActionNames(b *testing.B) {
	for b.Loop() {
		_ = KnownActionNames()
	}
}

func BenchmarkIsKnownActionName(b *testing.B) {
	for b.Loop() {
		_ = IsKnownActionName(ActionNameLeftClick)
	}
}

func BenchmarkIsKnownActionName_Unknown(b *testing.B) {
	for b.Loop() {
		_ = IsKnownActionName(ActionName("unknown"))
	}
}
