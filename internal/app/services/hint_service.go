package services

import (
	"context"
	"slices"
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
// It coordinates between the accessibility system, vision detection,
// hint generator, and overlay.
//
// Generators are cached per label direction so that switching direction
// (e.g. via the --label-direction CLI flag) does not require rebuilding
// the entire generator state. The configured label direction is always
// available as the default.
type HintService struct {
	BaseService

	mu               sync.RWMutex
	generators       map[string]hint.Generator // keyed by label direction
	defaultGenerator hint.Generator
	config           config.HintsConfig
	logger           *zap.Logger
	vision           ports.VisionPort
}

// NewHintService creates a new hint service with the given dependencies.
//
// The supplied generator is treated as the default (typically the configured
// label direction). Callers that need additional directions for per-activation
// overrides should use UpdateGenerator to register them.
func NewHintService(
	accessibility ports.AccessibilityPort,
	overlay ports.OverlayPort,
	system ports.SystemPort,
	generator hint.Generator,
	config config.HintsConfig,
	logger *zap.Logger,
	vision ports.VisionPort,
) *HintService {
	if logger == nil {
		logger = zap.NewNop()
	}

	generators := make(map[string]hint.Generator)

	if generator != nil {
		generators[generator.LabelDirection().String()] = generator
	}

	return &HintService{
		BaseService:      NewBaseService(accessibility, overlay, system),
		generators:       generators,
		defaultGenerator: generator,
		config:           config,
		logger:           logger.Named("service.hints"),
		vision:           vision,
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

	hints, err := s.GenerateHints(ctx, filterRoles, filterTextContains, "", "", "", false)
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
// If strategyOverride is non-empty, it overrides the config-derived strategy.
// If labelDirectionOverride is non-empty, it overrides the config-derived label direction.
func (s *HintService) GenerateHints(
	ctx context.Context,
	filterRoles []string,
	filterTextContains []string,
	bundleID string,
	strategyOverride string,
	labelDirectionOverride string,
	splitWord bool,
) ([]*hint.Interface, error) {
	s.mu.RLock()
	cfg := s.config
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

	// Determine strategy for the frontmost app (override takes precedence)
	strategy := cfg.StrategyForApp(bundleID)
	if strategyOverride != "" {
		strategy = strategyOverride
	}

	// Determine label direction for the frontmost app (override takes precedence)
	labelDirection := cfg.LabelDirectionForApp(bundleID)
	if labelDirectionOverride != "" {
		labelDirection = labelDirectionOverride
	}

	if splitWord && strategy != config.StrategyVision {
		return nil, derrors.New(
			derrors.CodeInvalidInput,
			"--split-word is only supported when resolved strategy is 'vision'",
		)
	}

	var (
		elements []*element.Element
		genErr   error
	)

	switch strategy {
	case config.StrategyVision:
		elements = s.generateHintsVision(ctx, bundleID, filter, splitWord)
	default:
		elements, genErr = s.generateHintsAX(ctx, filter)
	}

	if genErr != nil {
		return nil, genErr
	}

	if len(elements) == 0 {
		s.logger.Debug("No clickable elements found")

		return nil, nil
	}

	s.logger.Debug("Found clickable elements", zap.Int("count", len(elements)))

	gen := s.Generator(labelDirection)

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
		zap.String("label_direction", gen.LabelDirection().String()),
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

// Generator returns the registered hint generator for the given label
// direction. An empty direction resolves to the default generator. If no
// generator exists for the requested direction the default is returned as a
// fallback so hint generation never fails purely because of a direction
// mismatch (e.g. during the brief window after a config reload before the
// caller registers the new generator).
func (s *HintService) Generator(direction string) hint.Generator {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if direction != "" {
		if g, ok := s.generators[direction]; ok {
			return g
		}
	}

	return s.defaultGenerator
}

// UpdateGenerator registers a hint generator for a specific label direction.
// The first registration becomes the default fallback; subsequent
// registrations for the *same* direction also replace the default so a
// config reload that changes `hint_characters` keeps the empty/unknown
// direction fallback in sync with the configured generator. A nil
// generator is ignored to avoid replacing a live generator with nothing.
func (s *HintService) UpdateGenerator(_ context.Context, generator hint.Generator) {
	if generator == nil {
		s.logger.Warn("Attempted to set nil generator, ignoring")

		return
	}

	direction := generator.LabelDirection().String()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.defaultGenerator == nil ||
		s.defaultGenerator.LabelDirection() == generator.LabelDirection() {
		s.defaultGenerator = generator
	}

	s.generators[direction] = generator

	s.logger.Debug("Hint generator updated", zap.String("direction", direction))
}

// generateHintsAX collects elements using the AX tree (default strategy).
func (s *HintService) generateHintsAX(
	ctx context.Context,
	filter ports.ElementFilter,
) ([]*element.Element, error) {
	axStart := time.Now()
	elements, err := s.accessibility.ClickableElements(ctx, filter)
	s.logger.Debug("TIMING: ClickableElements (axtree)",
		zap.Duration("elapsed", time.Since(axStart)),
		zap.Int("element_count", len(elements)),
		zap.Error(err))

	if err != nil {
		s.logger.Error("Failed to get clickable elements via AX", zap.Error(err))

		return nil, derrors.WrapAccessibilityFailed(err, "get clickable elements")
	}

	return elements, nil
}

// generateHintsVision collects window elements via vision detection and
// supplementary elements (menubar, dock, etc.) via AX. This hybrid approach
// ensures system UI is always detected while the frontmost window content
// uses vision-based detection for apps with poor AX trees.
func (s *HintService) generateHintsVision(
	ctx context.Context,
	_ string,
	filter ports.ElementFilter,
	splitWord bool,
) []*element.Element {
	// Collect supplementary elements (menubar, dock, NC, etc.) via AX.
	// These are system-level components that vision should not attempt to detect.
	var allElements []*element.Element

	supplementStart := time.Now()
	supplementFilter := filter
	supplementFilter.Roles = nil               // no role filtering for supplementary elements
	supplementFilter.SkipWindowElements = true // vision handles the window

	supplementElements, err := s.accessibility.ClickableElements(ctx, supplementFilter)
	if err != nil {
		s.logger.Debug("Failed to get supplementary elements via AX", zap.Error(err))
	} else {
		allElements = append(allElements, supplementElements...)
	}

	s.logger.Debug("TIMING: Supplementary elements (AX)",
		zap.Duration("elapsed", time.Since(supplementStart)),
		zap.Int("count", len(supplementElements)))

	if s.vision == nil {
		s.logger.Warn("Vision strategy selected but vision port is unavailable")

		return allElements
	}

	// Get focused window bounds for vision detection
	windowBounds, found, boundsErr := s.system.FocusedWindowBounds(ctx)
	if boundsErr != nil || !found {
		s.logger.Debug(
			"No focused window bounds, falling back to full screen",
			zap.Error(boundsErr),
		)

		windowBounds, boundsErr = s.system.ScreenBounds(ctx)
		if boundsErr != nil {
			s.logger.Error("Failed to get screen bounds for vision detection", zap.Error(boundsErr))

			return allElements
		}
	}

	// Detect window elements via vision
	visionStart := time.Now()
	windowElements, visionErr := s.vision.DetectElements(
		ctx,
		windowBounds,
		s.config.Vision,
		splitWord,
	)
	s.logger.Debug("TIMING: Window elements (vision)",
		zap.Duration("elapsed", time.Since(visionStart)),
		zap.Int("count", len(windowElements)),
		zap.Error(visionErr))

	if visionErr != nil {
		s.logger.Error("Failed to detect elements via vision", zap.Error(visionErr))

		return allElements
	}

	// Filter vision-detected elements by configured roles
	for _, element := range windowElements {
		if len(filter.Roles) == 0 {
			allElements = append(allElements, element)

			continue
		}

		if slices.Contains(filter.Roles, element.Role()) {
			allElements = append(allElements, element)
		}
	}

	return allElements
}
