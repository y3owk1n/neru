package app_test

import (
	"context"
	"image"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/y3owk1n/neru/internal/adapter/ipc"
	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/event"
)

// snap sends `neru action snap` with args to the app.
func (h *simHarness) snap(args ...string) ipc.Response {
	return h.action(append([]string{"snap"}, args...)...)
}

// leftClick sends `neru action left_click` to the app.
func (h *simHarness) leftClick() ipc.Response {
	return h.action("left_click")
}

// action sends `neru action` with args to the app.
func (h *simHarness) action(args ...string) ipc.Response {
	return h.app.HandleCommand(context.Background(), ipc.Command{Action: "action", Args: args})
}

// zoomRecursiveGrid opens recursive grid, zooms one level into the top-left
// cell, and returns the region the zoom reached.
func (h *simHarness) zoomRecursiveGrid() image.Rectangle {
	h.t.Helper()

	h.pressHotkey(recursiveGridHotkey)
	h.waitMode(domain.ModeRecursiveGrid)
	h.waitFor("recursive grid drawn", func() bool {
		_, ok := h.overlay.lastRecursiveGridBounds()

		return ok
	})

	h.press("r")

	var region image.Rectangle

	h.waitFor("recursive grid zoomed in", func() bool {
		var ok bool

		region, ok = h.overlay.lastRecursiveGridBounds()

		return ok && region != simScreen
	})

	return region
}

func rectCenter(rect image.Rectangle) image.Point {
	return image.Point{X: rect.Min.X + rect.Dx()/2, Y: rect.Min.Y + rect.Dy()/2}
}

// TestSimulation_SnapMovesTheCursorOntoTheNearestElement covers a snap from
// idle. It lands on the element nearest the cursor and enters no mode.
func TestSimulation_SnapMovesTheCursorOntoTheNearestElement(t *testing.T) {
	near := image.Rect(100, 100, 140, 140)
	sim := newSimHarness(t, simConfig(), []*element.Element{
		simElement(t, "near", near, "Near"),
		simElement(t, "far", image.Rect(1500, 800, 1540, 840), "Far"),
	})
	sim.cursor.moveTo(image.Point{X: 90, Y: 90})

	resp := sim.snap()
	if !resp.Success {
		t.Fatalf("snap failed: %s (%s)", resp.Message, resp.Code)
	}

	if got, want := sim.cursor.position(), rectCenter(near); got != want {
		t.Errorf("cursor = %v after the snap, want %v, the nearest element's center", got, want)
	}

	if mode := sim.app.CurrentMode(); mode != domain.ModeIdle {
		t.Errorf("mode = %v after the snap, want idle", mode)
	}
}

// TestSimulation_SnapInsideAZoomedRecursiveGridStaysInTheRegion covers a snap
// once recursive grid has zoomed in. It ignores an element outside the region
// even when that one is nearer, moves the cursor, keeps the mode open, and the
// next click lands where the cursor went.
func TestSimulation_SnapInsideAZoomedRecursiveGridStaysInTheRegion(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	region := sim.zoomRecursiveGrid()
	if region.Max.Y >= simScreen.Max.Y {
		t.Fatalf(
			"zoomed region %v touches the bottom of the screen, the fixture needs room below",
			region,
		)
	}

	center := rectCenter(region)
	// In the region's corner, far from its center.
	inside := image.Rect(region.Min.X, region.Min.Y, region.Min.X+40, region.Min.Y+40)
	// Just below the region, nearer its center than the corner is.
	outside := image.Rect(center.X-20, region.Max.Y, center.X+20, region.Max.Y+40)
	sim.ax.setElements([]*element.Element{
		simElement(t, "inside", inside, "Inside"),
		simElement(t, "outside", outside, "Outside"),
	})

	resp := sim.snap()
	if !resp.Success {
		t.Fatalf("snap failed: %s (%s)", resp.Message, resp.Code)
	}

	want := rectCenter(inside)
	if got := sim.cursor.position(); got != want {
		t.Fatalf("cursor = %v after the snap, want %v, the element inside the region", got, want)
	}

	if mode := sim.app.CurrentMode(); mode != domain.ModeRecursiveGrid {
		t.Fatalf("mode = %v after the snap, want recursive grid still open", mode)
	}

	click := sim.leftClick()
	if !click.Success {
		t.Fatalf("left_click failed: %s (%s)", click.Message, click.Code)
	}

	clicks := sim.ax.recordedClicks()
	if len(clicks) != 1 || clicks[0].action != action.TypeLeftClick || clicks[0].point != want {
		t.Errorf(
			"clicks = %+v, want one left click at %v, where the snap put the cursor",
			clicks,
			want,
		)
	}
}

