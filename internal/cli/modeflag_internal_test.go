package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/y3owk1n/neru/internal/domain/modeflag"
)

// alwaysOffered are the flags every mode command has, regardless of what the
// mode supports.
func alwaysOffered() []modeflag.Name {
	return []modeflag.Name{
		modeflag.Action,
		modeflag.Modifier,
		modeflag.OnExit,
		modeflag.Repeat,
		modeflag.Toggle,
		modeflag.CursorSelectionMode,
	}
}

// optionalOffered maps each remaining flag to the ModeConfig field that turns it
// on, so a mode declaring support must actually get the flag.
func optionalOffered() map[modeflag.Name]func(*ModeConfig) {
	return map[modeflag.Name]func(*ModeConfig){
		modeflag.Search:            func(c *ModeConfig) { c.SupportSearch = true },
		modeflag.HideOnEmptySearch: func(c *ModeConfig) { c.SupportHideOnEmptySearch = true },
		modeflag.Role:              func(c *ModeConfig) { c.SupportFiltering = true },
		modeflag.Text:              func(c *ModeConfig) { c.SupportFiltering = true },
		modeflag.Strategy:          func(c *ModeConfig) { c.SupportStrategy = true },
		modeflag.Debug:             func(c *ModeConfig) { c.SupportDebug = true },
		modeflag.LabelDirection:    func(c *ModeConfig) { c.SupportLabelDirection = true },
		modeflag.SplitWord:         func(c *ModeConfig) { c.SupportSplitWord = true },
		modeflag.ZoomToDepth:       func(c *ModeConfig) { c.SupportZoomToDepth = true },
	}
}

// TestBuildModeCommand_EveryFlagIsAccountedFor stops the two lists above from drifting out of
// step with the vocabulary as flags are added.
//
// The daemon side pins that every flag in the shared vocabulary is one it acts
// on. These cases pin the other end: that the command a user types offers those
// same flags under those same names, and that what it puts on the wire is
// spelled the way the vocabulary says.
//
// Between the two, a flag renamed in one place and not the other fails here or
// there rather than going quietly dead in the gap.
func TestBuildModeCommand_EveryFlagIsAccountedFor(t *testing.T) {
	optional := optionalOffered()

	for _, spec := range modeflag.All() {
		_, isOptional := optional[spec.Name]
		if isOptional || slices.Contains(alwaysOffered(), spec.Name) {
			continue
		}

		t.Errorf("modeflag.%s is in neither list; say whether every mode offers it", spec.Name)
	}
}

func TestBuildModeCommand_OffersTheFlagsItDeclares(t *testing.T) {
	for _, name := range alwaysOffered() {
		t.Run(string(name), func(t *testing.T) {
			cmd := BuildModeCommand(ModeConfig{Name: modeHints})

			if cmd.Flags().Lookup(name.String()) == nil {
				t.Errorf("--%s is in the vocabulary but the command does not offer it", name)
			}
		})
	}

	for name, enable := range optionalOffered() {
		t.Run(string(name), func(t *testing.T) {
			config := ModeConfig{Name: modeHints}
			enable(&config)

			flag := BuildModeCommand(config).Flags().Lookup(name.String())
			if flag == nil {
				t.Fatalf("--%s is declared supported but the command does not offer it", name)
			}

			if flag.Shorthand != name.Short() {
				t.Errorf("--%s shorthand = %q, want %q from the vocabulary",
					name, flag.Shorthand, name.Short())
			}
		})
	}
}

// TestBuildModeCommand_WithholdsFlagsItDoesNotSupport is the other half: a mode that
// does not declare a flag must not offer it, or a user would be able to type
// something the mode cannot act on.
func TestBuildModeCommand_WithholdsFlagsItDoesNotSupport(t *testing.T) {
	bare := BuildModeCommand(ModeConfig{Name: modeGrid})

	for name := range optionalOffered() {
		if bare.Flags().Lookup(name.String()) != nil {
			t.Errorf("--%s is offered by a mode that does not declare support for it", name)
		}
	}
}

// TestIPCArgs_UsesTheSharedSpelling pins that what goes on the wire is written
// from the vocabulary. A flag the CLI spells itself would be a flag the daemon
// silently skips.
func TestIPCArgs_UsesTheSharedSpelling(t *testing.T) {
	args := modeFlags{
		action:              actLeftClick,
		modifier:            modCmd,
		onExitSteps:         []string{stepScroll},
		role:                "AXButton",
		text:                "OK",
		strategy:            "vision",
		labelDirection:      "reverse",
		cursorSelectionMode: cursorHold,
		repeat:              true,
		toggle:              true,
		search:              true,
		debug:               true,
		splitWord:           true,
		hideOnEmptySearch:   true,
		zoomToDepth:         3,
	}.ipcArgs(hintsMode())

	known := make([]string, 0, len(modeflag.All()))
	for _, spec := range modeflag.All() {
		known = append(known, spec.Name.Flag())
	}

	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			// The mode name and the positional action travel bare.
			continue
		}

		name, _, _ := strings.Cut(arg, "=")
		if !slices.Contains(known, name) {
			t.Errorf("ipcArgs emitted %q, which is not in the vocabulary the daemon reads", arg)
		}
	}
}
