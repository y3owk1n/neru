package modes

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/derrors"
	portmocks "github.com/y3owk1n/neru/internal/ports/mocks"
)

// notifyWait bounds each side of the handoff. It only has to outlast a
// goroutine start on a loaded machine, so anything longer just slows a failure
// down.
const notifyWait = 2 * time.Second

// TestHandlerState_ReportMonitorSelectNotSupported_NotifiesOffTheLock pins the
// reason the notification is handed to a goroutine. Since ShowNotification
// gained an error it is a session-bus round trip on Linux, and this runs from
// activateMonitorSelectMode with h.mu held, the one place the locking contract
// forbids a blocking call, because h.mu serializes key handling. A notification
// daemon that never answers must cost the user nothing, and the message must
// still be attempted.
func TestHandlerState_ReportMonitorSelectNotSupported_NotifiesOffTheLock(t *testing.T) {
	attempted := make(chan struct{}, 1)
	wedged := make(chan struct{})

	defer close(wedged)

	system := &portmocks.MockSystemPort{
		ShowNotificationFunc: func(_ context.Context, _, _ string) error {
			attempted <- struct{}{}
			// A notification daemon that has stopped answering.
			<-wedged

			return derrors.New(derrors.CodeNotSupported, "no notification daemon")
		},
	}

	handler := newHandlerWithState(handlerState{
		system: system,
		ctx:    context.Background(),
		logger: zap.NewNop(),
	})

	returned := make(chan struct{})

	go func() {
		handler.mu.Lock()
		defer handler.mu.Unlock()

		handler.reportMonitorSelectNotSupported()
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(notifyWait):
		t.Fatal(
			"reportMonitorSelectNotSupported waited on the notification while holding h.mu; " +
				"a wedged notification daemon would freeze key handling",
		)
	}

	select {
	case <-attempted:
	case <-time.After(notifyWait):
		t.Fatal("the notification was never attempted, so the user is told nothing")
	}
}