// TestSimulation_SnapWithNothingToSnapToLeavesTheGridClickWorking covers a
// snap that finds nothing inside a zoomed recursive grid. It fails without
// moving the cursor, the click after it in a binding still runs and lands on
// the grid's selection, and --bail-on-error skips the click instead.
func TestSimulation_SnapWithNothingToSnapToLeavesTheGridClickWorking(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)
	region := sim.zoomRecursiveGrid()
	moves := sim.cursor.moveCount()

	resp := sim.snap()
	if resp.Success || resp.Code != ipc.CodeActionFailed {
		t.Fatalf("snap = %+v, want %s", resp, ipc.CodeActionFailed)
	}

	if got := sim.cursor.moveCount(); got != moves {
		t.Errorf("cursor moved %d times, want none", got-moves)
	}

	sim.app.HandleCommand(context.Background(), ipc.Command{
		Action: domain.CommandRun,
		Args:   []string{"action snap --bail-on-error", "action left_click"},
	})

	if clicks := sim.ax.recordedClicks(); len(clicks) != 0 {
		t.Fatalf("clicks = %+v, want none after a snap with --bail-on-error found nothing", clicks)
	}

	sim.app.HandleCommand(context.Background(), ipc.Command{
		Action: domain.CommandRun,
		Args:   []string{"action snap", "action left_click"},
	})

	clicks := sim.ax.recordedClicks()
	if len(clicks) != 1 || clicks[0].point != rectCenter(region) {
		t.Errorf(
			"clicks = %+v, want one click at %v, the grid's selection",
			clicks,
			rectCenter(region),
		)
	}
}

// TestSimulation_SnapIsDroppedWhenTheSelectionMovesDuringTheScan covers a key
// typed while a snap scans. The mode takes the key, and the snap that started
// before it moves nothing.
func TestSimulation_SnapIsDroppedWhenTheSelectionMovesDuringTheScan(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	region := sim.zoomRecursiveGrid()
	inside := image.Rect(region.Min.X, region.Min.Y, region.Min.X+40, region.Min.Y+40)
	sim.ax.setElements([]*element.Element{simElement(t, "inside", inside, "Inside")})

	scanning := make(chan struct{})
	release := make(chan struct{})

	sim.ax.setScanGate(func() {
		close(scanning)
		<-release
	})

	answer := make(chan ipc.Response, 1)

	go func() { answer <- sim.snap() }()

	<-scanning

	drawsBefore := sim.overlay.recursiveGridDrawCount()
	sim.press("r")
	sim.waitFor("the key zoomed in while the snap scanned", func() bool {
		return sim.overlay.recursiveGridDrawCount() > drawsBefore
	})

	close(release)

	resp := <-answer
	if resp.Success || resp.Code != ipc.CodeChainBail {
		t.Fatalf("snap = %+v, want a chain bail once the selection moved", resp)
	}

	if got := sim.cursor.position(); got == rectCenter(inside) {
		t.Errorf("cursor = %v, the stale snap moved it anyway", got)
	}
}

