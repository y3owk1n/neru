package config

import (
	"strings"

	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
	"github.com/y3owk1n/neru/internal/domain/parity"
)

// Why a config option's platform column is narrower than every platform.
//
// Each note is worded as the sentence a user reads in the load-time warning and
// in the published table, so it names the gap rather than the code that has it.
const (
	noteScreenShareHide = "hiding the overlay from a screen share is an NSWindow sharing level, " +
		"a Quartz concept with no X11, Wayland or Win32 counterpart"
	noteVisionConfidence = "Windows.Media.Ocr reports no per-word confidence, so every word " +
		"scores one there and a floor keeps everything; the Vision framework and tesseract " +
		"score each word"
	noteVisionRectangles = "rectangle detection has no OCR answer, so it stays macOS-only " +
		"even where the vision strategy lands; that half is text-only"
	noteSmoothScroll = "the Windows scroll is injected in one step; macOS and Linux animate it, " +
		"and on X11 the steps are whole wheel notches because X has no smaller scroll to send"
	noteKeyboardLayout = "the keyboard layout is detected rather than chosen outside macOS"
	noteMacOSSurfaces  = "the menu bar, the Dock, Notification Center, Stage Manager, " +
		"picture-in-picture and the screen-capture chrome are macOS surfaces with no counterpart"
	noteMissionControl = "Mission Control is a macOS concept, so the detection never fires " +
		"and the hooks never run"
	noteTreeDepth = "only the AX walk takes a depth limit; the AT-SPI walk uses a fixed one " +
		"and the UIA walk records the option without reading it"
	noteClickableChecks = "the clickable and visibility checks are AX-specific; the AT-SPI and " +
		"UIA walks decide what is clickable their own way and never consult these"
	noteGridPrewarm = "only the darwin grid overlay prewarms its layers; the other backends " +
		"draw on demand"
)

// captureScopeOptions are the same six paths for capture_scope, which shadows
// the hints section per app the way strategy does. They are declared
// everywhere: the option only shapes the capture strategies, and every
// platform has a capture backend now.
var captureScopeOptions = []string{
	"hints.capture_scope",
	"hints.app_configs.capture_scope",
	"grid.app_configs.capture_scope",
	"recursive_grid.app_configs.capture_scope",
	"scroll.app_configs.capture_scope",
	"app_configs.capture_scope",
}

// darwinOnly and darwinAndLinux are the narrow columns this schema uses today,
// named so a reader compares two options by the same words. A darwin+windows
// column existed for the hints search badge until Linux drew it too, and a
// linux-only one for contour until macOS fed it a frame; add one back the
// moment an option needs it rather than reaching for the nearest fit.
var (
	darwinOnly     = parity.Platforms{parity.Darwin}
	darwinAndLinux = parity.Platforms{parity.Darwin, parity.Linux}
)

