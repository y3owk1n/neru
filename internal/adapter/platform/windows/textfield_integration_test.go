//go:build integration && windows

package windows

import (
	"errors"
	"image"
	"testing"
	"time"
)

// openTestTextField opens a field and records what it reports. It skips the
// test in a session that does not let the process take the foreground, which
// the field needs as much as bringToFront does.
func openTestTextField(t *testing.T) chan string {
	t.Helper()

	reported := make(chan string, 32)

	field, err := OpenTextField(image.Rect(100, 100, 400, 130), TextFieldEvents{
		QueryChanged: func(query string) { reported <- "query " + query },
		Confirm:      func() { reported <- "confirm" },
		Cancel:       func() { reported <- "cancel" },
	})
	if errors.Is(err, errTextFieldNotForeground) {
		t.Skip("skipping: this session does not let the test process take the foreground")
	}

	if err != nil {
		t.Fatalf("OpenTextField() error = %v", err)
	}

	t.Cleanup(field.Close)

	return reported
}

// waitForReport reads the field's reports until want arrives.
func waitForReport(t *testing.T, reported <-chan string, want string) {
	t.Helper()

	var seen []string

	deadline := time.After(5 * time.Second)

	for {
		select {
		case report := <-reported:
			seen = append(seen, report)
			if report == want {
				return
			}
		case <-deadline:
			t.Fatalf("never saw %q, saw %v", want, seen)
		}
	}
}

func feedKeys(t *testing.T, keys ...string) {
	t.Helper()

	for _, key := range keys {
		err := FeedKey(key)
		if err != nil {
			t.Fatalf("FeedKey(%q) error = %v", key, err)
		}
	}
}

// TestTextField_TypesTheQueryAndConfirms types through real key events. A
// shifted symbol arrives as the symbol, which the raw key stream cannot carry,
// and Return confirms rather than typing a line break.
func TestTextField_TypesTheQueryAndConfirms(t *testing.T) {
	reported := openTestTextField(t)

	feedKeys(t, "Shift+s", "a", "!")
	waitForReport(t, reported, "query Sa!")

	feedKeys(t, "Return")
	waitForReport(t, reported, "confirm")
}

// TestTextField_EscapeCancels pins the other way out of a search.
func TestTextField_EscapeCancels(t *testing.T) {
	reported := openTestTextField(t)

	feedKeys(t, "Escape")
	waitForReport(t, reported, "cancel")
}
