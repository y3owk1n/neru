package services

import (
	"context"
	"image"
	"sync"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// HintService orchestrates hint generation and display.
// It coordinates between the accessibility system, hint generator, and overlay.
type HintService struct {
	BaseService

	mu        sync.RWMutex
	generator hint.Generator
	config    config.HintsConfig
	logger    *zap.Logger
}

// NewHintService creates a new hint service with the given dependencies.
func NewHintService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	system ports.SystemPort,
	generator hint.Generator,
	config config.HintsConfig,
	logger *zap.Logger,
) *HintService {
	return &HintService{
		BaseService: NewBaseService(accessibility, overlay, system),
		generator:   generator,
		config:      config,
		logger:      logger,
	}
}

// ShowHints displays hints for clickable elements on the screen.
// If filterRoles or filterTextContains are provided, they override the configured values.
func (s *HintService) ShowHints(
	ctx context.Context,
	filterRoles []string,
	filterTextContains []string,
) ([]*hint.Interface, error) {
	s.logger.Info("Showing hints")

	hints, err := s.GenerateHints(ctx, filterRoles, filterTextContains, "")
	if err != nil {
		return nil, err
	}

	if len(hints) == 0 {
		return hints, nil
	}

	// Display hints
	showHintsErr := s.overlay.ShowHints(ctx, hints)
	if showHintsErr != nil {
		s.logger.Error("Failed to show hints overlay", zap.Error(showHintsErr))

		return nil, core.WrapOverlayFailed(showHintsErr, "show hints")
	}

	s.logger.Info("Hints displayed successfully")

	return hints, nil
}

// HintStreamBatch is a unit of streaming hint output. The stream sends interim
// batches as elements are discovered, and a final batch with Done=true.
type HintStreamBatch struct {
	Hints []*hint.Interface
	Done  bool
	Err   error
}

// StreamHints returns a channel that delivers hint batches progressively as
// elements are discovered from the accessibility tree. The channel is closed
// after the final batch (Done: true).
func (s *HintService) StreamHints(
	ctx context.Context,
	filterRoles []string,
	filterTextContains []string,
	bundleID string,
	screenBounds image.Rectangle,
	gen hint.Generator,
) (<-chan HintStreamBatch, <-chan struct{}, error) {
	s.mu.RLock()

	cfg := s.config
	if gen == nil {
		gen = s.generator
	}

	s.mu.RUnlock()

	filter := s.buildElementFilter(ctx, cfg, filterRoles, filterTextContains, bundleID)
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}

	var (
		streamCh  <-chan ports.ElementStreamResult
		doneCh    <-chan struct{}
		streamErr error
	)

	if cfg.Streaming.Enabled {
		streamCh, doneCh, streamErr = s.accessibility.StreamElements(ctx, filter)
	} else {
		var elements []*element.Element

		elements, streamErr = s.accessibility.ClickableElements(ctx, filter)
		if streamErr == nil {
			resultCh := make(chan ports.ElementStreamResult, len(elements))
			for _, e := range elements {
				resultCh <- ports.ElementStreamResult{Element: e}
			}

			close(resultCh)

			streamCh = resultCh

			dc := make(chan struct{})
			close(dc)
			doneCh = dc
		}
	}

	if streamErr != nil {
		s.logger.Error("Failed to start element stream", zap.Error(streamErr))

		return nil, nil, core.WrapAccessibilityFailed(streamErr, "element stream")
	}

	outCh := make(chan HintStreamBatch, 4) //nolint:mnd // small buffer for streaming batches
	hintsDoneCh := make(chan struct{})

	batchInterval := cfg.Streaming.BatchInterval
	if batchInterval < 1 {
		batchInterval = config.DefaultHintStreamBatchInterval
	}

	go s.streamHintsInternal(ctx, streamCh, doneCh, outCh, hintsDoneCh, gen, batchInterval)

	return outCh, hintsDoneCh, nil
}