// PlatformSupport declares, for every option in the schema, the platforms on
// which writing it does something.
//
// It is exhaustive on purpose. Being supported everywhere is written down
// rather than inherited from a section or assumed by omission, for the reason
// newDefaultConfig() assigns a field even when the value is Go's zero: a column
// nobody wrote cannot be told from a forgotten one, and the forgotten one is
// exactly how smooth_scroll reached the schema, the validators and the
// documentation with no Linux code path
// (docs/adr/0013-parity-is-measured-in-words-not-subsystems.md).
//
// TestEveryConfigOptionDeclaresItsPlatformSupport holds it to the schema in
// both directions, so an option added without a decision here fails the build
// rather than shipping as a silent no-op.
//
// A config.Color is declared at the field rather than at its light and dark
// leaves, because that is the option a person writes: a Color takes one value
// as readily as a table of two.
//
// The narrow columns below are the ones the tree can point at today — an
// entry in the Platform Exclusives table of docs/CROSS_PLATFORM.md, or a Known
// Gaps entry naming the whole option rather than part of its behavior. A
// capability that is partly there is not a column this type can state, and
// stays a Known Gaps entry.
func PlatformSupport() parity.Declaration {
	return parity.Join(
		parity.On(parity.KindOption, darwinOnly, noteScreenShareHide,
			"general.hide_overlay_in_screen_share",
		),
		parity.On(parity.KindOption, darwinOnly, noteKeyboardLayout,
			"general.kb_layout_to_use",
		),

		parity.On(parity.KindOption, darwinOnly, noteMacOSSurfaces,
			"hints.include_menubar_hints",
			"hints.additional_menubar_hints_targets",
			"hints.include_dock_hints",
			"hints.include_nc_hints",
			"hints.include_stage_manager_hints",
			"hints.include_pip_hints",
			"hints.include_screen_capture_hints",
		),
		parity.On(parity.KindOption, darwinOnly, noteMissionControl,
			"hints.detect_mission_control",
			"hints.on_mission_control_activated",
			"hints.on_mission_control_deactivated",
		),

		parity.On(parity.KindOption, darwinOnly, noteTreeDepth,
			"hints.max_depth",
		),
		parity.On(parity.KindOption, darwinOnly, noteClickableChecks,
			"hints.ignore_clickable_check",
			"hints.visible_check_enabled",
			"hints.app_configs.ignore_clickable_check",
			"hints.app_configs.visible_check_enabled",
			"grid.app_configs.ignore_clickable_check",
			"grid.app_configs.visible_check_enabled",
			"recursive_grid.app_configs.ignore_clickable_check",
			"recursive_grid.app_configs.visible_check_enabled",
			"scroll.app_configs.ignore_clickable_check",
			"scroll.app_configs.visible_check_enabled",
			"app_configs.ignore_clickable_check",
			"app_configs.visible_check_enabled",
		),

		parity.On(parity.KindOption, darwinOnly, noteGridPrewarm,
			"grid.prewarm_enabled",
		),

		parity.On(parity.KindOption, darwinAndLinux, noteVisionConfidence,
			"hints.vision.minimum_confidence",
			"hints.vision.button_min_confidence",
			"hints.vision.generic_clickable_min_confidence",
		),
		parity.On(parity.KindOption, darwinOnly, noteVisionRectangles,
			"hints.vision.detect_rectangles",
			"hints.vision.rectangle_max_candidates",
			"hints.vision.rectangle_min_size",
			"hints.vision.rectangle_min_aspect",
			"hints.vision.rectangle_max_aspect",
		),

		parity.Everywhere(parity.KindOption,
			"recursive_grid.animation.enabled",
			"recursive_grid.animation.duration_ms",
		),

		parity.Everywhere(parity.KindOption,
			"monitor_select.enabled",
			"monitor_select.characters",
			"monitor_select.ui.font_size",
			"monitor_select.ui.font_family",
			"monitor_select.ui.border_radius",
			"monitor_select.ui.padding_x",
			"monitor_select.ui.padding_y",
			"monitor_select.ui.border_width",
			"monitor_select.ui.subtitle_font_size",
			"monitor_select.ui.subtitle_font_family",
			"monitor_select.ui.background_color",
			"monitor_select.ui.text_color",
			"monitor_select.ui.matched_text_color",
			"monitor_select.ui.border_color",
			"monitor_select.ui.backdrop_color",
			"monitor_select.ui.subtitle_text_color",
			"mode_indicator.monitor_select.enabled",
			"mode_indicator.monitor_select.text",
			"mode_indicator.monitor_select.background_color",
			"mode_indicator.monitor_select.text_color",
			"mode_indicator.monitor_select.border_color",
		),

		parity.Everywhere(parity.KindOption,
			"smooth_cursor.move_mouse_enabled",
			"smooth_cursor.steps",
			"smooth_cursor.max_duration",
			"smooth_cursor.duration_per_pixel",
			"smooth_cursor.relative_movement_duration",
		),

		parity.On(parity.KindOption, darwinAndLinux, noteSmoothScroll,
			"smooth_scroll.enabled",
			"smooth_scroll.steps",
			"smooth_scroll.max_duration",
			"smooth_scroll.duration_per_pixel",
		),

		parity.Everywhere(parity.KindOption, captureScopeOptions...),
		// The vision strategy's text half runs everywhere: the Vision framework
		// on macOS, tesseract on Linux, Windows.Media.Ocr on Windows. The
		// strategy value itself carries no declaration, like contour.
		parity.Everywhere(parity.KindOption,
			"hints.vision.detect_text",
			"hints.vision.request_timeout_ms",
			"hints.vision.merge_iou_threshold",
			"hints.vision.button_min_aspect",
			"hints.vision.button_max_aspect",
			"hints.vision.button_icon_max_size",
			"hints.vision.link_min_aspect",
			"hints.vision.link_max_height",
			"hints.vision.link_min_width",
			"hints.vision.image_min_size",
			"hints.vision.checkbox_max_size",
		),
		// Unbound modifier chords reach the focused application on macOS, on
		// the Wayland evdev tap and on Windows. X11 cannot pass them through
		// at all, which is a display-server limit rather than a column: the
		// blessed Linux stack is Wayland (Known Gaps, docs/CROSS_PLATFORM.md).
		parity.Everywhere(parity.KindOption,
			"general.passthrough_unbounded_keys",
			"general.passthrough_unbounded_keys_blacklist",
			"general.should_exit_after_passthrough",
		),
		parity.Everywhere(parity.KindOption,
			"general.excluded_apps",
			"general.exec_shell",
			"general.exec_shell_args",

			"theme.light.surface",
			"theme.light.accent",
			"theme.light.accent_alt",
			"theme.light.on_accent_alt",
			"theme.light.text",
			"theme.dark.surface",
			"theme.dark.accent",
			"theme.dark.accent_alt",
			"theme.dark.on_accent_alt",
			"theme.dark.text",

			"macros",

			"hints.enabled",
			"hints.strategy",
			"hints.hint_characters",
			"hints.label_direction",
			"hints.ui.font_size",
			"hints.ui.font_family",
			"hints.ui.border_radius",
			"hints.ui.padding_x",
			"hints.ui.padding_y",
			"hints.ui.border_width",
			"hints.ui.placement",
			"hints.search_input_ui.font_size",
			"hints.search_input_ui.font_family",
			"hints.search_input_ui.border_radius",
			"hints.search_input_ui.padding_x",
			"hints.search_input_ui.padding_y",
			"hints.search_input_ui.border_width",
			"hints.search_input_ui.position",
			"hints.search_input_ui.x_offset",
			"hints.search_input_ui.y_offset",
			"hints.search_input_ui.width",
			"hints.boundary_highlight.enabled",
			"hints.boundary_highlight.border_width",
			"hints.boundary_highlight.border_radius",
			"hints.clickable_roles",
			"hints.app_configs",
			"hints.app_configs.bundle_id",
			"hints.app_configs.strategy",
			"hints.app_configs.label_direction",
			"hints.app_configs.additional_clickable_roles",
			"hints.app_configs.scroll_step",
			"hints.app_configs.scroll_step_half",
			"hints.app_configs.scroll_step_full",
			"hints.app_configs.hotkeys",

			"grid.enabled",
			"grid.characters",
			"grid.sublayer_keys",
			"grid.max_label_length",
			"grid.row_labels",
			"grid.col_labels",
			"grid.ui.font_size",
			"grid.ui.font_family",
			"grid.ui.border_width",
			"grid.live_match_update",
			"grid.hide_unmatched",
			"grid.enable_gc",
			"grid.app_configs",
			"grid.app_configs.bundle_id",
			"grid.app_configs.strategy",
			"grid.app_configs.label_direction",
			"grid.app_configs.additional_clickable_roles",
			"grid.app_configs.scroll_step",
			"grid.app_configs.scroll_step_half",
			"grid.app_configs.scroll_step_full",
			"grid.app_configs.hotkeys",

			"recursive_grid.enabled",
			"recursive_grid.grid_cols",
			"recursive_grid.grid_rows",
			"recursive_grid.keys",
			"recursive_grid.ui.line_width",
			"recursive_grid.ui.font_size",
			"recursive_grid.ui.font_family",
			"recursive_grid.ui.label_background",
			"recursive_grid.ui.label_background_padding_x",
			"recursive_grid.ui.label_background_padding_y",
			"recursive_grid.ui.label_background_border_radius",
			"recursive_grid.ui.label_background_border_width",
			"recursive_grid.ui.label_char",
			"recursive_grid.ui.label_autohide_multiplier",
			"recursive_grid.ui.sub_key_preview",
			"recursive_grid.ui.sub_key_preview_font_size",
			"recursive_grid.ui.sub_key_preview_autohide_multiplier",
			"recursive_grid.ui.sub_key_preview_label_char",
			"recursive_grid.min_size_width",
			"recursive_grid.min_size_height",
			"recursive_grid.max_depth",
			"recursive_grid.layers",
			"recursive_grid.layers.depth",
			"recursive_grid.layers.grid_cols",
			"recursive_grid.layers.grid_rows",
			"recursive_grid.layers.keys",
			"recursive_grid.app_configs",
			"recursive_grid.app_configs.bundle_id",
			"recursive_grid.app_configs.strategy",
			"recursive_grid.app_configs.label_direction",
			"recursive_grid.app_configs.additional_clickable_roles",
			"recursive_grid.app_configs.scroll_step",
			"recursive_grid.app_configs.scroll_step_half",
			"recursive_grid.app_configs.scroll_step_full",
			"recursive_grid.app_configs.hotkeys",

			"virtual_pointer.ui.char",
			"virtual_pointer.ui.font_size",
			"virtual_pointer.ui.font_family",

			"mouse_action_indicator.enabled",
			"mouse_action_indicator.actions",
			"mouse_action_indicator.ui.size",
			"mouse_action_indicator.ui.border_width",
			"mouse_action_indicator.ui.shape",
			"mouse_action_indicator.animation.duration_ms",
			"mouse_action_indicator.animation.start_scale",
			"mouse_action_indicator.animation.end_scale",
			"mouse_action_indicator.animation.start_opacity",
			"mouse_action_indicator.animation.end_opacity",
			"mouse_action_indicator.animation.easing",

			"scroll.scroll_step",
			"scroll.scroll_step_half",
			"scroll.scroll_step_full",
			"scroll.invert_scroll",
			"scroll.app_configs",
			"scroll.app_configs.bundle_id",
			"scroll.app_configs.strategy",
			"scroll.app_configs.label_direction",
			"scroll.app_configs.additional_clickable_roles",
			"scroll.app_configs.scroll_step",
			"scroll.app_configs.scroll_step_half",
			"scroll.app_configs.scroll_step_full",
			"scroll.app_configs.hotkeys",

			"mode_indicator.scroll.enabled",
			"mode_indicator.scroll.text",
			"mode_indicator.hints.enabled",
			"mode_indicator.hints.text",
			"mode_indicator.grid.enabled",
			"mode_indicator.grid.text",
			"mode_indicator.recursive_grid.enabled",
			"mode_indicator.recursive_grid.text",
			"mode_indicator.ui.font_size",
			"mode_indicator.ui.font_family",
			"mode_indicator.ui.border_width",
			"mode_indicator.ui.padding_x",
			"mode_indicator.ui.padding_y",
			"mode_indicator.ui.border_radius",
			"mode_indicator.ui.indicator_x_offset",
			"mode_indicator.ui.indicator_y_offset",

			"sticky_modifiers.enabled",
			"sticky_modifiers.tap_max_duration",
			"sticky_modifiers.ui.font_size",
			"sticky_modifiers.ui.font_family",
			"sticky_modifiers.ui.border_width",
			"sticky_modifiers.ui.padding_x",
			"sticky_modifiers.ui.padding_y",
			"sticky_modifiers.ui.border_radius",
			"sticky_modifiers.ui.indicator_x_offset",
			"sticky_modifiers.ui.indicator_y_offset",

			"logging.log_level",
			"logging.log_file",
			"logging.disable_file_logging",
			"logging.max_file_size",
			"logging.max_backups",
			"logging.max_age",

			"held_repeat.enabled",
			"held_repeat.initial_delay_ms",
			"held_repeat.interval_ms",
			"held_repeat.accel_enabled",
			"held_repeat.accel_ramp_ms",
			"held_repeat.accel_max_multiplier",
			"held_repeat.accel_targets",

			"systray.enabled",

			"app_configs",
			"app_configs.bundle_id",
			"app_configs.strategy",
			"app_configs.label_direction",
			"app_configs.additional_clickable_roles",
			"app_configs.scroll_step",
			"app_configs.scroll_step_half",
			"app_configs.scroll_step_full",
			"app_configs.hotkeys",

			"hints.ui.background_color",
			"hints.ui.text_color",
			"hints.ui.matched_text_color",
			"hints.ui.border_color",
			"hints.search_input_ui.background_color",
			"hints.search_input_ui.text_color",
			"hints.search_input_ui.border_color",
			"hints.boundary_highlight.border_color",
			"hints.boundary_highlight.background_color",

			"grid.ui.background_color",
			"grid.ui.text_color",
			"grid.ui.matched_text_color",
			"grid.ui.matched_background_color",
			"grid.ui.matched_border_color",
			"grid.ui.border_color",

			"recursive_grid.ui.line_color",
			"recursive_grid.ui.highlight_color",
			"recursive_grid.ui.text_color",
			"recursive_grid.ui.label_background_color",
			"recursive_grid.ui.sub_key_preview_text_color",

			"virtual_pointer.ui.text_color",

			"mouse_action_indicator.ui.background_color",
			"mouse_action_indicator.ui.border_color",

			"mode_indicator.scroll.background_color",
			"mode_indicator.scroll.text_color",
			"mode_indicator.scroll.border_color",
			"mode_indicator.hints.background_color",
			"mode_indicator.hints.text_color",
			"mode_indicator.hints.border_color",
			"mode_indicator.grid.background_color",
			"mode_indicator.grid.text_color",
			"mode_indicator.grid.border_color",
			"mode_indicator.recursive_grid.background_color",
			"mode_indicator.recursive_grid.text_color",
			"mode_indicator.recursive_grid.border_color",
			"mode_indicator.ui.background_color",
			"mode_indicator.ui.text_color",
			"mode_indicator.ui.border_color",

			"sticky_modifiers.ui.background_color",
			"sticky_modifiers.ui.text_color",
			"sticky_modifiers.ui.border_color",
		),
	)
}

