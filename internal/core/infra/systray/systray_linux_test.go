//go:build linux

// internal/core/infra/systray/systray_linux_test.go
// Regression test for the Linux quit race: Quit before Run/RunHeadless must
// not be lost (the daemon host can call Quit from the app goroutine before
// the systray loop creates its quit channel).
// Does not test the D-Bus SNI/dbusmenu transport; that needs a session bus.

package systray_test

import (
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/core/infra/systray"
)

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