// TestSimulation_SnapInsideAGridSubgridStaysInTheCell covers grid. Once a
// subgrid is open, a snap searches the cell it sits in and ignores a nearer
// element outside it.
func TestSimulation_SnapInsideAGridSubgridStaysInTheCell(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	cells := sim.overlay.lastGrid().Cells()
	cell := cells[len(cells)/2]

	sim.typeLabel(cell.Coordinate())
	sim.waitFor("subgrid open", func() bool { return sim.overlay.subgridCount() == 1 })

	bounds := cell.Bounds()
	center := rectCenter(bounds)
	// In the cell's corner, and just past its right edge, nearer its center.
	inside := image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+4, bounds.Min.Y+4)
	outside := image.Rect(bounds.Max.X, center.Y-2, bounds.Max.X+4, center.Y+2)
	sim.ax.setElements([]*element.Element{
		simElement(t, "inside", inside, "Inside"),
		simElement(t, "outside", outside, "Outside"),
	})

	resp := sim.snap()
	if !resp.Success {
		t.Fatalf("snap failed: %s (%s)", resp.Message, resp.Code)
	}

	if got, want := sim.cursor.position(), rectCenter(inside); got != want {
		t.Errorf("cursor = %v after the snap, want %v, the element inside the cell", got, want)
	}

	if mode := sim.app.CurrentMode(); mode != domain.ModeGrid {
		t.Errorf("mode = %v after the snap, want grid still open", mode)
	}
}

// TestSimulation_SnapIsDroppedWhenTheSubgridClosesDuringTheScan covers a
// backspace out of a grid subgrid while a snap scans. It clears the selection
// without writing a new one, and the snap that started inside the cell still
// moves nothing.
func TestSimulation_SnapIsDroppedWhenTheSubgridClosesDuringTheScan(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)

	sim.pressHotkey(gridHotkey)
	sim.waitMode(domain.ModeGrid)
	sim.waitFor("grid drawn", func() bool { return sim.overlay.lastGrid() != nil })

	cells := sim.overlay.lastGrid().Cells()
	cell := cells[len(cells)/2]

	sim.typeLabel(cell.Coordinate())
	sim.waitFor("subgrid open", func() bool { return sim.overlay.subgridCount() == 1 })

	bounds := cell.Bounds()
	inside := image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+4, bounds.Min.Y+4)
	sim.ax.setElements([]*element.Element{simElement(t, "inside", inside, "Inside")})

	scanning := make(chan struct{})
	release := make(chan struct{})

	sim.ax.setScanGate(func() {
		close(scanning)
		<-release
	})

	answer := make(chan ipc.Response, 1)

	go func() { answer <- sim.snap() }()

	<-scanning

	drawsBefore := sim.overlay.gridDrawCount()
	sim.press("Backspace")
	sim.waitFor("backspace left the subgrid while the snap scanned", func() bool {
		return sim.overlay.gridDrawCount() > drawsBefore
	})

	close(release)

	resp := <-answer
	if resp.Success || resp.Code != ipc.CodeChainBail {
		t.Fatalf("snap = %+v, want a chain bail once the subgrid closed", resp)
	}

	if got := sim.cursor.position(); got == rectCenter(inside) {
		t.Errorf("cursor = %v, the stale snap moved it anyway", got)
	}
}

