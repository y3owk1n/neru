package config

import (
	"testing"

	"github.com/y3owk1n/neru/internal/domain"
)

// placementSideways is a placement no validator accepts.
const placementSideways = "sideways"

func badColor() Color {
	return Color{Light: "nope", Dark: "nope"}
}

// hintsCase is one config and the report it has always produced.
type hintsCase struct {
	name string

	// breakIt introduces one fault, or several where the name says so.
	breakIt func(*Config)

	// message is the report, empty when the config is accepted.
	message string

	// placement is read after validation, since validation may set it.
	placement string
}

func hintsCases() []hintsCase {
	return []hintsCase{
		{
			name:      "clean",
			breakIt:   func(*Config) {},
			message:   "",
			placement: HintPlacementBottom,
		},
		{
			name:      "no clickable roles",
			breakIt:   func(c *Config) { c.Hints.ClickableRoles = nil },
			message:   "[INVALID_CONFIG] hints.clickable_roles cannot be empty when hints are enabled",
			placement: HintPlacementBottom,
		},
		{
			name:      "bad clickable role",
			breakIt:   func(c *Config) { c.Hints.ClickableRoles = []string{"!!"} },
			message:   "[INVALID_CONFIG] hints.clickable_roles: unknown role \"!!\" (run `neru roles` for the accepted role names)",
			placement: HintPlacementBottom,
		},
		{
			name:      "empty hint chars",
			breakIt:   func(c *Config) { c.Hints.HintCharacters = "  " },
			message:   "[INVALID_CONFIG] hint_characters cannot be empty",
			placement: HintPlacementBottom,
		},
		{
			name:      "one hint char",
			breakIt:   func(c *Config) { c.Hints.HintCharacters = "a" },
			message:   "[INVALID_CONFIG] hint_characters must contain at least 2 characters",
			placement: HintPlacementBottom,
		},
		{
			name:      "non-ascii hint chars",
			breakIt:   func(c *Config) { c.Hints.HintCharacters = "a\u00e9" },
			message:   "[INVALID_CONFIG] hint_characters can only contain ASCII characters",
			placement: HintPlacementBottom,
		},
		{
			name:      "duplicate hint chars",
			breakIt:   func(c *Config) { c.Hints.HintCharacters = "aA" },
			message:   "[INVALID_CONFIG] hint_characters contains duplicate character 'A'",
			placement: HintPlacementBottom,
		},
		{
			name:      "bad ui color",
			breakIt:   func(c *Config) { c.Hints.UI.BackgroundColor = badColor() },
			message:   "[INVALID_CONFIG] hints.ui.background_color (light) has invalid color format: nope",
			placement: HintPlacementBottom,
		},
		{
			name:      "bad search color",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.BackgroundColor = badColor() },
			message:   "[INVALID_CONFIG] hints.search_input_ui.background_color (light) has invalid color format: nope",
			placement: HintPlacementBottom,
		},
		{
			name:      "bad boundary color",
			breakIt:   func(c *Config) { c.Hints.BoundaryHighlight.BorderColor = badColor() },
			message:   "[INVALID_CONFIG] hints.boundary_highlight.border_color (light) has invalid color format: nope",
			placement: HintPlacementBottom,
		},
		{
			name:      "font size zero",
			breakIt:   func(c *Config) { c.Hints.UI.FontSize = 0 },
			message:   "[INVALID_CONFIG] hints.ui.font_size must be between 1 and 2147483647",
			placement: HintPlacementBottom,
		},
		{
			name:      "font size huge",
			breakIt:   func(c *Config) { c.Hints.UI.FontSize = 100000 },
			message:   "",
			placement: HintPlacementBottom,
		},
		{
			name:      "negative border radius",
			breakIt:   func(c *Config) { c.Hints.UI.BorderRadius = -5 },
			message:   "[INVALID_CONFIG] hints.ui.border_radius must be at least -1",
			placement: HintPlacementBottom,
		},
		{
			name:      "negative padding x",
			breakIt:   func(c *Config) { c.Hints.UI.PaddingX = -5 },
			message:   "[INVALID_CONFIG] hints.ui.padding_x must be at least -1",
			placement: HintPlacementBottom,
		},
		{
			name:      "negative padding y",
			breakIt:   func(c *Config) { c.Hints.UI.PaddingY = -5 },
			message:   "[INVALID_CONFIG] hints.ui.padding_y must be at least -1",
			placement: HintPlacementBottom,
		},
		{
			name:      "negative border width",
			breakIt:   func(c *Config) { c.Hints.UI.BorderWidth = -1 },
			message:   "[INVALID_CONFIG] hints.ui.border_width must be non-negative",
			placement: HintPlacementBottom,
		},
		{
			name:      "bad placement",
			breakIt:   func(c *Config) { c.Hints.UI.Placement = placementSideways },
			message:   "[INVALID_CONFIG] hints.ui.placement must be one of top, center, bottom",
			placement: "sideways",
		},
		{
			name:      "empty placement defaults",
			breakIt:   func(c *Config) { c.Hints.UI.Placement = "" },
			message:   "",
			placement: HintPlacementBottom,
		},
		{
			name:      "search font size zero",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.FontSize = 0 },
			message:   "[INVALID_CONFIG] hints.search_input_ui.font_size must be between 1 and 2147483647",
			placement: HintPlacementBottom,
		},
		{
			name:      "search negative radius",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.BorderRadius = -5 },
			message:   "[INVALID_CONFIG] hints.search_input_ui.border_radius must be at least -1",
			placement: HintPlacementBottom,
		},
		{
			name:      "search negative padding x",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.PaddingX = -5 },
			message:   "[INVALID_CONFIG] hints.search_input_ui.padding_x must be at least -1",
			placement: HintPlacementBottom,
		},
		{
			name:      "search negative padding y",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.PaddingY = -5 },
			message:   "[INVALID_CONFIG] hints.search_input_ui.padding_y must be at least -1",
			placement: HintPlacementBottom,
		},
		{
			name:      "search negative border width",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.BorderWidth = -1 },
			message:   "[INVALID_CONFIG] hints.search_input_ui.border_width must be non-negative",
			placement: HintPlacementBottom,
		},
		{
			name:      "boundary negative border width",
			breakIt:   func(c *Config) { c.Hints.BoundaryHighlight.BorderWidth = -1 },
			message:   "[INVALID_CONFIG] hints.boundary_highlight.border_width must be non-negative",
			placement: HintPlacementBottom,
		},
		{
			name:      "boundary negative radius",
			breakIt:   func(c *Config) { c.Hints.BoundaryHighlight.BorderRadius = -5 },
			message:   "[INVALID_CONFIG] hints.boundary_highlight.border_radius must be at least -1",
			placement: HintPlacementBottom,
		},
		{
			name:      "bad search position",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.Position = "nowhere" },
			message:   "[INVALID_CONFIG] hints.search_input_ui.position must be one of top_left, top_center, top_right, center, bottom_left, bottom_center, bottom_right",
			placement: HintPlacementBottom,
		},
		{
			name:      "search width zero",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.Width = 0 },
			message:   "[INVALID_CONFIG] hints.search_input_ui.width must be at least 1",
			placement: HintPlacementBottom,
		},
		{
			name:      "negative max depth",
			breakIt:   func(c *Config) { c.Hints.MaxDepth = -1 },
			message:   "[INVALID_CONFIG] hints.max_depth must be at least 0",
			placement: HintPlacementBottom,
		},
		{
			name: "mc actions without detect",
			breakIt: func(c *Config) {
				c.Hints.DetectMissionControl = false
				c.Hints.OnMissionControlActivated = []string{"hints"}
			},
			message:   "[INVALID_CONFIG] hints.on_mission_control_activated/deactivated requires hints.detect_mission_control = true",
			placement: HintPlacementBottom,
		},
		{
			name: "detect without dock",
			breakIt: func(c *Config) {
				c.Hints.DetectMissionControl = true
				c.Hints.IncludeDockHints = false
			},
			message:   "[INVALID_CONFIG] hints.detect_mission_control requires hints.include_dock_hints = true (dock windows are the only element source available during Mission Control)",
			placement: HintPlacementBottom,
		},
		{
			name: "empty mc activated step",
			breakIt: func(c *Config) {
				c.Hints.DetectMissionControl = true
				c.Hints.IncludeDockHints = true
				c.Hints.OnMissionControlActivated = []string{"  "}
			},
			message:   "[INVALID_CONFIG] hints.on_mission_control_activated[0] cannot be empty",
			placement: HintPlacementBottom,
		},
		{
			name: "bad mc activated step",
			breakIt: func(c *Config) {
				c.Hints.DetectMissionControl = true
				c.Hints.IncludeDockHints = true
				c.Hints.OnMissionControlActivated = []string{"not_a_command"}
			},
			message:   "[INVALID_CONFIG] hints.on_mission_control_activated[0]: [INVALID_CONFIG] unknown command: not_a_command",
			placement: HintPlacementBottom,
		},
		{
			name: "empty mc deactivated step",
			breakIt: func(c *Config) {
				c.Hints.DetectMissionControl = true
				c.Hints.IncludeDockHints = true
				c.Hints.OnMissionControlDeactivated = []string{"  "}
			},
			message:   "[INVALID_CONFIG] hints.on_mission_control_deactivated[0] cannot be empty",
			placement: HintPlacementBottom,
		},
		{
			name:      "bad strategy",
			breakIt:   func(c *Config) { c.Hints.Strategy = "telepathy" },
			message:   "[INVALID_CONFIG] hints.strategy must be \"axtree\", \"vision\", or \"contour\"",
			placement: HintPlacementBottom,
		},
		{
			name:      "bad capture scope",
			breakIt:   func(c *Config) { c.Hints.CaptureScope = "desk" },
			message:   "[INVALID_CONFIG] hints.capture_scope must be \"window\" or \"screen\"",
			placement: HintPlacementBottom,
		},
		{
			name:      "bad label direction",
			breakIt:   func(c *Config) { c.Hints.LabelDirection = placementSideways },
			message:   "[INVALID_CONFIG] hints.label_direction must be \"reverse\" or \"normal\"",
			placement: HintPlacementBottom,
		},
		{
			name: "vision detects nothing",
			breakIt: func(c *Config) {
				c.Hints.Strategy = domain.StrategyVision
				c.Hints.Vision.DetectText = false
				c.Hints.Vision.DetectRectangles = false
			},
			message:   "[INVALID_CONFIG] hints.vision must enable detect_text or detect_rectangles",
			placement: HintPlacementBottom,
		},
		{
			name: "bad boundary width AND bad search position",
			breakIt: func(c *Config) {
				c.Hints.BoundaryHighlight.BorderWidth = -1
				c.Hints.SearchInputUI.Position = "nowhere"
			},
			message:   "[INVALID_CONFIG] hints.boundary_highlight.border_width must be non-negative",
			placement: HintPlacementBottom,
		},
		{
			name: "bad search border width AND bad boundary width",
			breakIt: func(c *Config) {
				c.Hints.SearchInputUI.BorderWidth = -1
				c.Hints.BoundaryHighlight.BorderWidth = -1
			},
			message:   "[INVALID_CONFIG] hints.search_input_ui.border_width must be non-negative",
			placement: HintPlacementBottom,
		},
		{
			name: "bad chars AND bad color",
			breakIt: func(c *Config) {
				c.Hints.HintCharacters = "a"
				c.Hints.UI.BackgroundColor = badColor()
			},
			message:   "[INVALID_CONFIG] hint_characters must contain at least 2 characters",
			placement: HintPlacementBottom,
		},
		{
			name: "bad strategy AND bad label direction",
			breakIt: func(c *Config) {
				c.Hints.Strategy = "telepathy"
				c.Hints.LabelDirection = placementSideways
			},
			message:   "[INVALID_CONFIG] hints.strategy must be \"axtree\", \"vision\", or \"contour\"",
			placement: HintPlacementBottom,
		},
	}
}

