package services

import (
	"context"
	"image"
	"slices"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/domain/hint"
	"github.com/y3owk1n/neru/internal/ports"
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
	// visionNotice is the last "the vision strategy cannot run here" reason a
	// user was told, so the same one is not repeated on every activation.
	visionNotice string
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

// GenerateHints collects clickable elements and generates labels without
// drawing them, so mode handlers can filter and position hints before the
// first render. A non-empty bundleID skips the AX lookup; non-empty overrides
// win over the config-derived strategy and label direction.
func (s *HintService) GenerateHints(
	ctx context.Context,
	filterRoles []string,
	filterTextContains []string,
	bundleID string,
	strategyOverride string,
	labelDirectionOverride string,
	splitWord bool,
) ([]*hint.Interface, error) {
	// This read must not be widened to span the strategy switch below: the
	// vision branch takes s.mu for writing (notifyVisionUnavailable), and a
	// sync.RWMutex is neither reentrant nor upgradable, so a read lock still
	// held there would deadlock this goroutine against itself.
	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()

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

	filter, usable := s.hintFilter(cfg, bundleID, filterRoles, filterTextContains)
	if !usable {
		return nil, nil
	}

	strategy := cfg.StrategyForApp(bundleID)
	if strategyOverride != "" {
		strategy = strategyOverride
	}

	labelDirection := cfg.LabelDirectionForApp(bundleID)
	if labelDirectionOverride != "" {
		labelDirection = labelDirectionOverride
	}

	if splitWord && strategy != domain.StrategyVision {
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
	case domain.StrategyVision:
		elements = s.generateHintsVision(ctx, bundleID, filter, splitWord)
	case domain.StrategyWLKBPTR:
		elements = s.generateHintsWLKBPTR(ctx, bundleID)
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

	return s.labelElements(ctx, elements, labelDirection)
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
// Hint filters can therefore change without a restart.
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
		// The two ways of getting here are not the same event. found=false with
		// no error is a desktop with nothing focused — routine, and the whole
		// screen is the right answer. An error means the platform could not
		// answer at all, and then scanning the whole screen is a degradation
		// nobody asked for: slower, noisier, and silent until now.
		if boundsErr != nil {
			s.logger.Warn(
				"Could not read the focused window, scanning the whole screen instead",
				zap.Error(boundsErr),
			)
		} else {
			s.logger.Debug("No focused window, scanning the whole screen")
		}

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

		// CodeNotSupported here means the machine cannot run this strategy at
		// all, and the error names what to install or which display server has
		// no path. That has to reach a person: what a user otherwise sees is an
		// overlay with nothing on it, because the supplementary elements kept
		// above are macOS surfaces with no counterpart elsewhere, and a log
		// line reaches nobody (ADR 0002). Transient failures stay in the log.
		if derrors.IsNotSupported(visionErr) {
			s.notifyVisionUnavailable(ctx, visionErr.Error())
		}

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

// generateHintsWLKBPTR detects interactive targets across the active screen
// using the wl-kbptr contour detection algorithm. Scanning the active screen
// rather than only the focused window ensures that desktop notifications,
// status bars, and adjacent tiled windows are all detected as clickable targets.
func (s *HintService) generateHintsWLKBPTR(
	ctx context.Context,
	_ string,
) []*element.Element {
	screenBounds, screenErr := s.resolveWLKBPTRScreenBounds(ctx)
	if screenErr != nil || screenBounds.Empty() {
		s.logger.Error(
			"Failed to resolve screen bounds for wl-kbptr detection",
			zap.Error(screenErr),
		)

		return nil
	}

	wlkbptrStart := time.Now()
	elements, err := s.vision.DetectWLKBPTR(ctx, screenBounds)
	s.logger.Debug("TIMING: Window elements (wl-kbptr)",
		zap.Duration("elapsed", time.Since(wlkbptrStart)),
		zap.Int("count", len(elements)),
		zap.Error(err),
	)

	if err != nil {
		s.logger.Error("Failed to detect elements via wl-kbptr", zap.Error(err))

		return nil
	}

	return elements
}

// resolveWLKBPTRScreenBounds determines the active display bounds for wl-kbptr detection.
// It prefers the monitor holding the focused window (matching resolveHintsScreenBounds)
// so multi-monitor setups capture the display the user is interacting with, and falls back
// to the monitor holding the cursor.
func (s *HintService) resolveWLKBPTRScreenBounds(ctx context.Context) (image.Rectangle, error) {
	var fallback image.Rectangle
	if s.system != nil {
		bounds, boundsErr := s.system.ScreenBounds(ctx)
		if boundsErr == nil {
			fallback = bounds
		}
	}

	if s.system == nil {
		if !fallback.Empty() {
			return fallback, nil
		}

		return image.Rectangle{}, derrors.New(derrors.CodeInternal, "system port is nil")
	}

	windowBounds, found, err := s.system.FocusedWindowBounds(ctx)
	if err != nil || !found || windowBounds.Empty() {
		if !fallback.Empty() {
			return fallback, nil
		}

		return image.Rectangle{}, err
	}

	if fallback.Empty() {
		fallback = windowBounds
	}

	center := image.Point{
		X: windowBounds.Min.X + windowBounds.Dx()/2,
		Y: windowBounds.Min.Y + windowBounds.Dy()/2,
	}

	if !fallback.Empty() && center.In(fallback) {
		return fallback, nil
	}

	names, namesErr := s.system.ScreenNames(ctx)
	if namesErr != nil {
		names = nil
	}

	for _, name := range names {
		bounds, foundScreen, bErr := s.system.ScreenBoundsByName(ctx, name)
		if bErr != nil || !foundScreen {
			continue
		}

		if center.In(bounds) {
			return bounds, nil
		}
	}

	return fallback, nil
}

// notifyVisionUnavailable tells the user, once, that the vision strategy
// cannot run here, carrying the reason the port gave.
//
// Once per distinct reason: a user who keeps pressing the hotkey gets one
// notification rather than one per press, and a different reason (the language
// data arrived, the session changed) is a new thing worth saying. The notice is
// the port's own sentence, which names a package or a display server and never
// anything read off the screen.
//
// Three things about how it is sent, each of which decides whether it arrives:
//
// It goes out on its own goroutine, because showing a notification is a
// session-bus round trip on Linux and three of the four callers of
// GenerateHints reach here holding the mode handler's lock.
//
// It drops the activation context's cancellation. Those callers build the
// context with a hint timeout and cancel it on return, which is microseconds
// after this is reached — and the Linux notification path honors a caller's
// deadline, so keeping it would cancel the very first send, the one that also
// dials the session bus. The send is still bounded, by the deadline that path
// imposes itself.
//
// And "told" is only remembered if the telling worked. A send claimed under the
// lock so two activations cannot both fire, and released again when it fails,
// so a session that had no notification daemon at the first attempt is not
// silenced for the life of the daemon.
//
// Locking: s.mu sits below the mode handler's lock — nothing held under it does
// I/O or reaches the handler, which is what makes taking it from locked context
// safe (internal/app/modes/AGENTS.md).
func (s *HintService) notifyVisionUnavailable(ctx context.Context, reason string) {
	if s.system == nil {
		return
	}

	s.mu.Lock()

	alreadyTold := s.visionNotice == reason
	if !alreadyTold {
		s.visionNotice = reason
	}
	s.mu.Unlock()

	if alreadyTold {
		return
	}

	notifyCtx := context.WithoutCancel(ctx)
	system, log := s.system, s.logger

	go func() {
		err := system.ShowNotification(notifyCtx, "neru hints", reason)
		if err == nil {
			return
		}

		log.Warn("Could not notify that the vision strategy is unavailable", zap.Error(err))

		s.mu.Lock()
		if s.visionNotice == reason {
			s.visionNotice = ""
		}
		s.mu.Unlock()
	}()
}

// hintFilter builds the element filter for one activation. The second result
// is false when every requested role belongs to another platform: hinting
// everything would hide that misconfiguration, so nothing is hinted instead
// (`neru roles --explain` and `neru doctor` report the cause).
func (s *HintService) hintFilter(
	cfg config.HintsConfig,
	bundleID string,
	filterRoles []string,
	filterTextContains []string,
) (ports.ElementFilter, bool) {
	filter := ports.DefaultElementFilter()

	// requested holds the entries as written, before resolution, so "no filter
	// at all" stays distinguishable from "a filter that resolved to nothing".
	var roles, requested []string

	if len(filterRoles) > 0 {
		// `neru hints --role ...` accepts the same vocabulary as the config
		// and is resolved the same way.
		requested = filterRoles

		resolution := element.ResolveRolesForCurrentPlatform(filterRoles)
		roles = resolution.Native

		s.logger.Debug("Using override roles from activation options",
			zap.Int("requested", len(requested)),
			zap.Int("role_count", len(roles)))

		for _, message := range resolution.FatalMessages() {
			s.logger.Warn("Ignoring role filter entry", zap.String("reason", message))
		}
	} else {
		requested = cfg.MergedForApp(bundleID).ClickableRoles
		roles = cfg.ClickableRolesForApp(bundleID)

		s.logger.Debug("Resolved clickable roles for hints",
			zap.String("bundle_id", bundleID),
			zap.Int("requested", len(requested)),
			zap.Int("role_count", len(roles)))
	}

	filter.Roles = make([]element.Role, 0, len(roles))
	for _, role := range roles {
		if role == "" {
			continue
		}

		filter.Roles = append(filter.Roles, element.Role(role))
	}

	if len(filter.Roles) == 0 && len(requested) > 0 {
		s.logger.Warn(
			"No configured role applies on this platform; showing no hints",
			zap.Int("requested", len(requested)),
		)

		return filter, false
	}

	filter.IncludeMenubar = cfg.IncludeMenubarHints
	filter.AdditionalMenubarTargets = cfg.AdditionalMenubarHintsTargets
	filter.IncludeDock = cfg.IncludeDockHints
	filter.IncludeNotificationCenter = cfg.IncludeNCHints
	filter.IncludeStageManager = cfg.IncludeStageManagerHints
	filter.IncludePIP = cfg.IncludePIPHints
	filter.IncludeScreenCapture = cfg.IncludeScreenCaptureHints

	// Text filter: an element matches when any term matches.
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

	return filter, true
}

// labelElements turns collected elements into labeled hints.
func (s *HintService) labelElements(
	ctx context.Context,
	elements []*element.Element,
	labelDirection string,
) ([]*hint.Interface, error) {
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
