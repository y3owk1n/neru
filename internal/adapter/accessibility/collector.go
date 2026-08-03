package accessibility

import (
	"context"
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/adapter/accessibility/ax"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/ports"
)

// lowerFilter lowercases the filter's text comparisons once.
//
// The matching that follows runs per element, and on a busy web page that is
// thousands of comparisons, so converting here rather than at each of them keeps
// the conversion off the hot path.
func lowerFilter(filter ports.ElementFilter) ports.ElementFilter {
	filter.TitleContains = strings.ToLower(filter.TitleContains)
	filter.DescriptionContains = strings.ToLower(filter.DescriptionContains)
	filter.ValueContains = strings.ToLower(filter.ValueContains)

	if len(filter.TextContainsList) > 0 {
		lowered := make([]string, len(filter.TextContainsList))
		for index, text := range filter.TextContainsList {
			lowered[index] = strings.ToLower(text)
		}

		filter.TextContainsList = lowered
	}

	return filter
}

// elementCollector gathers elements from every source at once.
//
// The sources are independent — the frontmost window, the Dock, the menu bar —
// so they all run in parallel and append to one list under a single mutex.
type elementCollector struct {
	logger *zap.Logger

	wait sync.WaitGroup
	mu   sync.Mutex

	elements []*element.Element

	// firstError fails the whole collection. A source that cannot be queried at
	// all leaves the result untrustworthy rather than merely incomplete.
	firstError error

	// windowsError is held back rather than failing the collection, and is
	// surfaced only when no source produced anything. A popover that could not
	// be read must not discard the hints the frontmost window did yield.
	windowsError error
}

// newElementCollector starts an empty collection, sized for a typical page so
// the common case does not grow the slice.
func newElementCollector(logger *zap.Logger) *elementCollector {
	return &elementCollector{
		logger:   logger,
		elements: make([]*element.Element, 0, TypicalElementCount),
	}
}

// start runs one source. Sources are named so that a failure says which surface
// could not be read.
func (c *elementCollector) start(
	ctx context.Context,
	name string,
	query func() ([]*element.Element, error),
) {
	c.wait.Go(func() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		elements, queryErr := query()
		if queryErr != nil {
			c.recordFailure(name, queryErr)

			return
		}

		c.add(elements)

		c.logger.Debug("Collected elements from "+name, zap.Int("count", len(elements)))
	})
}

// add appends a source's elements to the shared result.
func (c *elementCollector) add(elements []*element.Element) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.elements = append(c.elements, elements...)
}

// recordFailure keeps the first source failure, which fails the collection.
func (c *elementCollector) recordFailure(name string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.firstError == nil {
		c.firstError = derrors.Wrap(
			err,
			derrors.CodeAccessibilityFailed,
			"failed to get elements from "+name,
		)
	}
}

// recordWindowsFailure keeps a window-scan failure without failing the
// collection. See windowsError.
func (c *elementCollector) recordWindowsFailure(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.windowsError == nil {
		c.windowsError = err
	}
}

// await blocks until every source has finished, or until the context ends
// first — which usually means the accessibility backend stopped responding.
func (c *elementCollector) await(ctx context.Context) error {
	done := make(chan struct{})

	go func() {
		c.wait.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		// Distinguish a real timeout from a cancellation instead of collapsing
		// both into one message: a timeout here is the actionable signal that
		// the backend was slow or unresponsive.
		return derrors.WrapContextCanceled(ctx, "element collection")
	}
}

// windowScan collects elements from the frontmost window and any popovers over
// it, several at a time.
type windowScan struct {
	wait sync.WaitGroup
	mu   sync.Mutex

	// sem caps how many windows are read at once, so a long window list cannot
	// spawn an unbounded number of goroutines against the accessibility API.
	sem chan struct{}

	elements []*element.Element

	// firstError is reported only when no window yielded anything. A transient
	// failure on one popover must not discard what the frontmost window
	// produced, but a hard failure — an unreachable AT-SPI bus, say — that
	// leaves nothing behind has to be reported rather than look like an empty
	// window.
	firstError error
}

// add appends one window's elements.
func (s *windowScan) add(elements []*element.Element) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.elements = append(s.elements, elements...)
}

// recordFailure keeps the first window failure.
func (s *windowScan) recordFailure(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.firstError == nil {
		s.firstError = err
	}
}