// TestValidateHints_AcceptsEverySearchInputPosition holds the validator to the
// vocabulary rather than to a list of its own: every value
// SearchInputPositions() names has to validate, so a position added there and
// not to the check is caught here instead of at the draw.
func TestValidateHints_AcceptsEverySearchInputPosition(t *testing.T) {
	positions := SearchInputPositions()
	if len(positions) == 0 {
		t.Fatal("SearchInputPositions() is empty; there is no vocabulary to validate against")
	}

	for _, position := range positions {
		t.Run(position, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Hints.SearchInputUI.Position = position

			validateErr := cfg.ValidateHints(nil)
			if validateErr != nil {
				t.Errorf("ValidateHints() = %v, want nil for position %q", validateErr, position)
			}
		})
	}
}

// ValidateHints reports the first problem it finds, so which problem a user is
// told about depends on the order the checks run in. These cases record the
// message for one broken setting at a time, and for a few configs broken in more
// than one way at once — those last are what pin the order, including the
// boundary-highlight checks sitting between the two halves of the search-input
// checks.
//
// One case is not about failure at all: an unset placement is defaulted rather
// than refused, so validating a config quietly writes to it.
//
// badColor is a color no validator will accept.
func TestValidateHints_ReportsTheFirstProblem(t *testing.T) {
	for _, testCase := range hintsCases() {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := DefaultConfig()
			testCase.breakIt(cfg)

			validateErr := cfg.ValidateHints(nil)

			message := ""
			if validateErr != nil {
				message = validateErr.Error()
			}

			if message != testCase.message {
				t.Errorf("ValidateHints() = %q, want %q", message, testCase.message)
			}

			if cfg.Hints.UI.Placement != testCase.placement {
				t.Errorf("placement = %q, want %q after validation",
					cfg.Hints.UI.Placement, testCase.placement)
			}
		})
	}
}
