package event

import (
	"sync"

	"go.uber.org/zap"
)

// Bus hands each published event to every subscriber, in one order for all of
// them.
//
// Publish never blocks, because events are raised from places that must not:
// the mode handler publishes under its lock, and the application watcher on
// macOS publishes from the main queue. A subscriber whose buffer is full
// misses the event, and the next one it does receive counts what it missed.
//
// mu is a leaf. It is held only across non-blocking channel sends and never
// across a call out of this package, logging included.
type Bus struct {
	logger *zap.Logger

	mu   sync.Mutex
	seq  uint64
	subs map[*subscription]struct{}
}

type subscription struct {
	events  chan Event
	dropped uint64
}

// NewBus returns a bus with no subscribers. A nil logger logs nothing.
func NewBus(logger *zap.Logger) *Bus {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Bus{logger: logger.Named("events"), subs: make(map[*subscription]struct{})}
}

// Publish numbers the event and offers it to every subscriber. A nil bus
// publishes nothing.
func (b *Bus) Publish(evt Event) {
	if b == nil {
		return
	}

	b.offer(&evt)

	// Checked first, so a daemon not logging at debug builds no fields.
	if ce := b.logger.Check(zap.DebugLevel, "Event published"); ce != nil {
		ce.Write(
			zap.Uint64("seq", evt.Seq),
			zap.String("event", string(evt.Name)),
			zap.String("mode", evt.Mode),
			zap.String("reason", string(evt.Reason)),
			zap.String("bundle_id", evt.BundleID),
			zap.Bool("ok", evt.OK))
	}
}

// Seq returns the number of the last event published, zero before the first.
// Every event numbered at or below it has already been offered to the
// subscribers.
func (b *Bus) Seq() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.seq
}

// Subscribe returns a channel of every event published from now on, holding
// up to buffer of them for a reader that falls behind, and the function that
// ends the subscription and closes the channel. The channel is closed only
// by that function. A nil bus returns a channel that is already closed.
func (b *Bus) Subscribe(buffer int) (<-chan Event, func()) {
	if b == nil {
		closed := make(chan Event)
		close(closed)

		return closed, func() {}
	}

	sub := &subscription{events: make(chan Event, buffer)}

	b.mu.Lock()
	b.subs[sub] = struct{}{}
	b.mu.Unlock()

	var once sync.Once

	return sub.events, func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()

			delete(b.subs, sub)
			close(sub.events)
		})
	}
}

// offer numbers evt and makes one non-blocking send of it to each subscriber.
func (b *Bus) offer(evt *Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.seq++
	evt.Seq = b.seq

	for sub := range b.subs {
		delivered := *evt
		delivered.Dropped = sub.dropped

		select {
		case sub.events <- delivered:
			sub.dropped = 0
		default:
			sub.dropped++
		}
	}
}