// GenerateHints collects clickable elements and generates labels without drawing
// them. Mode handlers use this to filter and position hints before the first
// render, avoiding an extra full overlay draw during activation.
// If bundleID is non-empty, it is used directly (skips AX call).
func (s *HintService) GenerateHints(
	ctx context.Context,
	filterRoles []string,
	filterTextContains []string,
	bundleID string,
) ([]*hint.Interface, error) {
	s.mu.RLock()
	cfg := s.config
	gen := s.generator
	s.mu.RUnlock()

	filter := ports.DefaultElementFilter()

	// Populate filter with configuration
	if bundleID == "" {
		var bundleIDErr error

		bundleID, bundleIDErr = s.accessibility.FocusedAppBundleID(ctx)
		if bundleIDErr != nil {
			s.logger.Debug(
				"Failed to get focused app bundle ID for hints roles",
				zap.Error(bundleIDErr),
			)
		}
	}

	// Use filterRoles if provided, otherwise use configured roles
	var roles []string
	if len(filterRoles) > 0 {
		roles = filterRoles
		s.logger.Debug("Using override roles from activation options",
			zap.Strings("roles", roles))
	} else {
		roles = cfg.ClickableRolesForApp(bundleID)
		s.logger.Debug("Resolved clickable roles for hints",
			zap.String("bundle_id", bundleID),
			zap.Int("role_count", len(roles)),
			zap.Strings("roles", roles))
	}

	filter.Roles = make([]element.Role, 0, len(roles))
	for _, role := range roles {
		if role == "" {
			continue
		}

		filter.Roles = append(filter.Roles, element.Role(role))
	}

	filter.IncludeMenubar = cfg.IncludeMenubarHints
	filter.AdditionalMenubarTargets = cfg.AdditionalMenubarHintsTargets
	filter.IncludeDock = cfg.IncludeDockHints
	filter.IncludeNotificationCenter = cfg.IncludeNCHints
	filter.IncludeStageManager = cfg.IncludeStageManagerHints
	filter.IncludePIP = cfg.IncludePIPHints
	filter.IncludeScreenCapture = cfg.IncludeScreenCaptureHints

	// Apply text filter if provided (OR match - element matches if any text contains match)
	if len(filterTextContains) > 0 {
		filter.TitleContains = filterTextContains[0]
		filter.DescriptionContains = filterTextContains[0]

		filter.ValueContains = filterTextContains[0]
		if len(filterTextContains) > 1 {
			filter.TextContainsList = filterTextContains[1:]
		}

		s.logger.Debug("Applying text filter",
			zap.Strings("text", filterTextContains))
	}

	// Get clickable elements
	elements, elementsErr := s.accessibility.ClickableElements(ctx, filter)
	if elementsErr != nil {
		s.logger.Error("Failed to get clickable elements", zap.Error(elementsErr))

		return nil, core.WrapAccessibilityFailed(elementsErr, "get clickable elements")
	}

	if len(elements) == 0 {
		s.logger.Info("No clickable elements found")

		return nil, nil
	}

	s.logger.Info("Found clickable elements", zap.Int("count", len(elements)))

	maxHints := gen.MaxHints()
	if maxHints > 0 && len(elements) > maxHints {
		s.logger.Warn(
			"Clickable element count exceeds available hint key combinations; showing as many as possible",
			zap.Int("element_count", len(elements)),
			zap.Int("max_hints", maxHints),
			zap.Int("omitted_count", len(elements)-maxHints),
		)
	}

	// Generate hints
	hints, elementsErr := gen.Generate(ctx, elements)
	if elementsErr != nil {
		s.logger.Error("Failed to generate hints", zap.Error(elementsErr))

		return nil, core.WrapInternalFailed(elementsErr, "generate hints")
	}

	s.logger.Info("Generated hints", zap.Int("count", len(hints)))

	return hints, nil
}

// HideHints removes the hint overlay from the screen.
func (s *HintService) HideHints(ctx context.Context) error {
	s.logger.Info("Hiding hints")

	err := s.HideOverlay(ctx, "hide hints")
	if err != nil {
		s.logger.Error("Failed to hide overlay", zap.Error(err))

		return err
	}

	s.logger.Info("Hints hidden successfully")

	return nil
}

// RefreshHints updates the hint display (e.g., after screen changes).
func (s *HintService) RefreshHints(ctx context.Context) error {
	s.logger.Info("Refreshing hints")

	if !s.overlay.IsVisible() {
		s.logger.Debug("Overlay not visible, skipping refresh")

		return nil
	}

	refreshOverlayErr := s.overlay.Refresh(ctx)
	if refreshOverlayErr != nil {
		s.logger.Error("Failed to refresh overlay", zap.Error(refreshOverlayErr))

		return core.WrapOverlayFailed(refreshOverlayErr, "refresh hints")
	}

	s.logger.Info("Hints refreshed successfully")

	return nil
}

// UpdateConfig updates the hints configuration.
// This allows changing hint filter settings at runtime.
func (s *HintService) UpdateConfig(config config.HintsConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config

	s.logger.Info("Hints configuration updated",
		zap.Bool("include_menubar", config.IncludeMenubarHints),
		zap.Bool("include_dock", config.IncludeDockHints),
		zap.Bool("include_nc", config.IncludeNCHints),
		zap.Bool("include_stage_manager", config.IncludeStageManagerHints),
		zap.Bool("include_pip", config.IncludePIPHints),
		zap.Bool("include_screen_capture", config.IncludeScreenCaptureHints))
}