// Written is a configuration as the person who wrote it left it: the TOML paths
// their files carry, mapped to the value at each, and the steps their bindings,
// macros and hooks are written with.
//
// It is what the platform-support warning is judged against, and it is the
// user's files rather than the loaded configuration on purpose. Only what
// somebody wrote can be reported to them: the shipped scroll bindings name
// scroll_left, which injects nothing on Windows, and telling every Windows user
// about a line they never typed is noise about somebody else's bug
// (docs/CROSS_PLATFORM.md, Known Gaps).
type Written struct {
	// Options maps a TOML path to the value written at it. A non-scalar value
	// is present with an empty string: the path is what says the option was
	// written, and only a declaration about a specific value needs more.
	Options map[string]string
	// Steps are the action strings the files write, in no particular order.
	Steps []string
}

// InertWords reports every word a configuration writes that does nothing on the
// given platform, options first and in declaration order.
//
// It is the one reading the load-time warning, `neru doctor` and any other
// projection share, so none of them can decide differently what "this does
// nothing here" means.
func InertWords(written Written, target parity.Platform) parity.Declaration {
	return parity.Join(
		inertOptions(written.Options, target),
		inertSteps(written.Steps, target),
	)
}

// inertOptions reports the options a user wrote that do nothing on the given
// platform.
//
// A path below a declared option resolves to that option — a Color written as a
// table reaches its light and dark leaves — so the warning names the option
// rather than the leaf.
func inertOptions(written map[string]string, target parity.Platform) parity.Declaration {
	declaration := PlatformSupport()

	var inert parity.Declaration

	for _, word := range declaration.InertOn(target) {
		value, wrote := writtenValue(written, word.Name)
		if !wrote {
			continue
		}

		// A declaration about one value of an option says nothing about the
		// option written with any other.
		if word.Value != "" && word.Value != value {
			continue
		}

		inert = append(inert, word)
	}

	return inert
}

