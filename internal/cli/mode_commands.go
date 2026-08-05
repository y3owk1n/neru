package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/derrors"
	"github.com/y3owk1n/neru/internal/domain"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/modecmd"
)

// flagDebug asks for a probe rather than an activation.
//
// It is the one mode-command flag the grammar does not hold, and deliberately:
// a probe reports what hints mode would target and enters nothing, so it
// travels as its own request. --debug stays its command-line spelling, and the
// CLI is the only place that has to know that.
const (
	flagDebug      modecmd.Flag = "debug"
	flagDebugShort string       = "d"
)

// shortOf returns a flag's single-letter alias as the grammar declares it.
// Cobra reads an empty shorthand as none.
func shortOf(flag modecmd.Flag) string {
	descriptor, known := modecmd.Lookup(flag)
	if !known {
		return ""
	}

	return descriptor.Short()
}

// trimOnExitSteps trims each --on-exit value, so that a step's padding does not
// travel into the sequence that runs once the mode's action is fulfilled.
//
// A blank one is kept rather than refused: an empty --on-exit is how a command
// says it wants the steps a previous activation stored cleared and nothing run
// in their place, and the grammar reads it that way wherever it is written.
func trimOnExitSteps(values []string) []string {
	steps := make([]string, 0, len(values))

	for _, value := range values {
		steps = append(steps, strings.TrimSpace(value))
	}

	return steps
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
				zoomToDepth, err := cmd.Flags().GetInt(modecmd.FlagZoomToDepth.String())
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

			// --debug asks what the mode would target rather than entering it,
			// so it travels as its own read-only command. Only the flags that
			// shape which elements get collected come along; the rest describe
			// an activation that is not going to happen.
			if flags.debug {
				return sendCommand(cmd, domain.CommandHintsProbe, flags.probeArgs())
			}

			return sendCommand(cmd, config.Name, flags.ipcArgs(config))
		},
	}

	registerModeFlags(cmd, config)

	return cmd
}

