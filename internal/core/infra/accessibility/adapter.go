package accessibility

import (
	"context"
	"image"
	"runtime"
	"sync"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

const (
	// TypicalElementCount is the estimated number of elements to pre-allocate.
	TypicalElementCount = 50

	// ConcurrentProcessingThreshold is the number of nodes that triggers concurrent processing.
	ConcurrentProcessingThreshold = 100

	// EstimatedFilteringRatio is the estimated ratio of nodes that pass the filter.
	EstimatedFilteringRatio = 2

	// contextCheckInterval is the interval for checking context cancellation.
	contextCheckInterval = 100

	// maxConcurrentWorkers is the maximum number of workers for concurrent processing.
	maxConcurrentWorkers = 8
)

// elementSlicePool is a pool of element slices for temporary use.
var elementSlicePool = sync.Pool{
	New: func() any {
		s := make([]*element.Element, 0, TypicalElementCount)

		return &s
	},
}

// Adapter implements ports.AccessibilityPort by wrapping the AXClient.
// It converts between domain models and infrastructure types.
type Adapter struct {
	// logger for adapter.
	logger          *zap.Logger
	client          AXClient
	excludedBundles map[string]bool
	// clickableRoles is the list of clickable roles.
	clickableRoles []string
}

// NewAdapter creates a new accessibility adapter.
func NewAdapter(
	logger *zap.Logger,
	excludedBundles []string,
	clickableRoles []string,
	client AXClient,
) *Adapter {
	excludedMap := make(map[string]bool, len(excludedBundles))
	for _, bundle := range excludedBundles {
		excludedMap[bundle] = true
	}

	return &Adapter{
		logger:          logger,
		client:          client,
		excludedBundles: excludedMap,
		clickableRoles:  clickableRoles,
	}
}

// Logger returns the logger for the adapter.
// It is used for testing mainly.
func (a *Adapter) Logger() *zap.Logger {
	return a.logger
}

// ClickableRoles returns the list of clickable roles.
// It is used for testing mainly.
func (a *Adapter) ClickableRoles() []string {
	return a.clickableRoles
}

// ClickableElements retrieves all clickable UI elements matching the filter.
func (a *Adapter) ClickableElements(
	ctx context.Context,
	filter ports.ElementFilter,
) ([]*element.Element, error) {
	// Check context
	err := a.checkContext(ctx)
	if err != nil {
		return nil, err
	}

	a.logger.Debug("Getting clickable elements", zap.Any("filter", filter))

	// Use bounded concurrency for parallel queries
	const maxConcurrency = 3 // Limit concurrent queries to avoid overwhelming the system

	semaphore := make(chan struct{}, maxConcurrency)
	// Initialize semaphore with tokens
	for range maxConcurrency {
		semaphore <- struct{}{}
	}

	var (
		waitGroup sync.WaitGroup
		mutex     sync.Mutex
		// Pre-allocate with estimated capacity for typical web page
		allElements = make([]*element.Element, 0, TypicalElementCount)
		firstError  error
	)

	// Function to collect elements from a source
	collectElements := func(sourceName string, queryFunc func() ([]*element.Element, error)) {
		defer waitGroup.Done()

		<-semaphore // Acquire semaphore

		defer func() {
			semaphore <- struct{}{} // Release semaphore
		}()

		elements, err := queryFunc()
		if err != nil {
			mutex.Lock()

			if firstError == nil {
				firstError = derrors.Wrap(err, derrors.CodeAccessibilityFailed,
					"failed to get elements from "+sourceName)
			}

			mutex.Unlock()

			return
		}

		mutex.Lock()

		allElements = append(allElements, elements...)

		mutex.Unlock()

		a.logger.Debug("Collected elements from "+sourceName, zap.Int("count", len(elements)))
	}

	if !IsMissionControlActive() {
		// Query frontmost window
		waitGroup.Add(1)

		go func() {
			collectElements("frontmost window", func() ([]*element.Element, error) {
				frontmostWindow, frontmostWindowErr := a.client.FrontmostWindow()
				if frontmostWindowErr != nil {
					return nil, frontmostWindowErr
				}
				defer frontmostWindow.Release()

				clickableNodes, clickableNodesErr := a.client.ClickableNodes(
					frontmostWindow,
					filter.IncludeOffscreen,
					nil,
				)
				if clickableNodesErr != nil {
					return nil, clickableNodesErr
				}

				return a.processClickableNodes(ctx, clickableNodes, filter)
			})
		}()
	}

	// Query supplementary elements in parallel
	if filter.IncludeMenubar || filter.IncludeDock || filter.IncludeStageManager ||
		len(filter.AdditionalMenubarTargets) > 0 {
		waitGroup.Add(1)

		go func() {
			collectElements("supplementary sources", func() ([]*element.Element, error) {
				return a.addSupplementaryElements(ctx, []*element.Element{}, filter), nil
			})
		}()
	}

	// Wait for all queries to complete
	waitGroup.Wait()

	if firstError != nil {
		return nil, firstError
	}

	a.logger.Info("Total elements collected", zap.Int("count", len(allElements)))

	return allElements, nil
}

// PerformAction executes an action on the specified element.
func (a *Adapter) PerformAction(
	ctx context.Context,
	element *element.Element,
	actionType action.Type,
) error {
	// Check context
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
	}

	a.logger.Info("Performing action",
		zap.String("action", actionType.String()),
		zap.String("element_id", string(element.ID())))

	center := element.Center()

	restoreCursor := a.getRestoreCursor()

	// Perform the action via client
	performActionErr := a.client.PerformAction(actionType, center, restoreCursor)
	if performActionErr != nil {
		return derrors.Wrap(performActionErr, derrors.CodeActionFailed, "failed to perform action")
	}

	return nil
}