// TestSimulation_SnapWithADirectionWalksFromElementToElement covers stepping.
// Each press moves on to the next element that way, past the toolbar the
// buttons sit in, and the end of the row fails and leaves the cursor on the
// last button.
func TestSimulation_SnapWithADirectionWalksFromElementToElement(t *testing.T) {
	first := image.Rect(10, 10, 50, 50)
	second := image.Rect(60, 10, 100, 50)
	third := image.Rect(400, 10, 440, 50)
	sim := newSimHarness(t, simConfig(), []*element.Element{
		simElement(t, "toolbar", image.Rect(0, 0, 1000, 60), "Toolbar"),
		simElement(t, "first", first, "First"),
		simElement(t, "second", second, "Second"),
		simElement(t, "third", third, "Third"),
	})
	sim.cursor.moveTo(rectCenter(first))

	for _, want := range []image.Rectangle{second, third} {
		resp := sim.snap("--direction=right")
		if !resp.Success {
			t.Fatalf("snap right failed: %s (%s)", resp.Message, resp.Code)
		}

		if got := sim.cursor.position(); got != rectCenter(want) {
			t.Fatalf("cursor = %v after snapping right, want %v", got, rectCenter(want))
		}
	}

	resp := sim.snap("--direction=right")
	if resp.Success || resp.Code != ipc.CodeActionFailed {
		t.Fatalf("snap right at the end of the row = %+v, want %s", resp, ipc.CodeActionFailed)
	}

	if got := sim.cursor.position(); got != rectCenter(third) {
		t.Errorf("cursor = %v after the end of the row, want it left on %v", got, rectCenter(third))
	}

	back := sim.snap("--direction=left")
	if !back.Success || sim.cursor.position() != rectCenter(second) {
		t.Errorf("snap left = %+v with the cursor at %v, want it back on %v",
			back, sim.cursor.position(), rectCenter(second))
	}
}

// TestSimulation_SnapReportsWhereItLanded covers what a snap tells a script
// and a watcher: the landing point, role and bounds in the reply, and a select
// event naming snap.
func TestSimulation_SnapReportsWhereItLanded(t *testing.T) {
	bounds := image.Rect(100, 100, 140, 140)
	button := simElement(t, "button", bounds, "Button")
	sim := newSimHarness(t, simConfig(), []*element.Element{button})
	sim.cursor.moveTo(image.Point{X: 90, Y: 90})

	events := subscribeEvents(t, sim)

	resp := sim.snap()
	if !resp.Success {
		t.Fatalf("snap failed: %s (%s)", resp.Message, resp.Code)
	}

	wantRole := string(button.Role())
	if vocab, known := element.CurrentVocabulary(); known {
		wantRole = string(vocab) + ":" + wantRole
	}

	want := ipc.SnapData{
		X: 120, Y: 120, Role: wantRole, Source: string(button.Source()),
		Bounds: ipc.SnapBounds{X: 100, Y: 100, Width: 40, Height: 40},
	}
	if got, ok := resp.Data.(ipc.SnapData); !ok || got != want {
		t.Errorf("data = %#v, want %#v", resp.Data, want)
	}

	events.expect(event.Event{
		Name:   event.Select,
		Mode:   domain.ModeString(domain.ModeIdle),
		Action: "snap",
		Point:  image.Point{X: 120, Y: 120},
	})
}

// TestSimulation_SnapNarrowsTheScanAsHintsDoes covers --role and --text. The
// scan a snap asks for is the one `neru query hints` asks for with the same
// filters.
func TestSimulation_SnapNarrowsTheScanAsHintsDoes(t *testing.T) {
	sim := newSimHarness(t, simConfig(), []*element.Element{
		simElement(t, "save", image.Rect(100, 100, 140, 140), "Save"),
	})

	query := sim.app.HandleCommand(context.Background(), ipc.Command{
		Action: domain.CommandQueryHints,
		Args:   []string{"--role=button", "--text=Save"},
	})
	if !query.Success {
		t.Fatalf("query hints failed: %s (%s)", query.Message, query.Code)
	}

	resp := sim.snap("--role=button", "--text=Save")
	if !resp.Success {
		t.Fatalf("snap failed: %s (%s)", resp.Message, resp.Code)
	}

	filters := sim.ax.scanFilters()
	if len(filters) != 2 {
		t.Fatalf("got %d scans, want the query's and the snap's", len(filters))
	}

	queried, snapped := filters[0], filters[1]
	if len(snapped.Roles) == 0 || !slices.Equal(snapped.Roles, queried.Roles) {
		t.Errorf("snap scanned roles %v, want the query's %v", snapped.Roles, queried.Roles)
	}

	if snapped.TitleContains != "Save" || snapped.TitleContains != queried.TitleContains {
		t.Errorf("snap filtered text %q, want the query's %q",
			snapped.TitleContains, queried.TitleContains)
	}
}