// collectWindowElements reads every window that should be scanned.
//
// A window failure is handed to the collector instead of being returned,
// because returning it would fail the whole collection and throw away what the
// other sources gathered in parallel.
func (a *Adapter) collectWindowElements(
	ctx context.Context,
	filter ports.ElementFilter,
	collector *elementCollector,
) ([]*element.Element, error) {
	windows, windowsErr := a.windowsToScan(ctx)
	if windowsErr != nil {
		return nil, windowsErr
	}

	scan := &windowScan{sem: make(chan struct{}, maxConcurrentWindows)}

	for _, window := range windows {
		scan.sem <- struct{}{}

		scan.wait.Add(1)

		go a.scanWindow(ctx, filter, scan, window)
	}

	scan.wait.Wait()

	if scan.firstError != nil {
		collector.recordWindowsFailure(scan.firstError)
	}

	return scan.elements, nil
}

// windowsToScan is the frontmost window together with any popovers over it,
// falling back to the frontmost window alone when the combined query comes back
// empty.
func (a *Adapter) windowsToScan(ctx context.Context) ([]ax.Window, error) {
	windows, windowsErr := a.client.FrontmostAndPopoverWindows(ctx)
	if windowsErr != nil {
		return nil, windowsErr
	}

	if len(windows) > 0 {
		return windows, nil
	}

	frontmost, frontmostErr := a.client.FrontmostWindow(ctx)
	if frontmostErr != nil {
		return nil, frontmostErr
	}

	return []ax.Window{frontmost}, nil
}

// scanWindow reads one window's clickable elements. It owns the window handle
// and always releases it.
func (a *Adapter) scanWindow(
	ctx context.Context,
	filter ports.ElementFilter,
	scan *windowScan,
	window ax.Window,
) {
	defer scan.wait.Done()
	defer func() { <-scan.sem }()
	defer window.Release()

	nodes, nodesErr := a.client.ClickableNodes(ctx, window, stringRoles(filter.Roles), 0)
	if nodesErr != nil {
		a.logger.Warn("Failed to collect clickable nodes from window", zap.Error(nodesErr))
		scan.recordFailure(nodesErr)

		return
	}

	elements, processErr := a.processClickableNodes(ctx, nodes, filter)
	if processErr != nil {
		a.logger.Warn("Failed to process clickable nodes from window", zap.Error(processErr))
		scan.recordFailure(processErr)

		return
	}

	scan.add(elements)
}

// supplementarySource is one of the surfaces outside the frontmost window.
type supplementarySource struct {
	name string

	// enabled is the filter flag that asks for this surface.
	enabled bool

	// duringMissionControl is whether the surface is still reachable while
	// Mission Control is up. Only the Dock is; the rest are covered by it.
	duringMissionControl bool

	collect func() []*element.Element
}

// supplementarySources lists the surfaces outside the frontmost window.
//
// These are macOS-specific and are resolved by system bundle ID. They do not
// exist elsewhere, which is why the caller asks the client whether to consider
// them at all rather than probing for apps that cannot be there.
func (a *Adapter) supplementarySources(
	ctx context.Context,
	filter ports.ElementFilter,
) []supplementarySource {
	return []supplementarySource{
		{
			name:    "menubar",
			enabled: filter.IncludeMenubar,
			collect: func() []*element.Element { return a.addMenubarElements(ctx, nil, filter) },
		},
		{
			name:                 "dock",
			enabled:              filter.IncludeDock,
			duringMissionControl: true,
			collect:              func() []*element.Element { return a.addDockElements(ctx, nil) },
		},
		{
			name:    "notification_center",
			enabled: filter.IncludeNotificationCenter,
			collect: func() []*element.Element { return a.addNotificationCenterElements(ctx, nil) },
		},
		{
			name:    "stage_manager",
			enabled: filter.IncludeStageManager,
			collect: func() []*element.Element { return a.addStageManagerElements(ctx, nil) },
		},
		{
			name:    "pip",
			enabled: filter.IncludePIP,
			collect: func() []*element.Element { return a.addPIPElements(ctx, nil) },
		},
		{
			name:    "screen_capture",
			enabled: filter.IncludeScreenCapture,
			collect: func() []*element.Element { return a.addScreenCaptureElements(ctx, nil) },
		},
	}
}
