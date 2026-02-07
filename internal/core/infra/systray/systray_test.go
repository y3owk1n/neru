package systray_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/systray"
)

func TestMain(m *testing.M) {
	m.Run()
}

func resetState(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		systray.ResetForTesting()
	})
}

func TestAddMenuItem(t *testing.T) {
	resetState(t)

	item := systray.AddMenuItem("Test Item")
	if item == nil {
		t.Fatal("AddMenuItem returned nil")
	}

	if item.Title() != "Test Item" {
		t.Errorf("Expected title 'Test Item', got '%s'", item.Title())
	}

	if item.ClickedCh == nil {
		t.Error("ClickedCh is nil")
	}
}

func TestAddSubMenuItem(t *testing.T) {
	resetState(t)

	parent := systray.AddMenuItem("Parent")
	child := parent.AddSubMenuItem("Child")

	if child == nil {
		t.Fatal("AddSubMenuItem returned nil")
	}

	if child.Title() != "Child" {
		t.Errorf("Expected title 'Child', got '%s'", child.Title())
	}

	if child.ClickedCh == nil {
		t.Error("Child ClickedCh is nil")
	}
}

func TestMenuItemMethods(t *testing.T) {
	resetState(t)

	item := systray.AddMenuItem("Test")

	item.SetTitle("New Title")

	if item.Title() != "New Title" {
		t.Errorf("Expected title 'New Title', got '%s'", item.Title())
	}

	item.Enable()

	if item.Disabled() {
		t.Error("Expected Disabled() to be false after Enable()")
	}

	item.Disable()

	if !item.Disabled() {
		t.Error("Expected Disabled() to be true after Disable()")
	}

	item.Check()

	if !item.Checked() {
		t.Error("Expected Checked() to be true after Check()")
	}

	item.Uncheck()

	if item.Checked() {
		t.Error("Expected Checked() to be false after Uncheck()")
	}

	item.Show()

	if item.Hidden() {
		t.Error("Expected Hidden() to be false after Show()")
	}

	item.Hide()

	if !item.Hidden() {
		t.Error("Expected Hidden() to be true after Hide()")
	}
}

func BenchmarkAddMenuItem(b *testing.B) {
	b.Cleanup(func() {
		systray.ResetForTesting()
	})

	for b.Loop() {
		systray.AddMenuItem("Benchmark Item")
	}
}

func BenchmarkUpdateItem(b *testing.B) {
	b.Cleanup(func() {
		systray.ResetForTesting()
	})

	item := systray.AddMenuItem("Benchmark Item")

	for b.Loop() {
		item.SetTitle("Updated Title")
	}
}
