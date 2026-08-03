package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/modeflag"
)

// validateOnExitSteps trims each --on-exit value and rejects blank ones, so a
// quoting mistake fails at the CLI rather than silently dropping a step from
// the sequence that runs once the mode's action is fulfilled.
func validateOnExitSteps(values []string) ([]string, error) {
	steps := make([]string, 0, len(values))

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil, derrors.New(derrors.CodeInvalidInput, "--on-exit steps cannot be empty")
		}

		steps = append(steps, trimmed)
	}

	return steps, nil
}

// ModeConfig holds configuration for creating a mode command.
type ModeConfig struct {
	Name                     string
	Short                    string
	Long                     string
	ActionDesc               string   // Description for the action flag (e.g., "hint selection" or "grid selection")
	Aliases                  []string // Optional CLI aliases (e.g., "recursive-grid" for "recursive_grid")
	SupportSearch            bool     // Whether this mode supports the --search flag
	SupportHideOnEmptySearch bool     // Whether this mode supports the --hide-on-empty-search flag
	SupportFiltering         bool     // Whether this mode supports --role and --text filter flags
	SupportStrategy          bool     // Whether this mode supports the --strategy flag
	SupportLabelDirection    bool     // Whether this mode supports the --label-direction flag
	SupportDebug             bool     // Whether this mode supports the --debug probe flag
	SupportSplitWord         bool     // Whether this mode supports the --split-word flag
	SupportZoomToDepth       bool     // Whether this mode supports the --zoom-to-depth flag
}

// BuildModeCommand creates a CLI command for a navigation mode (hints, grid, etc.).
func BuildModeCommand(config ModeConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:     config.Name,
		Aliases: config.Aliases,
		Short:   config.Short,
		Long:    config.Long,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			// Validate before requiring a running daemon so users get
			// immediate feedback on invalid arguments regardless of daemon state.
			if config.SupportZoomToDepth {
				zoomToDepth, err := cmd.Flags().GetInt(modeflag.ZoomToDepth.String())
				if err == nil && zoomToDepth < 0 {
					return derrors.New(
						derrors.CodeInvalidInput,
						"--zoom-to-depth requires a non-negative integer",
					)
				}
			}

			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags, err := readModeFlags(cmd, config)
			if err != nil {
				return err
			}

			validateErr := flags.validate()
			if validateErr != nil {
				return validateErr
			}

			return sendCommand(cmd, config.Name, flags.ipcArgs(config))
		},
	}

	cmd.Flags().StringP(
		modeflag.Action.String(),
		modeflag.Action.Short(),
		"",
		fmt.Sprintf(
			"Mouse button action to perform on %s (%s). Commas chain multiple actions "+
				"(e.g. left_click,left_click for double-click). Other actions, such as "+
				"scroll or move_mouse, run via 'neru action <name>'",
			config.ActionDesc,
			action.ModeActionNamesString(),
		),
	)

	cmd.Flags().BoolP(
		modeflag.Toggle.String(),
		modeflag.Toggle.Short(),
		false,
		"Toggle mode on/off (exit to idle if already active)",
	)

	cmd.Flags().BoolP(
		modeflag.Repeat.String(),
		modeflag.Repeat.Short(),
		false,
		"Re-activate mode after performing the action (requires --action)",
	)

	cmd.Flags().String(
		modeflag.Modifier.String(),
		"",
		"Comma-separated modifier keys to hold during action (cmd, super, meta, shift, alt, option, ctrl) (requires --action)",
	)
	cmd.Flags().StringArray(
		modeflag.OnExit.String(),
		nil,
		"Step to run after the action is fulfilled and the mode exits (same syntax as hotkeys, e.g. 'action left_click' or 'exec notify-send done'). Repeat the flag to run several steps in order. Requires --action; not run on manual escape/idle",
	)
	cmd.Flags().String(
		modeflag.CursorSelectionMode.String(),
		"",
		"How the real cursor should behave during selection: follow or hold",
	)

	if config.SupportZoomToDepth {
		cmd.Flags().Int(
			modeflag.ZoomToDepth.String(),
			0,
			"Auto-zoom to the specified depth in recursive-grid at the current cursor position",
		)
	}

	if config.SupportSearch {
		cmd.Flags().BoolP(
			modeflag.Search.String(),
			modeflag.Search.Short(),
			false,
			"Show search input when the mode is activated",
		)
	}

	if config.SupportHideOnEmptySearch {
		cmd.Flags().Bool(
			modeflag.HideOnEmptySearch.String(),
			false,
			"Hide all hints when search query is empty (requires --search)",
		)
	}

	if config.SupportFiltering {
		cmd.Flags().String(
			modeflag.Role.String(),
			"",
			"Filter by AX role (comma-separated: AXButton,AXLink)",
		)
		cmd.Flags().String(
			modeflag.Text.String(),
			"",
			"Filter elements by text content (comma-separated, case-insensitive substring match)",
		)
	}

	if config.SupportStrategy {
		cmd.Flags().String(
			modeflag.Strategy.String(),
			"",
			"Element detection strategy: axtree (macOS AX API) or vision (Vision Framework)",
		)
	}

	if config.SupportDebug {
		cmd.Flags().BoolP(
			modeflag.Debug.String(),
			modeflag.Debug.Short(),
			false,
			"Probe the focused window and print detected clickable elements without showing the overlay",
		)
	}

	if config.SupportLabelDirection {
		cmd.Flags().String(
			modeflag.LabelDirection.String(),
			"",
			"Hint label enumeration: normal (default, prefix-avoidance, prefers shorter labels) or reverse (spreads labels across the alphabet)",
		)
	}

	if config.SupportSplitWord {
		cmd.Flags().Bool(
			modeflag.SplitWord.String(),
			false,
			"Split detected text into word-level regions (requires vision strategy)",
		)
	}

	return cmd
}

