//go:build darwin

package axobserver

import (
	"github.com/y3owk1n/neru/internal/core/infra/platform/darwin"
)

const observerSupported = true

// observerMessagingTimeoutSeconds bounds the observer's synchronous AX calls to
// the app it watches, so a wedged app cannot hang the observer thread. It is set
// on that app's element only, never process-wide, so hint scanning keeps the
// default timeout.
const observerMessagingTimeoutSeconds = 0.25

// The darwin bridge owns the observer, the watched process, and the run-loop
// thread lifecycle: the thread runs only while a process is watched, so an idle
// neru has no observer thread and no background cost.

func platformWatch(pid int) bool {
	return darwin.WatchObserver(pid, observerMessagingTimeoutSeconds)
}

func platformUnwatch() {
	darwin.UnwatchObserver()
}

func platformSetChangeHandler(handler func(notif string)) {
	if handler == nil {
		darwin.SetAXObserverHandler(nil)

		return
	}

	darwin.SetAXObserverHandler(func(notif string) {
		handler(notif)
	})
}