// registerModeFlags declares the flags this mode supports.
func registerModeFlags(cmd *cobra.Command, config ModeConfig) {
	cmd.Flags().StringP(
		modecmd.FlagAction.String(),
		shortOf(modecmd.FlagAction),
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
		modecmd.FlagToggle.String(),
		shortOf(modecmd.FlagToggle),
		false,
		"Toggle mode on/off (exit to idle if already active)",
	)

	cmd.Flags().BoolP(
		modecmd.FlagRepeat.String(),
		shortOf(modecmd.FlagRepeat),
		false,
		"Re-activate mode after performing the action (requires --action)",
	)

	cmd.Flags().String(
		modecmd.FlagModifier.String(),
		"",
		"Comma-separated modifier keys to hold during action (cmd, super, meta, shift, alt, option, ctrl) (requires --action)",
	)
	cmd.Flags().StringArray(
		modecmd.FlagOnExit.String(),
		nil,
		"Step to run after the action is fulfilled and the mode exits (same syntax as hotkeys, e.g. 'action left_click' or 'exec notify-send done'). Repeat the flag to run several steps in order. Requires --action; not run on manual escape/idle",
	)
	cmd.Flags().String(
		modecmd.FlagCursorSelectionMode.String(),
		"",
		"How the real cursor should behave during selection: follow or hold",
	)

	if config.SupportZoomToDepth {
		cmd.Flags().Int(
			modecmd.FlagZoomToDepth.String(),
			0,
			"Auto-zoom to the specified depth in recursive-grid at the current cursor position",
		)
	}

	if config.SupportSearch {
		cmd.Flags().BoolP(
			modecmd.FlagSearch.String(),
			shortOf(modecmd.FlagSearch),
			false,
			"Show search input when the mode is activated",
		)
	}

	if config.SupportHideOnEmptySearch {
		cmd.Flags().Bool(
			modecmd.FlagHideOnEmptySearch.String(),
			false,
			"Hide all hints when search query is empty (requires --search)",
		)
	}

	if config.SupportFiltering {
		cmd.Flags().String(
			modecmd.FlagRole.String(),
			"",
			"Filter by AX role (comma-separated: AXButton,AXLink)",
		)
		cmd.Flags().String(
			modecmd.FlagText.String(),
			"",
			"Filter elements by text content (comma-separated, case-insensitive substring match)",
		)
	}

	if config.SupportStrategy {
		cmd.Flags().String(
			modecmd.FlagStrategy.String(),
			"",
			"Element detection strategy: axtree (macOS AX API) or vision (Vision Framework)",
		)
	}

	if config.SupportDebug {
		cmd.Flags().BoolP(
			flagDebug.String(),
			flagDebugShort,
			false,
			"Probe the focused window and print detected clickable elements without showing the overlay",
		)
	}

	if config.SupportLabelDirection {
		cmd.Flags().String(
			modecmd.FlagLabelDirection.String(),
			"",
			"Hint label enumeration: normal (default, prefix-avoidance, prefers shorter labels) or reverse (spreads labels across the alphabet)",
		)
	}

	if config.SupportSplitWord {
		cmd.Flags().Bool(
			modecmd.FlagSplitWord.String(),
			false,
			"Split detected text into word-level regions (requires vision strategy)",
		)
	}
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
	actionFlag, err := cmd.Flags().GetString(modecmd.FlagAction.String())
	if err != nil {
		return modeFlags{}, err
	}

	modifierFlag, err := cmd.Flags().GetString(modecmd.FlagModifier.String())
	if err != nil {
		return modeFlags{}, err
	}

	onExitFlag, err := cmd.Flags().GetStringArray(modecmd.FlagOnExit.String())
	if err != nil {
		return modeFlags{}, err
	}

	onExitSteps := trimOnExitSteps(onExitFlag)

	repeatFlag, err := cmd.Flags().GetBool(modecmd.FlagRepeat.String())
	if err != nil {
		return modeFlags{}, err
	}

	toggleFlag, err := cmd.Flags().GetBool(modecmd.FlagToggle.String())
	if err != nil {
		return modeFlags{}, err
	}

	var searchFlag bool
	if config.SupportSearch {
		searchFlag, err = cmd.Flags().GetBool(modecmd.FlagSearch.String())
		if err != nil {
			return modeFlags{}, err
		}
	}

	var roleFlag, textFlag string
	if config.SupportFiltering {
		roleFlag, err = cmd.Flags().GetString(modecmd.FlagRole.String())
		if err != nil {
			return modeFlags{}, err
		}

		textFlag, err = cmd.Flags().GetString(modecmd.FlagText.String())
		if err != nil {
			return modeFlags{}, err
		}
	}

	var strategyFlag string
	if config.SupportStrategy {
		strategyFlag, err = cmd.Flags().GetString(modecmd.FlagStrategy.String())
		if err != nil {
			return modeFlags{}, err
		}
	}

	var debugFlag bool
	if config.SupportDebug {
		debugFlag, err = cmd.Flags().GetBool(flagDebug.String())
		if err != nil {
			return modeFlags{}, err
		}
	}

	var splitWordFlag bool
	if config.SupportSplitWord {
		splitWordFlag, err = cmd.Flags().GetBool(modecmd.FlagSplitWord.String())
		if err != nil {
			return modeFlags{}, err
		}
	}

	var hideOnEmptySearchFlag bool
	if config.SupportHideOnEmptySearch {
		hideOnEmptySearchFlag, err = cmd.Flags().GetBool(modecmd.FlagHideOnEmptySearch.String())
		if err != nil {
			return modeFlags{}, err
		}
	}

	var labelDirectionFlag string
	if config.SupportLabelDirection {
		labelDirectionFlag, err = cmd.Flags().GetString(modecmd.FlagLabelDirection.String())
		if err != nil {
			return modeFlags{}, err
		}
	}

	var zoomToDepthFlag int
	if config.SupportZoomToDepth {
		zoomToDepthFlag, err = cmd.Flags().GetInt(modecmd.FlagZoomToDepth.String())
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

	cursorSelectionMode, err := cmd.Flags().GetString(modecmd.FlagCursorSelectionMode.String())
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
// enforces the same ones through the mode-command grammar; doing it here too is
// what makes a mistyped command fail immediately rather than after a round
// trip, and the wording is the grammar's word for word so that one rule gives
// one message wherever the command came from.
func (f modeFlags) validate() error {
	debugErr := f.validateDebugIsAlone()
	if debugErr != nil {
		return debugErr
	}

	if f.cursorSelectionMode != "" &&
		f.cursorSelectionMode != domain.CursorSelectionModeFollow &&
		f.cursorSelectionMode != domain.CursorSelectionModeHold {
		return derrors.New(
			derrors.CodeInvalidInput,
			"--cursor-selection-mode requires follow or hold",
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
					"scroll sub-action %q cannot be used as a mode action; only mouse button actions can",
					trimmed,
				)
			}

			actType, err := action.Name(trimmed).ToType()
			if err != nil || !actType.IsMouseButton() {
				return derrors.Newf(
					derrors.CodeInvalidInput,
					"%q cannot be used as a mode action; only mouse button actions can",
					trimmed,
				)
			}
		}
	}

	return nil
}

