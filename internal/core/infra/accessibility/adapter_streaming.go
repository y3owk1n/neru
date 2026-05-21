package accessibility

import (
	"context"
	"strings"
	"sync"

	"go.uber.org/zap"

	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// streamSink is the callback signature used by internal streaming helpers to
// deliver a single element or error to the output channel.
type streamSink func(result ports.ElementStreamResult)

// StreamElements returns a channel that delivers elements as they are
// discovered by the accessibility traversal. Elements are streamed at the
// per-node granularity from the frontmost window; supplementary sources
// (menubar, dock, etc.) are batched. The channel is closed when the
// traversal is complete.
func (a *Adapter) StreamElements(
	ctx context.Context,
	filter ports.ElementFilter,
) (<-chan ports.ElementStreamResult, error) {
	err := a.checkContext(ctx)
	if err != nil {
		return nil, err
	}

	// Pre-lowercase filter strings once to avoid repeating strings.ToLower on hot paths
	if filter.TitleContains != "" {
		filter.TitleContains = strings.ToLower(filter.TitleContains)
	}

	if filter.DescriptionContains != "" {
		filter.DescriptionContains = strings.ToLower(filter.DescriptionContains)
	}

	if filter.ValueContains != "" {
		filter.ValueContains = strings.ToLower(filter.ValueContains)
	}

	if len(filter.TextContainsList) > 0 {
		loweredList := make([]string, len(filter.TextContainsList))
		for i, text := range filter.TextContainsList {
			loweredList[i] = strings.ToLower(text)
		}

		filter.TextContainsList = loweredList
	}

	a.logger.Debug("Streaming elements", zap.Any("filter", filter))

	resultCh := make(
		chan ports.ElementStreamResult,
		100, //nolint:mnd
	)

	go a.streamElementsInternal(ctx, filter, resultCh)

	return resultCh, nil
}

// streamElementsInternal runs the element collection in a goroutine, sending
// results through ch and closing ch when done.
func (a *Adapter) streamElementsInternal(
	ctx context.Context,
	filter ports.ElementFilter,
	resultCh chan<- ports.ElementStreamResult,
) {
	defer close(resultCh)

	var (
		waitGroup sync.WaitGroup
		firstErr  error
		errMu     sync.Mutex
	)

	recordErr := func(err error) {
		errMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		errMu.Unlock()
	}

	send := func(result ports.ElementStreamResult) {
		select {
		case resultCh <- result:
		case <-ctx.Done():
		}
	}

	// Check Mission Control state once at the start
	var missionControlActive bool
	if a.detectMissionControl {
		missionControlActive = a.client.IsMissionControlActive()
	}

	if !missionControlActive {
		waitGroup.Go(func() {
			a.streamWindows(ctx, filter, send, recordErr)
		})
	}

	// Supplementary sources — batched (small), sent through the stream
	if !missionControlActive && filter.IncludeMenubar {
		waitGroup.Go(func() {
			elements := a.addMenubarElements(ctx, nil, filter)
			for _, e := range elements {
				send(ports.ElementStreamResult{Element: e})
			}
		})
	}

	if filter.IncludeDock {
		waitGroup.Go(func() {
			elements := a.addDockElements(ctx, nil)
			for _, e := range elements {
				send(ports.ElementStreamResult{Element: e})
			}
		})
	}

	if !missionControlActive && filter.IncludeNotificationCenter {
		waitGroup.Go(func() {
			elements := a.addNotificationCenterElements(ctx, nil)
			for _, e := range elements {
				send(ports.ElementStreamResult{Element: e})
			}
		})
	}

	if !missionControlActive && filter.IncludeStageManager {
		waitGroup.Go(func() {
			elements := a.addStageManagerElements(ctx, nil)
			for _, e := range elements {
				send(ports.ElementStreamResult{Element: e})
			}
		})
	}

	if !missionControlActive && filter.IncludePIP {
		waitGroup.Go(func() {
			elements := a.addPIPElements(ctx, nil)
			for _, e := range elements {
				send(ports.ElementStreamResult{Element: e})
			}
		})
	}

	if !missionControlActive && filter.IncludeScreenCapture {
		waitGroup.Go(func() {
			elements := a.addScreenCaptureElements(ctx, nil)
			for _, e := range elements {
				send(ports.ElementStreamResult{Element: e})
			}
		})
	}

	waitGroup.Wait()

	// Send first error after all goroutines complete
	errMu.Lock()
	if firstErr != nil {
		send(ports.ElementStreamResult{Err: firstErr})
	}
	errMu.Unlock()
}

// streamWindows discovers elements from the frontmost (and popover) windows,
// streaming each filtered element individually.
func (a *Adapter) streamWindows(
	ctx context.Context,
	filter ports.ElementFilter,
	send streamSink,
	recordErr func(error),
) {
	windowsToProcess, windowsErr := a.client.FrontmostAndPopoverWindows()
	if windowsErr != nil {
		recordErr(derrors.Wrap(windowsErr, derrors.CodeAccessibilityFailed,
			"failed to get frontmost and popover windows"))

		return
	}

	if len(windowsToProcess) == 0 {
		frontmost, frontmostErr := a.client.FrontmostWindow()
		if frontmostErr != nil {
			recordErr(derrors.Wrap(frontmostErr, derrors.CodeAccessibilityFailed,
				"failed to get frontmost window"))

			return
		}

		windowsToProcess = []AXWindow{frontmost}
	}

	var waitGroup sync.WaitGroup

	windowSem := make(chan struct{}, maxConcurrentWindows)

	for _, window := range windowsToProcess {
		windowSem <- struct{}{}

		waitGroup.Add(1)

		go func(w AXWindow) {
			defer waitGroup.Done()
			defer func() { <-windowSem }()

			a.streamWindowNodes(ctx, filter, w, send)
			w.Release()
		}(window)
	}

	waitGroup.Wait()
}

// streamWindowNodes streams clickable nodes from a single window using the
// streaming client API, converting and filtering each node as it arrives.
func (a *Adapter) streamWindowNodes(
	ctx context.Context,
	filter ports.ElementFilter,
	window AXWindow,
	send streamSink,
) {
	nodeCh, err := a.client.StreamClickableNodes(ctx, window, stringRoles(filter.Roles), 0)
	if err != nil {
		a.logger.Debug("Failed to stream clickable nodes for window",
			zap.Error(err))

		return
	}

	for node := range nodeCh {
		if ctx.Err() != nil {
			// Drain remaining nodes from the channel so the producer
			// goroutine isn't blocked. Lifecycle is managed by
			// streamClickableNodesGoroutine.
			for range nodeCh {
			}

			return
		}

		elem, convErr := a.convertToDomainElement(node)
		if convErr != nil {
			continue
		}

		if a.MatchesFilter(elem, filter) {
			send(ports.ElementStreamResult{Element: elem})
		}
	}
}
