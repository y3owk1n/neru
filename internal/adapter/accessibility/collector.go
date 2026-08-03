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

// lowerFilter lowercases the filter's text comparisons once, keeping the
// conversion out of the per-element matching that follows.
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

// elementCollector gathers elements from every source at once. The sources are
// independent, so they run in parallel and append to one list under one mutex.
type elementCollector struct {
	logger *zap.Logger

	wait sync.WaitGroup
	mu   sync.Mutex

	elements []*element.Element

	// firstError fails the collection: a source that cannot be queried leaves
	// the result untrustworthy, not merely incomplete.
	firstError error

	// windowsError is reported only when nothing at all was collected, so an
	// unreadable popover does not discard what the frontmost window yielded.
	windowsError error
}

// newElementCollector starts an empty collection, sized for a typical page.
func newElementCollector(logger *zap.Logger) *elementCollector {
	return &elementCollector{
		logger:   logger,
		elements: make([]*element.Element, 0, TypicalElementCount),
	}
}

// start runs one source. The name is what a failure reports.
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

// await blocks until every source finishes, or the context ends first.
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
		// Timeout and cancellation read differently here: a timeout means the
		// backend went slow or unresponsive, which is the actionable one.
		return derrors.WrapContextCanceled(ctx, "element collection")
	}
}

// windowScan reads the frontmost window and any popovers, several at a time.
type windowScan struct {
	wait sync.WaitGroup
	mu   sync.Mutex

	// sem caps concurrent reads so a long window list cannot spawn unbounded
	// goroutines against the accessibility API.
	sem chan struct{}

	elements []*element.Element

	// firstError is reported only when no window yielded anything: one bad
	// popover must not discard the frontmost window's elements, but an
	// unreachable AT-SPI bus must not look like an empty window either.
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

// collectWindowElements reads every window that should be scanned. A failure
// goes to the collector rather than being returned, since returning it would
// fail the collection and discard what the other sources found.
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

// windowsToScan is the frontmost window plus any popovers over it, falling back
// to the frontmost alone when the combined query comes back empty.
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

// scanWindow reads one window's clickable elements, releasing the handle.
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

	// duringMissionControl marks a surface still reachable while Mission
	// Control is up. Only the Dock is; the rest are covered by it.
	duringMissionControl bool

	collect func() []*element.Element
}

// supplementarySources lists the surfaces outside the frontmost window. They
// are macOS-only and resolved by bundle ID, which is why the caller asks the
// client whether to consider them rather than probing for absent apps.
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
