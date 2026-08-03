package config

import "testing"

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
			placement: placementBottom,
		},
		{
			name:      "no clickable roles",
			breakIt:   func(c *Config) { c.Hints.ClickableRoles = nil },
			message:   "[INVALID_CONFIG] hints.clickable_roles cannot be empty when hints are enabled",
			placement: placementBottom,
		},
		{
			name:      "bad clickable role",
			breakIt:   func(c *Config) { c.Hints.ClickableRoles = []string{"!!"} },
			message:   "[INVALID_CONFIG] hints.clickable_roles: unknown role \"!!\" (run `neru roles` for the accepted role names)",
			placement: placementBottom,
		},
		{
			name:      "empty hint chars",
			breakIt:   func(c *Config) { c.Hints.HintCharacters = "  " },
			message:   "[INVALID_CONFIG] hint_characters cannot be empty",
			placement: placementBottom,
		},
		{
			name:      "one hint char",
			breakIt:   func(c *Config) { c.Hints.HintCharacters = "a" },
			message:   "[INVALID_CONFIG] hint_characters must contain at least 2 characters",
			placement: placementBottom,
		},
		{
			name:      "non-ascii hint chars",
			breakIt:   func(c *Config) { c.Hints.HintCharacters = "a\u00e9" },
			message:   "[INVALID_CONFIG] hint_characters can only contain ASCII characters",
			placement: placementBottom,
		},
		{
			name:      "duplicate hint chars",
			breakIt:   func(c *Config) { c.Hints.HintCharacters = "aA" },
			message:   "[INVALID_CONFIG] hint_characters contains duplicate character 'A'",
			placement: placementBottom,
		},
		{
			name:      "bad ui color",
			breakIt:   func(c *Config) { c.Hints.UI.BackgroundColor = badColor() },
			message:   "[INVALID_CONFIG] hints.ui.background_color (light) has invalid color format: nope",
			placement: placementBottom,
		},
		{
			name:      "bad search color",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.BackgroundColor = badColor() },
			message:   "[INVALID_CONFIG] hints.search_input_ui.background_color (light) has invalid color format: nope",
			placement: placementBottom,
		},
		{
			name:      "bad boundary color",
			breakIt:   func(c *Config) { c.Hints.BoundaryHighlight.BorderColor = badColor() },
			message:   "[INVALID_CONFIG] hints.boundary_highlight.border_color (light) has invalid color format: nope",
			placement: placementBottom,
		},
		{
			name:      "font size zero",
			breakIt:   func(c *Config) { c.Hints.UI.FontSize = 0 },
			message:   "[INVALID_CONFIG] hints.ui.font_size must be between 1 and 2147483647",
			placement: placementBottom,
		},
		{
			name:      "font size huge",
			breakIt:   func(c *Config) { c.Hints.UI.FontSize = 100000 },
			message:   "",
			placement: placementBottom,
		},
		{
			name:      "negative border radius",
			breakIt:   func(c *Config) { c.Hints.UI.BorderRadius = -5 },
			message:   "[INVALID_CONFIG] hints.ui.border_radius must be at least -1",
			placement: placementBottom,
		},
		{
			name:      "negative padding x",
			breakIt:   func(c *Config) { c.Hints.UI.PaddingX = -5 },
			message:   "[INVALID_CONFIG] hints.ui.padding_x must be at least -1",
			placement: placementBottom,
		},
		{
			name:      "negative padding y",
			breakIt:   func(c *Config) { c.Hints.UI.PaddingY = -5 },
			message:   "[INVALID_CONFIG] hints.ui.padding_y must be at least -1",
			placement: placementBottom,
		},
		{
			name:      "negative border width",
			breakIt:   func(c *Config) { c.Hints.UI.BorderWidth = -1 },
			message:   "[INVALID_CONFIG] hints.ui.border_width must be non-negative",
			placement: placementBottom,
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
			placement: placementBottom,
		},
		{
			name:      "search font size zero",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.FontSize = 0 },
			message:   "[INVALID_CONFIG] hints.search_input_ui.font_size must be between 1 and 2147483647",
			placement: placementBottom,
		},
		{
			name:      "search negative radius",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.BorderRadius = -5 },
			message:   "[INVALID_CONFIG] hints.search_input_ui.border_radius must be at least -1",
			placement: placementBottom,
		},
		{
			name:      "search negative padding x",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.PaddingX = -5 },
			message:   "[INVALID_CONFIG] hints.search_input_ui.padding_x must be at least -1",
			placement: placementBottom,
		},
		{
			name:      "search negative padding y",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.PaddingY = -5 },
			message:   "[INVALID_CONFIG] hints.search_input_ui.padding_y must be at least -1",
			placement: placementBottom,
		},
		{
			name:      "search negative border width",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.BorderWidth = -1 },
			message:   "[INVALID_CONFIG] hints.search_input_ui.border_width must be non-negative",
			placement: placementBottom,
		},
		{
			name:      "boundary negative border width",
			breakIt:   func(c *Config) { c.Hints.BoundaryHighlight.BorderWidth = -1 },
			message:   "[INVALID_CONFIG] hints.boundary_highlight.border_width must be non-negative",
			placement: placementBottom,
		},
		{
			name:      "boundary negative radius",
			breakIt:   func(c *Config) { c.Hints.BoundaryHighlight.BorderRadius = -5 },
			message:   "[INVALID_CONFIG] hints.boundary_highlight.border_radius must be at least -1",
			placement: placementBottom,
		},
		{
			name:      "bad search position",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.Position = "nowhere" },
			message:   "[INVALID_CONFIG] hints.search_input_ui.position must be one of top_left, top_center, top_right, center, bottom_left, bottom_center, bottom_right",
			placement: placementBottom,
		},
		{
			name:      "search width zero",
			breakIt:   func(c *Config) { c.Hints.SearchInputUI.Width = 0 },
			message:   "[INVALID_CONFIG] hints.search_input_ui.width must be at least 1",
			placement: placementBottom,
		},
		{
			name:      "negative max depth",
			breakIt:   func(c *Config) { c.Hints.MaxDepth = -1 },
			message:   "[INVALID_CONFIG] hints.max_depth must be at least 0",
			placement: placementBottom,
		},
		{
			name: "mc actions without detect",
			breakIt: func(c *Config) {
				c.Hints.DetectMissionControl = false
				c.Hints.OnMissionControlActivated = []string{"hints"}
			},
			message:   "[INVALID_CONFIG] hints.on_mission_control_activated/deactivated requires hints.detect_mission_control = true",
			placement: placementBottom,
		},
		{
			name: "detect without dock",
			breakIt: func(c *Config) {
				c.Hints.DetectMissionControl = true
				c.Hints.IncludeDockHints = false
			},
			message:   "[INVALID_CONFIG] hints.detect_mission_control requires hints.include_dock_hints = true (dock windows are the only element source available during Mission Control)",
			placement: placementBottom,
		},
		{
			name: "empty mc activated step",
			breakIt: func(c *Config) {
				c.Hints.DetectMissionControl = true
				c.Hints.IncludeDockHints = true
				c.Hints.OnMissionControlActivated = []string{"  "}
			},
			message:   "[INVALID_CONFIG] hints.on_mission_control_activated[0] cannot be empty",
			placement: placementBottom,
		},
		{
			name: "bad mc activated step",
			breakIt: func(c *Config) {
				c.Hints.DetectMissionControl = true
				c.Hints.IncludeDockHints = true
				c.Hints.OnMissionControlActivated = []string{"not_a_command"}
			},
			message:   "[INVALID_CONFIG] hints.on_mission_control_activated[0]: [INVALID_CONFIG] unknown command: not_a_command",
			placement: placementBottom,
		},
		{
			name: "empty mc deactivated step",
			breakIt: func(c *Config) {
				c.Hints.DetectMissionControl = true
				c.Hints.IncludeDockHints = true
				c.Hints.OnMissionControlDeactivated = []string{"  "}
			},
			message:   "[INVALID_CONFIG] hints.on_mission_control_deactivated[0] cannot be empty",
			placement: placementBottom,
		},
		{
			name:      "bad strategy",
			breakIt:   func(c *Config) { c.Hints.Strategy = "telepathy" },
			message:   "[INVALID_CONFIG] hints.strategy must be \"axtree\" or \"vision\"",
			placement: placementBottom,
		},
		{
			name:      "bad label direction",
			breakIt:   func(c *Config) { c.Hints.LabelDirection = placementSideways },
			message:   "[INVALID_CONFIG] hints.label_direction must be \"reverse\" or \"normal\"",
			placement: placementBottom,
		},
		{
			name: "vision detects nothing",
			breakIt: func(c *Config) {
				c.Hints.Strategy = StrategyVision
				c.Hints.Vision.DetectText = false
				c.Hints.Vision.DetectRectangles = false
			},
			message:   "[INVALID_CONFIG] hints.vision must enable detect_text or detect_rectangles",
			placement: placementBottom,
		},
		{
			name: "bad boundary width AND bad search position",
			breakIt: func(c *Config) {
				c.Hints.BoundaryHighlight.BorderWidth = -1
				c.Hints.SearchInputUI.Position = "nowhere"
			},
			message:   "[INVALID_CONFIG] hints.boundary_highlight.border_width must be non-negative",
			placement: placementBottom,
		},
		{
			name: "bad search border width AND bad boundary width",
			breakIt: func(c *Config) {
				c.Hints.SearchInputUI.BorderWidth = -1
				c.Hints.BoundaryHighlight.BorderWidth = -1
			},
			message:   "[INVALID_CONFIG] hints.search_input_ui.border_width must be non-negative",
			placement: placementBottom,
		},
		{
			name: "bad chars AND bad color",
			breakIt: func(c *Config) {
				c.Hints.HintCharacters = "a"
				c.Hints.UI.BackgroundColor = badColor()
			},
			message:   "[INVALID_CONFIG] hint_characters must contain at least 2 characters",
			placement: placementBottom,
		},
		{
			name: "bad strategy AND bad label direction",
			breakIt: func(c *Config) {
				c.Hints.Strategy = "telepathy"
				c.Hints.LabelDirection = placementSideways
			},
			message:   "[INVALID_CONFIG] hints.strategy must be \"axtree\" or \"vision\"",
			placement: placementBottom,
		},
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

			validateErr := cfg.ValidateHints()

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
