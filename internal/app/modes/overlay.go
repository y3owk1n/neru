package modes

import (
	"context"
	"image"

	"github.com/y3owk1n/neru/internal/domain"
	domainGrid "github.com/y3owk1n/neru/internal/domain/grid"
	"github.com/y3owk1n/neru/internal/ports"
)

// overlaySurface is the part of ports.OverlayPort this package draws through:
// frames, the grid updates that run per keystroke, the hint search input, the
// keyboard grab, the flush and the active screen. Every method's contract is
// the port's — this declares which of them the handler may reach, not what
// they mean.
//
// It is declared at the consumer rather than taken from the port, which is
// already this repo's idiom (internal/app/keybinding/hotkey.go says so in as
// many words; monitor.go does it inline). It is deliberately not named *Port:
// that suffix belongs to internal/ports.
//
// What it leaves out is the point — but only the first group below is a
// lock-safety guarantee, and it is the compiler that makes it one. The other
// two leave because they are not this package's business, not because they are
// unreachable: the indicator draws still run under h.mu, through the services
// that own them, which the locking contract documents and allows.
//
//   - Config, theme and shutdown — ApplyConfig, RefreshStyles,
//     SetHiddenInScreenShare and Destroy. These are the four calls the locking
//     contract forbids the handler, and AGENTS.md in this directory says which
//     inversion of the lock order each one is. They belong to the app's own
//     config, theme and shutdown paths, which run unlocked; made from here
//     they hang the application rather than failing it, and nothing else would
//     catch one — the port mocks are counter no-ops that can neither block nor
//     fail, so the deadlock reproduces only against a real display server.
//     Until #1213 the resolver lived on App and the compiler enforced this;
//     since #1423 this type does, which is why the rule is no longer a
//     sentence anyone has to remember.
//   - The indicator primitives — the four Draw calls, ShowIndicator,
//     HideIndicator and ResizeIndicatorToActiveScreen. An indicator's whole
//     life belongs to the service that owns it (internal/app/services/
//     modeindicator, stickyindicator, virtualpointer), each holding its own
//     ports.OverlayPort; the handler drives those services and draws no
//     indicator itself.
//   - Lifecycle and liveness — Health, Refresh and IsVisible, all of which the
//     app answers from unlocked context and none of which a mode asks: the
//     services report their own health through the port they hold, the app's
//     screen-change path is what refreshes the overlay, and the hint service is
//     the one caller that asks whether anything is on screen.
//
// A method the package stops calling leaves; one it starts needing is copied
// down from the port. Copying down one of the first group is the change to
// stop and reconsider.
type overlaySurface interface {
	// Frames: the transition half of the port, plus the display its
	// screen-local content belongs to.
	ShowFrame(ctx context.Context, frame ports.Frame) error
	RedrawFrame(ctx context.Context, frame ports.Frame) error
	ClearFrame(ctx context.Context) error
	SetActiveScreen(screen image.Rectangle)

	// Hint search: the input drawn over a hints frame, and where it sits so
	// the platform's IME field can be placed over it.
	DrawHintSearch(search ports.HintSearch) error
	HideHintSearch()
	HintSearchBounds(screen image.Rectangle) image.Rectangle

	// Grid updates: the incremental half, which runs per keystroke and stays
	// off the frame path for the latency reason ADR 0003 gives.
	UpdateGridMatches(prefix string)
	SetGridHideUnmatched(hide bool)
	ShowGridSubgrid(cell *domainGrid.Cell)
	UpdateGridPointer(mode domain.Mode, pointer ports.GridPointer)

	// SetKeyboardCaptureEnabled is the Linux keyboard grab, gated on the event
	// tap reporting overlay keyboard passthrough (#1213).
	SetKeyboardCaptureEnabled(enabled bool)

	// Flush commits a tick's indicator draws in one go, so no intermediate
	// state shows. The handler owns the tick even though the services own the
	// draws, which is why this one method of the indicator path stays.
	Flush()
}

// A ports.OverlayPort satisfies this surface by construction. The assertion is
// what fails, at the declaration rather than at the composition root, if a
// method above drifts from the port's signature.
var _ overlaySurface = (ports.OverlayPort)(nil)