// UpdateGenerator updates the hint generator.
// This allows changing the hint generation strategy at runtime.
func (s *HintService) UpdateGenerator(_ context.Context, generator hint.Generator) {
	if generator == nil {
		s.logger.Warn("Attempted to set nil generator, ignoring")

		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.generator = generator

	s.logger.Info("Hint generator updated")
}

// streamHintsInternal reads from the element stream, accumulates elements,
// and sends interim hint batches as elements arrive. The final batch
// (Done=true) uses proper position-sorted labels via gen.Generate. Interim
// batches use index-based labels (no sort) so hint generation stays O(N).
func (s *HintService) streamHintsInternal(
	ctx context.Context,
	streamCh <-chan ports.ElementStreamResult,
	doneCh <-chan struct{},
	outCh chan<- HintStreamBatch,
	hintsDoneCh chan<- struct{},
	gen hint.Generator,
	batchInterval int,
) {
	defer func() {
		// Wait for upstream to be fully done before signaling completion to avoid CGO concurrency
		if doneCh != nil {
			<-doneCh
		}

		close(hintsDoneCh)
	}()
	defer close(outCh)

	var (
		allElements    []*element.Element
		lastBatchCount int
	)

	trySend := func(batch HintStreamBatch) {
		select {
		case outCh <- batch:
		case <-ctx.Done():
		}
	}

	sendBatch := func(done bool) {
		if len(allElements) == 0 {
			if done {
				trySend(HintStreamBatch{Done: true})
			}

			return
		}

		elements := allElements

		maxHints := gen.MaxHints()
		if maxHints > 0 && len(elements) > maxHints {
			elements = elements[:maxHints]
		}

		hints, hintsErr := gen.Generate(ctx, elements)
		if hintsErr != nil {
			s.logger.Error("Failed to generate final hints", zap.Error(hintsErr))

			trySend(HintStreamBatch{Err: hintsErr, Done: done})

			return
		}

		trySend(HintStreamBatch{
			Hints: hints,
			Done:  done,
		})
	}

	for {
		select {
		case <-ctx.Done():
			sendBatch(true)

			return

		case result, ok := <-streamCh:
			if !ok {
				// Stream closed — send final batch
				sendBatch(true)

				return
			}

			if result.Err != nil {
				s.logger.Error("Stream element error", zap.Error(result.Err))

				trySend(HintStreamBatch{Err: result.Err})

				continue
			}

			if result.Element != nil {
				allElements = append(allElements, result.Element)
			}

			// Emit interim batch every batchInterval elements.
			// Uses index-based labels (no sort) to avoid O(N²) regeneration.
			if len(allElements)-lastBatchCount >= batchInterval {
				lastBatchCount = len(allElements)

				hints := make([]*hint.Interface, len(allElements))
				for index, elem := range allElements {
					h, err := hint.NewHint(indexToLabel(index), elem, elem.Center())
					if err != nil {
						continue
					}

					hints[index] = h
				}

				trySend(HintStreamBatch{Hints: hints, Done: false})
			}
		}
	}
}

// buildElementFilter constructs an ElementFilter from the service configuration
// and the provided override parameters.
func (s *HintService) buildElementFilter(
	ctx context.Context,
	cfg config.HintsConfig,
	filterRoles []string,
	filterTextContains []string,
	bundleID string,
) ports.ElementFilter {
	filter := ports.DefaultElementFilter()

	// Resolve bundle ID if not provided
	if bundleID == "" {
		var bundleIDErr error

		bundleID, bundleIDErr = s.accessibility.FocusedAppBundleID(ctx)
		if bundleIDErr != nil {
			s.logger.Debug(
				"Failed to get focused app bundle ID for hints roles",
				zap.Error(bundleIDErr),
			)
		}
	}

	// Use filterRoles if provided, otherwise use configured roles
	var roles []string
	if len(filterRoles) > 0 {
		roles = filterRoles
		s.logger.Debug("Using override roles from activation options",
			zap.Strings("roles", roles))
	} else {
		roles = cfg.ClickableRolesForApp(bundleID)
		s.logger.Debug("Resolved clickable roles for hints",
			zap.String("bundle_id", bundleID),
			zap.Int("role_count", len(roles)),
			zap.Strings("roles", roles))
	}

	filter.Roles = make([]element.Role, 0, len(roles))
	for _, role := range roles {
		if role == "" {
			continue
		}

		filter.Roles = append(filter.Roles, element.Role(role))
	}

	filter.IncludeMenubar = cfg.IncludeMenubarHints
	filter.AdditionalMenubarTargets = cfg.AdditionalMenubarHintsTargets
	filter.IncludeDock = cfg.IncludeDockHints
	filter.IncludeNotificationCenter = cfg.IncludeNCHints
	filter.IncludeStageManager = cfg.IncludeStageManagerHints
	filter.IncludePIP = cfg.IncludePIPHints
	filter.IncludeScreenCapture = cfg.IncludeScreenCaptureHints

	if len(filterTextContains) > 0 {
		filter.TitleContains = filterTextContains[0]
		filter.DescriptionContains = filterTextContains[0]

		filter.ValueContains = filterTextContains[0]
		if len(filterTextContains) > 1 {
			filter.TextContainsList = filterTextContains[1:]
		}
	}

	return filter
}

// indexToLabel converts a zero-based element index to a unique letter label
// using base-26 encoding (a-z). 0→"a", 1→"b", …, 25→"z", 26→"aa", 27→"ab".
// This is O(1) per element and avoids the O(N log N) sort of gen.Generate.
func indexToLabel(i int) string {
	num := i + 1

	var rev [16]byte

	pos := len(rev)

	for num > 0 {
		num--
		pos--
		rev[pos] = 'a' + byte(num%26) //nolint:mnd
		num /= 26
	}

	return string(rev[pos:])
}
