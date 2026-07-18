package axobserver

// Platform is the OS-specific observer backend.
//
// Arm, Disarm, DisarmAll, and Close are called by the Manager while it holds
// its lock, so an implementation may assume they are serialized. The function
// passed to SetChangeHandler is invoked from the platform's own callback
// thread and must not call back into the Manager.
type Platform interface {
	// Arm registers an observer for pid. Re-arming a pid replaces its previous
	// observer. It returns false if the observer could not be registered.
	Arm(pid int) bool

	// Disarm removes the observer for pid, if any.
	Disarm(pid int)

	// DisarmAll removes every observer and releases any run-loop resources.
	DisarmAll()

	// SetChangeHandler registers the change callback, which receives the firing
	// pid and the name of the notification that fired (for debug logging).
	// Passing nil clears it so no further callbacks are delivered.
	SetChangeHandler(func(pid int, notif string))

	// Close clears the change handler and disarms everything. It is idempotent.
	Close()
}
