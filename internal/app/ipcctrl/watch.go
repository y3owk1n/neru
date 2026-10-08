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

// watchLine is one line of a watch stream, on the wire.
type watchLine struct {
	Seq      uint64         `json:"seq"`
	Event    string         `json:"event"`
	Mode     string         `json:"mode,omitempty"`
	Reason   string         `json:"reason,omitempty"`
	BundleID string         `json:"bundle_id,omitempty"` //nolint:tagliatelle // snake_case like the status keys.
	OK       *bool          `json:"ok,omitempty"`
	Dropped  uint64         `json:"dropped,omitempty"`
	Status   map[string]any `json:"status,omitempty"`
}

// HandleWatch serves `neru watch`: the status, then every event the daemon
// publishes, until the client goes away or the daemon stops. It answers while
// Neru is stopped too, or nobody would see the enable event.
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

	seq := c.Events.Seq()

	status, ok := c.infoHandler.statusData()
	if !ok {
		return derrors.New(derrors.CodeActionFailed, "config not available")
	}

	err := emit(watchLine{Seq: seq, Event: watchSnapshot, Status: status})
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case evt, open := <-events:
			if !open {
				return nil
			}

			if evt.Seq <= seq {
				continue
			}

			err = emit(watchLineFor(evt))
			if err != nil {
				return err
			}
		}
	}
}

// watchLineFor puts an event on the wire. ok is sent only for a config reload,
// where false is the answer that matters.
func watchLineFor(evt event.Event) watchLine {
	line := watchLine{
		Seq:      evt.Seq,
		Event:    string(evt.Name),
		Mode:     evt.Mode,
		Reason:   string(evt.Reason),
		BundleID: evt.BundleID,
		Dropped:  evt.Dropped,
	}

	if evt.Name == event.ConfigReload {
		line.OK = &evt.OK
	}

	return line
}
