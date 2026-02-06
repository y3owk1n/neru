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

	item := systray.AddMenuItem("Test Item", "Test Tooltip")
	if item == nil {
		t.Fatal("AddMenuItem returned nil")
	}

	if item.Title() != "Test Item" {
		t.Errorf("Expected title 'Test Item', got '%s'", item.Title())
	}

	if item.Tooltip() != "Test Tooltip" {
		t.Errorf("Expected tooltip 'Test Tooltip', got '%s'", item.Tooltip())
	}

	if item.ClickedCh == nil {
		t.Error("ClickedCh is nil")
	}
}

func TestAddSubMenuItem(t *testing.T) {
	resetState(t)

	parent := systray.AddMenuItem("Parent", "Parent Tooltip")
	child := parent.AddSubMenuItem("Child", "Child Tooltip")

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

	item := systray.AddMenuItem("Test", "Tooltip")

	item.SetTitle("New Title")
	item.SetTooltip("New Tooltip")
	item.Enable()
	item.Disable()
	item.Check()
	item.Uncheck()
	item.Show()
	item.Hide()

	if item.Title() != "New Title" {
		t.Errorf("Title not updated in struct")
	}

	if item.Tooltip() != "New Tooltip" {
		t.Errorf("Tooltip not updated in struct")
	}
}

func BenchmarkAddMenuItem(b *testing.B) {
	b.Cleanup(func() {
		systray.ResetForTesting()
	})

	for b.Loop() {
		systray.AddMenuItem("Benchmark Item", "Benchmark Tooltip")
	}
}

func BenchmarkUpdateItem(b *testing.B) {
	b.Cleanup(func() {
		systray.ResetForTesting()
	})

	item := systray.AddMenuItem("Benchmark Item", "Benchmark Tooltip")

	for b.Loop() {
		item.SetTitle("Updated Title")
	}
}
