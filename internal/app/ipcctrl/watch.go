package ipcctrl

import (
	"context"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/event"
)

const (
	// watchBuffer is how many events a watcher holds while its client is
	// busy reading. One that falls further behind is told how many it missed.
	watchBuffer = 64

	// watchSnapshot names the line a watch opens with: the status, at the
	// sequence number the events that follow it start after.
	watchSnapshot = "snapshot"
)

// HandleWatch serves `neru watch`: the status, then every event the daemon
// publishes, until the client goes away or the daemon stops. It answers while
// Neru is stopped too, or nobody would see the enable event.
//
// A watcher that falls behind gets a fresh snapshot in place of the events it
// missed, so a client that keeps state from the lines is never left on a
// stale one.
func (c *Controller) HandleWatch(
	ctx context.Context,
	_ ipc.Command,
	emit func(value any) error,
) error {
	if c.Events == nil {
		return derrors.New(derrors.CodeNotSupported, "this daemon publishes no events")
	}

	// Subscribe before reading the status, so no event falls between the
	// two. One published between reading Seq and the status shows in both.
	events, stop := c.Events.Subscribe(watchBuffer)
	defer stop()

	line, err := c.snapshotLine()
	if err != nil {
		return err
	}

	err = emit(line)
	if err != nil {
		return err
	}

	// last is the seq of the newest event the client has, in a line or a
	// snapshot. The bus numbers every event for every subscriber, so a gap
	// after it is events this watcher missed.
	last := line.Seq
	behind := false

	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, open := <-events:
			if !open {
				return nil
			}

			// The bus drops only while the buffer is full, and only this
			// loop drains it, so a buffer full up to this receive is the one
			// sign that events may have been dropped after the ones queued.
			// That holds for an event a snapshot already covered too, since
			// a queue full of those is what drops the newer ones.
			behind = behind || len(events) == cap(events)-1

			switch {
			case evt.Seq <= last:
			case evt.Seq == last+1:
				err = emit(watchLineFor(evt))
				last = evt.Seq
			default:
				last, err = c.resync(emit, last)
			}

			if err != nil {
				return err
			}

			// Caught up with no event left to carry the news of a drop, so
			// check the bus for one. Seq is read before the length, so an
			// event still to come is queued rather than counted as missed.
			if !behind {
				continue
			}

			if newest := c.Events.Seq(); len(events) == 0 {
				behind = false

				if newest > last {
					last, err = c.resync(emit, last)
					if err != nil {
						return err
					}
				}
			}
		}
	}
}

// snapshotLine is the status as of the newest event published.
func (c *Controller) snapshotLine() (ipc.WatchLine, error) {
	seq := c.Events.Seq()

	status, ok := c.infoHandler.statusData()
	if !ok {
		return ipc.WatchLine{}, derrors.New(derrors.CodeActionFailed, "config not available")
	}

	return ipc.WatchLine{Seq: seq, Event: watchSnapshot, Status: &status}, nil
}

// resync sends a snapshot in place of the events since last, counting them in
// dropped, and returns the seq it is at.
func (c *Controller) resync(emit func(value any) error, last uint64) (uint64, error) {
	line, err := c.snapshotLine()
	if err != nil {
		return last, err
	}

	line.Dropped = line.Seq - last

	return line.Seq, emit(line)
}

// watchLineFor puts an event on the wire. A field whose zero value is an
// answer, such as ok false or no sticky modifiers, is sent only for the events
// that carry it.
func watchLineFor(evt event.Event) ipc.WatchLine {
	line := ipc.WatchLine{
		Seq:      evt.Seq,
		Event:    string(evt.Name),
		Mode:     evt.Mode,
		Reason:   string(evt.Reason),
		Action:   evt.Action,
		BundleID: evt.BundleID,
		Slot:     evt.Slot,
		Monitor:  evt.Monitor,
	}

	if evt.Name == event.ConfigReload {
		line.OK = &evt.OK
	}

	if evt.Name == event.ScrollInvert || evt.Name == event.ScreenShareHide {
		line.On = &evt.On
	}

	if evt.Name == event.CursorSave || evt.Name == event.Select {
		line.X, line.Y = &evt.Point.X, &evt.Point.Y
	}

	if evt.Name == event.StickyModifiers {
		line.Modifiers = &evt.Modifiers
	}

	return line
}
