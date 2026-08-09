package config_test

import (
	"math"
	"testing"

	"github.com/y3owk1n/neru/internal/config"
	"github.com/y3owk1n/neru/internal/derrors"
)

// intBound describes an integer config field with an inclusive lower limit.
type intBound struct {
	// name is the TOML path, used only for failure messages.
	name string
	// set writes the field on a config.
	set func(*config.Config, int)
	// validate runs the validator that owns the field.
	validate func(*config.Config) error
	// minValid is the smallest accepted value.
	minValid int
	// maxValid is the largest accepted value, or 0 when unbounded above.
	maxValid int
	// enable turns on the feature that owns the field. Several validators
	// return early for a disabled feature — deliberately, so a user can leave
	// garbage in a section they do not use — so the boundary is only reachable
	// with the feature switched on.
	enable func(*config.Config)
}

// fontSizeBounds covers the font-size fields, all of which share the same
// `< 1 || > maxFontSize` guard.
func fontSizeBounds() []intBound {
	return []intBound{
		{
			name:     "hints.ui.font_size",
			set:      func(c *config.Config, v int) { c.Hints.UI.FontSize = v },
			validate: func(c *config.Config) error { return c.ValidateHints(nil) },
			minValid: 1, maxValid: math.MaxInt32,
		},
		{
			name:     "hints.search_input_ui.font_size",
			set:      func(c *config.Config, v int) { c.Hints.SearchInputUI.FontSize = v },
			validate: func(c *config.Config) error { return c.ValidateHints(nil) },
			minValid: 1, maxValid: math.MaxInt32,
		},
		{
			name:     "grid.ui.font_size",
			set:      func(c *config.Config, v int) { c.Grid.UI.FontSize = v },
			validate: func(c *config.Config) error { return c.ValidateGrid(nil, config.WrittenConfig{}) },
			minValid: 1,
			maxValid: math.MaxInt32,
			enable:   func(c *config.Config) { c.Grid.Enabled = true },
		},
		{
			name:     "monitor_select.ui.font_size",
			set:      func(c *config.Config, v int) { c.MonitorSelect.UI.FontSize = v },
			validate: (*config.Config).ValidateMonitorSelect,
			minValid: 1, maxValid: math.MaxInt32,
			enable: func(c *config.Config) { c.MonitorSelect.Enabled = true },
		},
		{
			name:     "monitor_select.ui.subtitle_font_size",
			set:      func(c *config.Config, v int) { c.MonitorSelect.UI.SubtitleFontSize = v },
			validate: (*config.Config).ValidateMonitorSelect,
			minValid: 1, maxValid: math.MaxInt32,
			enable: func(c *config.Config) { c.MonitorSelect.Enabled = true },
		},
		{
			name:     "sticky_modifiers.ui.font_size",
			set:      func(c *config.Config, v int) { c.StickyModifiers.UI.FontSize = v },
			validate: (*config.Config).ValidateStickyModifiers,
			minValid: 1, maxValid: math.MaxInt32,
			enable: func(c *config.Config) { c.StickyModifiers.Enabled = true },
		},
		{
			name:     "recursive_grid.ui.font_size",
			set:      func(c *config.Config, v int) { c.RecursiveGrid.UI.FontSize = v },
			validate: (*config.Config).ValidateRecursiveGrid,
			minValid: 1, maxValid: math.MaxInt32,
			enable: func(c *config.Config) { c.RecursiveGrid.Enabled = true },
		},
		{
			name:     "recursive_grid.ui.sub_key_preview_font_size",
			set:      func(c *config.Config, v int) { c.RecursiveGrid.UI.SubKeyPreviewFontSize = v },
			validate: (*config.Config).ValidateRecursiveGrid,
			minValid: 1,
			maxValid: math.MaxInt32,
			enable:   func(c *config.Config) { c.RecursiveGrid.Enabled = true },
		},
	}
}

