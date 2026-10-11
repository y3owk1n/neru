package axobserver

import (
	"go.uber.org/zap"
)

// The observer is process-wide: one application is watched at a time, through
// one OS-level observer slot and one registered change callback. The slot, the
// watched pid, and the run-loop thread all live in the platform layer; this
// package is the thin Go face over them.
var observerLogger = zap.NewNop()

// Init installs the change callback and the logger. onChange fires each time
// the watched application's UI changes; it runs on the observer's callback
// thread, so it must be cheap and must not call Watch or Unwatch, which would
// deadlock against a concurrent teardown. Call Init once at startup, before
// Watch; a later Init replaces the callback. logger may be nil.
func Init(onChange func(), logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}

	observerLogger = logger
	platformSetChangeHandler(newChangeHandler(onChange, logger))
}

// Watch makes pid the watched application, replacing whatever was watched
// before; the platform tears the previous observer down as part of the switch.
// Watching the pid already watched is a no-op. After a failed watch nothing is
// watched, so a later Watch of the same pid retries from a clean state.
func Watch(pid int) {
	if !platformWatch(pid) {
		observerLogger.Debug("observer watch failed", zap.Int("pid", pid))
	}
}

// Unwatch stops watching the current application. It is a no-op when nothing
// is watched.
func Unwatch() {
	platformUnwatch()
}

// Supported reports whether this platform has an observer backend. Where it
// returns false, Watch always fails and no change is ever reported.
func Supported() bool {
	return observerSupported
}

// newChangeHandler builds the function the platform invokes on every observed
// notification: it logs the notification name and forwards to the caller's
// onChange.
func newChangeHandler(onChange func(), logger *zap.Logger) func(notif string) {
	return func(notif string) {
		logger.Debug("ax notification", zap.String("notif", notif))

		if onChange != nil {
			onChange()
		}
	}
}
