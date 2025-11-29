//go:build unit

package domain_test

import (
	"slices"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain"
)

func TestKnownActionNames(t *testing.T) {
	names := domain.KnownActionNames()

	// Verify we have the expected number of action names
	expectedCount := 6
	if len(names) != expectedCount {
		t.Errorf("KnownActionNames() returned %d names, want %d", len(names), expectedCount)
	}

	// Verify all expected actions are present
	expectedActions := []domain.ActionName{
		domain.ActionNameLeftClick,
		domain.ActionNameRightClick,
		domain.ActionNameMiddleClick,
		domain.ActionNameMouseDown,
		domain.ActionNameMouseUp,
		domain.ActionNameScroll,
	}

	for _, expected := range expectedActions {
		if !slices.Contains(names, expected) {
			t.Errorf("KnownActionNames() missing expected action: %s", expected)
		}
	}

	// Verify no duplicates
	seen := make(map[domain.ActionName]bool)
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
		action domain.ActionName
		want   bool
	}{
		{
			name:   "left_click is known",
			action: domain.ActionNameLeftClick,
			want:   true,
		},
		{
			name:   "right_click is known",
			action: domain.ActionNameRightClick,
			want:   true,
		},
		{
			name:   "middle_click is known",
			action: domain.ActionNameMiddleClick,
			want:   true,
		},
		{
			name:   "mouse_down is known",
			action: domain.ActionNameMouseDown,
			want:   true,
		},
		{
			name:   "mouse_up is known",
			action: domain.ActionNameMouseUp,
			want:   true,
		},
		{
			name:   "scroll is known",
			action: domain.ActionNameScroll,
			want:   true,
		},
		{
			name:   "unknown action",
			action: domain.ActionName("unknown"),
			want:   false,
		},
		{
			name:   "empty string",
			action: domain.ActionName(""),
			want:   false,
		},
		{
			name:   "exec prefix (not a known action name)",
			action: domain.ActionName(domain.ActionPrefixExec),
			want:   false,
		},
		{
			name:   "random string",
			action: domain.ActionName("random_action"),
			want:   false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := domain.IsKnownActionName(testCase.action)
			if got != testCase.want {
				t.Errorf("IsKnownActionName(%q) = %v, want %v", testCase.action, got, testCase.want)
			}
		})
	}
}

// TestKnownActionNames_Consistency verifies that all actions returned by
// KnownActionNames() are recognized by IsKnownActionName().
func TestKnownActionNames_Consistency(t *testing.T) {
	names := domain.KnownActionNames()

	for _, name := range names {
		if !domain.IsKnownActionName(name) {
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
		constant domain.ActionName
		expected string
	}{
		{"ActionNameLeftClick", domain.ActionNameLeftClick, "left_click"},
		{"ActionNameRightClick", domain.ActionNameRightClick, "right_click"},
		{"ActionNameMiddleClick", domain.ActionNameMiddleClick, "middle_click"},
		{"ActionNameMouseDown", domain.ActionNameMouseDown, "mouse_down"},
		{"ActionNameMouseUp", domain.ActionNameMouseUp, "mouse_up"},
		{"ActionNameScroll", domain.ActionNameScroll, "scroll"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if string(testCase.constant) != testCase.expected {
				t.Errorf("%s = %q, want %q", testCase.name, testCase.constant, testCase.expected)
			}
		})
	}
}

// TestActionPrefixExec verifies the exec prefix constant.
func TestActionPrefixExec(t *testing.T) {
	if domain.ActionPrefixExec != "exec" {
		t.Errorf("ActionPrefixExec = %q, want %q", domain.ActionPrefixExec, "exec")
	}
}

// TestActionEnumValues verifies that Action enum values are sequential.
func TestActionEnumValues(t *testing.T) {
	// Verify the iota sequence
	if domain.ActionLeftClick != 0 {
		t.Errorf("ActionLeftClick = %d, want 0", domain.ActionLeftClick)
	}

	if domain.ActionRightClick != 1 {
		t.Errorf("ActionRightClick = %d, want 1", domain.ActionRightClick)
	}

	if domain.ActionMouseUp != 2 {
		t.Errorf("ActionMouseUp = %d, want 2", domain.ActionMouseUp)
	}

	if domain.ActionMouseDown != 3 {
		t.Errorf("ActionMouseDown = %d, want 3", domain.ActionMouseDown)
	}

	if domain.ActionMiddleClick != 4 {
		t.Errorf("ActionMiddleClick = %d, want 4", domain.ActionMiddleClick)
	}

	if domain.ActionMoveMouse != 5 {
		t.Errorf("ActionMoveMouse = %d, want 5", domain.ActionMoveMouse)
	}

	if domain.ActionScroll != 6 {
		t.Errorf("ActionScroll = %d, want 6", domain.ActionScroll)
	}
}
