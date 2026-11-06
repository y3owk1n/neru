//go:build linux

package atspi

import (
	"context"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
)

// Event-driven active-window tracking. With no mapping from a Wayland toplevel
// to an AT-SPI object, frame selection would scan every application. This
// listener subscribes once to window:activate and records the window, and
// findActiveFrame validates that cache against the live focus identity before
// trusting it.
const (
	// atspiRegistryObjPath is the AT-SPI registry object that RegisterEvent is
	// called on (distinct from the accessible-tree root).
	atspiRegistryObjPath = dbus.ObjectPath("/org/a11y/atspi/registry")

	// atspiRegistryRegisterEvent tells the registry which event classes a client
	// wants, so toolkits that gate emission on a registered listener send them.
	atspiRegistryRegisterEvent = "org.a11y.atspi.Registry.RegisterEvent"

	// atspiRegisterWindowActivate is the RegisterEvent argument (colon form) for
	// window activation.
	atspiRegisterWindowActivate = "window:activate"

	// atspiEventWindowIfc / atspiSignalWindowActivate are the D-Bus interface and
	// fully-qualified signal name the activation event is delivered as.
	atspiEventWindowIfc       = "org.a11y.atspi.Event.Window"
	atspiSignalWindowActivate = "org.a11y.atspi.Event.Window.activate"

	// atspiEventChanBuffer sizes the signal channel. Activations are rare, and a
	// full channel drops rather than blocks, which the scan fallback covers.
	atspiEventChanBuffer = 64
)

// atspiActiveWindow is the window that last emitted window:activate, with the
// identity needed to validate it against the focused toplevel.
type atspiActiveWindow struct {
	frame   accRef
	appName string // resolved application name, for the focused-app_id cross-check
	title   string // window title, to disambiguate sibling windows of one app
}

// ensureA11yEvents registers the window:activate listener on conn once. Every
// failure is non-fatal, since frame selection falls back to the registry scan.
func (c *Client) ensureA11yEvents(conn *dbus.Conn) {
	// Claim the setup and publish the stop channel under activeMu, so a concurrent
	// Close always finds and closes it, even before the goroutine starts. The D-Bus
	// registration runs without the lock so a wedged registry stalls nobody else.
	c.activeMu.Lock()
	if c.eventsClosed || c.eventsReady {
		c.activeMu.Unlock()

		return
	}

	stop := make(chan struct{})
	c.eventsStop = stop
	c.eventsReady = true
	c.activeMu.Unlock()

	matchCtx, cancelMatch := context.WithTimeout(context.Background(), atspiCallTimeout)
	err := conn.AddMatchSignalContext(matchCtx,
		dbus.WithMatchInterface(atspiEventWindowIfc),
		dbus.WithMatchMember("activate"),
	)

	cancelMatch()

	if err != nil {
		c.logger.Debug("AT-SPI AddMatchSignal failed; relying on scan fallback",
			zap.Error(err))

		// Roll back the claim so a later request can retry, unless Close already
		// took over this listener's stop.
		c.activeMu.Lock()

		if c.eventsStop == stop {
			c.eventsStop = nil
			c.eventsReady = false
		}

		c.activeMu.Unlock()

		return
	}

	// Register only after the match succeeds, so a failed match leaves nothing for
	// the retry to repeat. The timeout keeps a wedged bus from hanging the first
	// hints request. Failure is fine, since some desktops broadcast anyway.
	regCtx, cancel := context.WithTimeout(context.Background(), atspiCallTimeout)
	regCall := conn.Object(atspiRegistryDest, atspiRegistryObjPath).
		CallWithContext(regCtx, atspiRegistryRegisterEvent, 0, atspiRegisterWindowActivate)

	cancel()

	if regCall.Err != nil {
		c.logger.Debug("AT-SPI RegisterEvent failed; relying on scan fallback",
			zap.Error(regCall.Err))
	}

	signals := make(chan *dbus.Signal, atspiEventChanBuffer)
	conn.Signal(signals)

	go c.consumeWindowEvents(conn, signals, stop)

	c.logger.Debug("AT-SPI window:activate listener started")
}

// consumeWindowEvents records the source window of each window:activate signal
// until stop is closed or the connection's signal channel is closed.
func (c *Client) consumeWindowEvents(
	conn *dbus.Conn,
	signals chan *dbus.Signal,
	stop chan struct{},
) {
	for {
		select {
		case <-stop:
			return
		case sig, ok := <-signals:
			if !ok {
				return
			}

			if sig == nil {
				continue
			}

			// Log every delivered signal so a silent listener (no events arriving)
			// is distinguishable from a cross-check that keeps rejecting them.
			c.logger.Debug("AT-SPI event signal",
				zap.String("name", sig.Name),
				zap.String("sender", sig.Sender),
				zap.String("path", string(sig.Path)))

			if sig.Name != atspiSignalWindowActivate {
				continue
			}

			// The source window is the signal's sender and object path.
			c.recordActiveWindow(conn, accRef{Name: sig.Sender, Path: sig.Path})
		}
	}
}

// recordActiveWindow resolves and caches a newly activated window's identity
// on the listener goroutine, off the hints hot path.
func (c *Client) recordActiveWindow(conn *dbus.Conn, frame accRef) {
	if frame.Name == "" || frame.Path == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), atspiCallTimeout)
	defer cancel()

	// Cache only genuine top-level windows, and ignore any other source that
	// slips through the signal match.
	role := c.roleName(ctx, conn, frame)
	if !isWindowRole(role) {
		return
	}

	active := &atspiActiveWindow{
		frame:   frame,
		appName: c.name(ctx, conn, accRef{Name: frame.Name, Path: atspiRootPath}),
		title:   c.name(ctx, conn, frame),
	}

	c.activeMu.Lock()
	c.activeWindow = active
	c.activeMu.Unlock()

	c.logger.Debug("AT-SPI recorded active window",
		zap.String("bus", frame.Name),
		zap.String("app", active.appName))
}

// activeWindowMatching returns the cached active window when it still matches
// the focused toplevel, or false to fall back to the scan. The cross-check
// keeps a stale or missing entry from resolving to the wrong window.
func (c *Client) activeWindowMatching(focusedAppID, focusedTitle string) (accRef, bool) {
	// Without a focus identity (GNOME without the extension) there is nothing to
	// validate the cache against, so it cannot be trusted.
	if focusedAppID == "" {
		return accRef{}, false
	}

	c.activeMu.Lock()
	active := c.activeWindow
	c.activeMu.Unlock()

	if active == nil {
		return accRef{}, false
	}

	// The cached window must belong to the application the compositor currently
	// reports as focused.
	if !appMatchesFocusedID(active.appName, focusedAppID) {
		return accRef{}, false
	}

	// A reported title must name the cached window, so switching between siblings
	// of one app is no cache hit. An untitled cached window is rejected against a
	// titled focus, or a failed title read would let a stale sibling pass.
	if focusedTitle != "" &&
		(active.title == "" || !titleMatchesFocused(active.title, focusedTitle)) {
		return accRef{}, false
	}

	return active.frame, true
}
