package config

import (
	"runtime"
	"slices"
	"strings"
	"unicode"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/element"
)

// The two places a clickable-role list is written. Both a refusal and a warning
// name one of them, and a user matching a report to a line in their file is
// what the names are for, so they are spelled once.
const (
	fieldClickableRoles           = "hints.clickable_roles"
	fieldAdditionalClickableRoles = "hints.app_configs.additional_clickable_roles"
)

// validateClickableRoles rejects role entries that name neither a semantic role
// nor a native role in a known vocabulary. Entries that are well-formed but do
// not apply to this machine are not refused here, so one config can serve
// several: those go to [Config.warnUnresolvableClickableRoles] or, for a native
// entry addressed to another platform, to `neru roles` / `neru doctor`.
func validateClickableRoles(field string, roles []string) error {
	resolution := element.ResolveRolesForCurrentPlatform(roles)

	messages := resolution.FatalMessages()
	if len(messages) == 0 {
		return nil
	}

	return derrors.Newf(
		derrors.CodeInvalidConfig,
		"%s: %s (run `neru roles` for the accepted role names)",
		field,
		strings.Join(messages, "; "),
	)
}

// warnUnresolvableClickableRoles reports the configured roles that name nothing
// this platform's accessibility vocabulary can express, so `neru config
// validate` stops calling a configuration valid when part of it draws no hints
// at all (ADR 0002's tier, ADR 0008).
//
// Such an entry is not refused — a list written on a Mac and carried to a
// Windows machine is the shape the semantic vocabulary exists to allow, and
// refusing it would replace the user's whole configuration with the defaults
// over a line that costs them one role. Entries addressed to another platform's
// vocabulary ("atspi:push button" on macOS) stay silent for the same reason:
// those are the other machine's lines and are expected to be there.
//
// The shipped list is exempt, and that gate is the point of this pass rather
// than a detail of it. hints.clickable_roles ships one list for every platform
// and several of its roles have no Linux or Windows counterpart, so reporting
// them would greet every user of two platforms with a complaint about a file
// they never wrote. A list that differs from the shipped one, on the other
// hand, is a list the user wrote out in full — the option replaces rather than
// merges — so every entry in it is a line they can go and edit.
//
// The blind spot the gate buys is a user who types the shipped list out
// verbatim and is told nothing. That is the same trade #1281 named for the
// derived grid labels: comparing against what the default would be is right for
// the common case and knowingly wrong for the one person who wrote it by hand.
// Unlike the labels there is no written configuration to ask instead — the
// option's default is a list, not the blank that means "infer" — so the
// comparison is the answer rather than a fallback.
//
// An application's extra roles are never shipped, so there is nothing to
// compare a list of those against: every entry in one is the user's own.
func (c *Config) warnUnresolvableClickableRoles(warnings *Warnings, goos string) {
	if !clickableRolesAreShipped(c.Hints.ClickableRoles) {
		warnUnresolvableRoles(warnings, fieldClickableRoles, c.Hints.ClickableRoles, goos)
	}

	for _, appConfig := range c.Hints.AppConfigs {
		warnUnresolvableRoles(
			warnings,
			fieldAdditionalClickableRoles,
			appConfig.AdditionalClickable,
			goos,
		)
	}
}

// clickableRolesAreShipped reports whether a role list is the one neru ships,
// which is the list a user who has never touched the option is running.
func clickableRolesAreShipped(roles []string) bool {
	return slices.Equal(roles, DefaultConfig().Hints.ClickableRoles)
}

// warnUnresolvableRoles reports the entries of one role list that this
// platform's vocabulary has no name for, one warning each so a list with two
// dead entries does not hide one behind the other.
func warnUnresolvableRoles(warnings *Warnings, field string, roles []string, goos string) {
	for _, diagnostic := range element.ResolveRoles(roles, goos).Diagnostics {
		if diagnostic.Kind != element.RoleDiagnosticNoNativeEquivalent {
			continue
		}

		warnings.Addf("%s: %s", field, diagnostic.Message())
	}
}

