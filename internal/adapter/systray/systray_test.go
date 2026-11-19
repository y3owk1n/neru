package systray_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/systray"
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

// TestAdapter_AddMenuItem_TracksCheckedAndClicks drives the live backend
// through ports.SystrayPort, whichever backend this build selected. The menu
// component reads Checked to flip its toggles and selects on Clicked, so a nil
// channel would leave an item that can never be picked.
func TestAdapter_AddMenuItem_TracksCheckedAndClicks(t *testing.T) {
	resetState(t)

	item := systray.NewAdapter().AddMenuItem("Parent")
	child := item.AddSubMenuItem("Child")

	if item.Clicked() == nil {
		t.Error("top-level item Clicked() is nil")
	}

	if child.Clicked() == nil {
		t.Error("submenu item Clicked() is nil")
	}

	item.Check()

	if !item.Checked() {
		t.Error("Checked() = false after Check(), want true")
	}

	item.Uncheck()

	if item.Checked() {
		t.Error("Checked() = true after Uncheck(), want false")
	}
}