// TestSimulation_SnapRefusesWhatItCannotUse covers the flags snap refuses
// before it scans anything.
func TestSimulation_SnapRefusesWhatItCannotUse(t *testing.T) {
	sim := newSimHarness(t, simConfig(), []*element.Element{
		simElement(t, "button", image.Rect(100, 100, 140, 140), "Button"),
	})

	for _, args := range [][]string{
		{"--direction=diagonal"},
		{"--strategy=axtree,vision"},
		{"--count=2"},
	} {
		resp := sim.snap(args...)
		if resp.Success || resp.Code != ipc.CodeInvalidInput {
			t.Errorf("snap %v = %+v, want %s", args, resp, ipc.CodeInvalidInput)
		}
	}

	if scans := len(sim.ax.scanFilters()); scans != 0 {
		t.Errorf("refused snaps scanned %d times, want none", scans)
	}
}

// TestSimulation_OverlappingSnapsEachTakeAStep covers a held directed snap,
// whose first press and first repeat can be in flight together. The second
// starts from where the first landed, so two presses are two steps.
func TestSimulation_OverlappingSnapsEachTakeAStep(t *testing.T) {
	first := image.Rect(10, 10, 50, 50)
	third := image.Rect(110, 10, 150, 50)
	sim := newSimHarness(t, simConfig(), []*element.Element{
		simElement(t, "first", first, "First"),
		simElement(t, "second", image.Rect(60, 10, 100, 50), "Second"),
		simElement(t, "third", third, "Third"),
	})
	sim.cursor.moveTo(rectCenter(first))

	scanning := make(chan struct{})
	release := make(chan struct{})

	var once sync.Once

	// Only the first scan is held open. Any later one passes straight through.
	sim.ax.setScanGate(func() {
		held := false

		once.Do(func() {
			held = true

			close(scanning)
		})

		if held {
			<-release
		}
	})

	answers := make(chan ipc.Response, 2)

	go func() { answers <- sim.snap("--direction=right") }()

	<-scanning

	go func() { answers <- sim.snap("--direction=right") }()

	// Room for a second snap that did not wait to reach its scan, start from
	// the same cursor and land first. The assertion below holds however long
	// a waiting one takes.
	time.Sleep(20 * time.Millisecond)
	close(release)

	for range 2 {
		if resp := <-answers; !resp.Success {
			t.Fatalf("snap right failed: %s (%s)", resp.Message, resp.Code)
		}
	}

	if got := sim.cursor.position(); got != rectCenter(third) {
		t.Errorf("cursor = %v after two overlapping steps right, want %v, two elements on",
			got, rectCenter(third))
	}
}

// TestSimulation_SnapWithAPixelStrategyReadsPastTheZoomedRegion covers a
// capture strategy inside a zoomed recursive grid. The capture covers the
// whole window, as hints would read it, because a detector sees a control by
// its whole outline and a capture cropped to the cell would cut through it. A
// control larger than the cell is still found, and the cursor lands on its
// part inside the cell.
func TestSimulation_SnapWithAPixelStrategyReadsPastTheZoomedRegion(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)
	region := sim.zoomRecursiveGrid()

	var (
		scannedMu sync.Mutex
		scanned   []image.Rectangle
	)

	sim.vision.ExcludesOwnWindows = true
	sim.vision.DetectSegmentsFunc = func(
		_ context.Context,
		bounds image.Rectangle,
		_ config.HintsSegmentConfig,
	) ([]*element.Element, error) {
		scannedMu.Lock()

		scanned = append(scanned, bounds)
		scannedMu.Unlock()

		// A button reaching past the cell on every side.
		button, err := element.NewElement("wide", region.Inset(-20), element.RoleButton)
		if err != nil {
			return nil, err
		}

		return []*element.Element{button}, nil
	}

	resp := sim.snap("--strategy=segment")
	if !resp.Success {
		t.Fatalf("snap failed: %s (%s)", resp.Message, resp.Code)
	}

	scannedMu.Lock()
	defer scannedMu.Unlock()

	if len(scanned) != 1 || scanned[0] != simScreen {
		t.Errorf("segment scanned %v, want one scan of the whole window %v, not the cell %v",
			scanned, simScreen, region)
	}

	if got, want := sim.cursor.position(), rectCenter(region); got != want {
		t.Errorf("cursor = %v, want %v, the wide button's part inside the cell", got, want)
	}
}