// modeFlags is a mode command's flags, read once so the checks and the
// request that follow work from values rather than from the command.
type modeFlags struct {
	action              string
	modifier            string
	onExitSteps         []string
	role                string
	text                string
	strategy            string
	labelDirection      string
	cursorSelectionMode string
	repeat              bool
	toggle              bool
	search              bool
	debug               bool
	splitWord           bool
	hideOnEmptySearch   bool
	zoomToDepth         int
}

// readModeFlags reads every flag the mode declares. A flag the mode does not
// support keeps its zero value, which the checks and the request both read as
// absent.
func readModeFlags(cmd *cobra.Command, config ModeConfig) (modeFlags, error) {
	actionFlag, err := cmd.Flags().GetString(modeflag.Action.String())
	if err != nil {
		return modeFlags{}, err
	}

	modifierFlag, err := cmd.Flags().GetString(modeflag.Modifier.String())
	if err != nil {
		return modeFlags{}, err
	}

	onExitFlag, err := cmd.Flags().GetStringArray(modeflag.OnExit.String())
	if err != nil {
		return modeFlags{}, err
	}

	onExitSteps, err := validateOnExitSteps(onExitFlag)
	if err != nil {
		return modeFlags{}, err
	}

	repeatFlag, err := cmd.Flags().GetBool(modeflag.Repeat.String())
	if err != nil {
		return modeFlags{}, err
	}

	toggleFlag, err := cmd.Flags().GetBool(modeflag.Toggle.String())
	if err != nil {
		return modeFlags{}, err
	}

	var searchFlag bool
	if config.SupportSearch {
		searchFlag, err = cmd.Flags().GetBool(modeflag.Search.String())
		if err != nil {
			return modeFlags{}, err
		}
	}

	var roleFlag, textFlag string
	if config.SupportFiltering {
		roleFlag, err = cmd.Flags().GetString(modeflag.Role.String())
		if err != nil {
			return modeFlags{}, err
		}

		textFlag, err = cmd.Flags().GetString(modeflag.Text.String())
		if err != nil {
			return modeFlags{}, err
		}
	}

	var strategyFlag string
	if config.SupportStrategy {
		strategyFlag, err = cmd.Flags().GetString(modeflag.Strategy.String())
		if err != nil {
			return modeFlags{}, err
		}
	}

	var debugFlag bool
	if config.SupportDebug {
		debugFlag, err = cmd.Flags().GetBool(modeflag.Debug.String())
		if err != nil {
			return modeFlags{}, err
		}
	}

	var splitWordFlag bool
	if config.SupportSplitWord {
		splitWordFlag, err = cmd.Flags().GetBool(modeflag.SplitWord.String())
		if err != nil {
			return modeFlags{}, err
		}
	}

	var hideOnEmptySearchFlag bool
	if config.SupportHideOnEmptySearch {
		hideOnEmptySearchFlag, err = cmd.Flags().GetBool(modeflag.HideOnEmptySearch.String())
		if err != nil {
			return modeFlags{}, err
		}
	}

	var labelDirectionFlag string
	if config.SupportLabelDirection {
		labelDirectionFlag, err = cmd.Flags().GetString(modeflag.LabelDirection.String())
		if err != nil {
			return modeFlags{}, err
		}
	}

	var zoomToDepthFlag int
	if config.SupportZoomToDepth {
		zoomToDepthFlag, err = cmd.Flags().GetInt("zoom-to-depth")
		if err != nil {
			return modeFlags{}, err
		}

		if zoomToDepthFlag < 0 {
			return modeFlags{}, derrors.New(
				derrors.CodeInvalidInput,
				"--zoom-to-depth requires a non-negative integer",
			)
		}
	}

	cursorSelectionMode, err := cmd.Flags().GetString(modeflag.CursorSelectionMode.String())
	if err != nil {
		return modeFlags{}, err
	}

	return modeFlags{
		action:              actionFlag,
		modifier:            modifierFlag,
		onExitSteps:         onExitSteps,
		role:                roleFlag,
		text:                textFlag,
		strategy:            strategyFlag,
		labelDirection:      labelDirectionFlag,
		cursorSelectionMode: cursorSelectionMode,
		repeat:              repeatFlag,
		toggle:              toggleFlag,
		search:              searchFlag,
		debug:               debugFlag,
		splitWord:           splitWordFlag,
		hideOnEmptySearch:   hideOnEmptySearchFlag,
		zoomToDepth:         zoomToDepthFlag,
	}, nil
}

