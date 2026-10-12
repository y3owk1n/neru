package event_test

import (
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/domain/event"
)

func TestBus_Publish_NumbersEventsInOrder(t *testing.T) {
	bus := event.NewBus(nil)

	first, stopFirst := bus.Subscribe(4)
	defer stopFirst()

	second, stopSecond := bus.Subscribe(4)
	defer stopSecond()

	bus.Publish(event.Event{Name: event.ModeEnter, Mode: "hints"})
	bus.Publish(event.Event{Name: event.ModeExit, Mode: "hints"})

	for _, events := range []<-chan event.Event{first, second} {
		enter, exit := <-events, <-events

		if enter.Seq != 1 || enter.Name != event.ModeEnter {
			t.Errorf("first event = %+v, want seq 1 %s", enter, event.ModeEnter)
		}

		if exit.Seq != 2 || exit.Name != event.ModeExit {
			t.Errorf("second event = %+v, want seq 2 %s", exit, event.ModeExit)
		}
	}
}

func TestBus_Publish_CountsWhatAFullSubscriberMissed(t *testing.T) {
	bus := event.NewBus(nil)

	events, stop := bus.Subscribe(1)
	defer stop()

	for range 3 {
		bus.Publish(event.Event{Name: event.Enable})
	}

	if got := <-events; got.Seq != 1 || got.Dropped != 0 {
		t.Fatalf("buffered event = %+v, want seq 1 with nothing dropped", got)
	}

	bus.Publish(event.Event{Name: event.Disable})

	got := <-events
	if got.Seq != 4 || got.Dropped != 2 {
		t.Errorf("event after the overflow = %+v, want seq 4 with 2 dropped", got)
	}
}

// TestBus_SubscribeTo_OffersOnlyTheEventsNamed pins a filtered subscription:
// it receives only the events it named, an event left out never counts as
// missed however many go by, and the numbers stay the bus's own.
func TestBus_SubscribeTo_OffersOnlyTheEventsNamed(t *testing.T) {
	bus := event.NewBus(nil)

	sub := bus.SubscribeTo(1, []event.Name{event.Enable})
	defer sub.Stop()

	for range 5 {
		bus.Publish(event.Event{Name: event.Disable})
	}

	bus.Publish(event.Event{Name: event.Enable})

	if got := <-sub.Events(); got.Name != event.Enable || got.Seq != 6 || got.Dropped != 0 {
		t.Errorf("event = %+v, want enable at seq 6 with nothing dropped", got)
	}
}

// TestSubscription_Drain_TakesEverythingTheBufferHolds pins the catch-up a
// reader asks for: every queued event and every loss, counted with the newest
// event's number, which covers all of them, and none reported again after.
func TestSubscription_Drain_TakesEverythingTheBufferHolds(t *testing.T) {
	bus := event.NewBus(nil)

	sub := bus.SubscribeTo(2, nil)
	defer sub.Stop()

	for range 5 {
		bus.Publish(event.Event{Name: event.Enable})
	}

	if missed := sub.Missed(); missed != 3 {
		t.Fatalf("Missed() = %d, want the 3 the full buffer lost", missed)
	}

	if taken, seq := sub.Drain(); taken != 5 || seq != 5 {
		t.Fatalf("Drain() = %d, %d, want the 2 queued and 3 lost, through seq 5", taken, seq)
	}

	if missed := sub.Missed(); missed != 0 {
		t.Errorf("Missed() after Drain = %d, want the loss already taken", missed)
	}

	bus.Publish(event.Event{Name: event.Disable})

	if got := <-sub.Events(); got.Seq != 6 || got.Dropped != 0 {
		t.Errorf("next event = %+v, want seq 6 with nothing dropped", got)
	}
}

func TestBus_Publish_DoesNotBlockOnASubscriberThatNeverReads(t *testing.T) {
	bus := event.NewBus(nil)

	_, stop := bus.Subscribe(0)
	defer stop()

	published := make(chan struct{})

	go func() {
		bus.Publish(event.Event{Name: event.Enable})
		close(published)
	}()

	select {
	case <-published:
	case <-time.After(time.Second):
		t.Fatal("Publish blocked on a subscriber that never reads")
	}
}

func TestBus_Subscribe_StopClosesTheChannelAndEndsDelivery(t *testing.T) {
	bus := event.NewBus(nil)

	events, stop := bus.Subscribe(1)
	stop()
	stop()

	bus.Publish(event.Event{Name: event.Enable})

	if got, open := <-events; open {
		t.Errorf("received %+v after stop, want a closed channel", got)
	}
}

// TestBus_Publish_RacesSubscribeAndStop is the -race pin for closing a
// subscription under the bus lock: a close moved outside it would let a
// concurrent Publish send on a closed channel and panic.
func TestBus_Publish_RacesSubscribeAndStop(t *testing.T) {
	bus := event.NewBus(nil)

	var publishers sync.WaitGroup

	for range 4 {
		publishers.Go(func() {
			for range 1000 {
				bus.Publish(event.Event{Name: event.Enable})
			}
		})
	}

	for range 1000 {
		events, stop := bus.Subscribe(1)
		stop()

		for range events {
		}
	}

	publishers.Wait()

	events, stop := bus.Subscribe(1)
	defer stop()

	bus.Publish(event.Event{Name: event.Disable})

	if got := <-events; got.Seq != 4001 {
		t.Errorf("seq after the race = %d, want 4001", got.Seq)
	}
}

func TestBus_Subscribe_NilBusReturnsAClosedChannel(t *testing.T) {
	var bus *event.Bus

	events, stop := bus.Subscribe(1)
	defer stop()

	if got, open := <-events; open {
		t.Errorf("received %+v from a nil bus, want a closed channel", got)
	}
}