// visionIntBounds covers the vision thresholds guarded by `<= 0`.
func visionIntBounds() []intBound {
	return []intBound{
		{
			name:     "hints.vision.request_timeout_ms",
			set:      func(c *config.Config, v int) { c.Hints.Vision.RequestTimeoutMS = v },
			validate: func(c *config.Config) error { return c.ValidateHints(nil) },
			minValid: 1,
		},
		{
			name:     "hints.vision.rectangle_max_candidates",
			set:      func(c *config.Config, v int) { c.Hints.Vision.RectangleMaxCandidates = v },
			validate: func(c *config.Config) error { return c.ValidateHints(nil) },
			minValid: 1,
		},
	}
}

// smoothCursorIntBounds covers the smooth cursor integer fields with an
// inclusive lower limit. The relative duration is checked unconditionally —
// not gated on move_mouse_enabled — because it is read straight by the
// animator on the next relative move after a hot reload flips the toggle.
func smoothCursorIntBounds() []intBound {
	return []intBound{
		{
			name:     "smooth_cursor.relative_movement_duration",
			set:      func(c *config.Config, v int) { c.SmoothCursor.RelativeMovementDuration = v },
			validate: (*config.Config).ValidateSmoothCursor,
			minValid: config.MinSmoothCursorAnimationDuration,
		},
	}
}

func TestConfig_SmoothCursorRelativeBoundaries(t *testing.T) {
	runIntBounds(t, smoothCursorIntBounds())
}

// validBase returns a config that every validator accepts, so a failure in a
// table case can only come from the single field that case changed.
func validBase(t *testing.T) *config.Config {
	t.Helper()

	cfg := config.DefaultConfig()

	assertConfigValid(t, cfg, "the default config")

	return cfg
}

// assertConfigValid fails if any validator rejects cfg.
func assertConfigValid(t *testing.T, cfg *config.Config, what string) {
	t.Helper()

	validators := map[string]func(*config.Config) error{
		"ValidateHints":           func(c *config.Config) error { return c.ValidateHints(nil) },
		"ValidateGrid":            func(c *config.Config) error { return c.ValidateGrid(nil, config.WrittenConfig{}) },
		"ValidateMonitorSelect":   (*config.Config).ValidateMonitorSelect,
		"ValidateStickyModifiers": (*config.Config).ValidateStickyModifiers,
		"ValidateRecursiveGrid":   (*config.Config).ValidateRecursiveGrid,
		"ValidateVirtualPointer":  (*config.Config).ValidateVirtualPointer,
		"ValidateMouseAction":     (*config.Config).ValidateMouseAction,
		"ValidateSmoothCursor":    (*config.Config).ValidateSmoothCursor,
		"ValidateSmoothScroll":    (*config.Config).ValidateSmoothScroll,
		// Warnings are not what this asserts: the sink is nil so only a refusal
		// can fail the case.
		"ValidateHeldRepeat": func(c *config.Config) error { return c.ValidateHeldRepeat(nil) },
	}

	for name, validate := range validators {
		err := validate(cfg)
		if err != nil {
			t.Fatalf("%s rejected %s: %v", name, what, err)
		}
	}
}

// assertRejected requires an error carrying the invalid-config code, so a field
// that fails for an unrelated reason does not read as correct validation.
func assertRejected(t *testing.T, err error, field string, value any) {
	t.Helper()

	if err == nil {
		t.Errorf("%s = %v was accepted, want rejected", field, value)

		return
	}

	if code := derrors.GetCode(err); code != derrors.CodeInvalidConfig {
		t.Errorf("%s = %v rejected with code %q, want %q",
			field, value, code, derrors.CodeInvalidConfig)
	}
}

