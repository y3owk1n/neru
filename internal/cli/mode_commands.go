package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/y3owk1n/neru/internal/app/modes"
	"github.com/y3owk1n/neru/internal/core/domain/action"
	derrors "github.com/y3owk1n/neru/internal/core/errors"
)

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
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return requiresRunningInstance()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			actionFlag, err := cmd.Flags().GetString("action")
			if err != nil {
				return err
			}

			modifierFlag, err := cmd.Flags().GetString("modifier")
			if err != nil {
				return err
			}

			repeatFlag, err := cmd.Flags().GetBool("repeat")
			if err != nil {
				return err
			}

			toggleFlag, err := cmd.Flags().GetBool("toggle")
			if err != nil {
				return err
			}

			var searchFlag bool
			if config.SupportSearch {
				searchFlag, err = cmd.Flags().GetBool("search")
				if err != nil {
					return err
				}
			}

			var roleFlag, textFlag string
			if config.SupportFiltering {
				roleFlag, err = cmd.Flags().GetString("role")
				if err != nil {
					return err
				}

				textFlag, err = cmd.Flags().GetString("text")
				if err != nil {
					return err
				}
			}

			var strategyFlag string
			if config.SupportStrategy {
				strategyFlag, err = cmd.Flags().GetString("strategy")
				if err != nil {
					return err
				}
			}

			var debugFlag bool
			if config.SupportDebug {
				debugFlag, err = cmd.Flags().GetBool("debug")
				if err != nil {
					return err
				}
			}

			var splitWordFlag bool
			if config.SupportSplitWord {
				splitWordFlag, err = cmd.Flags().GetBool("split-word")
				if err != nil {
					return err
				}
			}

			var hideOnEmptySearchFlag bool
			if config.SupportHideOnEmptySearch {
				hideOnEmptySearchFlag, err = cmd.Flags().GetBool("hide-on-empty-search")
				if err != nil {
					return err
				}
			}

			var labelDirectionFlag string
			if config.SupportLabelDirection {
				labelDirectionFlag, err = cmd.Flags().GetString("label-direction")
				if err != nil {
					return err
				}
			}

			var zoomToDepthFlag int
			if config.SupportZoomToDepth {
				zoomToDepthFlag, err = cmd.Flags().GetInt("zoom-to-depth")
				if err != nil {
					return err
				}

				if zoomToDepthFlag < 0 {
					return derrors.New(
						derrors.CodeInvalidInput,
						"--zoom-to-depth requires a non-negative integer",
					)
				}
			}

			cursorSelectionMode, err := cmd.Flags().GetString("cursor-selection-mode")
			if err != nil {
				return err
			}

			if repeatFlag && actionFlag == "" {
				return derrors.New(
					derrors.CodeInvalidInput,
					"--repeat requires --action",
				)
			}

			if hideOnEmptySearchFlag && !searchFlag {
				return derrors.New(
					derrors.CodeInvalidInput,
					"--hide-on-empty-search requires --search",
				)
			}

			if modifierFlag != "" {
				if actionFlag == "" {
					return derrors.New(
						derrors.CodeInvalidInput,
						"--modifier requires --action",
					)
				}

				mods, modErr := action.ParseModifiers(modifierFlag)
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

			if actionFlag != "" {
				// Split comma-separated actions and validate each one.
				// This enables multi-click sequences like:
				//   neru hints --action left_click,left_click
				// which produce a double-click via the native click-counting layer.
				actions := strings.Split(actionFlag, ",")
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

			var params []string

			params = append(params, config.Name)
			if actionFlag != "" {
				params = append(params, actionFlag)
			}

			if modifierFlag != "" {
				params = append(params, "--modifier="+modifierFlag)
			}

			if repeatFlag {
				params = append(params, "--repeat")
			}

			if toggleFlag {
				params = append(params, "--toggle")
			}

			if searchFlag {
				params = append(params, "--search")
			}

			if hideOnEmptySearchFlag {
				params = append(params, "--hide-on-empty-search")
			}

			if roleFlag != "" {
				params = append(params, "--role="+roleFlag)
			}

			if textFlag != "" {
				params = append(params, "--text="+textFlag)
			}

			if cursorSelectionMode != "" {
				if cursorSelectionMode != modes.CursorSelectionModeFollow &&
					cursorSelectionMode != modes.CursorSelectionModeHold {
					return derrors.New(
						derrors.CodeInvalidInput,
						"--cursor-selection-mode must be either follow or hold",
					)
				}

				params = append(params, "--cursor-selection-mode="+cursorSelectionMode)
			}

			if config.SupportZoomToDepth && zoomToDepthFlag > 0 {
				params = append(params, fmt.Sprintf("--zoom-to-depth=%d", zoomToDepthFlag))
			}

			if strategyFlag != "" {
				params = append(params, "--strategy="+strategyFlag)
			}

			if debugFlag {
				params = append(params, "--debug")
			}

			if labelDirectionFlag != "" {
				params = append(params, "--label-direction="+labelDirectionFlag)
			}

			if splitWordFlag {
				params = append(params, "--split-word")
			}

			return sendCommand(cmd, config.Name, params)
		},
	}

	cmd.Flags().StringP(
		"action",
		"a",
		"",
		fmt.Sprintf(
			"Action to perform on %s (%s). Commas chain multiple actions (e.g. left_click,left_click for double-click)",
			config.ActionDesc,
			action.SupportedNamesString(),
		),
	)

	cmd.Flags().BoolP(
		"toggle",
		"t",
		false,
		"Toggle mode on/off (exit to idle if already active)",
	)

	cmd.Flags().BoolP(
		"repeat",
		"r",
		false,
		"Re-activate mode after performing the action (requires --action)",
	)

	cmd.Flags().String(
		"modifier",
		"",
		"Comma-separated modifier keys to hold during action (cmd, super, meta, shift, alt, option, ctrl) (requires --action)",
	)
	cmd.Flags().String(
		"cursor-selection-mode",
		"",
		"How the real cursor should behave during selection: follow or hold",
	)

	if config.SupportZoomToDepth {
		cmd.Flags().Int(
			"zoom-to-depth",
			0,
			"Auto-zoom to the specified depth in recursive-grid at the current cursor position",
		)
	}

	if config.SupportSearch {
		cmd.Flags().BoolP(
			"search",
			"s",
			false,
			"Show search input when the mode is activated",
		)
	}

	if config.SupportHideOnEmptySearch {
		cmd.Flags().Bool(
			"hide-on-empty-search",
			false,
			"Hide all hints when search query is empty (requires --search)",
		)
	}

	if config.SupportFiltering {
		cmd.Flags().String(
			"role",
			"",
			"Filter by AX role (comma-separated: AXButton,AXLink)",
		)
		cmd.Flags().String(
			"text",
			"",
			"Filter elements by text content (comma-separated, case-insensitive substring match)",
		)
	}

	if config.SupportStrategy {
		cmd.Flags().String(
			"strategy",
			"",
			"Element detection strategy: axtree (macOS AX API) or vision (Vision Framework)",
		)
	}

	if config.SupportDebug {
		cmd.Flags().BoolP(
			"debug",
			"d",
			false,
			"Probe the focused window and print detected clickable elements without showing the overlay",
		)
	}

	if config.SupportLabelDirection {
		cmd.Flags().String(
			"label-direction",
			"",
			"Hint label enumeration: normal (default, prefix-avoidance, prefers shorter labels) or reverse (spreads labels across the alphabet)",
		)
	}

	if config.SupportSplitWord {
		cmd.Flags().Bool(
			"split-word",
			false,
			"Split detected text into word-level regions (requires vision strategy)",
		)
	}

	return cmd
}
