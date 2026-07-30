//go:build linux

// internal/core/infra/accessibility/atspi_events_linux.go
// Event-driven active-window tracking for the Linux AT-SPI client.
//
// Frame selection otherwise has to scan every application on the a11y bus to
// find the focused window (there is no direct mapping from a Wayland toplevel to
// an AT-SPI object). Instead, this listener subscribes once to the AT-SPI
// window:activate event — emitted by the toolkit when a window gains keyboard
// focus — and records the activated window. findActiveFrame can then resolve the
// frame from that cache with no scan, validating it against the compositor's
// live focus identity so a stale or missing cache falls back to the scan rather
// than returning the wrong window.

package accessibility

import (
	"context"

	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
)

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

	// atspiEventChanBuffer sizes the signal channel. Window activations are
	// infrequent (only on focus changes); godbus drops rather than blocks if it
	// ever fills, and the scan fallback still covers a missed event.
	atspiEventChanBuffer = 64
)

// atspiActiveWindow is a snapshot of the window that most recently emitted a
// window:activate event, with the identity frame selection needs to validate it
// against the compositor's focused toplevel before trusting it.
type atspiActiveWindow struct {
	frame   accRef
	appName string // resolved application name, for the focused-app_id cross-check
	title   string // window title, to disambiguate sibling windows of one app
}

// ensureA11yEvents registers the window:activate listener on conn exactly once.
// Every failure path is non-fatal: frame selection falls back to scanning the
// registry, so accessibility still works where the events are unavailable.
func (c *ATSPIClient) ensureA11yEvents(conn *dbus.Conn) {
	// Claim the one-time setup and publish the stop channel up front, all under
	// activeMu, so that a concurrent Close always finds this listener's stop and
	// closes it — the goroutine below then exits even if it starts after Close.
	// The D-Bus registration is deliberately done *without* the lock held (see
	// below), so a wedged registry cannot stall other activeMu/conn users.
	c.activeMu.Lock()
	if c.eventsClosed || c.eventsReady {
		c.activeMu.Unlock()

		return
	}

	stop := make(chan struct{})
	c.eventsStop = stop
	c.eventsReady = true
	c.activeMu.Unlock()

	// Ask the registry to forward window activations, bounded so a wedged bus
	// cannot hang the first hints request. A failure is not fatal: some desktops
	// broadcast the event regardless, and any gap is covered by the scan fallback.
	regCtx, cancel := context.WithTimeout(context.Background(), atspiCallTimeout)
	regCall := conn.Object(atspiRegistryDest, atspiRegistryObjPath).
		CallWithContext(regCtx, atspiRegistryRegisterEvent, 0, atspiRegisterWindowActivate)

	cancel()

	if regCall.Err != nil {
		c.logger.Debug("AT-SPI RegisterEvent failed; relying on scan fallback",
			zap.Error(regCall.Err))
	}

	matchCtx, cancelMatch := context.WithTimeout(context.Background(), atspiCallTimeout)
	err := conn.AddMatchSignalContext(matchCtx,
		dbus.WithMatchInterface(atspiEventWindowIfc),
		dbus.WithMatchMember("activate"),
	)

	cancelMatch()

	if err != nil {
		c.logger.Debug("AT-SPI AddMatchSignal failed; relying on scan fallback",
			zap.Error(err))

		// Roll back the claim so a later request can retry — unless Close already
		// took over this listener's stop.
		c.activeMu.Lock()

		if c.eventsStop == stop {
			c.eventsStop = nil
			c.eventsReady = false
		}

		c.activeMu.Unlock()

		return
	}

	signals := make(chan *dbus.Signal, atspiEventChanBuffer)
	conn.Signal(signals)

	go c.consumeWindowEvents(conn, signals, stop)

	c.logger.Debug("AT-SPI window:activate listener started")
}

// consumeWindowEvents records the source window of each window:activate signal
// until stop is closed or the connection's signal channel is closed.
func (c *ATSPIClient) consumeWindowEvents(
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

			// The event's source window is identified by the signal's sender bus
			// name and object path.
			c.recordActiveWindow(conn, accRef{Name: sig.Sender, Path: sig.Path})
		}
	}
}

// recordActiveWindow resolves and caches the identity of a newly activated
// window. It runs on the listener goroutine, off the hints hot path, so the
// per-frame identity reads never delay a hints request.
func (c *ATSPIClient) recordActiveWindow(conn *dbus.Conn, frame accRef) {
	if frame.Name == "" || frame.Path == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), atspiCallTimeout)
	defer cancel()

	// Cache only genuine top-level windows; ignore any other source that slips
	// through the signal match.
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
		zap.String("app", active.appName),
		zap.String("title", active.title))
}

// activeWindowMatching returns the event-tracked active window when it still
// matches the compositor's focused toplevel, or ok=false to fall back to the
// registry scan. The cross-check against the live focus identity is what makes
// the cache safe: a stale entry (focus moved without a window:activate) or a
// missing one can never resolve to the wrong window.
func (c *ATSPIClient) activeWindowMatching(focusedAppID, focusedTitle string) (accRef, bool) {
	// Without a compositor focus identity (X11/GNOME) there is nothing to
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

	// When the compositor reports a title, it must name the cached window, so a
	// switch between sibling windows of one application (shared app_id) is not
	// mistaken for a cache hit. A cached window with no title cannot be confirmed
	// against a titled focus, so it is rejected rather than trusted — otherwise a
	// failed title read would let a stale sibling pass.
	if focusedTitle != "" &&
		(active.title == "" || !titleMatchesFocused(active.title, focusedTitle)) {
		return accRef{}, false
	}

	return active.frame, true
}