// writtenValue finds what the user wrote at an option's path, counting a path
// written below it — the light and dark leaves of a Color — as writing the
// option itself. The value it returns is the one written at the option's own
// path, empty when only a leaf below it was.
func writtenValue(written map[string]string, path string) (string, bool) {
	if value, wrote := written[path]; wrote {
		return value, true
	}

	for candidate := range written {
		if strings.HasPrefix(candidate, path+".") {
			return "", true
		}
	}

	return "", false
}

// inertSteps reports the actions and mode flags a set of steps names that do
// nothing on the given platform.
//
// A word is reported once however many steps carry it: the answer is about the
// word, and a person who bound hide_cursor to four keys has one thing to learn,
// not four.
func inertSteps(steps []string, target parity.Platform) parity.Declaration {
	declaration := parity.Join(action.PlatformSupport(), modecmd.PlatformSupport())

	var inert parity.Declaration

	seen := make(map[string]bool)

	for _, step := range steps {
		for _, written := range wordsWritten(step) {
			word, declared := declaration.Lookup(written.Kind, written.Name, written.Value)
			if !declared {
				// A value nothing declares narrows nothing, so the answer is
				// the word's own column.
				word, declared = declaration.Lookup(written.Kind, written.Name, "")
			}

			if !declared || word.Platforms.Supports(target) || seen[word.Written()] {
				continue
			}

			seen[word.Written()] = true

			inert = append(inert, word)
		}
	}

	return inert
}

