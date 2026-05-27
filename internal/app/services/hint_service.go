package services

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/domain/hint"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
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
	if logger == nil {
		logger = zap.NewNop()
	}

	return &HintService{
		BaseService: NewBaseService(accessibility, overlay, system),
		generator:   generator,
		config:      config,
		logger:      logger.Named("service.hints"),
	}
}

// ShowHints displays hints for clickable elements on the screen.
// If filterRoles or filterTextContains are provided, they override the configured values.
func (s *HintService) ShowHints(
	ctx context.Context,
	filterRoles []string,
	filterTextContains []string,
) ([]*hint.Interface, error) {
	s.logger.Debug("Showing hints")

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

		return nil, derrors.WrapOverlayFailed(showHintsErr, "show hints")
	}

	s.logger.Debug("Hints displayed successfully", zap.Int("count", len(hints)))

	return hints, nil
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
			zap.Int("term_count", len(filterTextContains)))
	}

	// Get clickable elements
	axStart := time.Now()
	elements, elementsErr := s.accessibility.ClickableElements(ctx, filter)
	s.logger.Debug("TIMING: ClickableElements",
		zap.Duration("elapsed", time.Since(axStart)),
		zap.Int("element_count", len(elements)),
		zap.Error(elementsErr))

	if elementsErr != nil {
		s.logger.Error("Failed to get clickable elements", zap.Error(elementsErr))

		return nil, derrors.WrapAccessibilityFailed(elementsErr, "get clickable elements")
	}

	if len(elements) == 0 {
		s.logger.Debug("No clickable elements found")

		return nil, nil
	}

	s.logger.Debug("Found clickable elements", zap.Int("count", len(elements)))

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
	genStart := time.Now()
	hints, elementsErr := gen.Generate(ctx, elements)
	s.logger.Debug("TIMING: HintGenerator.Generate",
		zap.Duration("elapsed", time.Since(genStart)),
		zap.Int("element_count", len(elements)),
		zap.Int("hint_count", len(hints)),
		zap.Error(elementsErr))

	if elementsErr != nil {
		s.logger.Error("Failed to generate hints", zap.Error(elementsErr))

		return nil, derrors.WrapInternalFailed(elementsErr, "generate hints")
	}

	s.logger.Debug("Generated hints", zap.Int("count", len(hints)))

	return hints, nil
}

// HideHints removes the hint overlay from the screen.
func (s *HintService) HideHints(ctx context.Context) error {
	s.logger.Debug("Hiding hints")

	err := s.HideOverlay(ctx, "hide hints")
	if err != nil {
		s.logger.Error("Failed to hide overlay", zap.Error(err))

		return err
	}

	s.logger.Debug("Hints hidden successfully")

	return nil
}

// RefreshHints updates the hint display (e.g., after screen changes).
func (s *HintService) RefreshHints(ctx context.Context) error {
	s.logger.Debug("Refreshing hints")

	if !s.overlay.IsVisible() {
		s.logger.Debug("Overlay not visible, skipping refresh")

		return nil
	}

	refreshOverlayErr := s.overlay.Refresh(ctx)
	if refreshOverlayErr != nil {
		s.logger.Error("Failed to refresh overlay", zap.Error(refreshOverlayErr))

		return derrors.WrapOverlayFailed(refreshOverlayErr, "refresh hints")
	}

	s.logger.Debug("Hints refreshed successfully")

	return nil
}

// UpdateConfig updates the hints configuration.
// This allows changing hint filter settings at runtime.
func (s *HintService) UpdateConfig(config config.HintsConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config

	s.logger.Debug("Hints configuration updated",
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

	s.logger.Debug("Hint generator updated")
}
