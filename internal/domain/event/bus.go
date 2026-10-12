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
// mu is a leaf. It is held only across non-blocking channel sends and
// receives, and never across a call out of this package, logging included.
type Bus struct {
	logger *zap.Logger

	mu   sync.Mutex
	seq  uint64
	subs map[*subscription]struct{}
}

type subscription struct {
	events  chan Event
	dropped uint64
	// names are the events offered, every one when nil.
	names map[Name]bool
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
			zap.String("action", evt.Action),
			zap.String("bundle_id", evt.BundleID),
			zap.Bool("ok", evt.OK),
			zap.Bool("on", evt.On),
			zap.String("slot", evt.Slot),
			zap.Int("x", evt.Point.X),
			zap.Int("y", evt.Point.Y),
			zap.String("modifiers", evt.Modifiers),
			zap.String("monitor", evt.Monitor))
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
	sub := b.SubscribeTo(buffer, nil)

	return sub.Events(), sub.Stop
}

// Subscription receives the events it was opened for, and can ask how many
// of them it has lost.
type Subscription struct {
	bus  *Bus
	sub  *subscription
	stop func()
}

// SubscribeTo is Subscribe for only the events named, every event when names
// is empty. An event left out is never offered, so it can neither fill the
// buffer nor count as missed. Its number still advances the bus's Seq.
func (b *Bus) SubscribeTo(buffer int, names []Name) *Subscription {
	if b == nil {
		closed := make(chan Event)
		close(closed)

		return &Subscription{sub: &subscription{events: closed}, stop: func() {}}
	}

	sub := &subscription{events: make(chan Event, buffer)}

	if len(names) > 0 {
		sub.names = make(map[Name]bool, len(names))
		for _, name := range names {
			sub.names[name] = true
		}
	}

	b.mu.Lock()
	b.subs[sub] = struct{}{}
	b.mu.Unlock()

	var once sync.Once

	return &Subscription{bus: b, sub: sub, stop: func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()

			delete(b.subs, sub)
			close(sub.events)
		})
	}}
}

// Events is the channel the subscription's events arrive on. Only Stop
// closes it.
func (s *Subscription) Events() <-chan Event {
	return s.sub.events
}

// Stop ends the subscription and closes its channel.
func (s *Subscription) Stop() {
	s.stop()
}

// Missed reports how many events the subscription has lost since the last
// one it was handed, without clearing the count.
func (s *Subscription) Missed() uint64 {
	if s.bus == nil {
		return 0
	}

	s.bus.mu.Lock()
	defer s.bus.mu.Unlock()

	return s.sub.dropped
}

// Drain takes every event still queued for the subscription and every loss
// it has not been told of. It returns how many events that is, counting each
// queued event with the losses it reports, and the number of the newest event
// published. It holds the lock publishers take, so no event arrives while it
// runs, and every event it counts is numbered at or below that one.
func (s *Subscription) Drain() (uint64, uint64) {
	if s.bus == nil {
		return 0, 0
	}

	s.bus.mu.Lock()
	defer s.bus.mu.Unlock()

	taken := s.sub.dropped
	s.sub.dropped = 0

	for {
		select {
		case evt, open := <-s.sub.events:
			if !open {
				return taken, s.bus.seq
			}

			taken += 1 + evt.Dropped
		default:
			return taken, s.bus.seq
		}
	}
}

// offer numbers evt and makes one non-blocking send of it to each subscriber.
func (b *Bus) offer(evt *Event) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.seq++
	evt.Seq = b.seq

	for sub := range b.subs {
		if sub.names != nil && !sub.names[evt.Name] {
			continue
		}

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
