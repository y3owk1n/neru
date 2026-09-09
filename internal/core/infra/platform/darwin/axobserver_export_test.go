//go:build darwin

package darwin

// HandleAXObserverNotification synthesizes an observer notification,
// dispatching it through the registered handler. It lets the callback wiring be
// tested without a live accessibility notification.
func HandleAXObserverNotification(notif string) {
	dispatchAXObserverNotification(notif)
}