// runIntBounds asserts the accept/reject edge for each field in the table.
func runIntBounds(t *testing.T, bounds []intBound) {
	t.Helper()

	for _, bound := range bounds {
		t.Run(bound.name, func(t *testing.T) {
			newCfg := func() *config.Config {
				cfg := validBase(t)
				if bound.enable != nil {
					bound.enable(cfg)
				}

				return cfg
			}

			// One below the minimum must be rejected...
			cfg := newCfg()
			bound.set(cfg, bound.minValid-1)
			assertRejected(t, bound.validate(cfg), bound.name, bound.minValid-1)

			// ...as must clearly-out-of-range values.
			for _, value := range []int{-1, math.MinInt32} {
				if value >= bound.minValid {
					continue
				}

				cfg = newCfg()
				bound.set(cfg, value)
				assertRejected(t, bound.validate(cfg), bound.name, value)
			}

			// The minimum itself must be accepted — the common off-by-one here
			// silently forbids a legal setting.
			cfg = newCfg()
			bound.set(cfg, bound.minValid)

			err := bound.validate(cfg)
			if err != nil {
				t.Errorf("%s = %d (the minimum) was rejected: %v",
					bound.name, bound.minValid, err)
			}

			if bound.maxValid > 0 {
				cfg = newCfg()
				bound.set(cfg, bound.maxValid)

				err := bound.validate(cfg)
				if err != nil {
					t.Errorf("%s = %d (the maximum) was rejected: %v",
						bound.name, bound.maxValid, err)
				}
			}
		})
	}
}

// Every numeric config field is guarded by a range check, and every one of
// those checks is a boundary the tests never exercised: the suite validated
// wholesale-valid and wholesale-garbage configs, but never the value one step
// either side of the limit.
//
// That is where these bugs live. A `< 1` that drifts to `<= 1` locks users out
// of a legal setting; a `<= 0` that drifts to `< 0` admits a zero timeout or a
// zero-size threshold that then divides by zero or spins at runtime. Neither
// changes anything a coarse valid/invalid test would notice.
//
// The tables below assert both directions for each field: the smallest legal
// value is accepted, and the largest illegal one is rejected.
func TestConfig_FontSizeBoundaries(t *testing.T) {
	runIntBounds(t, fontSizeBounds())
}

func TestConfig_VisionIntBoundaries(t *testing.T) {
	runIntBounds(t, visionIntBounds())
}

// unitFloatBound describes a float field constrained to a 0..1 confidence or
// ratio range. inclusiveZero distinguishes validateUnitFloat (accepts 0) from
// validatePositiveUnitFloat (rejects 0) — mixing the two up would let a
// confidence threshold of zero through, matching everything on screen.
type unitFloatBound struct {
	name          string
	set           func(*config.Config, float64)
	inclusiveZero bool
}

func unitFloatBounds() []unitFloatBound {
	return []unitFloatBound{
		{
			name:          "hints.vision.minimum_confidence",
			set:           func(c *config.Config, v float64) { c.Hints.Vision.MinimumConfidence = v },
			inclusiveZero: true,
		},
		{
			name:          "hints.vision.rectangle_min_size",
			set:           func(c *config.Config, v float64) { c.Hints.Vision.RectangleMinSize = v },
			inclusiveZero: true,
		},
		{
			name:          "hints.vision.merge_iou_threshold",
			set:           func(c *config.Config, v float64) { c.Hints.Vision.MergeIOUThreshold = v },
			inclusiveZero: false,
		},
		{
			name:          "hints.vision.button_min_confidence",
			set:           func(c *config.Config, v float64) { c.Hints.Vision.ButtonMinConfidence = v },
			inclusiveZero: false,
		},
		{
			name: "hints.vision.generic_clickable_min_confidence",
			set: func(c *config.Config, v float64) {
				c.Hints.Vision.GenericClickableMinConfidence = v
			},
			inclusiveZero: false,
		},
	}
}

// TestConfig_UnitFloatBoundaries pins the 0..1 range on every confidence and
// ratio field, including whether zero is legal for that particular field.
func TestConfig_UnitFloatBoundaries(t *testing.T) {
	for _, bound := range unitFloatBounds() {
		t.Run(bound.name, func(t *testing.T) {
			// Above the range and below it are always rejected.
			for _, value := range []float64{-0.001, -1, 1.001, 2} {
				cfg := validBase(t)
				bound.set(cfg, value)
				assertRejected(t, cfg.ValidateHints(nil), bound.name, value)
			}

			// 1.0 is the inclusive upper bound for both variants.
			cfg := validBase(t)
			bound.set(cfg, 1)

			err := cfg.ValidateHints(nil)
			if err != nil {
				t.Errorf("%s = 1 (the inclusive maximum) was rejected: %v", bound.name, err)
			}

			// Zero is where the two variants differ.
			cfg = validBase(t)
			bound.set(cfg, 0)

			err = cfg.ValidateHints(nil)
			if bound.inclusiveZero {
				if err != nil {
					t.Errorf("%s = 0 was rejected, but this field allows zero: %v", bound.name, err)
				}
			} else {
				assertRejected(t, err, bound.name, 0.0)
			}

			// A mid-range value is always fine.
			cfg = validBase(t)
			bound.set(cfg, 0.5)

			err = cfg.ValidateHints(nil)
			if err != nil {
				t.Errorf("%s = 0.5 was rejected: %v", bound.name, err)
			}
		})
	}
}