// validateDebugIsAlone rejects --debug alongside a flag that only describes an
// activation.
//
// A probe reports what the mode would target and then stops; it never selects
// anything, so there is nothing for an action to act on, nothing to repeat and
// nothing to run on exit. Accepting those quietly would send a caller a summary
// while dropping the rest of what they asked for.
func (f modeFlags) validateDebugIsAlone() error {
	if !f.debug {
		return nil
	}

	activationOnly := []struct {
		flag modecmd.Flag
		set  bool
	}{
		{modecmd.FlagAction, f.action != ""},
		{modecmd.FlagModifier, f.modifier != ""},
		{modecmd.FlagOnExit, len(f.onExitSteps) > 0},
		{modecmd.FlagRepeat, f.repeat},
		{modecmd.FlagToggle, f.toggle},
		{modecmd.FlagSearch, f.search},
		{modecmd.FlagHideOnEmptySearch, f.hideOnEmptySearch},
		{modecmd.FlagLabelDirection, f.labelDirection != ""},
		{modecmd.FlagZoomToDepth, f.zoomToDepth > 0},
		{modecmd.FlagCursorSelectionMode, f.cursorSelectionMode != ""},
	}

	for _, candidate := range activationOnly {
		if candidate.set {
			return derrors.Newf(
				derrors.CodeInvalidInput,
				"%s cannot be combined with %s: a probe reports what would be targeted without entering the mode",
				flagDebug.Long(),
				candidate.flag.Long(),
			)
		}
	}

	return nil
}

// probeArgs builds the argument list for a hints probe.
//
// A probe collects elements and reports them, so it takes the flags that
// decide which elements are collected and nothing else. --debug itself is not
// among them: it named the command, and the command is now its own.
func (f modeFlags) probeArgs() []string {
	var params []string

	if f.role != "" {
		params = append(params, modecmd.FlagRole.Assign(f.role))
	}

	if f.text != "" {
		params = append(params, modecmd.FlagText.Assign(f.text))
	}

	if f.strategy != "" {
		params = append(params, modecmd.FlagStrategy.Assign(f.strategy))
	}

	if f.splitWord {
		params = append(params, modecmd.FlagSplitWord.Long())
	}

	return params
}

// ipcArgs builds the argument list sent to the daemon.
func (f modeFlags) ipcArgs(config ModeConfig) []string {
	var params []string

	params = append(params, config.Name)
	if f.action != "" {
		params = append(params, f.action)
	}

	if f.modifier != "" {
		params = append(params, modecmd.FlagModifier.Assign(f.modifier))
	}

	for _, step := range f.onExitSteps {
		params = append(params, modecmd.FlagOnExit.Assign(step))
	}

	if f.repeat {
		params = append(params, modecmd.FlagRepeat.Long())
	}

	if f.toggle {
		params = append(params, modecmd.FlagToggle.Long())
	}

	if f.search {
		params = append(params, modecmd.FlagSearch.Long())
	}

	if f.hideOnEmptySearch {
		params = append(params, modecmd.FlagHideOnEmptySearch.Long())
	}

	if f.role != "" {
		params = append(params, modecmd.FlagRole.Assign(f.role))
	}

	if f.text != "" {
		params = append(params, modecmd.FlagText.Assign(f.text))
	}

	if f.cursorSelectionMode != "" {
		params = append(params, modecmd.FlagCursorSelectionMode.Assign(f.cursorSelectionMode))
	}

	if config.SupportZoomToDepth && f.zoomToDepth > 0 {
		params = append(params, modecmd.FlagZoomToDepth.Assign(strconv.Itoa(f.zoomToDepth)))
	}

	if f.strategy != "" {
		params = append(params, modecmd.FlagStrategy.Assign(f.strategy))
	}

	if f.labelDirection != "" {
		params = append(params, modecmd.FlagLabelDirection.Assign(f.labelDirection))
	}

	if f.splitWord {
		params = append(params, modecmd.FlagSplitWord.Long())
	}

	return params
}
