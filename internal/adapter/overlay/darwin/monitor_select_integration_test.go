//go:build integration && darwin

package darwin_test

import (
	"image"
	"os"
	"runtime"
	"testing"

	"github.com/y3owk1n/neru/internal/adapter/overlay/darwin"
	"github.com/y3owk1n/neru/internal/adapter/overlay/manager"
)

// onMainThread carries calls for TestMain to run on the main thread.
var onMainThread = make(chan func())

// Pin the main thread during package init so TestMain still runs on it.
func init() {
	runtime.LockOSThread()
}

// TestMain runs the tests' calls on the main thread and never runs the Cocoa
// loop. The daemon is in the same state at shutdown, which runs on the main
// thread after the loop has stopped.
func TestMain(m *testing.M) {
	result := make(chan int, 1)

	go func() { result <- m.Run() }()

	for {
		select {
		case code := <-result:
			os.Exit(code)
		case call := <-onMainThread:
			call()
		}
	}
}

// runOnMainThread runs call on the main thread and waits for it.
func runOnMainThread(call func()) {
	done := make(chan struct{})

	onMainThread <- func() {
		defer close(done)

		call()
	}

	<-done
}

// requireDesktop skips unless this run opted into tests that drive the real
// desktop. `just test-desktop` sets the variable.
func requireDesktop(t *testing.T) {
	t.Helper()

	if os.Getenv("NERU_DESKTOP_TESTS") == "" {
		t.Skip("skipping desktop-driving test; run `just test-desktop` to include it")
	}
}

// TestManager_MonitorSelectPanelsComeDownOnTheMainThread pins that drawing and
// removing the picker works on the main thread while the Cocoa loop is
// stopped. Quitting with the picker open removes it there, and libdispatch
// traps a dispatch_sync onto the main queue from the main thread.
func TestManager_MonitorSelectPanelsComeDownOnTheMainThread(t *testing.T) {
	requireDesktop(t)

	var overlay darwin.Manager

	runOnMainThread(func() {
		err := overlay.DrawMonitorSelect([]manager.MonitorSelectTarget{
			{Bounds: image.Rect(0, 0, 200, 120), Label: "a", Subtitle: "test"},
		}, manager.MonitorSelectStyle{})
		if err != nil {
			t.Errorf("DrawMonitorSelect() error = %v", err)
		}
	})

	runOnMainThread(overlay.HideMonitorSelect)
}