// aspectPair describes a min/max aspect-ratio pair validated together.
type aspectPair struct {
	name    string
	setMin  func(*config.Config, float64)
	setMax  func(*config.Config, float64)
	readMin func(*config.Config) float64
	readMax func(*config.Config) float64
}

func aspectPairs() []aspectPair {
	return []aspectPair{
		{
			name:    "hints.vision rectangle aspect",
			setMin:  func(c *config.Config, v float64) { c.Hints.Vision.RectangleMinAspect = v },
			setMax:  func(c *config.Config, v float64) { c.Hints.Vision.RectangleMaxAspect = v },
			readMin: func(c *config.Config) float64 { return c.Hints.Vision.RectangleMinAspect },
			readMax: func(c *config.Config) float64 { return c.Hints.Vision.RectangleMaxAspect },
		},
		{
			name:    "hints.vision button aspect",
			setMin:  func(c *config.Config, v float64) { c.Hints.Vision.ButtonMinAspect = v },
			setMax:  func(c *config.Config, v float64) { c.Hints.Vision.ButtonMaxAspect = v },
			readMin: func(c *config.Config) float64 { return c.Hints.Vision.ButtonMinAspect },
			readMax: func(c *config.Config) float64 { return c.Hints.Vision.ButtonMaxAspect },
		},
	}
}

// TestConfig_AspectPairBoundaries pins all three clauses of the aspect guard:
// both ends must be positive, and min must not exceed max. An inverted pair
// would silently match no rectangles at all.
func TestConfig_AspectPairBoundaries(t *testing.T) {
	for _, pair := range aspectPairs() {
		t.Run(pair.name, func(t *testing.T) {
			// A non-positive minimum is rejected.
			for _, value := range []float64{0, -0.5} {
				cfg := validBase(t)
				pair.setMin(cfg, value)
				assertRejected(t, cfg.ValidateHints(nil), pair.name+" min", value)
			}

			// A non-positive maximum is rejected.
			for _, value := range []float64{0, -0.5} {
				cfg := validBase(t)
				pair.setMax(cfg, value)
				assertRejected(t, cfg.ValidateHints(nil), pair.name+" max", value)
			}

			// min > max is rejected even when both are positive.
			cfg := validBase(t)
			pair.setMin(cfg, 5)
			pair.setMax(cfg, 2)
			assertRejected(t, cfg.ValidateHints(nil), pair.name+" min>max", "5 > 2")

			// min == max is the inclusive edge and must be accepted.
			cfg = validBase(t)
			pair.setMin(cfg, 2)
			pair.setMax(cfg, 2)

			err := cfg.ValidateHints(nil)
			if err != nil {
				t.Errorf("%s with min == max == 2 was rejected: %v", pair.name, err)
			}

			// And the default pair must itself satisfy min <= max.
			cfg = validBase(t)
			if pair.readMin(cfg) > pair.readMax(cfg) {
				t.Errorf("%s default has min %v > max %v",
					pair.name, pair.readMin(cfg), pair.readMax(cfg))
			}
		})
	}
}

