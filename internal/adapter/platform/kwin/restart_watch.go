//go:build linux

package kwin

import (
	"sync"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
)

// Noticing that KWin came or went. The script lives inside the compositor, so
// `kwin --replace` or a Plasma crash takes it along. Without this watch the
// cache would serve the last rectangle for the rest of the daemon's life.

const (
	dbusNameOwnerMember = "NameOwnerChanged"
	dbusNameOwnerSignal = dbusIface + "." + dbusNameOwnerMember

	// NameOwnerChanged carries (name, oldOwner, newOwner).
	nameOwnerArgs     = 3
	nameOwnerNewIndex = 2

	// ownerSignalBuffer is sized for every signal godbus delivers on this
	// connection, including unmatched ones such as NameAcquired and NameLost,
	// not just NameOwnerChanged. It keeps the common case off godbus's
	// overflow path (a goroutine per signal).
	ownerSignalBuffer = 32
)

// claim is the restart watch's ownership, keyed by connection. One connection
// gets one watcher. A replacement connection takes its own claim even before
// the retired watcher exits, and a retired watcher releases only what it took.
type claim struct {
	mu   sync.Mutex
	conn *dbus.Conn
}

// take reports whether the caller is the one that should arm the watch on conn.
func (c *claim) take(conn *dbus.Conn) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == conn {
		return false
	}

	c.conn = conn

	return true
}

// release hands the claim back, for the taker of conn only.
func (c *claim) release(conn *dbus.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == conn {
		c.conn = nil
	}
}

// watchKWin asks the bus to say when org.kde.KWin changes hands. Losing the
// name empties the cache. Gaining it installs the script into the new KWin.
//
// It is armed before KWin's presence is checked, because a daemon starting
// with the session often runs before KWin is on the bus, and that is where the
// watch matters most. This stays inside #1430, since a match rule is a
// subscription, not a claim. Nothing is exported, owned or written until KWin
// answers. The watch is armed once per connection and outlives restarts.
func (g *Geometry) watchKWin(conn *dbus.Conn) {
	if !g.watching.take(conn) {
		return
	}

	matchErr := conn.AddMatchSignal(
		dbus.WithMatchInterface(dbusIface),
		dbus.WithMatchMember(dbusNameOwnerMember),
		dbus.WithMatchArg(0, scriptingDest),
	)
	if matchErr != nil {
		g.watching.release(conn)
		g.log().Debug("KWin restart watch unavailable", zap.Error(matchErr))

		return
	}

	signals := make(chan *dbus.Signal, ownerSignalBuffer)
	conn.Signal(signals)

	go g.serveOwnerChanges(conn, signals)
}

// serveOwnerChanges runs as long as the connection does. When the channel
// closes, the watch, receiver and name are gone, so the cache is emptied, the
// reason recorded, and a reinstall on a fresh connection scheduled.
func (g *Geometry) serveOwnerChanges(conn *dbus.Conn, signals <-chan *dbus.Signal) {
	for signal := range signals {
		owner, ok := kwinOwnerFrom(signal)
		if !ok {
			continue
		}

		g.kwinOwnerChanged(owner)
	}

	g.watching.release(conn)

	if !g.connectionClosed(conn) {
		return
	}

	g.log().Debug("KWin bridge connection closed; reinstalling on a new one")

	g.invalidate()
	g.recordAttempt(g.forgetInstall(), errBusClosed)
	g.EnsureStarted()
}

// kwinOwnerFrom reads a NameOwnerChanged for org.kde.KWin and returns its new
// owner, empty when the name was released. The shape is checked because the
// bus also delivers signals no match rule asked for, such as NameAcquired.
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

// kwinOwnerChanged reacts to KWin leaving or joining the bus. Both directions
// empty the cache and forget the install, because the script feeding it is
// gone.
//
// A departure records errKWinAbsent, because an empty cache with no reason
// means "nothing is focused". The AT-SPI origin installs once at construction,
// so without this it would degrade silently. The next successful install
// clears it.
func (g *Geometry) kwinOwnerChanged(owner string) {
	g.invalidate()

	// Retire attempts in flight, or one against the departed compositor could
	// land later and overwrite the reason below.
	generation := g.forgetInstall()

	if owner == "" {
		g.recordAttempt(generation, errKWinAbsent)

		return
	}

	g.log().Debug("KWin is on the session bus; installing the geometry script")

	g.EnsureStarted()
}