// wordsWritten reads the vocabulary out of one step: the action it names, or
// the mode flags it is written with.
//
// The flags come back through the same rendering the grammar writes them with,
// so there is one answer to what a flag written with a value looks like rather
// than a second parser here that could disagree with it.
//
// Only the step's own flags, and not the steps nested inside an --on-exit:
// reading those means reading a step out of the argument list it was quoted
// into, which is the runtime's job at the moment it dispatches them, and is the
// same line ValidateModeCommands draws.
func wordsWritten(actionStr string) []parity.Word {
	tokens := SplitStepArgs(strings.TrimSpace(actionStr))
	if len(tokens) == 0 {
		return nil
	}

	if tokens[0] == action.PrefixAction && len(tokens) > 1 {
		return []parity.Word{{Kind: parity.KindAction, Name: tokens[1]}}
	}

	mode, args, isModeCommand := parseModeCommand(actionStr)
	if !isModeCommand {
		return nil
	}

	activation, parseErr := modecmd.Parse(mode, args)
	if parseErr != nil {
		return nil
	}

	var words []parity.Word

	for _, descriptor := range modecmd.All() {
		for _, rendered := range descriptor.Render(activation) {
			name, value, _ := strings.Cut(strings.TrimPrefix(rendered, "--"), "=")

			words = append(words, parity.Word{
				Kind:  parity.KindModeFlag,
				Name:  name,
				Value: value,
			})
		}
	}

	return words
}
