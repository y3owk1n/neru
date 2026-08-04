package loader_test

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/config/loader"
)

// blockingAlertProvider stands in for the macOS alert, which is a modal dialog
// that returns only once the user dismisses it.
type blockingAlertProvider struct {
	shown    chan struct{}
	release  chan struct{}
	showing  atomic.Bool
	released bool
}

func newBlockingAlertProvider() *blockingAlertProvider {
	return &blockingAlertProvider{
		shown:   make(chan struct{}, 1),
		release: make(chan struct{}),
	}
}

func (p *blockingAlertProvider) ShowAlert(_ context.Context, _, _ string) error {
	p.showing.Store(true)
	defer p.showing.Store(false)

	p.shown <- struct{}{}

	<-p.release

	return nil
}

// isShowing reports whether an alert is currently up.
func (p *blockingAlertProvider) isShowing() bool {
	return p.showing.Load()
}

func (p *blockingAlertProvider) dismiss() {
	if !p.released {
		p.released = true

		close(p.release)
	}
}

// A reload of an invalid config must report the error to whoever asked for it
// rather than waiting behind the alert. Held inline, the dialog made
// `neru config reload` fail with a receive timeout — the daemon looking hung
// when the real problem was a bad config file.
func TestReloadWithAppContext_DoesNotWaitForTheAlert(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	// An action no command provides, so validation fails on load.
	invalid := "[hotkeys]\n\"Primary+Shift+Y\" = \"action bogus_thing\"\n"

	writeErr := os.WriteFile(path, []byte(invalid), 0o600)
	if writeErr != nil {
		t.Fatalf("write config: %v", writeErr)
	}

	alert := newBlockingAlertProvider()
	t.Cleanup(alert.dismiss)

	service := loader.NewService(config.DefaultConfig(), path, zap.NewNop(), alert)

	reloaded := make(chan error, 1)

	go func() {
		_, err := service.ReloadWithAppContext(context.Background(), path, zap.NewNop())
		reloaded <- err
	}()

	select {
	case err := <-reloaded:
		if err == nil {
			t.Fatal("ReloadWithAppContext() = nil, want the validation error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ReloadWithAppContext() blocked on the alert instead of reporting the error")
	}

	// The alert is still shown — it is the only feedback the systray menu
	// item has — just not on the reply's path.
	select {
	case <-alert.shown:
	case <-time.After(5 * time.Second):
		t.Fatal("the alert was never shown")
	}
}

// Repeated failing reloads must not stack dialogs. Shown inline the alert
// serialized them — a second reload could not start until the first was
// dismissed — and moving it off the reply's path removed that, so the limit
// has to be explicit.
func TestReloadWithAppContext_ShowsOneAlertAtATime(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	invalid := "[hotkeys]\n\"Primary+Shift+Y\" = \"action bogus_thing\"\n"

	writeErr := os.WriteFile(path, []byte(invalid), 0o600)
	if writeErr != nil {
		t.Fatalf("write config: %v", writeErr)
	}

	alert := newBlockingAlertProvider()
	t.Cleanup(alert.dismiss)

	service := loader.NewService(config.DefaultConfig(), path, zap.NewNop(), alert)

	const reloads = 5

	for range reloads {
		_, err := service.ReloadWithAppContext(context.Background(), path, zap.NewNop())
		if err == nil {
			t.Fatal("ReloadWithAppContext() = nil, want the validation error")
		}
	}

	// The first alert is still up and undismissed, so no other may have been
	// started behind it.
	select {
	case <-alert.shown:
	case <-time.After(5 * time.Second):
		t.Fatal("the first alert was never shown")
	}

	select {
	case <-alert.shown:
		t.Fatal("a second alert was queued while the first was still showing")
	case <-time.After(200 * time.Millisecond):
	}

	// Once it is dismissed, a later failure is reported again rather than
	// being suppressed for good.
	alert.dismiss()

	waitFor(t, func() bool { return !alert.isShowing() })

	_, err := service.ReloadWithAppContext(context.Background(), path, zap.NewNop())
	if err == nil {
		t.Fatal("ReloadWithAppContext() = nil, want the validation error")
	}

	select {
	case <-alert.shown:
	case <-time.After(5 * time.Second):
		t.Fatal("no alert after the previous one was dismissed")
	}
}

// waitFor polls until cond holds, so a test does not depend on how promptly a
// goroutine is scheduled.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("condition was not met in time")
}
