//go:build integration && windows

package windows

import (
	"errors"
	"image"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// openTestTextField opens a field and records what it reports. The field
// takes the keyboard and the tests type real keys, so they run only under
// `just test-desktop`. It also skips in a session that does not let the
// process take the foreground, which the field needs as much as bringToFront
// does.
func openTestTextField(t *testing.T) (*TextField, chan string) {
	t.Helper()

	if testing.Short() || os.Getenv("NERU_DESKTOP_TESTS") == "" {
		t.Skip("skipping desktop-driving test; run `just test-desktop` to include it")
	}

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

	return field, reported
}

// waitForReport reads the field's reports until want arrives. A cancel it
// was not waiting for fails the test, since a stray one would close the
// search.
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

			if report == "cancel" {
				t.Fatalf("the field canceled while waiting for %q, saw %v", want, seen)
			}
		case <-deadline:
			t.Fatalf("never saw %q, saw %v", want, seen)
		}
	}
}

// feedKeys types keys into the field. Each key is sent only while the field
// holds the foreground, so a key can never land in another application.
func feedKeys(t *testing.T, field *TextField, keys ...string) {
	t.Helper()

	for _, key := range keys {
		if windows.GetForegroundWindow() != windows.HWND(field.hwnd) {
			t.Skip("skipping: the field lost the foreground before the keys were typed")
		}

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
	field, reported := openTestTextField(t)

	feedKeys(t, field, "Shift+s", "a", "!")
	waitForReport(t, reported, "query Sa!")

	feedKeys(t, field, "Return")
	waitForReport(t, reported, "confirm")
}

// TestTextField_EscapeCancels pins the other way out of a search.
func TestTextField_EscapeCancels(t *testing.T) {
	field, reported := openTestTextField(t)

	feedKeys(t, field, "Escape")
	waitForReport(t, reported, "cancel")
}