// visionSizeThresholds are the size fields validated together by a single
// `<= 0` chain; each must be individually rejected when non-positive, or one
// clause of the chain could be dropped unnoticed.
func visionSizeThresholds() []intBound {
	return []intBound{
		{
			name:     "hints.vision.button_icon_max_size",
			set:      func(c *config.Config, v int) { c.Hints.Vision.ButtonIconMaxSize = v },
			validate: func(c *config.Config) error { return c.ValidateHints(nil) },
			minValid: 1,
		},
		{
			name:     "hints.vision.link_max_height",
			set:      func(c *config.Config, v int) { c.Hints.Vision.LinkMaxHeight = v },
			validate: func(c *config.Config) error { return c.ValidateHints(nil) },
			minValid: 1,
		},
		{
			name:     "hints.vision.link_min_width",
			set:      func(c *config.Config, v int) { c.Hints.Vision.LinkMinWidth = v },
			validate: func(c *config.Config) error { return c.ValidateHints(nil) },
			minValid: 1,
		},
		{
			name:     "hints.vision.image_min_size",
			set:      func(c *config.Config, v int) { c.Hints.Vision.ImageMinSize = v },
			validate: func(c *config.Config) error { return c.ValidateHints(nil) },
			minValid: 1,
		},
		{
			name:     "hints.vision.checkbox_max_size",
			set:      func(c *config.Config, v int) { c.Hints.Vision.CheckboxMaxSize = v },
			validate: func(c *config.Config) error { return c.ValidateHints(nil) },
			minValid: 1,
		},
	}
}

func TestConfig_VisionSizeThresholdBoundaries(t *testing.T) {
	runIntBounds(t, visionSizeThresholds())
}

// TestConfig_VisionLinkMinAspectBoundary covers the standalone positive check.
func TestConfig_VisionLinkMinAspectBoundary(t *testing.T) {
	for _, value := range []float64{0, -0.1, -1} {
		cfg := validBase(t)
		cfg.Hints.Vision.LinkMinAspect = value
		assertRejected(t, cfg.ValidateHints(nil), "hints.vision.link_min_aspect", value)
	}

	for _, value := range []float64{0.001, 1, 10} {
		cfg := validBase(t)
		cfg.Hints.Vision.LinkMinAspect = value

		err := cfg.ValidateHints(nil)
		if err != nil {
			t.Errorf("hints.vision.link_min_aspect = %v was rejected: %v", value, err)
		}
	}
}

// TestConfig_VisionRequiresADetector pins the "at least one detector" rule.
// With both off the vision strategy would run and find nothing, which looks
// identical to "no elements on screen".
func TestConfig_VisionRequiresADetector(t *testing.T) {
	cfg := validBase(t)
	cfg.Hints.Vision.DetectText = false
	cfg.Hints.Vision.DetectRectangles = false

	assertRejected(t, cfg.ValidateHints(nil), "hints.vision detectors", "both disabled")

	// Either one alone is enough.
	for _, useText := range []bool{true, false} {
		cfg = validBase(t)
		cfg.Hints.Vision.DetectText = useText
		cfg.Hints.Vision.DetectRectangles = !useText

		err := cfg.ValidateHints(nil)
		if err != nil {
			t.Errorf("vision with detect_text=%t, detect_rectangles=%t was rejected: %v",
				useText, !useText, err)
		}
	}
}

// TestConfig_MouseActionAnimationScalesRejectNegatives covers the animation
// scale guard, where a negative scale would invert the indicator.
func TestConfig_MouseActionAnimationScalesRejectNegatives(t *testing.T) {
	for _, value := range []float64{-0.001, -1} {
		cfg := validBase(t)
		cfg.MouseAction.Animation.StartScale = value
		assertRejected(t, cfg.ValidateMouseAction(), "mouse_action.animation.start_scale", value)

		cfg = validBase(t)
		cfg.MouseAction.Animation.EndScale = value
		assertRejected(t, cfg.ValidateMouseAction(), "mouse_action.animation.end_scale", value)
	}

	// Zero is the inclusive lower edge for both.
	cfg := validBase(t)
	cfg.MouseAction.Animation.StartScale = 0
	cfg.MouseAction.Animation.EndScale = 0

	err := cfg.ValidateMouseAction()
	if err != nil {
		t.Errorf("animation scales of 0 were rejected: %v", err)
	}
}