// TestSimulation_AClickAfterASnapAimsAtTheElement covers a click right after a
// snap inside a grid mode, on a platform that has not caught up with the move
// yet and still reports the cursor where it was. The click aims at the point
// the snap chose, as one after move_mouse does, rather than at the stale
// cursor read.
func TestSimulation_AClickAfterASnapAimsAtTheElement(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)
	region := sim.zoomRecursiveGrid()

	inside := image.Rect(region.Min.X, region.Min.Y, region.Min.X+40, region.Min.Y+40)
	sim.ax.setElements([]*element.Element{simElement(t, "inside", inside, "Inside")})

	before := sim.cursor.position()

	resp := sim.snap()
	if !resp.Success {
		t.Fatalf("snap failed: %s (%s)", resp.Message, resp.Code)
	}

	// The platform still answers with the position before the snap.
	sim.cursor.moveTo(before)

	click := sim.leftClick()
	if !click.Success {
		t.Fatalf("left_click failed: %s (%s)", click.Message, click.Code)
	}

	want := rectCenter(inside)

	clicks := sim.ax.recordedClicks()
	if len(clicks) != 1 || clicks[0].point != want {
		t.Errorf("clicks = %+v, want one click at %v, the snapped element, not the stale %v",
			clicks, want, before)
	}
}

// TestSimulation_SnapInsideAZoomedCellSkipsTheRowCoveringIt covers a zoomed
// cell that a clickable row covers whole. The row, clipped to the cell, would
// land on the cell's center, where the selection already is, so the snap goes
// to the button inside the row instead and the cursor moves.
func TestSimulation_SnapInsideAZoomedCellSkipsTheRowCoveringIt(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)
	region := sim.zoomRecursiveGrid()

	button := image.Rect(region.Min.X+10, region.Min.Y+10, region.Min.X+50, region.Min.Y+40)
	sim.ax.setElements([]*element.Element{
		simElement(t, "row", region.Inset(-30), "Row"),
		simElement(t, "button", button, "Button"),
	})

	resp := sim.snap()
	if !resp.Success {
		t.Fatalf("snap failed: %s (%s)", resp.Message, resp.Code)
	}

	if got, want := sim.cursor.position(), rectCenter(button); got != want {
		t.Errorf("cursor = %v after the snap, want %v, the button rather than the cell's center %v",
			got, want, rectCenter(region))
	}
}

// holdScan holds the next scan open until the returned release is called, and
// returns a channel that closes once the scan has started.
func (h *simHarness) holdScan() (<-chan struct{}, func()) {
	scanning := make(chan struct{})
	release := make(chan struct{})

	var once sync.Once

	h.ax.setScanGate(func() {
		held := false

		once.Do(func() {
			held = true

			close(scanning)
		})

		if held {
			<-release
		}
	})

	return scanning, func() { close(release) }
}

// TestSimulation_SnapThatFindsNothingIsDroppedWhenTheModeExitsDuringTheScan
// covers Escape pressed while a snap that will find nothing scans. The snap
// answers with a chain bail rather than a plain failure, so the click after
// it in a binding does not run on a mode the user left.
func TestSimulation_SnapThatFindsNothingIsDroppedWhenTheModeExitsDuringTheScan(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)
	sim.zoomRecursiveGrid()

	scanning, release := sim.holdScan()
	answer := make(chan ipc.Response, 1)

	go func() { answer <- sim.snap() }()

	<-scanning

	sim.press("Escape")
	sim.waitMode(domain.ModeIdle)
	release()

	if resp := <-answer; resp.Success || resp.Code != ipc.CodeChainBail {
		t.Fatalf("snap = %+v, want a chain bail once the mode exited", resp)
	}
}

