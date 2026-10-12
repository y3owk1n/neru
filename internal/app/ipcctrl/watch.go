package ipcctrl

import (
	"context"
	"slices"
	"strings"

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

	// watchEventFlag names the events a watch sends.
	watchEventFlag = "--event"

	// msgWatchEventValue is what --event says without a usable value.
	msgWatchEventValue = "--event requires a value (use comma-separated: --event=mode_enter,mode_exit)"
)

// HandleWatch serves `neru watch`: the status, then every event the daemon
// publishes, until the client goes away or the daemon stops. It answers while
// Neru is stopped too, or nobody would see the enable event.
//
// A watcher that falls behind gets a fresh snapshot in place of the events it
// missed, so a client that keeps state from the lines is never left on a
// stale one. With --event it sends only the events named, and every snapshot
// still, since a snapshot is the state the events change.
func (c *Controller) HandleWatch(
	ctx context.Context,
	cmd ipc.Command,
	emit func(value any) error,
) error {
	if c.Events == nil {
		return derrors.New(derrors.CodeNotSupported, "this daemon publishes no events")
	}

	wanted, err := watchFilter(cmd.Args)
	if err != nil {
		return err
	}

	// Subscribe before reading the status, so no event falls between the
	// two. One published between reading Seq and the status shows in both.
	// The bus offers only the events wanted, so one left out can neither
	// fill the buffer nor count as missed.
	sub := c.Events.SubscribeTo(watchBuffer, wanted)
	defer sub.Stop()

	line, err := c.snapshotLine()
	if err != nil {
		return err
	}

	err = emit(line)
	if err != nil {
		return err
	}

	// last is the seq of the newest event the client has, in a line or a
	// snapshot. An event at or below it is already covered.
	last := line.Seq

	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, open := <-sub.Events():
			if !open {
				return nil
			}

			switch {
			case evt.Seq <= last:
			case evt.Dropped > 0:
				// The bus lost events just before this one, so a snapshot
				// replaces them and this one too.
				last, err = c.resync(emit, sub, evt.Dropped+1)
			default:
				err = emit(watchLineFor(evt))
				last = evt.Seq
			}

			if err != nil {
				return err
			}

			// Caught up, with no event left to report a loss in its
			// Dropped, so ask the bus. It loses events only while the buffer
			// is full, so a loss it still holds happened before the buffer
			// emptied.
			if len(sub.Events()) > 0 {
				continue
			}

			if sub.Missed() > 0 {
				last, err = c.resync(emit, sub, 0)
				if err != nil {
					return err
				}
			}
		}
	}
}

// watchFilter reads the --event flags a watch was given into the events it
// sends, nil when it sends every one. It refuses anything else before the
// first line, so a client learns what this daemon cannot filter on.
func watchFilter(args []string) ([]event.Name, error) {
	var wanted []event.Name

	for idx := 0; idx < len(args); idx++ {
		value, found := strings.CutPrefix(args[idx], watchEventFlag+"=")
		if !found {
			if args[idx] != watchEventFlag {
				return nil, derrors.New(
					derrors.CodeInvalidInput,
					"unexpected argument for watch: "+args[idx],
				)
			}

			if idx+1 == len(args) {
				return nil, derrors.New(derrors.CodeInvalidInput, msgWatchEventValue)
			}

			idx++
			value = args[idx]
		}

		for name := range strings.SplitSeq(value, ",") {
			err := checkWatchEvent(name)
			if err != nil {
				return nil, err
			}

			wanted = append(wanted, event.Name(name))
		}
	}

	return wanted, nil
}

// checkWatchEvent refuses a name that is no event a watch streams.
func checkWatchEvent(name string) error {
	switch {
	case name == "":
		return derrors.New(derrors.CodeInvalidInput, msgWatchEventValue)
	case name == watchSnapshot:
		return derrors.New(
			derrors.CodeInvalidInput,
			"snapshot lines always come, so --event takes the events only",
		)
	case slices.Contains(event.All(), event.Name(name)):
		return nil
	}

	names := make([]string, 0, len(event.All()))
	for _, known := range event.All() {
		names = append(names, string(known))
	}

	return derrors.Newf(
		derrors.CodeInvalidInput,
		"%q is not an event watch streams, which are %s",
		name,
		strings.Join(names, ", "),
	)
}

// snapshotLine is the status as of the newest event published.
func (c *Controller) snapshotLine() (ipc.WatchLine, error) {
	return c.snapshotAt(c.Events.Seq())
}

// snapshotAt is the status, read now, as a snapshot covering the events up to
// seq. One published since seq may show in it as well as after it.
func (c *Controller) snapshotAt(seq uint64) (ipc.WatchLine, error) {
	status, ok := c.infoHandler.statusData()
	if !ok {
		return ipc.WatchLine{}, derrors.New(derrors.CodeActionFailed, "config not available")
	}

	return ipc.WatchLine{Seq: seq, Event: watchSnapshot, Status: &status}, nil
}

// resync sends a snapshot in place of the events since the last line, and
// returns its seq. replaced starts as the count of events already known to be
// replaced. The snapshot also replaces every event still queued, along with
// the events each one reports lost before it, and every loss the bus holds.
//
// It takes all of those and the snapshot's seq in one hold of the bus's lock,
// so no event can arrive in between to be covered and not counted. The bus
// keeps any later loss for the next event or the next empty buffer to report.
func (c *Controller) resync(
	emit func(value any) error,
	sub *event.Subscription,
	replaced uint64,
) (uint64, error) {
	taken, seq := sub.Drain()

	line, err := c.snapshotAt(seq)
	if err != nil {
		return 0, err
	}

	line.Dropped = replaced + taken

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