// PerformActionAtPoint executes an action at the specified point.
func (a *Adapter) PerformActionAtPoint(
	ctx context.Context,
	actionType action.Type,
	point image.Point,
) error {
	// Check context
	err := a.checkContext(ctx)
	if err != nil {
		return err
	}

	a.logger.Info("Performing action at point",
		zap.String("action", actionType.String()),
		zap.Int("x", point.X),
		zap.Int("y", point.Y))

	restoreCursor := a.getRestoreCursor()

	// Perform the action via client
	performActionErr := a.client.PerformAction(actionType, point, restoreCursor)
	if performActionErr != nil {
		return derrors.Wrap(
			performActionErr,
			derrors.CodeActionFailed,
			"failed to perform action at point",
		)
	}

	return nil
}

// Scroll performs a scroll action at the current cursor position.
func (a *Adapter) Scroll(_ context.Context, deltaX, deltaY int) error {
	a.logger.Debug("Performing scroll",
		zap.Int("deltaX", deltaX),
		zap.Int("deltaY", deltaY))

	scrollErr := a.client.Scroll(deltaX, deltaY)
	if scrollErr != nil {
		return derrors.Wrap(scrollErr, derrors.CodeActionFailed, "failed to scroll")
	}

	a.logger.Debug("Scroll completed")

	return nil
}

// MoveCursorToPoint moves the mouse cursor to the specified point.
func (a *Adapter) MoveCursorToPoint(_ context.Context, point image.Point, bypassSmooth bool) error {
	a.logger.Debug("Moving cursor to point",
		zap.Int("x", point.X),
		zap.Int("y", point.Y),
		zap.Bool("bypassSmooth", bypassSmooth))

	a.client.MoveMouse(point, bypassSmooth)

	return nil
}

// CursorPosition returns the current cursor position.
func (a *Adapter) CursorPosition(_ context.Context) (image.Point, error) {
	return a.client.CursorPosition(), nil
}

// FocusedAppBundleID returns the bundle ID of the currently focused application.
func (a *Adapter) FocusedAppBundleID(ctx context.Context) (string, error) {
	// Check context
	err := a.checkContext(ctx)
	if err != nil {
		return "", err
	}

	focusedApp, focusedAppErr := a.client.FocusedApplication()
	if focusedAppErr != nil {
		return "", derrors.New(derrors.CodeAccessibilityFailed, "failed to get focused application")
	}
	defer focusedApp.Release()

	bundleID := focusedApp.BundleIdentifier()
	if bundleID == "" {
		return "", derrors.New(derrors.CodeAccessibilityFailed, "failed to get bundle ID")
	}

	return bundleID, nil
}

// IsAppExcluded checks if the given bundle ID is in the exclusion list.
func (a *Adapter) IsAppExcluded(_ context.Context, bundleID string) bool {
	return a.excludedBundles[bundleID]
}

// ScreenBounds returns the bounds of the active screen.
func (a *Adapter) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	// Check context
	err := a.checkContext(ctx)
	if err != nil {
		return image.Rectangle{}, err
	}

	return a.client.ActiveScreenBounds(), nil
}

// CheckPermissions verifies that accessibility permissions are granted.
func (a *Adapter) CheckPermissions(ctx context.Context) error {
	// Check context
	err := a.checkContext(ctx)
	if err != nil {
		return err
	}

	if !a.client.CheckPermissions() {
		return derrors.New(derrors.CodeAccessibilityDenied,
			"accessibility permissions not granted - please enable in System Preferences")
	}

	return nil
}

// Health checks if the accessibility permissions are granted.
func (a *Adapter) Health(ctx context.Context) error {
	return a.CheckPermissions(ctx)
}

