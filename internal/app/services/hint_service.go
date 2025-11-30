package services

import (
	"context"
	"crypto/md5"
	"fmt"
	"image"
	"sync"
	"time"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/ports"
	"go.uber.org/zap"
)

// elementCacheEntry represents a cached element with its position hash.
type elementCacheEntry struct {
	element   *element.Element
	position  image.Rectangle
	hash      string
	timestamp time.Time
}

// HintService orchestrates hint generation and display.
// It coordinates between the accessibility system, hint generator, and overlay.
type HintService struct {
	accessibility ports.AccessibilityPort
	overlay       ports.OverlayPort
	generator     hint.Generator
	config        config.HintsConfig
	logger        *zap.Logger

	// Cache for incremental updates
	elementCache map[string]*elementCacheEntry
	cacheMutex   sync.RWMutex
}

// NewHintService creates a new hint service with the given dependencies.
func NewHintService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	generator hint.Generator,
	config config.HintsConfig,
	logger *zap.Logger,
) *HintService {
	return &HintService{
		accessibility: accessibility,
		overlay:       overlay,
		generator:     generator,
		config:        config,
		logger:        logger,
		elementCache:  make(map[string]*elementCacheEntry),
	}
}

// ShowHints displays hints for clickable elements on the screen.
func (s *HintService) ShowHints(
	ctx context.Context,
) ([]*hint.Interface, error) {
	s.logger.Info("Showing hints")

	filter := ports.DefaultElementFilter()

	// Populate filter with configuration
	filter.IncludeMenubar = s.config.IncludeMenubarHints
	filter.AdditionalMenubarTargets = s.config.AdditionalMenubarHintsTargets
	filter.IncludeDock = s.config.IncludeDockHints
	filter.IncludeNotificationCenter = s.config.IncludeNCHints

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

	// Check for incremental updates
	elements = s.filterChangedElements(elements)

	s.logger.Info("Elements after incremental filtering", zap.Int("count", len(elements)))

	// Update cache with new elements
	s.updateElementCache(elements)

	// Generate hints
	hints, elementsErr := s.generator.Generate(ctx, elements)
	if elementsErr != nil {
		s.logger.Error("Failed to generate hints", zap.Error(elementsErr))

		return nil, core.WrapInternalFailed(elementsErr, "generate hints")
	}

	s.logger.Info("Generated hints", zap.Int("count", len(hints)))

	// Display hints
	showHintsErr := s.overlay.ShowHints(ctx, hints)
	if showHintsErr != nil {
		s.logger.Error("Failed to show hints overlay", zap.Error(showHintsErr))

		return nil, core.WrapOverlayFailed(showHintsErr, "show hints")
	}

	s.logger.Info("Hints displayed successfully")

	return hints, nil
}

// HideHints removes the hint overlay from the screen.
func (s *HintService) HideHints(ctx context.Context) error {
	s.logger.Info("Hiding hints")

	hideOverlayErr := s.overlay.Hide(ctx)
	if hideOverlayErr != nil {
		s.logger.Error("Failed to hide overlay", zap.Error(hideOverlayErr))

		return core.WrapOverlayFailed(hideOverlayErr, "hide hints")
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

// UpdateGenerator updates the hint generator.
// This allows changing the hint generation strategy at runtime.
func (s *HintService) UpdateGenerator(_ context.Context, generator hint.Generator) {
	if generator == nil {
		s.logger.Warn("Attempted to set nil generator, ignoring")

		return
	}

	s.generator = generator
	s.logger.Info("Hint generator updated")
}

// Health checks the health of the service's dependencies.
func (s *HintService) Health(ctx context.Context) map[string]error {
	return map[string]error{
		"accessibility": s.accessibility.Health(ctx),
		"overlay":       s.overlay.Health(ctx),
	}
}

// filterChangedElements filters elements to only include those that have changed position or are new.
func (s *HintService) filterChangedElements(elements []*element.Element) []*element.Element {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	var changedElements []*element.Element

	for _, elem := range elements {
		bounds := elem.Bounds()
		elementID := string(elem.ID())

		// Create hash of element position and size
		hashInput := fmt.Sprintf(
			"%s:%d:%d:%d:%d",
			elementID,
			bounds.Min.X,
			bounds.Min.Y,
			bounds.Max.X,
			bounds.Max.Y,
		)
		hash := fmt.Sprintf("%x", md5.Sum([]byte(hashInput)))

		cached, exists := s.elementCache[elementID]
		if !exists || cached.hash != hash {
			// Element is new or has changed
			changedElements = append(changedElements, elem)
		}
	}

	s.logger.Debug("Incremental update filtering",
		zap.Int("total_elements", len(elements)),
		zap.Int("changed_elements", len(changedElements)))

	return changedElements
}

// updateElementCache updates the cache with new element positions.
func (s *HintService) updateElementCache(elements []*element.Element) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	now := time.Now()

	// Clear old cache entries (older than 30 seconds)
	for elementID, entry := range s.elementCache {
		if now.Sub(entry.timestamp) > 30*time.Second {
			delete(s.elementCache, elementID)
		}
	}

	// Update cache with new elements
	for _, elem := range elements {
		bounds := elem.Bounds()
		elementID := string(elem.ID())

		hashInput := fmt.Sprintf(
			"%s:%d:%d:%d:%d",
			elementID,
			bounds.Min.X,
			bounds.Min.Y,
			bounds.Max.X,
			bounds.Max.Y,
		)
		hash := fmt.Sprintf("%x", md5.Sum([]byte(hashInput)))

		s.elementCache[elementID] = &elementCacheEntry{
			element:   elem,
			position:  bounds,
			hash:      hash,
			timestamp: now,
		}
	}

	s.logger.Debug("Updated element cache", zap.Int("cached_elements", len(s.elementCache)))
}
