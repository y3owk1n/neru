//go:build linux

package linux_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/systray"
)

// Regression test for the Linux quit race. Quit before Run/RunHeadless must
// not be lost (the daemon host can call Quit from the app goroutine before
// the systray loop creates its quit channel).
// It does not test the D-Bus SNI/dbusmenu transport, which needs a session bus.
//
// resetState clears the tray's process-wide state after the test. The parent
// package has its own copy for the backend-agnostic tests. Duplicating three
// lines is cheaper than exporting a test helper across a package boundary.
func resetState(t *testing.T) {
	t.Helper()
	t.Cleanup(systray.ResetForTesting)
}

func TestQuitBeforeRunHeadlessDoesNotBlock(t *testing.T) {
	resetState(t)

	systray.Quit()

	done := make(chan struct{})

	go func() {
		systray.RunHeadless(func() {}, func() {})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunHeadless blocked forever after an early Quit")
	}
}
