package modes

import (
	"image"
	"testing"

	"github.com/y3owk1n/neru/internal/core/domain/element"
	domainHint "github.com/y3owk1n/neru/internal/core/domain/hint"
)

func fpHint(t *testing.T, stableID string, bounds image.Rectangle) *domainHint.Interface {
	t.Helper()

	el, err := element.NewElement(
		element.ID(stableID+"#ptr"),
		bounds,
		element.Role("AXButton"),
		element.WithStableID(stableID),
	)
	if err != nil {
		t.Fatalf("NewElement: %v", err)
	}

	h, err := domainHint.NewHint("a", el, el.Center())
	if err != nil {
		t.Fatalf("NewHint: %v", err)
	}

	return h
}

// The fingerprint is what lets endObserverScanWindow tell a real change from the
// create/destroy churn a scan induces: an unchanged set (even reordered) must
// hash equal so the post-scan margin opens and the flicker loop is broken, while
// any add, removal, or move must hash differently so a real change stays hot.
func TestFingerprintHints(t *testing.T) {
	a := image.Rect(0, 0, 10, 10)
	b := image.Rect(20, 20, 40, 44)
	c := image.Rect(5, 5, 15, 30)

	base := []*domainHint.Interface{
		fpHint(t, "x1", a),
		fpHint(t, "x2", b),
	}

	reordered := []*domainHint.Interface{
		fpHint(t, "x2", b),
		fpHint(t, "x1", a),
	}

	added := []*domainHint.Interface{
		fpHint(t, "x1", a),
		fpHint(t, "x2", b),
		fpHint(t, "x3", c),
	}

	removed := []*domainHint.Interface{
		fpHint(t, "x1", a),
	}

	moved := []*domainHint.Interface{
		fpHint(t, "x1", a),
		fpHint(t, "x2", c), // same identity, different bounds
	}

	reidentified := []*domainHint.Interface{
		fpHint(t, "x1", a),
		fpHint(t, "x9", b), // same bounds, different identity
	}

	baseFP := fingerprintHints(base)

	if got := fingerprintHints(reordered); got != baseFP {
		t.Errorf("reordered set changed fingerprint: %#x != %#x", got, baseFP)
	}

	for name, set := range map[string][]*domainHint.Interface{
		"added":        added,
		"removed":      removed,
		"moved":        moved,
		"reidentified": reidentified,
	} {
		if got := fingerprintHints(set); got == baseFP {
			t.Errorf("%s set did not change fingerprint (both %#x)", name, got)
		}
	}

	if fingerprintHints(nil) != fingerprintHints([]*domainHint.Interface{}) {
		t.Error("nil and empty sets must hash equal")
	}
}
