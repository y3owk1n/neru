//go:build darwin

package darwin_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

func TestAXObserverHandlerDispatch(t *testing.T) {
	gotNotif := ""
	calls := 0

	darwin.SetAXObserverHandler(func(notif string) {
		gotNotif = notif
		calls++
	})
	t.Cleanup(func() { darwin.SetAXObserverHandler(nil) })

	darwin.HandleAXObserverNotification("AXCreated")

	if gotNotif != "AXCreated" || calls != 1 {
		t.Fatalf("handler got notif=%q calls=%d, want notif=AXCreated calls=1", gotNotif, calls)
	}
}

func TestAXObserverHandlerClearedDropsCallback(t *testing.T) {
	calls := 0

	darwin.SetAXObserverHandler(func(string) { calls++ })
	darwin.HandleAXObserverNotification("AXCreated")

	darwin.SetAXObserverHandler(nil)
	darwin.HandleAXObserverNotification("AXCreated")

	if calls != 1 {
		t.Fatalf("handler fired %d times, want 1 (the callback after clear must be dropped)", calls)
	}
}