// UpdateClickableRoles updates the list of clickable roles.
func (a *Adapter) UpdateClickableRoles(roles []string) {
	a.logger.Info("Updating clickable roles", zap.Int("count", len(roles)))
	a.clickableRoles = roles
	a.client.SetClickableRoles(roles)
}

// UpdateExcludedBundles updates the list of excluded bundle IDs.
func (a *Adapter) UpdateExcludedBundles(bundles []string) {
	a.logger.Info("Updating excluded bundles", zap.Int("count", len(bundles)))

	a.excludedBundles = make(map[string]bool, len(bundles))
	for _, bundle := range bundles {
		a.excludedBundles[bundle] = true
	}
}

// checkContext checks if the context is canceled and returns an error if so.
func (a *Adapter) checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return derrors.Wrap(ctx.Err(), derrors.CodeContextCanceled, "operation canceled")
	default:
		return nil
	}
}

// getRestoreCursor retrieves the restore cursor setting from global config.
func (a *Adapter) getRestoreCursor() bool {
	cfg := config.Global()

	return cfg != nil && cfg.General.RestoreCursorPosition
}

// processClickableNodes converts and filters clickable nodes to domain elements.
func (a *Adapter) processClickableNodes(
	ctx context.Context,
	clickableNodes []AXNode,
	filter ports.ElementFilter,
) ([]*element.Element, error) {
	// Get pooled slice and reset it
	elementsPtr, ok := elementSlicePool.Get().(*[]*element.Element)
	if !ok {
		s := make([]*element.Element, 0, TypicalElementCount)
		elementsPtr = &s
	}

	elements := (*elementsPtr)[:0] // Reset to zero length but keep capacity
	defer func() {
		// Clear references before returning to pool
		for i := range elements {
			elements[i] = nil
		}

		elementSlicePool.Put(elementsPtr)
	}()

	// Concurrent processing for large number of nodes
	if len(clickableNodes) > ConcurrentProcessingThreshold {
		return a.processClickableNodesConcurrent(ctx, clickableNodes, filter)
	}

	for index, node := range clickableNodes {
		// Check context periodically
		if index%contextCheckInterval == 0 {
			err := a.checkContext(ctx)
			if err != nil {
				return nil, err
			}
		}

		elem, err := a.convertToDomainElement(node)
		if err != nil {
			a.logger.Warn("Failed to convert element", zap.Error(err))

			continue
		}

		// Apply filter
		if a.MatchesFilter(elem, filter) {
			elements = append(elements, elem)
		}
	}

	// Make a copy to return since we're returning the pooled slice
	result := make([]*element.Element, len(elements))
	copy(result, elements)

	return result, nil
}

// processClickableNodesConcurrent processes nodes in parallel using a worker pool.
func (a *Adapter) processClickableNodesConcurrent(
	ctx context.Context,
	nodes []AXNode,
	filter ports.ElementFilter,
) ([]*element.Element, error) {
	numWorkers := min(
		// Use available parallelism
		runtime.GOMAXPROCS(0),
		// Cap to avoid diminishing returns
		maxConcurrentWorkers)

	chunkSize := (len(nodes) + numWorkers - 1) / numWorkers

	type result struct {
		elements []*element.Element
		err      error
	}

	results := make(chan result, numWorkers)

	var waitGroup sync.WaitGroup

	for i := range numWorkers {
		start := i * chunkSize

		end := start + chunkSize
		if start >= len(nodes) {
			break
		}

		if end > len(nodes) {
			end = len(nodes)
		}

		waitGroup.Add(1)

		go func(chunk []AXNode) {
			defer waitGroup.Done()

			// Use local slice to avoid locking
			localElements := make([]*element.Element, 0, len(chunk))

			for idx, node := range chunk {
				if idx%contextCheckInterval == 0 && ctx.Err() != nil {
					results <- result{err: ctx.Err()}

					return
				}

				elem, err := a.convertToDomainElement(node)
				if err != nil {
					continue
				}

				if a.MatchesFilter(elem, filter) {
					localElements = append(localElements, elem)
				}
			}

			results <- result{elements: localElements}
		}(nodes[start:end])
	}

	go func() {
		waitGroup.Wait()
		close(results)
	}()

	// Collect results
	// Pre-allocate based on input size estimate (conservative)
	allElements := make([]*element.Element, 0, len(nodes)/EstimatedFilteringRatio)

	for res := range results {
		if res.err != nil {
			return nil, res.err
		}

		allElements = append(allElements, res.elements...)
	}

	return allElements, nil
}

// Ensure Adapter implements ports.AccessibilityPort.
var _ ports.AccessibilityPort = (*Adapter)(nil)