// TestSimulation_SnapIsDroppedWhenTheSelectionIsClearedDuringTheScan covers a
// selection cleared while a snap scans, as a relative cursor move or the start
// of a held glide clears it. A clear writes no selection, and the snap still
// moves nothing, so it cannot put back a selection the user let go of.
func TestSimulation_SnapIsDroppedWhenTheSelectionIsClearedDuringTheScan(t *testing.T) {
	sim := newSimHarness(t, simConfig(), nil)
	region := sim.zoomRecursiveGrid()

	inside := image.Rect(region.Min.X, region.Min.Y, region.Min.X+40, region.Min.Y+40)
	sim.ax.setElements([]*element.Element{simElement(t, "inside", inside, "Inside")})

	scanning, release := sim.holdScan()
	answer := make(chan ipc.Response, 1)

	go func() { answer <- sim.snap() }()

	<-scanning

	if moved := sim.action("move_mouse_relative", "--dx=5", "--dy=0"); !moved.Success {
		t.Fatalf("move_mouse_relative failed: %s (%s)", moved.Message, moved.Code)
	}

	release()

	if resp := <-answer; resp.Success || resp.Code != ipc.CodeChainBail {
		t.Fatalf("snap = %+v, want a chain bail once the selection was cleared", resp)
	}

	if got := sim.cursor.position(); got == rectCenter(inside) {
		t.Errorf("cursor = %v, the stale snap moved it anyway", got)
	}
}

// TestSimulation_SnapInABottomedOutRecursiveGridSearchesTheFinalCell covers a
// recursive grid that can divide no further. The last pick selects a cell
// without zooming into it, and the mode stays open while the cursor holds
// still. A snap then searches that cell, not the region around it.
func TestSimulation_SnapInABottomedOutRecursiveGridSearchesTheFinalCell(t *testing.T) {
	cfg := simConfig()
	cfg.RecursiveGrid.MaxDepth = 1

	sim := newSimHarness(t, cfg, nil)

	resp := sim.app.HandleCommand(context.Background(), ipc.Command{
		Action: domain.ModeNameRecursiveGrid,
		Args:   []string{"--cursor-selection-mode=hold"},
	})
	if !resp.Success {
		t.Fatalf("recursive_grid failed: %s (%s)", resp.Message, resp.Code)
	}

	sim.waitMode(domain.ModeRecursiveGrid)
	sim.press("r")

	var region image.Rectangle

	sim.waitFor("recursive grid zoomed in", func() bool {
		var ok bool

		region, ok = sim.overlay.lastRecursiveGridBounds()

		return ok && region != simScreen
	})

	sim.press("r")

	// The top-left cell of the 3x3 split of the region.
	final := image.Rect(region.Min.X, region.Min.Y,
		region.Min.X+region.Dx()/3, region.Min.Y+region.Dy()/3)

	sim.waitFor("the final cell selected", func() bool {
		return sim.app.CurrentMode() == domain.ModeRecursiveGrid
	})

	finalCenter := rectCenter(final)
	// In the final cell's corner, and just past its right edge, nearer its
	// center. A few pixels clear of the edge, since a third of the region
	// rounds either way.
	inside := image.Rect(final.Min.X, final.Min.Y, final.Min.X+4, final.Min.Y+4)
	outside := image.Rect(final.Max.X+3, finalCenter.Y-2, final.Max.X+7, finalCenter.Y+2)
	sim.ax.setElements([]*element.Element{
		simElement(t, "inside", inside, "Inside"),
		simElement(t, "outside", outside, "Outside"),
	})

	snapped := sim.snap()
	if !snapped.Success {
		t.Fatalf("snap failed: %s (%s)", snapped.Message, snapped.Code)
	}

	if got, want := sim.cursor.position(), rectCenter(inside); got != want {
		t.Errorf("cursor = %v after the snap, want %v, the element inside the final cell %v",
			got, want, final)
	}
}