// ValidateHints checks the hints configuration, reporting the first problem it
// finds and collecting, into warnings, the parts of it that load and will draw
// no hints. A nil sink discards them; see [Warnings].
//
// The order the checks run in is the order a user sees their mistakes, so it is
// listed here rather than left implicit in one long body. It is also why the
// search-input checks come in two parts with the boundary-highlight checks
// between them: that is where they have always run, and moving them would change
// which problem a config with several is told about first.
func (c *Config) ValidateHints(warnings *Warnings) error {
	checks := []func() error{
		func() error { return c.validateHintClickableRoles(warnings) },
		c.validateHintCharacters,
		c.validateHintColors,
		c.validateHintLabelUI,
		c.validateHintSearchInputGeometry,
		c.validateHintBoundaryHighlight,
		c.validateHintSearchInputPlacement,
		c.validateHintScanDepth,
		c.validateHintMissionControl,
		c.validateHintVocabulary,
		func() error { return validateHintsVisionConfig(c.Hints.Vision) },
	}

	for _, check := range checks {
		checkErr := check()
		if checkErr != nil {
			return checkErr
		}
	}

	return nil
}

// validateHintClickableRoles checks the roles hints are drawn for, including the
// extra roles an individual application adds.
//
// The refusals run first and to the end: an entry naming no role at all is the
// whole file's problem, and a configuration that is going to be replaced by the
// defaults has nothing to be advised about.
func (c *Config) validateHintClickableRoles(warnings *Warnings) error {
	if c.Hints.Enabled && len(c.Hints.ClickableRoles) == 0 {
		return derrors.New(derrors.CodeInvalidConfig,
			"hints.clickable_roles cannot be empty when hints are enabled")
	}

	rolesErr := validateClickableRoles(fieldClickableRoles, c.Hints.ClickableRoles)
	if rolesErr != nil {
		return rolesErr
	}

	for _, appConfig := range c.Hints.AppConfigs {
		appRolesErr := validateClickableRoles(
			fieldAdditionalClickableRoles,
			appConfig.AdditionalClickable,
		)
		if appRolesErr != nil {
			return appRolesErr
		}
	}

	c.warnUnresolvableClickableRoles(warnings, runtime.GOOS)

	return nil
}

// validateHintCharacters checks the alphabet hint labels are drawn from.
//
// Labels are typed, so the alphabet has to be typeable and unambiguous: at least
// two characters to build labels out of, ASCII so every keyboard can produce
// them, and no character twice once case is folded, since matching is
// case-insensitive and a repeat would make two labels indistinguishable.
func (c *Config) validateHintCharacters() error {
	if strings.TrimSpace(c.Hints.HintCharacters) == "" {
		return derrors.New(derrors.CodeInvalidConfig, "hint_characters cannot be empty")
	}

	if len(c.Hints.HintCharacters) < MinCharactersLength {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hint_characters must contain at least 2 characters",
		)
	}

	for _, char := range c.Hints.HintCharacters {
		if char > unicode.MaxASCII {
			return derrors.New(
				derrors.CodeInvalidConfig,
				"hint_characters can only contain ASCII characters",
			)
		}
	}

	seen := make(map[rune]struct{}, len(c.Hints.HintCharacters))

	for _, char := range c.Hints.HintCharacters {
		upper := unicode.ToUpper(char)

		_, duplicate := seen[upper]
		if duplicate {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"hint_characters contains duplicate character %q",
				char,
			)
		}

		seen[upper] = struct{}{}
	}

	return nil
}

// validateHintColors checks every color the hints overlay draws with.
func (c *Config) validateHintColors() error {
	return validateColors([]colorField{
		{c.Hints.UI.BackgroundColor, "hints.ui.background_color"},
		{c.Hints.UI.TextColor, "hints.ui.text_color"},
		{c.Hints.UI.MatchedTextColor, "hints.ui.matched_text_color"},
		{c.Hints.UI.BorderColor, "hints.ui.border_color"},
		{c.Hints.SearchInputUI.BackgroundColor, "hints.search_input_ui.background_color"},
		{c.Hints.SearchInputUI.TextColor, "hints.search_input_ui.text_color"},
		{c.Hints.SearchInputUI.BorderColor, "hints.search_input_ui.border_color"},
		{c.Hints.BoundaryHighlight.BackgroundColor, "hints.boundary_highlight.background_color"},
		{c.Hints.BoundaryHighlight.BorderColor, "hints.boundary_highlight.border_color"},
	})
}

