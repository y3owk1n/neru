//go:build linux

package atspi

import (
	"context"
	"image"
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/y3owk1n/neru/internal/adapter/accessibility/ax"
)

// atspiMaxHitSteps bounds the descent to the element at a point and the walk
// back up through its parents. Real trees are far shallower. The bound only
// stops a toolkit whose tree loops.
const atspiMaxHitSteps = 64

// atspiNullPath is the object path AT-SPI returns for "no accessible".
const atspiNullPath = dbus.ObjectPath("/org/a11y/atspi/null")

// visibleCheckFor reports whether hints.visible_check_enabled is set for
// appID.
func (c *Client) visibleCheckFor(appID string) bool {
	if c.configProvider == nil {
		return false
	}

	cfg := c.configProvider.Get()

	return cfg != nil && cfg.ShouldEnableVisibleCheckForApp(appID)
}

// visibleNodes keeps the nodes whose center hit-tests to the node or to an
// element inside it, for hints.visible_check_enabled. A node that something
// else in the window covers is dropped. The app answers the hit-test from its
// own widgets, so other windows never count, on X11 or Wayland. The hit-tests
// run in parallel, bounded like the Collection reads.
func (c *Client) visibleNodes(
	ctx context.Context,
	conn *dbus.Conn,
	frame accRef,
	nodes []ax.Node,
	place placement,
) []ax.Node {
	keep := make([]bool, len(nodes))
	sem := make(chan struct{}, collectionMaxWorkers)

	var waitGroup sync.WaitGroup

	for index, node := range nodes {
		atspi, ok := node.(*atspiNode)
		if !ok || ctx.Err() != nil {
			keep[index] = true

			continue
		}

		waitGroup.Add(1)

		sem <- struct{}{}

		go func(slot int, node *atspiNode) {
			defer waitGroup.Done()
			defer func() { <-sem }()

			// The scan may have been canceled while this waited for a slot.
			if ctx.Err() != nil {
				keep[slot] = true

				return
			}

			bounds := node.rect.Sub(place.offset)
			center := image.Pt(bounds.Min.X+bounds.Dx()/2, bounds.Min.Y+bounds.Dy()/2)

			keep[slot] = c.hitReaches(ctx, conn, frame, node.ref, center, place.coordType)
		}(index, atspi)
	}

	waitGroup.Wait()

	kept := nodes[:0]

	for index, node := range nodes {
		if keep[index] {
			kept = append(kept, node)
		}
	}

	return kept
}

// hitReaches reports whether the deepest element at point inside frame is
// target or lies inside it. Firefox, Qt and GTK 3 answer the point lookup one
// level at a time, so the descent passes through target. Chromium jumps to the
// deepest element, often a text leaf inside a link, so the walk back up its
// parents finds target. GTK 4 answers nothing, and an app that cannot answer,
// or a lookup or parent read that fails partway, keeps the element rather
// than losing hints.
func (c *Client) hitReaches(
	ctx context.Context,
	conn *dbus.Conn,
	frame, target accRef,
	point image.Point,
	coordType uint32,
) bool {
	current := frame

	for range atspiMaxHitSteps {
		next, found, err := c.accessibleAtPoint(ctx, conn, current, point, coordType)
		if err != nil {
			return true
		}

		if !found || next == current {
			break
		}

		if next == target {
			return true
		}

		current = next
	}

	if current == frame {
		return true
	}

	for range atspiMaxHitSteps {
		parent, found, err := c.parent(ctx, conn, current)
		if err != nil {
			return true
		}

		if !found || parent == frame {
			return false
		}

		if parent == target {
			return true
		}

		current = parent
	}

	return false
}

// accessibleAtPoint returns the child of ref at point, false when there is
// none, and an error when the app does not answer.
func (c *Client) accessibleAtPoint(
	ctx context.Context,
	conn *dbus.Conn,
	ref accRef,
	point image.Point,
	coordType uint32,
) (accRef, bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, atspiCallTimeout)
	defer cancel()

	var hit accRef

	err := conn.Object(ref.Name, ref.Path).CallWithContext(callCtx,
		atspiComponentIfc+".GetAccessibleAtPoint", 0,
		int32(point.X), int32(point.Y), coordType).Store(&hit)
	if err != nil {
		c.noteCallErr(ctx, err)

		return accRef{}, false, err
	}

	return hit, hit.Name != "" && hit.Path != atspiNullPath, nil
}

// parent returns the parent of ref, false when it has none, and an error when
// the app does not answer.
func (c *Client) parent(ctx context.Context, conn *dbus.Conn, ref accRef) (accRef, bool, error) {
	callCtx, cancel := context.WithTimeout(ctx, atspiCallTimeout)
	defer cancel()

	var value dbus.Variant

	err := conn.Object(ref.Name, ref.Path).CallWithContext(callCtx, atspiPropertiesGet, 0,
		atspiAccessibleIfc, "Parent").Store(&value)
	if err != nil {
		c.noteCallErr(ctx, err)

		return accRef{}, false, err
	}

	var parent accRef

	err = value.Store(&parent)
	if err != nil {
		return accRef{}, false, err
	}

	return parent, parent.Name != "" && parent.Path != atspiNullPath, nil
}
