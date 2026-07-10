//go:build darwin

package darwin_test

import (
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

func TestAXObserverHandlerDispatch(t *testing.T) {
	var (
		gotPID   int
		gotNotif string
	)

	calls := 0

	darwin.SetAXObserverHandler(func(pid int, notif string) {
		gotPID = pid
		gotNotif = notif
		calls++
	})
	t.Cleanup(func() { darwin.SetAXObserverHandler(nil) })

	darwin.HandleAXObserverNotification(77, "AXCreated")

	if gotPID != 77 || gotNotif != "AXCreated" || calls != 1 {
		t.Fatalf("handler got pid=%d notif=%q calls=%d, want pid=77 notif=AXCreated calls=1",
			gotPID, gotNotif, calls)
	}
}

func TestAXObserverHandlerClearedDropsCallback(t *testing.T) {
	calls := 0

	darwin.SetAXObserverHandler(func(int, string) { calls++ })
	darwin.HandleAXObserverNotification(1, "AXCreated")

	darwin.SetAXObserverHandler(nil)
	darwin.HandleAXObserverNotification(2, "AXCreated")

	if calls != 1 {
		t.Fatalf("handler fired %d times, want 1 (the callback after clear must be dropped)", calls)
	}
}