// validate applies the rules that involve more than one flag. The daemon
// enforces the same ones; doing it here too is what makes a mistyped command
// fail immediately rather than after a round trip.
func (f modeFlags) validate() error {
	if f.cursorSelectionMode != "" &&
		f.cursorSelectionMode != modes.CursorSelectionModeFollow &&
		f.cursorSelectionMode != modes.CursorSelectionModeHold {
		return derrors.New(
			derrors.CodeInvalidInput,
			"--cursor-selection-mode must be either follow or hold",
		)
	}

	if f.repeat && f.action == "" {
		return derrors.New(
			derrors.CodeInvalidInput,
			"--repeat requires --action",
		)
	}

	if len(f.onExitSteps) > 0 && f.action == "" {
		return derrors.New(
			derrors.CodeInvalidInput,
			"--on-exit requires --action (it runs only when the action is fulfilled)",
		)
	}

	if f.hideOnEmptySearch && !f.search {
		return derrors.New(
			derrors.CodeInvalidInput,
			"--hide-on-empty-search requires --search",
		)
	}

	if f.modifier != "" {
		if f.action == "" {
			return derrors.New(
				derrors.CodeInvalidInput,
				"--modifier requires --action",
			)
		}

		mods, modErr := action.ParseModifiers(f.modifier)
		if modErr != nil {
			return modErr
		}

		if mods == 0 {
			return derrors.New(
				derrors.CodeInvalidInput,
				"modifier values cannot be empty",
			)
		}
	}

	if f.action != "" {
		// Split comma-separated actions and validate each one.
		// This enables multi-click sequences like:
		//   neru hints --action left_click,left_click
		// which produce a double-click via the native click-counting layer.
		actions := strings.Split(f.action, ",")
		for actionIdx, a := range actions {
			trimmed := strings.TrimSpace(a)
			if trimmed == "" {
				return derrors.Newf(
					derrors.CodeInvalidInput,
					"invalid --action at position %d: empty action in comma-separated list",
					actionIdx,
				)
			}

			if !action.IsKnownName(action.Name(trimmed)) {
				return derrors.Newf(
					derrors.CodeInvalidInput,
					"invalid action: %s. Supported actions: %s",
					trimmed,
					action.SupportedNamesString(),
				)
			}

			// Scroll sub-actions (scroll_up, page_down, etc.) are only
			// valid as standalone CLI/IPC commands, not as pending mode
			// actions. Reject them here so the user gets immediate
			// feedback instead of a silent failure when the mode completes.
			if action.IsScrollSubAction(trimmed) {
				return derrors.Newf(
					derrors.CodeInvalidInput,
					"scroll sub-action %q cannot be used as a mode --action flag; use 'neru action %s' instead",
					trimmed,
					trimmed,
				)
			}

			actType, err := action.Name(trimmed).ToType()
			if err != nil || !actType.IsMouseButton() {
				return derrors.Newf(
					derrors.CodeInvalidInput,
					"%q cannot be used as a mode --action flag; use 'neru action %s' instead",
					trimmed,
					trimmed,
				)
			}
		}
	}

	return nil
}

// ipcArgs builds the argument list sent to the daemon.
func (f modeFlags) ipcArgs(config ModeConfig) []string {
	var params []string

	params = append(params, config.Name)
	if f.action != "" {
		params = append(params, f.action)
	}

	if f.modifier != "" {
		params = append(params, modeflag.Modifier.Assign(f.modifier))
	}

	for _, step := range f.onExitSteps {
		params = append(params, modeflag.OnExit.Assign(step))
	}

	if f.repeat {
		params = append(params, modeflag.Repeat.Flag())
	}

	if f.toggle {
		params = append(params, modeflag.Toggle.Flag())
	}

	if f.search {
		params = append(params, modeflag.Search.Flag())
	}

	if f.hideOnEmptySearch {
		params = append(params, modeflag.HideOnEmptySearch.Flag())
	}

	if f.role != "" {
		params = append(params, modeflag.Role.Assign(f.role))
	}

	if f.text != "" {
		params = append(params, modeflag.Text.Assign(f.text))
	}

	if f.cursorSelectionMode != "" {
		params = append(params, modeflag.CursorSelectionMode.Assign(f.cursorSelectionMode))
	}

	if config.SupportZoomToDepth && f.zoomToDepth > 0 {
		params = append(params, modeflag.ZoomToDepth.Assign(strconv.Itoa(f.zoomToDepth)))
	}

	if f.strategy != "" {
		params = append(params, modeflag.Strategy.Assign(f.strategy))
	}

	if f.debug {
		params = append(params, modeflag.Debug.Flag())
	}

	if f.labelDirection != "" {
		params = append(params, modeflag.LabelDirection.Assign(f.labelDirection))
	}

	if f.splitWord {
		params = append(params, modeflag.SplitWord.Flag())
	}

	return params
}
