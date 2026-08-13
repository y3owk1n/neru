//go:build linux

package kwin

import (
	"sync"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
)

// Noticing that KWin came or went.
//
// A script lives inside the compositor, so `kwin --replace`, a Plasma crash or
// a session-wide restart takes it with it. Nothing else here would hear about
// that: no push would arrive again, and an installed bridge never reinstalls —
// so the cache would stay frozen at the last rectangle and serve it as the
// truth for the rest of the daemon's life, which is the same confidently wrong
// answer the bridge was written to end, reached by a different route.

const (
	dbusNameOwnerMember = "NameOwnerChanged"
	dbusNameOwnerSignal = dbusIface + "." + dbusNameOwnerMember

	// NameOwnerChanged carries (name, oldOwner, newOwner).
	nameOwnerArgs     = 3
	nameOwnerNewIndex = 2

	// ownerSignalBuffer is the restart watch's queue. It is not sized for
	// NameOwnerChanged, which fires about org.kde.KWin once a session: this is
	// the process-wide session connection, so godbus delivers everything
	// *anything* in this process subscribed to into this channel too, and the
	// buffer is what keeps the common case off godbus's overflow path (a
	// goroutine per signal — it defers rather than drops).
	ownerSignalBuffer = 32
)

// claim is a one-shot flag: the first taker gets it and everyone after is told
// no, until whoever holds it gives it back. It exists so setup that must happen
// once cannot happen twice when two callers race, without each such flag
// growing its own mutex and its own pair of methods.
type claim struct {
	mu   sync.Mutex
	held bool
}

// take reports whether the caller is the one that should do the work.
func (c *claim) take() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.held {
		return false
	}

	c.held = true

	return true
}

// release hands the claim back, for a taker that could not finish.
func (c *claim) release() {
	c.mu.Lock()
	c.held = false
	c.mu.Unlock()
}

// watchKWin asks the bus to say when org.kde.KWin changes hands. Losing the
// name empties the cache, because a window on a compositor that no longer
// exists is not anywhere; gaining it installs the script into the new one.
//
// It is armed before KWin's presence is checked, not after the script loads,
// because the session where it matters most is the one where there is no KWin
// to install into yet: a daemon starting with the session reaches this before
// the compositor is on the bus, and a watch armed only on success would be
// armed only where it was least needed. Without it that daemon still recovers —
// the next caller retries the install — but recovery would wait for someone to
// ask, which for the AT-SPI origin means the person who asks pays for it with a
// screenful of misplaced hints.
//
// This is the one thing the bridge does before knowing KWin is there, and it
// stays inside what #1430 asked for: a match rule is a subscription, not a
// claim. Nothing is exported, no name is owned, and nothing is written to disk
// until KWin has answered for itself.
//
// The watch is armed once and outlives any number of restarts.
func (g *Geometry) watchKWin(conn *dbus.Conn) {
	if !g.watching.take() {
		return
	}

	matchErr := conn.AddMatchSignal(
		dbus.WithMatchInterface(dbusIface),
		dbus.WithMatchMember(dbusNameOwnerMember),
		dbus.WithMatchArg(0, scriptingDest),
	)
	if matchErr != nil {
		g.watching.release()
		g.log().Debug("KWin restart watch unavailable", zap.Error(matchErr))

		return
	}

	signals := make(chan *dbus.Signal, ownerSignalBuffer)
	conn.Signal(signals)

	go g.serveOwnerChanges(signals)
}

// serveOwnerChanges runs for the daemon's life, which is the same life the
// compositor it watches has.
func (g *Geometry) serveOwnerChanges(signals <-chan *dbus.Signal) {
	for signal := range signals {
		owner, ok := kwinOwnerFrom(signal)
		if !ok {
			continue
		}

		g.kwinOwnerChanged(owner)
	}
}

// kwinOwnerFrom reads a NameOwnerChanged for org.kde.KWin and returns its new
// owner, empty when the name was released.
//
// The signal channel is filtered by a match rule, but a match rule is a request
// to the bus rather than a guarantee about what arrives: this connection is the
// process-wide session bus, so anything else that subscribes on it is delivered
// here too. The shape is therefore checked rather than assumed.
func kwinOwnerFrom(signal *dbus.Signal) (string, bool) {
	if signal == nil || signal.Name != dbusNameOwnerSignal || len(signal.Body) < nameOwnerArgs {
		return "", false
	}

	name, nameOK := signal.Body[0].(string)
	if !nameOK || name != scriptingDest {
		return "", false
	}

	owner, ownerOK := signal.Body[nameOwnerNewIndex].(string)
	if !ownerOK {
		return "", false
	}

	return owner, true
}

// kwinOwnerChanged reacts to KWin leaving or joining the bus.
//
// Both directions empty the cache and forget the install, because both mean the
// script that was feeding it is gone. Only one of them has somewhere to install
// a new one.
//
// A departure also records why, rather than leaving an empty cache to speak for
// itself. An empty cache with no reason means "the bridge works and nothing is
// focused", and a compositor that is not on the bus is the other answer
// entirely — the one errKWinAbsent exists to name. The focused-window arm would
// reach that answer on its own, because every call there reinstalls and the
// attempt fails loudly; the AT-SPI origin arm would not, because it installs
// once at construction and would spend the gap degrading silently with nothing
// for the log to say. Recording it here is what keeps the two arms of one
// bridge answering one way. The next successful install clears it.
func (g *Geometry) kwinOwnerChanged(owner string) {
	g.invalidate()
	g.forgetInstall()

	if owner == "" {
		g.recordAttempt(errKWinAbsent)

		return
	}

	g.log().Debug("KWin is on the session bus; installing the geometry script")

	g.EnsureStarted()
}
