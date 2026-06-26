package cli

// HintsCmd is the CLI hints command.
var HintsCmd = BuildModeCommand(ModeConfig{
	Name:  "hints",
	Short: "Launch hints mode for clickable elements",
	Long: `Assign letter hints to on-screen elements for keyboard-driven clicking.

  Hints mode scans the focused window for interactive elements (buttons,
  links, inputs, etc.) and overlays short letter codes on each one.
  Type the code to click that element.

  Use --action to perform an action immediately upon selecting a hint,
  and --repeat to stay in hints mode after the action (useful for
  multi-step workflows).

  When --search is enabled, the mode shows a search/filter input
  instead of navigating by hint keys directly.

  Use --role and --text to filter which elements get hinted:
    --role AXButton,AXLink       Only hint buttons and links
    --text "Submit,Cancel"        Only hint elements containing "Submit" or "Cancel"

  Use --strategy vision to use the Vision Framework (macOS) for element
  detection instead of the default AX API.

  Use --debug to probe the focused window and print the clickable elements
  that would be hinted (count plus a sample) without showing the overlay.
  This is handy for verifying the platform accessibility pipeline.

  Use --label-direction to override the configured hint label enumeration
  for this activation. "normal" (default) uses the prefix-avoidance
  algorithm and prefers shorter labels; "reverse" spreads labels across
  the alphabet so same-prefix labels never cluster.

  Examples:
    neru hints                               Activate hints mode
    neru hints --action left_click           Select a hint to click once
    neru hints --action left_click --repeat  Click multiple elements in sequence
    neru hints --search                      Start with search input shown
    neru hints --role AXButton               Hint only buttons
    neru hints --strategy vision             Use Vision Framework detection
    neru hints --debug                       Print detected elements, no overlay (used on windows),
    neru hints --label-direction reverse     Use spread labels for this run`,

	ActionDesc:            "hint selection",
	SupportSearch:         true,
	SupportFiltering:      true,
	SupportStrategy:       true,
	SupportDebug:          true,
	SupportLabelDirection: true,
})

func init() {
	RootCmd.AddCommand(HintsCmd)
}