// validateHintLabelUI checks the size and shape of a hint label.
//
// It also settles the placement of the label relative to its element, defaulting
// an unset placement rather than refusing it.
func (c *Config) validateHintLabelUI() error {
	if c.Hints.UI.FontSize < 1 || c.Hints.UI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	radiusErr := validateMinValue(c.Hints.UI.BorderRadius, -1, "hints.ui.border_radius")
	if radiusErr != nil {
		return radiusErr
	}

	paddingXErr := validateMinValue(c.Hints.UI.PaddingX, -1, "hints.ui.padding_x")
	if paddingXErr != nil {
		return paddingXErr
	}

	paddingYErr := validateMinValue(c.Hints.UI.PaddingY, -1, "hints.ui.padding_y")
	if paddingYErr != nil {
		return paddingYErr
	}

	if c.Hints.UI.BorderWidth < 0 {
		return derrors.New(derrors.CodeInvalidConfig, "hints.ui.border_width must be non-negative")
	}

	return c.settleHintPlacement()
}

// settleHintPlacement defaults an unset hints.ui.placement and refuses any
// value the vocabulary does not name. Both the accepted set and the error
// message read HintPlacements(), so a placement added there is accepted here
// without this function being touched.
func (c *Config) settleHintPlacement() error {
	if c.Hints.UI.Placement == "" {
		c.Hints.UI.Placement = HintPlacementDefault

		return nil
	}

	placements := HintPlacements()
	if slices.Contains(placements, c.Hints.UI.Placement) {
		return nil
	}

	return derrors.Newf(
		derrors.CodeInvalidConfig,
		"hints.ui.placement must be one of %s",
		strings.Join(placements, ", "),
	)
}

// validateHintSearchInputGeometry checks the size and shape of the search input.
// Where it sits on screen is checked separately, after the boundary highlight.
func (c *Config) validateHintSearchInputGeometry() error {
	if c.Hints.SearchInputUI.FontSize < 1 || c.Hints.SearchInputUI.FontSize > maxFontSize {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.search_input_ui.font_size must be between 1 and %d",
			maxFontSize,
		)
	}

	radiusErr := validateMinValue(
		c.Hints.SearchInputUI.BorderRadius,
		-1,
		"hints.search_input_ui.border_radius",
	)
	if radiusErr != nil {
		return radiusErr
	}

	paddingXErr := validateMinValue(
		c.Hints.SearchInputUI.PaddingX,
		-1,
		"hints.search_input_ui.padding_x",
	)
	if paddingXErr != nil {
		return paddingXErr
	}

	paddingYErr := validateMinValue(
		c.Hints.SearchInputUI.PaddingY,
		-1,
		"hints.search_input_ui.padding_y",
	)
	if paddingYErr != nil {
		return paddingYErr
	}

	if c.Hints.SearchInputUI.BorderWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.search_input_ui.border_width must be non-negative",
		)
	}

	return nil
}

// validateHintBoundaryHighlight checks the outline drawn around the element a
// hint points at.
func (c *Config) validateHintBoundaryHighlight() error {
	if c.Hints.BoundaryHighlight.BorderWidth < 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.boundary_highlight.border_width must be non-negative",
		)
	}

	return validateMinValue(
		c.Hints.BoundaryHighlight.BorderRadius,
		-1,
		"hints.boundary_highlight.border_radius",
	)
}

// validateHintSearchInputPlacement checks where the search input sits and how
// wide it is. Both the accepted set and the error message read
// SearchInputPositions(), so a position added there is accepted here — and
// named in the message — without this function being touched.
func (c *Config) validateHintSearchInputPlacement() error {
	positions := SearchInputPositions()
	if !slices.Contains(positions, c.Hints.SearchInputUI.Position) {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.search_input_ui.position must be one of %s",
			strings.Join(positions, ", "),
		)
	}

	return validateMinValue(c.Hints.SearchInputUI.Width, 1, "hints.search_input_ui.width")
}

// validateHintScanDepth checks how deep the accessibility tree walk may go.
func (c *Config) validateHintScanDepth() error {
	return validateMinValue(c.Hints.MaxDepth, 0, "hints.max_depth")
}

