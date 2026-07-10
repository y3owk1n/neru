package axobserver

import (
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/core/infra/axnotify"
)

func TestNotificationBitsCoverVocabulary(t *testing.T) {
	names := axnotify.AllNames()

	if len(notificationBits) != len(names) {
		t.Fatalf("notificationBits has %d entries, axnotify vocabulary has %d", len(notificationBits), len(names))
	}

	for _, name := range names {
		if _, ok := notificationBits[name]; !ok {
			t.Errorf("axnotify name %q has no mask bit", name)
		}
	}
}

func TestMaskFromNamesBuildsUnion(t *testing.T) {
	mask, err := MaskFromNames([]string{axnotify.Created, axnotify.MenuOpened, axnotify.MenuClosed})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := notifCreated | notifMenuOpened | notifMenuClosed
	if mask != want {
		t.Fatalf("mask = %d, want %d", mask, want)
	}
}

func TestMaskFromNamesEveryNameMapsToADistinctBit(t *testing.T) {
	seen := Mask(0)

	for _, name := range axnotify.AllNames() {
		mask, err := MaskFromNames([]string{name})
		if err != nil {
			t.Fatalf("MaskFromNames(%q): %v", name, err)
		}

		if mask == 0 {
			t.Errorf("%q mapped to the zero mask", name)
		}

		if seen&mask != 0 {
			t.Errorf("%q shares a bit with an earlier name", name)
		}

		seen |= mask
	}
}

func TestMaskFromNamesIsIdempotentForDuplicates(t *testing.T) {
	mask, err := MaskFromNames([]string{axnotify.Created, axnotify.Created, axnotify.Created})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mask != notifCreated {
		t.Fatalf("duplicate names should collapse to one bit; mask = %d, want %d", mask, notifCreated)
	}
}

func TestMaskFromNamesEmptyIsZeroNoError(t *testing.T) {
	mask, err := MaskFromNames(nil)
	if err != nil {
		t.Fatalf("empty list should not error: %v", err)
	}

	if mask != 0 {
		t.Fatalf("empty list should yield the zero mask; got %d", mask)
	}
}

func TestMaskFromNamesRejectsUnknownName(t *testing.T) {
	_, err := MaskFromNames([]string{axnotify.Created, "not_a_real_notification"})
	if err == nil {
		t.Fatal("an unknown notification name must be an error")
	}

	if !strings.Contains(err.Error(), "not_a_real_notification") {
		t.Errorf("error should name the offending notification, got: %v", err)
	}

	if !strings.Contains(err.Error(), axnotify.LayoutChanged) {
		t.Errorf("error should list the supported names, got: %v", err)
	}
}
