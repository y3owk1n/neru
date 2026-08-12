//go:build linux

package kwin

import (
	"errors"
	"image"
	"testing"

	"go.uber.org/zap"
)

// errTestInstallFailed stands in for whatever kept the KWin script from being
// installed; Bounds only has to carry it back, not interpret it.
var errTestInstallFailed = errors.New("org.kde.KWin is not on the session bus")

// TestGeometry_BoundsBeforeAnyUpdate pins the cold answer. Nothing has been
// pushed yet, so the bridge reports "nothing to say" — not a zero rectangle a
// caller could mistake for a window at the origin, and not a failure either.
func TestGeometry_BoundsBeforeAnyUpdate(t *testing.T) {
	geometry := newGeometry(zap.NewNop())

	rect, ok, err := geometry.Bounds()
	if ok || err != nil {
		t.Fatalf("Bounds() on a fresh bridge = (%v, %v, %v), want (empty, false, nil)",
			rect, ok, err)
	}
}

// TestGeometry_UpdateActiveWindowFeedsBounds is the whole point of the bridge:
// what the KWin script pushes is what FocusedWindowBounds later answers with.
func TestGeometry_UpdateActiveWindowFeedsBounds(t *testing.T) {
	geometry := newGeometry(zap.NewNop())

	dbusErr := geometry.UpdateActiveWindow("100,50,800,600,konsole")
	if dbusErr != nil {
		t.Fatalf("UpdateActiveWindow returned %v, want nil", dbusErr)
	}

	rect, ok, err := geometry.Bounds()
	if !ok || err != nil {
		t.Fatalf("Bounds() = (%v, %v, %v), want a rectangle with ok=true", rect, ok, err)
	}

	want := image.Rect(100, 50, 900, 650)
	if rect != want {
		t.Errorf("Bounds() = %v, want %v", rect, want)
	}
}

// TestGeometry_UpdateActiveWindowRejectsUnusablePayloads keeps a malformed or
// degenerate push from replacing a good cache. The script is remote code as far
// as this process is concerned: a truncated payload, a non-numeric field or a
// zero-area window must leave the last known geometry alone rather than
// answering with a rectangle nothing can be inside.
func TestGeometry_UpdateActiveWindowRejectsUnusablePayloads(t *testing.T) {
	good := image.Rect(100, 50, 900, 650)

	payloads := []struct {
		name    string
		payload string
	}{
		{"too few fields", "100,50,800"},
		{"non-numeric origin", "x,50,800,600,konsole"},
		{"non-numeric size", "100,50,wide,600,konsole"},
		{"zero width", "100,50,0,600,konsole"},
		{"negative height", "100,50,800,-600,konsole"},
		{"empty", ""},
	}

	for _, testCase := range payloads {
		t.Run(testCase.name, func(t *testing.T) {
			geometry := newGeometry(zap.NewNop())
			_ = geometry.UpdateActiveWindow("100,50,800,600,konsole")

			dbusErr := geometry.UpdateActiveWindow(testCase.payload)
			if dbusErr != nil {
				t.Fatalf("UpdateActiveWindow returned %v; malformed pushes are dropped, "+
					"not reported back to KWin", dbusErr)
			}

			rect, ok, err := geometry.Bounds()
			if !ok || err != nil || rect != good {
				t.Errorf("after %q Bounds() = (%v, %v, %v), want the previous %v",
					testCase.payload, rect, ok, err, good)
			}
		})
	}
}

// TestGeometry_UpdateActiveWindowAcceptsAMissingResourceClass keeps the geometry
// working when the class is absent — it is a diagnostic, not part of the
// answer.
func TestGeometry_UpdateActiveWindowAcceptsAMissingResourceClass(t *testing.T) {
	geometry := newGeometry(zap.NewNop())

	_ = geometry.UpdateActiveWindow("0,0,1920,1080")

	rect, ok, err := geometry.Bounds()
	if !ok || err != nil || rect != image.Rect(0, 0, 1920, 1080) {
		t.Errorf("Bounds() = (%v, %v, %v), want the pushed rectangle", rect, ok, err)
	}
}

// TestGeometry_BoundsReportsAFailedInstall is the honesty half. A bridge that
// never installed cannot answer, and saying "no focused window" there is the
// silent fallback this source exists to end: the caller would constrain to the
// active screen believing it had asked and been told there was no window.
func TestGeometry_BoundsReportsAFailedInstall(t *testing.T) {
	geometry := newGeometry(zap.NewNop())

	geometry.endStart(errTestInstallFailed)

	_, ok, err := geometry.Bounds()
	if ok {
		t.Fatal("Bounds() reported ok after a failed install")
	}

	if !errors.Is(err, errTestInstallFailed) {
		t.Errorf("Bounds() error = %v, want it to carry %v", err, errTestInstallFailed)
	}
}

// TestGeometry_BoundsPrefersLiveGeometryOverAStaleFailure covers the ordering
// that matters when the script is reinstalled after a transient failure: a
// geometry push proves the bridge works, so the recorded reason is spent.
func TestGeometry_BoundsPrefersLiveGeometryOverAStaleFailure(t *testing.T) {
	geometry := newGeometry(zap.NewNop())

	geometry.endStart(errTestInstallFailed)
	_ = geometry.UpdateActiveWindow("10,20,300,400,kate")

	rect, ok, err := geometry.Bounds()
	if !ok || err != nil {
		t.Fatalf("Bounds() = (%v, %v, %v), want the pushed rectangle", rect, ok, err)
	}
}

// TestGeometry_BeginStartRetriesUntilTheScriptIsInstalled pins the install
// guard.
//
// The daemon starts when the session does, so the first attempt can land before
// the session bus or KWin is up. A once-only attempt would spend the daemon's
// only try on that race and leave the whole run with no focused-window
// geometry, which is the failure this source exists to remove — so a failed
// attempt is retried by the next caller, while one already in flight is not
// duplicated and a successful one is never repeated.
func TestGeometry_BeginStartRetriesUntilTheScriptIsInstalled(t *testing.T) {
	geometry := newGeometry(zap.NewNop())

	if !geometry.beginStart() {
		t.Fatal("the first caller was not allowed to install")
	}

	if geometry.beginStart() {
		t.Error("a second caller started a duplicate install while one was in flight")
	}

	geometry.endStart(errTestInstallFailed)

	if !geometry.beginStart() {
		t.Fatal("a failed install was never retried; the daemon would run blind until restart")
	}

	geometry.endStart(nil)

	if geometry.beginStart() {
		t.Error("an installed script was reinstalled")
	}
}

// TestGeometry_ASuccessfulRetryClearsTheRecordedReason keeps a spent reason
// from outliving the condition it described.
func TestGeometry_ASuccessfulRetryClearsTheRecordedReason(t *testing.T) {
	geometry := newGeometry(zap.NewNop())

	geometry.beginStart()
	geometry.endStart(errTestInstallFailed)

	geometry.beginStart()
	geometry.endStart(nil)

	_, ok, err := geometry.Bounds()
	if ok || err != nil {
		t.Fatalf("Bounds() after a successful retry = (%v, %v), want (false, nil) — "+
			"installed, with nothing reported yet", ok, err)
	}
}

// TestShared_ReturnsOneBridge pins the shared derivation: both callers must
// read the same cache, because each one that builds its own also owns the
// D-Bus name and installs the script a second time.
func TestShared_ReturnsOneBridge(t *testing.T) {
	first := Shared(zap.NewNop())
	second := Shared(nil)

	if first != second {
		t.Fatal("Shared() handed out two caches; KDE geometry must have one source")
	}
}