// validateHintMissionControl checks the steps run as Mission Control opens and
// closes, and the settings they depend on.
func (c *Config) validateHintMissionControl() error {
	if (len(c.Hints.OnMissionControlActivated) > 0 ||
		len(c.Hints.OnMissionControlDeactivated) > 0) &&
		!c.Hints.DetectMissionControl {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.on_mission_control_activated/deactivated requires hints.detect_mission_control = true",
		)
	}

	if c.Hints.DetectMissionControl && !c.Hints.IncludeDockHints {
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.detect_mission_control requires hints.include_dock_hints = true "+
				"(dock windows are the only element source available during Mission Control)",
		)
	}

	activatedErr := validateMissionControlSteps(
		"hints.on_mission_control_activated",
		c.Hints.OnMissionControlActivated,
	)
	if activatedErr != nil {
		return activatedErr
	}

	return validateMissionControlSteps(
		"hints.on_mission_control_deactivated",
		c.Hints.OnMissionControlDeactivated,
	)
}

// validateMissionControlSteps checks one list of Mission Control steps.
func validateMissionControlSteps(field string, steps []string) error {
	for idx, actionStr := range steps {
		trimmed := strings.TrimSpace(actionStr)
		if trimmed == "" {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s[%d] cannot be empty",
				field,
				idx,
			)
		}

		stepErr := validateHotkeyActionString(trimmed)
		if stepErr != nil {
			return derrors.Newf(
				derrors.CodeInvalidConfig,
				"%s[%d]: %v",
				field,
				idx,
				stepErr,
			)
		}
	}

	return nil
}

// validateHintVocabulary checks the settings that name one of a fixed set of
// behaviors: how elements are found, and how labels are enumerated.
func (c *Config) validateHintVocabulary() error {
	switch c.Hints.Strategy {
	case domain.StrategyAXTree, domain.StrategyVision, "":
	default:
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.strategy must be %q or %q",
			domain.StrategyAXTree, domain.StrategyVision,
		)
	}

	switch c.Hints.LabelDirection {
	case domain.LabelDirectionReverse, domain.LabelDirectionNormal, "":
	default:
		return derrors.Newf(
			derrors.CodeInvalidConfig,
			"hints.label_direction must be %q or %q",
			domain.LabelDirectionReverse, domain.LabelDirectionNormal,
		)
	}

	return nil
}

func validateHintsVisionConfig(vision HintsVisionConfig) error {
	if !vision.DetectText && !vision.DetectRectangles {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.vision must enable detect_text or detect_rectangles",
		)
	}

	if vision.RequestTimeoutMS <= 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.vision.request_timeout_ms must be greater than 0",
		)
	}

	if vision.RectangleMaxCandidates <= 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.vision.rectangle_max_candidates must be greater than 0",
		)
	}

	err := validateUnitFloat(
		"hints.vision.minimum_confidence",
		vision.MinimumConfidence,
	)
	if err != nil {
		return err
	}

	err = validatePositiveUnitFloat(
		"hints.vision.merge_iou_threshold",
		vision.MergeIOUThreshold,
	)
	if err != nil {
		return err
	}

	err = validateUnitFloat(
		"hints.vision.rectangle_min_size",
		vision.RectangleMinSize,
	)
	if err != nil {
		return err
	}

	err = validatePositiveUnitFloat(
		"hints.vision.button_min_confidence",
		vision.ButtonMinConfidence,
	)
	if err != nil {
		return err
	}

	err = validatePositiveUnitFloat(
		"hints.vision.generic_clickable_min_confidence",
		vision.GenericClickableMinConfidence,
	)
	if err != nil {
		return err
	}

	if vision.RectangleMinAspect <= 0 || vision.RectangleMaxAspect <= 0 ||
		vision.RectangleMinAspect > vision.RectangleMaxAspect {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.vision rectangle aspect limits must be > 0 and min <= max",
		)
	}

	if vision.ButtonMinAspect <= 0 || vision.ButtonMaxAspect <= 0 ||
		vision.ButtonMinAspect > vision.ButtonMaxAspect {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.vision button aspect limits must be > 0 and min <= max",
		)
	}

	if vision.ButtonIconMaxSize <= 0 || vision.LinkMaxHeight <= 0 ||
		vision.LinkMinWidth <= 0 || vision.ImageMinSize <= 0 ||
		vision.CheckboxMaxSize <= 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.vision size thresholds must be greater than 0",
		)
	}

	if vision.LinkMinAspect <= 0 {
		return derrors.New(
			derrors.CodeInvalidConfig,
			"hints.vision.link_min_aspect must be greater than 0",
		)
	}

	return nil
}
