//go:build linux

package atspi

import (
	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/platform"
	"github.com/y3owk1n/neru/internal/adapter/platform/linux"
)

// Brings the AT-SPI bus up. Toolkits build their accessibility tree only once
// assistive-tech mode is on, so this must succeed before a scan finds anything.
//
// readA11yStatus reads the IsEnabled and ScreenReaderEnabled properties from
// org.a11y.Status.
func (c *Client) readA11yStatus() (bool, bool, error) {
	conn, connErr := dbus.SessionBus()
	if connErr != nil {
		return false, false, connErr
	}

	obj := conn.Object(a11yBusDest, a11yBusPath)

	var isVariant dbus.Variant

	getErr := obj.Call("org.freedesktop.DBus.Properties.Get", 0,
		a11yStatusIfc, a11yPropIsEnabled).Store(&isVariant)
	if getErr != nil {
		return false, false, getErr
	}

	isOn, _ := isVariant.Value().(bool)

	var srVariant dbus.Variant

	getErr = obj.Call("org.freedesktop.DBus.Properties.Get", 0,
		a11yStatusIfc, a11yPropScreenReader).Store(&srVariant)
	if getErr != nil {
		return false, false, getErr
	}

	srOn, _ := srVariant.Value().(bool)

	return isOn, srOn, nil
}

// setA11yStatus writes the given values for IsEnabled and ScreenReaderEnabled
// on org.a11y.Status. When disabling, ScreenReaderEnabled is cleared before
// IsEnabled. When enabling, IsEnabled is set first.
func (c *Client) setA11yStatus(srEnabled, isEnabled bool) error {
	conn, connErr := dbus.SessionBus()
	if connErr != nil {
		return connErr
	}

	obj := conn.Object(a11yBusDest, a11yBusPath)

	var props []string
	if !srEnabled {
		props = []string{a11yPropScreenReader, a11yPropIsEnabled}
	} else {
		props = []string{a11yPropIsEnabled, a11yPropScreenReader}
	}

	for _, prop := range props {
		var propVal bool
		switch prop {
		case a11yPropIsEnabled:
			propVal = isEnabled
		case a11yPropScreenReader:
			propVal = srEnabled
		}

		err := obj.Call("org.freedesktop.DBus.Properties.Set", 0,
			a11yStatusIfc, prop, dbus.MakeVariant(propVal)).Err
		if err != nil {
			return err
		}
	}

	c.logger.Debug("AT-SPI status set",
		zap.Bool("is_enabled", isEnabled),
		zap.Bool("screen_reader_enabled", srEnabled))

	return nil
}

// setA11yProp writes a single org.a11y.Status property.
func (c *Client) setA11yProp(prop string, val bool) error {
	conn, err := dbus.SessionBus()
	if err != nil {
		return err
	}

	return conn.Object(a11yBusDest, a11yBusPath).
		Call("org.freedesktop.DBus.Properties.Set", 0,
			a11yStatusIfc, prop, dbus.MakeVariant(val)).Err
}

// ensureA11yEnabled activates AT-SPI once and saves the original status for
// Close to restore. a11yMu is held across the D-Bus calls, so Close sees
// activation either finished or not started.
func (c *Client) ensureA11yEnabled() error {
	c.a11yMu.Lock()
	defer c.a11yMu.Unlock()

	if c.activated {
		return nil
	}

	if c.closed {
		return errClientClosed
	}

	savedIsOn, savedSrOn, err := c.readA11yStatus()
	if err != nil {
		return err
	}

	c.savedIsOn = savedIsOn
	c.savedSrOn = savedSrOn

	err = c.setA11yProp(a11yPropIsEnabled, true)
	if err != nil {
		return err
	}

	err = c.setA11yProp(a11yPropScreenReader, true)
	if err != nil {
		_ = c.setA11yProp(a11yPropIsEnabled, c.savedIsOn) // best-effort rollback

		return err
	}

	go c.windowOrigin.start()

	c.activated = true

	c.logger.Debug("AT-SPI accessibility enabled (lazy)")

	return nil
}

// ensureA11yConn lazily connects to the dedicated AT-SPI bus.
func (c *Client) ensureA11yConn() (*dbus.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Re-check closed under mu. Close sets closed (under a11yMu) before its mu
	// section, so a call taking mu after Close's teardown bails, and one taking
	// it first publishes c.a11y for Close to close. Either way nothing leaks.
	c.a11yMu.Lock()
	closed := c.closed
	c.a11yMu.Unlock()

	if closed {
		return nil, errClientClosed
	}

	if c.a11yReady && c.a11y != nil && c.a11y.Connected() {
		return c.a11y, nil
	}

	session, err := dbus.SessionBus()
	if err != nil {
		return nil, err
	}

	var addr string

	getAddrErr := session.Object(a11yBusDest, a11yBusPath).
		Call("org.a11y.Bus.GetAddress", 0).Store(&addr)
	if getAddrErr != nil {
		return nil, getAddrErr
	}

	conn, connErr := dbus.Connect(addr)
	if connErr != nil {
		return nil, connErr
	}

	c.a11y = conn
	c.a11yReady = true

	// The window:activate listener belongs to the old connection. Stop it, force
	// a re-register, and drop the cached window. Closing the stop here keeps
	// listener lifetime independent of godbus internals.
	c.activeMu.Lock()

	if c.eventsStop != nil {
		close(c.eventsStop)
		c.eventsStop = nil
	}

	c.eventsReady = false
	c.activeWindow = nil

	c.activeMu.Unlock()

	return conn, nil
}

// focusStableSince reports whether the focused window still matches the one
// captured at frame selection, by app_id and, for sibling windows sharing one,
// title. A mismatch means the frame is stale and must not be walked or offset.
// With no live app_id (X11) it returns true. Identical or empty titles hide a
// switch between siblings.
func (c *Client) focusStableSince(selectedAppID, selectedTitle string) bool {
	currentAppID, currentTitle, ok := linux.FocusedAppIdentity(
		platform.DetectLinuxBackend().String(),
	)
	if !ok {
		return true
	}

	return currentAppID == selectedAppID && currentTitle == selectedTitle
}
